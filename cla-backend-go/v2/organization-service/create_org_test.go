// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package organization_service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linuxfoundation/easycla/cla-backend-go/token"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/client/organizations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	orgTestPlatformHost = "platform.example.invalid"
	orgTestAuthHost     = "auth.example.invalid"
)

// orgTestTransport answers the org-service and Auth0 calls CreateOrg makes and records them; anything else fails the test
type orgTestTransport struct {
	t          *testing.T
	lookupHit  bool
	mu         sync.Mutex
	calls      []string
	createBody string
}

func (h *orgTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	h.mu.Lock()
	h.calls = append(h.calls, r.Method+" "+r.URL.Path)
	h.mu.Unlock()
	reply := func(status int, body string) (*http.Response, error) {
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	}
	switch {
	case r.URL.Host == orgTestAuthHost:
		return reply(http.StatusOK, `{"access_token":"test-token","token_type":"Bearer","expires_in":86400}`)
	case r.URL.Host == orgTestPlatformHost && r.URL.Path == "/organization-service/v1/lookup":
		if h.lookupHit {
			return reply(http.StatusOK, `{"ID":"0014100000Existing","Name":"Existing Org"}`)
		}
		return reply(http.StatusNotFound, `{"Code":"404","Message":"not found"}`)
	case r.URL.Host == orgTestPlatformHost && r.URL.Path == "/organization-service/v1/orgs/search":
		return reply(http.StatusOK, `{"Data":[{"ID":"0014100000TheLF","Name":"The Linux Foundation","LogoURL":"https://logo.example.invalid/lf.png?v=1"}],"Metadata":{"TotalSize":1,"Offset":0,"PageSize":100}}`)
	case r.URL.Host == orgTestPlatformHost && r.URL.Path == "/organization-service/v1/orgs" && r.Method == http.MethodPost:
		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			h.t.Errorf("reading the create request body: %v", readErr)
		}
		h.mu.Lock()
		h.createBody = string(body)
		h.mu.Unlock()
		return reply(http.StatusCreated, `{"ID":"0014100000Created","Name":"Acme Inc"}`)
	default:
		h.t.Errorf("unexpected HTTP request: %s %s", r.Method, r.URL)
		return nil, fmt.Errorf("unexpected HTTP request: %s %s", r.Method, r.URL)
	}
}

func (h *orgTestTransport) recorded() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.calls...)
}

type orgTestSwitch struct {
	mu      sync.Mutex
	current http.RoundTripper
}

func (s *orgTestSwitch) set(rt http.RoundTripper) { s.mu.Lock(); s.current = rt; s.mu.Unlock() }
func (s *orgTestSwitch) RoundTrip(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	rt := s.current
	s.mu.Unlock()
	if rt == nil {
		return nil, fmt.Errorf("no test transport installed: %s %s", r.Method, r.URL)
	}
	return rt.RoundTrip(r)
}

var (
	orgSwitch     = &orgTestSwitch{}
	orgSwitchOnce sync.Once
)

func newOrgTestClient(t *testing.T, lookupHit bool) *orgTestTransport {
	t.Helper()
	transport := &orgTestTransport{t: t, lookupHit: lookupHit}
	previousTransport, previousClient := http.DefaultTransport, http.DefaultClient
	http.DefaultTransport = orgSwitch
	http.DefaultClient = &http.Client{Transport: orgSwitch}
	orgSwitchOnce.Do(func() {
		token.Init("test-client", "test-secret", "https://"+orgTestAuthHost+"/oauth/token", "test-audience")
	})
	orgSwitch.set(transport)
	t.Cleanup(func() {
		orgSwitch.set(nil)
		http.DefaultTransport, http.DefaultClient = previousTransport, previousClient
	})
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := token.GetToken(); err == nil {
			break
		}
		require.True(t, time.Now().Before(deadline), "mock token initialization did not complete")
		time.Sleep(10 * time.Millisecond)
	}
	InitClient("https://"+orgTestPlatformHost, nil)
	return transport
}

// an organization that already owns the website domain is reported as a conflict: it is neither updated nor returned (lfx-self-serve-ops#343)
func TestCreateOrgRefusesExistingDomainOwner(t *testing.T) {
	transport := newOrgTestClient(t, true)
	org, err := GetClient().CreateOrg(context.Background(), "Acme Inc", "Acme Inc", "https://acme.example.invalid")
	require.Error(t, err)
	var conflict *organizations.CreateOrgConflict
	require.ErrorAs(t, err, &conflict)
	assert.Contains(t, conflict.Payload.Message, "0014100000Existing")
	assert.Nil(t, org)
	for _, call := range transport.recorded() {
		assert.NotContains(t, call, "/organization-service/v1/orgs/0014100000Existing", "the existing organization must not be read or updated")
		assert.NotEqual(t, "POST /organization-service/v1/orgs", call, "no organization may be created on a domain hit")
	}
}

func TestCreateOrgCreatesWhenNoOrganizationOwnsTheDomain(t *testing.T) {
	transport := newOrgTestClient(t, false)
	org, err := GetClient().CreateOrg(context.Background(), "Acme Inc", "", "https://acme.example.invalid")
	require.NoError(t, err)
	require.NotNil(t, org)
	assert.Equal(t, "0014100000Created", org.ID)
	assert.Contains(t, transport.recorded(), "POST /organization-service/v1/orgs")
	assert.Contains(t, transport.createBody, `"SigningEntityName":["Acme Inc"]`)
	assert.Contains(t, transport.createBody, `"LogoURL":"https://logo.example.invalid/lf.png"`)
	for _, call := range transport.recorded() {
		assert.False(t, strings.HasPrefix(call, "PUT "), "no organization update expected")
	}
}
