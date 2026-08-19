// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTrustedClientID   = "ss-confidential-client-id"
	testUntrustedClientID = "some-other-client-id"
	testKeyID             = "kid-1"
)

func testVerifier(t *testing.T, key *rsa.PrivateKey) (*TrustedCallerVerifier, *int) {
	t.Helper()
	verifier, err := NewTrustedCallerVerifier("linuxfoundation-dev.auth0.com", "RS256", []string{testTrustedClientID})
	require.NoError(t, err)

	fetches := 0
	verifier.fetchKeys = func() (map[string]*rsa.PublicKey, error) {
		fetches++
		return map[string]*rsa.PublicKey{testKeyID: &key.PublicKey}, nil
	}
	return verifier, &fetches
}

func testToken(t *testing.T, key *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	if kid != "" {
		token.Header["kid"] = kid
	}
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	return signed
}

func testClaims(clientID string) jwt.MapClaims {
	return jwt.MapClaims{
		"iss": "https://linuxfoundation-dev.auth0.com/",
		"sub": clientID + "@clients",
		"azp": clientID,
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Add(-time.Minute).Unix(),
	}
}

func testKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func TestTrustedCallerVerifierEnabled(t *testing.T) {
	var nilVerifier *TrustedCallerVerifier
	assert.False(t, nilVerifier.Enabled())

	disabled, err := NewTrustedCallerVerifier("", "", nil)
	require.NoError(t, err, "an unset allow-list must not fail startup, it only disables the trusted caller path")
	assert.False(t, disabled.Enabled())

	blank, err := NewTrustedCallerVerifier("", "", []string{" ", ""})
	require.NoError(t, err)
	assert.False(t, blank.Enabled())

	_, err = NewTrustedCallerVerifier("", "", []string{testTrustedClientID})
	assert.Error(t, err, "a configured allow-list without an Auth0 domain must fail startup rather than trust blindly")

	enabled, err := NewTrustedCallerVerifier("linuxfoundation-dev.auth0.com", "", []string{" " + testTrustedClientID + " "})
	require.NoError(t, err)
	assert.True(t, enabled.Enabled())
	assert.Equal(t, defaultJWTAlgorithm, enabled.algorithm)
	assert.Equal(t, "https://linuxfoundation-dev.auth0.com/.well-known/jwks.json", enabled.wellKnownURL)
	assert.True(t, enabled.allowedClientIDs[testTrustedClientID], "a padded client ID must be trimmed, not stored verbatim")

	trailingSlash, err := NewTrustedCallerVerifier("linuxfoundation-dev.auth0.com/", "RS512", []string{testTrustedClientID})
	require.NoError(t, err)
	assert.Equal(t, "https://linuxfoundation-dev.auth0.com/.well-known/jwks.json", trailingSlash.wellKnownURL)
	assert.Equal(t, "RS512", trailingSlash.algorithm)
}

// an absent header is the same as a malformed one: the traefik aws-lambda middleware drops
// duplicated headers, so a duplicated Authorization header arrives as an absent one
func TestVerifyMissingBearerToken(t *testing.T) {
	verifier, fetches := testVerifier(t, testKey(t))

	basic := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
	for _, header := range []string{"", "   ", basic, "Bearer", "Bearer ", "Bearer    ", "bearer\t", "token abc", "Bearer\n"} {
		caller, err := verifier.Verify(header)
		assert.Nil(t, caller)
		assert.ErrorIs(t, err, ErrNoBearerToken, "header %q must be denied", header)
	}
	assert.Zero(t, *fetches, "an unparseable header must not trigger a JWKS lookup")
}

func TestVerifyTrustedAndUntrustedClients(t *testing.T) {
	key := testKey(t)
	verifier, _ := testVerifier(t, key)

	trusted, err := verifier.Verify("Bearer " + testToken(t, key, testKeyID, testClaims(testTrustedClientID)))
	require.NoError(t, err)
	assert.True(t, trusted.Trusted)
	assert.Equal(t, testTrustedClientID, trusted.ClientID)
	assert.Equal(t, testTrustedClientID+"@clients", trusted.Subject)

	// a valid token from any other client is verified but never trusted
	untrusted, err := verifier.Verify("bearer " + testToken(t, key, testKeyID, testClaims(testUntrustedClientID)))
	require.NoError(t, err)
	assert.False(t, untrusted.Trusted)
	assert.Equal(t, testUntrustedClientID, untrusted.ClientID)

	// a client ID that only differs in case must not match the allow-list
	mixedCase, err := verifier.Verify("Bearer " + testToken(t, key, testKeyID, testClaims(strings.ToUpper(testTrustedClientID))))
	require.NoError(t, err)
	assert.False(t, mixedCase.Trusted)
}

// a verified token carrying no usable azp claim can never match the allow-list, so it is
// untrusted rather than denied and keeps the pre-existing per-identity verification
func TestVerifyNeverTrustsATokenWithoutAnAzpClaim(t *testing.T) {
	key := testKey(t)
	verifier, _ := testVerifier(t, key)

	absent := testClaims(testTrustedClientID)
	delete(absent, "azp")
	empty := testClaims(testTrustedClientID)
	empty["azp"] = ""
	blank := testClaims(testTrustedClientID)
	blank["azp"] = "   "
	numeric := testClaims(testTrustedClientID)
	numeric["azp"] = 42

	for name, claims := range map[string]jwt.MapClaims{"absent": absent, "empty": empty, "blank": blank, "non-string": numeric} {
		t.Run(name, func(t *testing.T) {
			caller, err := verifier.Verify("Bearer " + testToken(t, key, testKeyID, claims))
			require.NoError(t, err)
			assert.False(t, caller.Trusted)
		})
	}
}

func TestVerifyRejectsInvalidTokens(t *testing.T) {
	key := testKey(t)
	otherKey := testKey(t)
	verifier, _ := testVerifier(t, key)

	expired := testClaims(testTrustedClientID)
	expired["exp"] = time.Now().Add(-time.Minute).Unix()

	noExpiration := testClaims(testTrustedClientID)
	delete(noExpiration, "exp")

	notYetValid := testClaims(testTrustedClientID)
	notYetValid["nbf"] = time.Now().Add(time.Hour).Unix()

	issuedInTheFuture := testClaims(testTrustedClientID)
	issuedInTheFuture["iat"] = time.Now().Add(time.Hour).Unix()

	tests := []struct {
		name  string
		token string
	}{
		{"expired", testToken(t, key, testKeyID, expired)},
		{"no expiration", testToken(t, key, testKeyID, noExpiration)},
		{"not yet valid", testToken(t, key, testKeyID, notYetValid)},
		{"issued in the future", testToken(t, key, testKeyID, issuedInTheFuture)},
		{"no key ID", testToken(t, key, "", testClaims(testTrustedClientID))},
		{"unknown key ID", testToken(t, key, "kid-does-not-exist", testClaims(testTrustedClientID))},
		{"signed by another key", testToken(t, otherKey, testKeyID, testClaims(testTrustedClientID))},
		{"not a JWT", "not-a-jwt"},
		{"only a header", base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"kid-1"}`))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caller, err := verifier.Verify("Bearer " + test.token)
			assert.Nil(t, caller)
			assert.Error(t, err)
		})
	}
}

// the signing algorithm must be pinned: an HMAC token would otherwise be verified with the
// JWKS RSA modulus as its shared secret, and "alg": "none" would skip verification entirely
func TestVerifyRejectsOtherSigningAlgorithms(t *testing.T) {
	key := testKey(t)
	verifier, _ := testVerifier(t, key)

	hmacToken := jwt.NewWithClaims(jwt.SigningMethodHS256, testClaims(testTrustedClientID))
	hmacToken.Header["kid"] = testKeyID
	signed, err := hmacToken.SignedString(key.PublicKey.N.Bytes())
	require.NoError(t, err)

	caller, err := verifier.Verify("Bearer " + signed)
	assert.Nil(t, caller)
	assert.Error(t, err)

	noneToken := jwt.NewWithClaims(jwt.SigningMethodNone, testClaims(testTrustedClientID))
	noneToken.Header["kid"] = testKeyID
	unsigned, err := noneToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	caller, err = verifier.Verify("Bearer " + unsigned)
	assert.Nil(t, caller)
	assert.Error(t, err)
}

func TestVerifyCachesTheJWKS(t *testing.T) {
	key := testKey(t)
	verifier, fetches := testVerifier(t, key)
	clock := time.Now()
	verifier.now = func() time.Time { return clock }

	for i := 0; i < 3; i++ {
		_, err := verifier.Verify("Bearer " + testToken(t, key, testKeyID, testClaims(testTrustedClientID)))
		require.NoError(t, err)
	}
	assert.Equal(t, 1, *fetches, "the JWKS must be fetched once and cached")

	// an unknown key ID triggers at most one refresh per cooldown window, so a flood of
	// unknown-kid tokens cannot be used to hammer the Auth0 JWKS endpoint
	unknown := "Bearer " + testToken(t, key, "kid-does-not-exist", testClaims(testTrustedClientID))
	for i := 0; i < 5; i++ {
		_, err := verifier.Verify(unknown)
		require.Error(t, err)
	}
	assert.Equal(t, 1, *fetches, "the cooldown started by the initial fetch is still open")

	clock = clock.Add(jwksRefreshCooldown + time.Second)
	_, err := verifier.Verify(unknown)
	require.Error(t, err)
	assert.Equal(t, 2, *fetches, "after the cooldown an unknown key ID may refresh the key set again")

	clock = clock.Add(jwksCacheTTL)
	_, err = verifier.Verify("Bearer " + testToken(t, key, testKeyID, testClaims(testTrustedClientID)))
	require.NoError(t, err)
	assert.Equal(t, 3, *fetches, "an expired key set is reloaded")
}

// a JWKS endpoint outage must not invalidate an already cached key
func TestVerifyFallsBackToTheCachedKey(t *testing.T) {
	key := testKey(t)
	verifier, _ := testVerifier(t, key)
	clock := time.Now()
	verifier.now = func() time.Time { return clock }

	_, err := verifier.Verify("Bearer " + testToken(t, key, testKeyID, testClaims(testTrustedClientID)))
	require.NoError(t, err)

	verifier.fetchKeys = func() (map[string]*rsa.PublicKey, error) {
		return nil, assert.AnError
	}
	clock = clock.Add(jwksCacheTTL + time.Minute)

	caller, err := verifier.Verify("Bearer " + testToken(t, key, testKeyID, testClaims(testTrustedClientID)))
	require.NoError(t, err)
	assert.True(t, caller.Trusted)

	caller, err = verifier.Verify("Bearer " + testToken(t, key, "kid-does-not-exist", testClaims(testTrustedClientID)))
	assert.Nil(t, caller, "an unknown key ID must still be denied when the JWKS cannot be reloaded")
	assert.Error(t, err)
}

// the fallback above is bounded: a revoked key must not stay usable for an unlimited outage
func TestVerifyRejectsAStaleCachedKey(t *testing.T) {
	key := testKey(t)
	verifier, _ := testVerifier(t, key)
	clock := time.Now()
	verifier.now = func() time.Time { return clock }

	_, err := verifier.Verify("Bearer " + testToken(t, key, testKeyID, testClaims(testTrustedClientID)))
	require.NoError(t, err)

	verifier.fetchKeys = func() (map[string]*rsa.PublicKey, error) {
		return nil, assert.AnError
	}

	clock = clock.Add(jwksCacheTTL + jwksStaleKeyGrace - time.Minute)
	caller, err := verifier.Verify("Bearer " + testToken(t, key, testKeyID, testClaims(testTrustedClientID)))
	require.NoError(t, err, "within the grace period the cached key is still used")
	assert.True(t, caller.Trusted)

	clock = clock.Add(2 * time.Minute)
	caller, err = verifier.Verify("Bearer " + testToken(t, key, testKeyID, testClaims(testTrustedClientID)))
	assert.Nil(t, caller)
	assert.Error(t, err, "past the grace period a key that cannot be re-fetched must be rejected")
}

// a misconfigured cla-auth0-algorithm cannot open a hole: the verifier only ever hands the parser
// an RSA public key, so a non-RSA algorithm fails closed instead of accepting forged signatures
func TestVerifyRejectsMisconfiguredAlgorithms(t *testing.T) {
	key := testKey(t)

	hmac := jwt.NewWithClaims(jwt.SigningMethodHS256, testClaims(testTrustedClientID))
	hmac.Header["kid"] = testKeyID
	hmacToken, err := hmac.SignedString(key.PublicKey.N.Bytes())
	require.NoError(t, err)

	none := jwt.NewWithClaims(jwt.SigningMethodNone, testClaims(testTrustedClientID))
	none.Header["kid"] = testKeyID
	noneToken, err := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	for algorithm, rawToken := range map[string]string{"HS256": hmacToken, "none": noneToken} {
		verifier, verifierErr := NewTrustedCallerVerifier("linuxfoundation-dev.auth0.com", algorithm, []string{testTrustedClientID})
		require.NoError(t, verifierErr)
		verifier.fetchKeys = func() (map[string]*rsa.PublicKey, error) {
			return map[string]*rsa.PublicKey{testKeyID: &key.PublicKey}, nil
		}

		caller, verifyErr := verifier.Verify("Bearer " + rawToken)
		assert.Nil(t, caller, "a verifier configured with %s must not accept a token", algorithm)
		assert.Error(t, verifyErr)
	}
}

// the grace window is exclusive: at exactly keysExpireAt+jwksStaleKeyGrace the cached key is gone
func TestVerifyRejectsTheCachedKeyAtTheGraceBoundary(t *testing.T) {
	key := testKey(t)
	verifier, _ := testVerifier(t, key)
	clock := time.Now()
	verifier.now = func() time.Time { return clock }
	token := "Bearer " + testToken(t, key, testKeyID, testClaims(testTrustedClientID))

	_, err := verifier.Verify(token)
	require.NoError(t, err)
	verifier.fetchKeys = func() (map[string]*rsa.PublicKey, error) { return nil, assert.AnError }
	boundary := verifier.keysExpireAt.Add(jwksStaleKeyGrace)

	clock = boundary.Add(-time.Nanosecond)
	_, err = verifier.Verify(token)
	require.NoError(t, err, "a nanosecond before the boundary the cached key is still served")

	clock = boundary
	_, err = verifier.Verify(token)
	assert.Error(t, err, "at the boundary the cached key must be rejected")
}

// the tuning has to keep these relations: a reload is rate limited for less than the cache
// lifetime, one attempt cannot outlast its own rate limit, and the outage grace outlives the cache
func TestJWKSCacheTuning(t *testing.T) {
	assert.Positive(t, jwksRefreshCooldown)
	assert.Less(t, jwksRefreshCooldown, jwksCacheTTL)
	assert.LessOrEqual(t, jwksRequestTimeout, jwksRefreshCooldown)
	assert.Greater(t, jwksStaleKeyGrace, jwksCacheTTL)
}

// a failed reload must not be retried per request: once the TTL expires during a JWKS outage the
// cooldown has to serve the grace-bounded cached key instead of paying the fetch timeout again
func TestVerifyRateLimitsReloadsDuringAnOutage(t *testing.T) {
	key := testKey(t)
	verifier, _ := testVerifier(t, key)
	clock := time.Now()
	verifier.now = func() time.Time { return clock }
	token := "Bearer " + testToken(t, key, testKeyID, testClaims(testTrustedClientID))

	_, err := verifier.Verify(token)
	require.NoError(t, err)

	attempts := 0
	verifier.fetchKeys = func() (map[string]*rsa.PublicKey, error) {
		attempts++
		return nil, assert.AnError
	}
	clock = clock.Add(jwksCacheTTL + time.Second)

	for i := 0; i < 5; i++ {
		caller, verifyErr := verifier.Verify(token)
		require.NoError(t, verifyErr)
		assert.True(t, caller.Trusted)
	}
	assert.Equal(t, 1, attempts, "a stale key set must be reloaded at most once per cooldown window")

	clock = clock.Add(jwksRefreshCooldown + time.Second)
	_, err = verifier.Verify(token)
	require.NoError(t, err)
	assert.Equal(t, 2, attempts, "after the cooldown the reload is attempted again")

	caller, err := verifier.Verify("Bearer " + testToken(t, key, "kid-does-not-exist", testClaims(testTrustedClientID)))
	assert.Nil(t, caller)
	assert.ErrorContains(t, err, "rate limited", "the denial must come from the cooldown, not from a reload attempt")
	assert.Equal(t, 2, attempts, "an unknown key ID must not reload the JWKS while the cooldown is open")

	// the grace bound still applies when the cooldown short-circuits the reload
	clock = clock.Add(jwksStaleKeyGrace)
	_, err = verifier.Verify(token)
	require.Error(t, err)
	require.Equal(t, 3, attempts)
	caller, err = verifier.Verify(token)
	assert.Nil(t, caller)
	assert.Error(t, err, "past the grace period the cooldown must not serve the cached key either")
	assert.Equal(t, 3, attempts)
}

func TestVerifyIsSafeForConcurrentUse(t *testing.T) {
	key := testKey(t)
	verifier, fetches := testVerifier(t, key)
	token := "Bearer " + testToken(t, key, testKeyID, testClaims(testTrustedClientID))

	var wg sync.WaitGroup
	results := make([]bool, 16)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			caller, err := verifier.Verify(token)
			if err == nil && caller != nil {
				results[i] = caller.Trusted
			}
		}(i)
	}
	wg.Wait()

	for i, trusted := range results {
		assert.True(t, trusted, "concurrent verify %d must succeed", i)
	}
	assert.Equal(t, 1, *fetches, "concurrent verifies must share a single JWKS fetch")
}

func TestFetchJWKS(t *testing.T) {
	key := testKey(t)
	body := map[string]interface{}{
		"keys": []map[string]interface{}{
			{"kty": "EC", "kid": "not-rsa", "use": "sig"},
			{"kty": "RSA", "kid": "encryption-key", "use": "enc"},
			{"kty": "RSA", "kid": "malformed", "use": "sig", "n": "!!!", "e": "AQAB"},
			{"kty": "RSA", "kid": testKeyID, "use": "sig",
				"n": base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
				"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes())},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(body))
	}))
	defer server.Close()

	verifier, err := NewTrustedCallerVerifier("linuxfoundation-dev.auth0.com", "RS256", []string{testTrustedClientID})
	require.NoError(t, err)
	verifier.wellKnownURL = server.URL

	keys, err := verifier.fetchJWKS()
	require.NoError(t, err)
	require.Len(t, keys, 1, "only usable RSA signing keys are kept")
	assert.Equal(t, key.PublicKey.N, keys[testKeyID].N)
	assert.Equal(t, key.PublicKey.E, keys[testKeyID].E)

	caller, err := verifier.Verify("Bearer " + testToken(t, key, testKeyID, testClaims(testTrustedClientID)))
	require.NoError(t, err)
	assert.True(t, caller.Trusted)

	failureModes := map[string]http.HandlerFunc{
		"server error": func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
		"not found":    func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) },
		"unauthorized": func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) },
		"not json":     func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "{not json") },
		"no keys":      func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{"keys":[]}`) },
		"only unusable keys": func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, `{"keys":[{"kty":"EC","kid":"ec","use":"sig"}]}`)
		},
	}
	for name, handler := range failureModes {
		t.Run(name, func(t *testing.T) {
			failing := httptest.NewServer(handler)
			defer failing.Close()
			verifier.wellKnownURL = failing.URL
			keySet, fetchErr := verifier.fetchJWKS()
			assert.Nil(t, keySet)
			assert.Error(t, fetchErr)
		})
	}

	t.Run("unreachable", func(t *testing.T) {
		unreachable := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		verifier.wellKnownURL = unreachable.URL
		unreachable.Close()
		keySet, fetchErr := verifier.fetchJWKS()
		assert.Nil(t, keySet)
		assert.Error(t, fetchErr)
	})
}

func TestParseRSAPublicKey(t *testing.T) {
	key := testKey(t)

	parsed, err := parseRSAPublicKey(jsonWebKeys{
		Kid: testKeyID,
		N:   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
	})
	require.NoError(t, err)
	assert.True(t, parsed.Equal(&key.PublicKey))

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "trusted-caller-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	parsed, err = parseRSAPublicKey(jsonWebKeys{Kid: testKeyID, X5c: []string{base64.StdEncoding.EncodeToString(der)}})
	require.NoError(t, err)
	assert.True(t, parsed.Equal(&key.PublicKey))

	modulus := base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes())
	malformed := map[string]jsonWebKeys{
		"neither a key nor a certificate": {Kid: testKeyID},
		"malformed modulus":               {Kid: testKeyID, N: "!!!", E: "AQAB"},
		"malformed exponent":              {Kid: testKeyID, N: modulus, E: "!!!"},
		"empty exponent":                  {Kid: testKeyID, N: modulus, E: "="},
		"oversized exponent":              {Kid: testKeyID, N: modulus, E: base64.RawURLEncoding.EncodeToString([]byte{1, 2, 3, 4, 5})},
		"invalid certificate":             {Kid: testKeyID, X5c: []string{"not-a-certificate"}},
	}
	for name, webKey := range malformed {
		t.Run(name, func(t *testing.T) {
			key, keyErr := parseRSAPublicKey(webKey)
			assert.Nil(t, key)
			assert.Error(t, keyErr)
		})
	}
}
