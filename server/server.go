// Package server implements the stateless HTTP API handlers (ADR-0007, ADR-0008, ADR-0010).
package server

import "net/http"

// NewHandler returns the root HTTP handler with a /healthz endpoint.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	return mux
}
