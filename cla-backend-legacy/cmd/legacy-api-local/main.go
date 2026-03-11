package main

import (
	"log"
	"net/http"
	"os"

	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/server"
)

// Local entrypoint to run the legacy API router as a normal HTTP server.
//
// This is intentionally minimal and only meant to speed up endpoint-by-endpoint migration.
// It supports proxy mode via LEGACY_UPSTREAM_BASE_URL, same as the lambda deployment.
func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	h := server.NewHTTPHandler()
	log.Printf("cla-backend-legacy local listening on %s", addr)
	log.Printf("STAGE=%q LEGACY_UPSTREAM_BASE_URL=%q", os.Getenv("STAGE"), os.Getenv("LEGACY_UPSTREAM_BASE_URL"))

	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
