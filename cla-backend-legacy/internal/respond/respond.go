package respond

import (
	"encoding/json"
	"net/http"
)

type ErrorBody struct {
	Message string `json:"message"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func NotImplemented(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusNotImplemented, ErrorBody{Message: "not implemented"})
}

func NotFound(w http.ResponseWriter, r *http.Request) {
	// Python/Hug parity: V2 APIs commonly return 404 for undefined routes in the form:
	//   {"404": "The API call you tried to make was not defined..."}
	// Existing Cypress functional tests rely on this behavior.
	//
	// Note: This handler is used only for unknown/unmapped routes.
	JSON(w, http.StatusNotFound, map[string]any{
		"404": "The API call you tried to make was not defined. Check your spelling and try again.",
	})
}

func MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	// Python/Hug parity: V2 APIs often return method errors in the form:
	//   {"errors": {"405 Method Not Allowed": null}}
	// This is relied upon by existing Cypress tests (see tests/functional/cypress).
	JSON(w, http.StatusMethodNotAllowed, map[string]any{"errors": map[string]any{"405 Method Not Allowed": nil}})
}
