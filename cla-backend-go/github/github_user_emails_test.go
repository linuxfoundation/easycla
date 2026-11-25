// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListVerifiedEmails(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/user/emails", r.URL.Path)
		assert.Equal(t, "100", r.URL.Query().Get("per_page"))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "2" {
			_, err := io.WriteString(w, `[{"email":" Second@Example.org ","verified":true}]`)
			assert.NoError(t, err)
			return
		}
		w.Header().Set("Link", fmt.Sprintf(`<%s/user/emails?page=2&per_page=100>; rel="next"`, srv.URL))
		_, err := io.WriteString(w, `[{"email":"first@example.org","verified":true,"primary":true},{"email":"pending@example.org","verified":false},{"email":" ","verified":true}]`)
		assert.NoError(t, err)
	}))
	defer srv.Close()

	emails, err := listVerifiedEmails(context.Background(), newTestGithubClient(t, srv))

	assert.NoError(t, err)
	assert.Equal(t, []string{"first@example.org", "Second@Example.org"}, emails)
}

func TestListVerifiedEmailsPrefersAddressesThatAreNotNoreply(t *testing.T) {
	tests := []struct {
		name, body string
		want       []string
	}{
		{"a real address next to a noreply one", `[{"email":"1+octocat@users.noreply.github.com","verified":true},{"email":"real@example.org","verified":true}]`, []string{"real@example.org"}},
		{"only a noreply address", `[{"email":"1+octocat@users.noreply.github.com","verified":true},{"email":"pending@example.org","verified":false}]`, []string{"1+octocat@users.noreply.github.com"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, err := io.WriteString(w, tt.body)
				assert.NoError(t, err)
			}))
			defer srv.Close()

			emails, err := listVerifiedEmails(context.Background(), newTestGithubClient(t, srv))

			assert.NoError(t, err)
			assert.Equal(t, tt.want, emails)
		})
	}
}

func TestListVerifiedEmailsFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, err := io.WriteString(w, `{"message":"Bad credentials"}`)
		assert.NoError(t, err)
	}))
	defer srv.Close()

	emails, err := listVerifiedEmails(context.Background(), newTestGithubClient(t, srv))

	assert.Error(t, err)
	assert.Nil(t, emails)
}
