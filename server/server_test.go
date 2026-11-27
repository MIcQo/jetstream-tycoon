package server_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/go-chi/chi/v5"

	"github.com/MIcQo/jetstream-tycoon/server"
)

func discard() *slog.Logger { return slog.New(slog.DiscardHandler) }

func scrape(t *testing.T, m *server.Metrics) string {
	t.Helper()
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	return rec.Body.String()
}

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.NewHandler(server.NewMetrics(), discard()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"status":"ok"}` {
		t.Fatalf("body = %q, want %q", body, `{"status":"ok"}`)
	}
}

func TestMetricsUseRouteTemplates(t *testing.T) {
	m := server.NewMetrics()
	h := server.NewHandler(m, discard())
	for _, p := range []string{"/healthz", "/healthz", "/no/such/user-123"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, p, nil))
	}
	out := scrape(t, m)
	for _, want := range []string{
		`http_requests_total{method="GET",route="/healthz",status="200"} 2`,
		`http_requests_total{method="GET",route="unmatched",status="404"} 1`,
		`http_request_duration_seconds_count{method="GET",route="/healthz"} 2`,
		"go_goroutines",
		"process_",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("metrics output missing %q", want)
		}
	}
	if strings.Contains(out, "user-123") {
		t.Error("raw path leaked into metric labels")
	}
}

func TestMetricsDefaultStatusIs200(t *testing.T) {
	m := server.NewMetrics()
	r := chi.NewRouter()
	r.Use(m.Middleware)
	r.Get("/silent", func(http.ResponseWriter, *http.Request) {}) // writes nothing
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/silent", nil))
	if out := scrape(t, m); !strings.Contains(out, `http_requests_total{method="GET",route="/silent",status="200"} 1`) {
		t.Errorf("handler without WriteHeader not counted as 200:\n%s", out)
	}
}

func TestPanicRecoveredAs500(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	m := server.NewMetrics()
	h := server.NewHandler(m, logger)
	router, ok := h.(*chi.Mux)
	if !ok {
		t.Fatalf("NewHandler returned %T, want *chi.Mux", h)
	}
	router.Get("/boom/{id}", func(http.ResponseWriter, *http.Request) { panic("kaboom") })

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom/secret-42", nil))

	if rec.Code != http.StatusInternalServerError || strings.TrimSpace(rec.Body.String()) != `{"error":"internal"}` {
		t.Fatalf("panic response = %d %q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(logs.String(), "route=/boom/{id}") || !strings.Contains(logs.String(), "kaboom") {
		t.Errorf("panic log missing route template or message: %s", logs.String())
	}
	if strings.Contains(logs.String(), "secret-42") {
		t.Error("raw path leaked into logs")
	}
	if out := scrape(t, m); !strings.Contains(out, `http_requests_total{method="GET",route="/boom/{id}",status="500"} 1`) {
		t.Errorf("500 not counted:\n%s", out)
	}
}

func TestAbortHandlerPanicPropagates(t *testing.T) {
	h := server.NewHandler(server.NewMetrics(), discard())
	h.(*chi.Mux).Get("/abort", func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })
	defer func() {
		if r := recover(); r != http.ErrAbortHandler { //nolint:errorlint // comparing recovered sentinel
			t.Fatalf("recovered %v, want http.ErrAbortHandler re-panicked", r)
		}
	}()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/abort", nil))
}

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := server.LoadConfig(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":8080" || cfg.MetricsAddr != ":9091" || cfg.Environment != "development" ||
		cfg.ShutdownTimeout != 15*time.Second || cfg.SentryDSN != "" || cfg.Release != "" {
		t.Fatalf("defaults = %+v", cfg)
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	cfg, err := server.LoadConfig(env(map[string]string{
		"ADDR": ":1", "METRICS_ADDR": ":2", "ENVIRONMENT": "prod", "RELEASE": "server@1.0.0",
		"SENTRY_DSN": "https://k@example.invalid/1", "SHUTDOWN_TIMEOUT_SECONDS": "3",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":1" || cfg.MetricsAddr != ":2" || cfg.Environment != "prod" || cfg.Release != "server@1.0.0" ||
		cfg.SentryDSN != "https://k@example.invalid/1" || cfg.ShutdownTimeout != 3*time.Second {
		t.Fatalf("overrides = %+v", cfg)
	}
}

func TestLoadConfigErrors(t *testing.T) {
	for _, e := range []map[string]string{
		{"SHUTDOWN_TIMEOUT_SECONDS": "abc"},
		{"SHUTDOWN_TIMEOUT_SECONDS": "0"},
		{"SHUTDOWN_TIMEOUT_SECONDS": "-5"},
		{"ADDR": ":7000", "METRICS_ADDR": ":7000"},
	} {
		if _, err := server.LoadConfig(env(e)); err == nil {
			t.Errorf("LoadConfig(%v) succeeded, want error", e)
		}
	}
	if cfg, err := server.LoadConfig(env(map[string]string{"SHUTDOWN_TIMEOUT_SECONDS": "1"})); err != nil || cfg.ShutdownTimeout != time.Second {
		t.Errorf("SHUTDOWN_TIMEOUT_SECONDS=1 → %v, %v", cfg.ShutdownTimeout, err)
	}
}

func TestInitSentry(t *testing.T) {
	if err := server.InitSentry(server.Config{}); err != nil {
		t.Fatalf("empty DSN should be a no-op, got %v", err)
	}
	if err := server.InitSentry(server.Config{SentryDSN: "not a dsn"}); err == nil {
		t.Fatal("invalid DSN should fail")
	}

	var sent []*sentry.Event
	transport := &captureTransport{events: &sent}
	defer sentry.Init(sentry.ClientOptions{}) //nolint:errcheck // reset global client
	if err := server.InitSentryWithTransport(server.Config{SentryDSN: "https://key@example.invalid/1", Release: "r1", Environment: "test"}, transport); err != nil {
		t.Fatal(err)
	}
	hub := sentry.CurrentHub()
	hub.Scope().SetUser(sentry.User{ID: "u1", IPAddress: "10.0.0.1", Email: "a@b.c", Username: "bob"})
	hub.Scope().SetRequest(httptest.NewRequest(http.MethodGet, "/x?token=secret", nil))
	hub.CaptureMessage("hello")
	sentry.Flush(time.Second)

	if len(sent) != 1 {
		t.Fatalf("captured %d events, want 1", len(sent))
	}
	ev := sent[0]
	if ev.Request != nil || ev.User.IPAddress != "" || ev.User.Email != "" || ev.User.Username != "" {
		t.Errorf("personal data not scrubbed: request=%v user=%+v", ev.Request, ev.User)
	}
	if ev.User.ID != "u1" || ev.Release != "r1" || ev.Environment != "test" {
		t.Errorf("event lost pseudonymous id/release/env: %+v", ev)
	}
}

type captureTransport struct{ events *[]*sentry.Event }

func (c *captureTransport) Configure(sentry.ClientOptions)        {}
func (c *captureTransport) SendEvent(e *sentry.Event)             { *c.events = append(*c.events, e) }
func (c *captureTransport) Flush(time.Duration) bool              { return true }
func (c *captureTransport) FlushWithContext(context.Context) bool { return true }
func (c *captureTransport) Close()                                {}

func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url) //nolint:noctx // test helper
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestServeAndGracefulShutdown(t *testing.T) {
	apiLn, metricsLn := listen(t), listen(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- server.Serve(ctx, server.Config{ShutdownTimeout: time.Second}, discard(), apiLn, metricsLn)
	}()

	if code, body := get(t, "http://"+apiLn.Addr().String()+"/healthz"); code != 200 || !strings.Contains(body, "ok") {
		t.Fatalf("api healthz = %d %q", code, body)
	}
	if code, body := get(t, "http://"+metricsLn.Addr().String()+"/metrics"); code != 200 || !strings.Contains(body, `route="/healthz"`) {
		t.Fatalf("metrics = %d, missing healthz series", code)
	}
	if code, _ := get(t, "http://"+apiLn.Addr().String()+"/metrics"); code != http.StatusNotFound {
		t.Fatalf("/metrics must not be exposed on the public API, got %d", code)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned %v after cancel", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not stop after context cancel")
	}
}

func TestServeReturnsListenerError(t *testing.T) {
	apiLn, metricsLn := listen(t), listen(t)
	_ = metricsLn.Close() // a closed listener makes Serve fail immediately
	err := server.Serve(context.Background(), server.Config{ShutdownTimeout: time.Second}, discard(), apiLn, metricsLn)
	if err == nil {
		t.Fatal("Serve with a closed listener returned nil")
	}
}

func TestRunListenErrors(t *testing.T) {
	busy := listen(t)
	defer func() { _ = busy.Close() }()
	addr := busy.Addr().String()

	if err := server.Run(context.Background(), server.Config{Addr: addr, MetricsAddr: "127.0.0.1:0"}, discard()); err == nil ||
		!strings.Contains(err.Error(), "listen api") {
		t.Errorf("busy api addr: err = %v", err)
	}
	if err := server.Run(context.Background(), server.Config{Addr: "127.0.0.1:0", MetricsAddr: addr}, discard()); err == nil ||
		!strings.Contains(err.Error(), "listen metrics") {
		t.Errorf("busy metrics addr: err = %v", err)
	}
}

func TestRunServesUntilCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := server.Run(ctx, server.Config{Addr: "127.0.0.1:0", MetricsAddr: "127.0.0.1:0", ShutdownTimeout: time.Second}, discard()); err != nil {
		t.Fatalf("Run with cancelled ctx = %v, want nil", err)
	}
}

func TestContentLatest(t *testing.T) {
	rec := httptest.NewRecorder()
	server.NewHandler(server.NewMetrics(), discard()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/content/latest", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "public, max-age=60" {
		t.Fatalf("status %d, cache %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	if body := rec.Body.String(); !strings.HasPrefix(body, `{"contentVersion":"c`) {
		t.Errorf("body = %s", body)
	}
}
