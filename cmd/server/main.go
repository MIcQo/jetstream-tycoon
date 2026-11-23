// Command server runs the stateless HTTP API server.
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/MIcQo/jetstream-tycoon/server"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           server.NewHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Println("server starting")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
