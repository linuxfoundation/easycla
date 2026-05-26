// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/server"
)

// Local entrypoint to run the legacy Go v1/v2 router as a normal HTTP server.
func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":5000"
	}

	h := server.NewHTTPHandler()
	log.Printf("cla-backend-legacy local listening on %s", addr)
	log.Printf("STAGE=%q", os.Getenv("STAGE"))

	srv := &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
