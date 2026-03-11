package server

import (
	"net/http"

	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/api"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/telemetry"
)

// NewHTTPHandler builds the HTTP handler for both Lambda (via adapter) and local runs.
//
// Note: router-level middleware already handles request logging and CORS.
// We intentionally avoid double-wrapping here to keep behavior consistent.
func NewHTTPHandler() http.Handler {
	h := api.NewHandlers()
	router := api.NewRouter(h)
	return telemetry.WrapHTTPHandler(router)
}
