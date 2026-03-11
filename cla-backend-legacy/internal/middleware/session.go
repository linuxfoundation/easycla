// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/store"
)

// Session is a minimal server-side session map compatible with the legacy Python
// hug.middleware.sessions.SessionMiddleware usage.
//
// Cookie: cla-sid
// Storage: DynamoDB store table via KVStore (value is JSON)
// Cookie MaxAge: 300 seconds (matches Python)
// Store TTL: KVStore default (45 minutes, matches Python Store model)
type Session map[string]any

type ctxKeySession struct{}

var sessionKey = ctxKeySession{}

// SessionFromContext returns the request session map if present.
func SessionFromContext(ctx context.Context) Session {
	if ctx == nil {
		return nil
	}
	if v := ctx.Value(sessionKey); v != nil {
		if s, ok := v.(Session); ok {
			return s
		}
		// Backwards compatibility: sometimes handlers may store map[string]any.
		if m, ok := v.(map[string]any); ok {
			return Session(m)
		}
	}
	return nil
}

func withSession(ctx context.Context, s Session) context.Context {
	return context.WithValue(ctx, sessionKey, s)
}

// SessionMiddleware attaches a server-side session to request context and persists
// it to the configured KVStore.
//
// This is intentionally minimal and matches legacy behavior closely:
//   - cookie_name: cla-sid
//   - cookie_max_age: 300
//   - secure: false
//   - domain: none
//   - context_name: session
func SessionMiddleware(kv *store.KVStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Load or create session id.
			sid := ""
			if c, err := r.Cookie("cla-sid"); err == nil {
				sid = strings.TrimSpace(c.Value)
			}
			if sid == "" {
				sid = uuid.New().String()
			}

			sess := make(Session)

			// Load from store.
			if kv != nil {
				if raw, ok, err := kv.Get(r.Context(), sid); err == nil && ok {
					raw = strings.TrimSpace(raw)
					if raw != "" {
						var m map[string]any
						if err := json.Unmarshal([]byte(raw), &m); err == nil {
							sess = Session(m)
						}
					}
				}
			}

			// Always refresh cookie max-age like the Python middleware.
			http.SetCookie(w, &http.Cookie{
				Name:     "cla-sid",
				Value:    sid,
				Path:     "/",
				MaxAge:   300,
				Secure:   true,
				HttpOnly: true,
			})

			// Attach session to request context.
			r = r.WithContext(withSession(r.Context(), sess))

			// Continue request processing.
			next.ServeHTTP(w, r)

			// Persist session. Best-effort; match legacy behavior where session persistence
			// should never fail the request after handler logic completed.
			if kv != nil {
				if b, err := json.Marshal(sess); err == nil {
					// Keep store TTL aligned with KVStore default (45 minutes).
					_ = kv.SetWithTTL(context.Background(), sid, string(b), 45*time.Minute)
				}
			}
		})
	}
}
