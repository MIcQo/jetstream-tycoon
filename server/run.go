package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
)

// Config is the server configuration, read from environment variables.
type Config struct {
	Addr            string        // public API listener, ADDR (default ":8080")
	MetricsAddr     string        // internal metrics listener, METRICS_ADDR (default ":9091")
	Environment     string        // ENVIRONMENT (default "development")
	Release         string        // RELEASE, e.g. "server@1.2.0" (Sentry release)
	SentryDSN       string        // SENTRY_DSN; empty disables Sentry
	ShutdownTimeout time.Duration // SHUTDOWN_TIMEOUT_SECONDS (default 15)
}

// LoadConfig builds a Config from getenv (normally os.Getenv).
func LoadConfig(getenv func(string) string) (Config, error) {
	cfg := Config{
		Addr:            withDefault(getenv("ADDR"), ":8080"),
		MetricsAddr:     withDefault(getenv("METRICS_ADDR"), ":9091"),
		Environment:     withDefault(getenv("ENVIRONMENT"), "development"),
		Release:         getenv("RELEASE"),
		SentryDSN:       getenv("SENTRY_DSN"),
		ShutdownTimeout: 15 * time.Second,
	}
	if v := getenv("SHUTDOWN_TIMEOUT_SECONDS"); v != "" {
		secs, err := strconv.Atoi(v)
		if err != nil || secs <= 0 {
			return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT_SECONDS must be a positive integer, got %q", v)
		}
		cfg.ShutdownTimeout = time.Duration(secs) * time.Second
	}
	if cfg.Addr == cfg.MetricsAddr {
		return Config{}, errors.New("ADDR and METRICS_ADDR must differ: metrics stay on an internal listener")
	}
	return cfg, nil
}

func withDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// InitSentry configures error reporting (ADR-0014). Personal data is never sent:
// the SDK default collects no personal data, and scrubEvent strips request data and
// user identifiers other than the pseudonymous ID before sending.
func InitSentry(cfg Config) error {
	return initSentry(cfg, nil)
}

// initSentry allows tests to inject a transport; nil uses Sentry's HTTP transport.
func initSentry(cfg Config, transport sentry.Transport) error {
	if cfg.SentryDSN == "" {
		return nil
	}
	return sentry.Init(sentry.ClientOptions{
		Dsn:         cfg.SentryDSN,
		Environment: cfg.Environment,
		Release:     cfg.Release,
		BeforeSend:  scrubEvent,
		Transport:   transport,
	})
}

// scrubEvent removes request data and user IP before an event leaves the process.
func scrubEvent(ev *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	ev.Request = nil
	ev.User.IPAddress = ""
	ev.User.Email = ""
	ev.User.Username = ""
	return ev
}

// Run serves the API on cfg.Addr and metrics on cfg.MetricsAddr until ctx is
// cancelled, then shuts both down gracefully within cfg.ShutdownTimeout.
func Run(ctx context.Context, cfg Config, logger *slog.Logger) error {
	apiLn, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen api: %w", err)
	}
	metricsLn, err := net.Listen("tcp", cfg.MetricsAddr)
	if err != nil {
		_ = apiLn.Close()
		return fmt.Errorf("listen metrics: %w", err)
	}
	return Serve(ctx, cfg, logger, apiLn, metricsLn)
}

// Serve is Run on already-open listeners (used by tests).
func Serve(ctx context.Context, cfg Config, logger *slog.Logger, apiLn, metricsLn net.Listener) error {
	metrics := NewMetrics()
	api := &http.Server{Handler: NewHandler(metrics, logger), ReadHeaderTimeout: 5 * time.Second}
	internal := &http.Server{Handler: metrics.Handler(), ReadHeaderTimeout: 5 * time.Second}

	errc := make(chan error, 2)
	go func() { errc <- serveOne(api, apiLn) }()
	go func() { errc <- serveOne(internal, metricsLn) }()
	logger.Info("server started", "api", apiLn.Addr().String(), "metrics", metricsLn.Addr().String())

	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-errc:
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	shutdownErr := errors.Join(api.Shutdown(shutdownCtx), internal.Shutdown(shutdownCtx))
	sentry.Flush(2 * time.Second)
	logger.Info("server stopped")
	return errors.Join(runErr, shutdownErr)
}

func serveOne(s *http.Server, ln net.Listener) error {
	if err := s.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
