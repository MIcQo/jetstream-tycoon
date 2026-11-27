// Package server implements the stateless HTTP API (ADR-0007, ADR-0008, ADR-0010).
// The server never runs the game loop (ADR-0002).
package server

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/MIcQo/jetstream-tycoon/sim/content/bundle"
)

// NewHandler returns the public API router. Every request is measured by m, and
// panics are recovered, reported to Sentry (when initialised) and answered with 500.
func NewHandler(m *Metrics, logger *slog.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(m.Middleware)
	r.Use(recoverer(logger))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, `{"status":"ok"}`)
	})
	r.Get("/content/latest", contentLatest)
	return r
}

// contentLatest reports the content version compiled into this server, so clients
// can tell whether a newer content bundle exists (TECH_DESIGN §13). The response is
// the same for everyone and safe for the CDN to cache briefly.
func contentLatest(w http.ResponseWriter, _ *http.Request) {
	b, err := bundle.Load()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, `{"error":"content unavailable"}`)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSON(w, http.StatusOK, fmt.Sprintf(`{"contentVersion":%q}`, b.ContentVersion))
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

// recoverer turns a panic into a 500 response, logs it without request data
// (no personal data, golden rule 7) and sends it to Sentry.
func recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if rec == http.ErrAbortHandler { //nolint:errorlint // sentinel value re-panicked as net/http expects
					panic(rec)
				}
				logger.Error("panic in handler", "route", routePattern(r), "panic", fmt.Sprint(rec))
				sentry.CurrentHub().Recover(rec)
				writeJSON(w, http.StatusInternalServerError, `{"error":"internal"}`)
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// routePattern returns the matched chi route template (e.g. "/saves/{id}"), never
// the raw path, so metrics and logs stay low-cardinality and free of IDs.
func routePattern(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if p := rc.RoutePattern(); p != "" {
			return p
		}
	}
	return "unmatched"
}
