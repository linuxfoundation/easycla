// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
)

const (
	bearerPrefix          = "bearer "
	defaultJWTAlgorithm   = "RS256"
	jwksCacheTTL          = 15 * time.Minute
	jwksRefreshCooldown   = time.Minute
	jwksStaleKeyGrace     = 24 * time.Hour
	jwksResponseSizeLimit = 1 << 20
	jwksRequestTimeout    = 10 * time.Second
)

// ErrNoBearerToken is returned when the request carries no parseable bearer token. An absent
// header must be denied like an invalid one: the traefik aws-lambda middleware drops duplicated
// headers, so a duplicated Authorization header reaches the lambda as an absent one.
var ErrNoBearerToken = errors.New("no bearer token in the Authorization header")

// TrustedCaller is the OAuth2 client (azp) and subject of a signature-verified bearer token
type TrustedCaller struct {
	ClientID string
	Subject  string
	Trusted  bool
}

// TrustedCallerVerifier verifies bearer tokens against a single Auth0 tenant's JWKS and reports
// whether the token's azp claim names an allow-listed client
type TrustedCallerVerifier struct {
	wellKnownURL     string
	algorithm        string
	allowedClientIDs map[string]bool

	mu            sync.Mutex
	keys          map[string]*rsa.PublicKey
	keysExpireAt  time.Time
	nextRefreshAt time.Time

	fetchKeys func() (map[string]*rsa.PublicKey, error)
	now       func() time.Time
}

// NewTrustedCallerVerifier creates a verifier for the Auth0 tenant at the given domain that
// trusts the given client IDs. An empty allow-list yields a disabled verifier (see Enabled).
func NewTrustedCallerVerifier(domain, algorithm string, allowedClientIDs []string) (*TrustedCallerVerifier, error) {
	allowed := make(map[string]bool, len(allowedClientIDs))
	for _, clientID := range allowedClientIDs {
		if clientID = strings.TrimSpace(clientID); clientID != "" {
			allowed[clientID] = true
		}
	}
	if len(allowed) > 0 && domain == "" {
		return nil, errors.New("missing Domain for the trusted caller verifier")
	}
	if algorithm == "" {
		algorithm = defaultJWTAlgorithm
	}

	verifier := &TrustedCallerVerifier{
		wellKnownURL:     "https://" + path.Join(domain, ".well-known/jwks.json"),
		algorithm:        algorithm,
		allowedClientIDs: allowed,
		now:              time.Now,
	}
	verifier.fetchKeys = verifier.fetchJWKS
	return verifier, nil
}

// Enabled reports whether any trusted client ID is configured
func (v *TrustedCallerVerifier) Enabled() bool {
	return v != nil && len(v.allowedClientIDs) > 0
}

// Verify signature-verifies the bearer token in the Authorization header against the tenant JWKS
// and reports the client (azp) and subject it was issued to
func (v *TrustedCallerVerifier) Verify(authorization string) (*TrustedCaller, error) {
	rawToken, err := bearerToken(authorization)
	if err != nil {
		return nil, err
	}

	claims := jwt.MapClaims{}
	parser := jwt.NewParser(jwt.WithValidMethods([]string{v.algorithm}))
	token, err := parser.ParseWithClaims(rawToken, claims, v.signingKey)
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid bearer token")
	}
	// jwt.MapClaims treats exp as optional
	if _, ok := claims["exp"]; !ok {
		return nil, errors.New("bearer token carries no expiration")
	}
	clientID := stringClaim(claims, "azp")

	// Matching azp against an allow-list is sound ONLY because LFX Self Serve is a confidential
	// backend client: its tokens are minted server-side with a client secret and never reach a
	// browser, so an allow-listed azp means the SS backend built this request. If that client ID
	// is ever reused by a public/SPA client - or its tokens otherwise become user-visible - this
	// boundary silently collapses and any user could pass any identity.
	//
	// Transitional mechanism (P3/P9 of the trust-SS decision): at M6, once EasyCLA runs on the
	// K8s cluster, it should call lfx.auth-service.user_identity.list itself over NATS and drop
	// both this azp check and the caller-supplied identity list it authorizes.
	return &TrustedCaller{
		ClientID: clientID,
		Subject:  stringClaim(claims, "sub"),
		Trusted:  clientID != "" && v.allowedClientIDs[clientID],
	}, nil
}

func stringClaim(claims jwt.MapClaims, name string) string {
	value, ok := claims[name].(string)
	if !ok {
		return ""
	}
	return value
}

func bearerToken(authorization string) (string, error) {
	value := strings.TrimSpace(authorization)
	if len(value) <= len(bearerPrefix) || !strings.EqualFold(value[:len(bearerPrefix)], bearerPrefix) {
		return "", ErrNoBearerToken
	}
	rawToken := strings.TrimSpace(value[len(bearerPrefix):])
	if rawToken == "" {
		return "", ErrNoBearerToken
	}
	return rawToken, nil
}

func (v *TrustedCallerVerifier) signingKey(token *jwt.Token) (interface{}, error) {
	kid, ok := token.Header["kid"].(string)
	if !ok || kid == "" {
		return nil, errors.New("bearer token carries no key ID")
	}
	return v.publicKey(kid)
}

func (v *TrustedCallerVerifier) publicKey(kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := v.now()
	key, cached := v.keys[kid]
	if cached && now.Before(v.keysExpireAt) {
		return key, nil
	}
	usableWhileStale := cached && now.Before(v.keysExpireAt.Add(jwksStaleKeyGrace))

	// a cache miss means a tenant key rotation or a JWKS outage, so rate limit the reload it
	// triggers - without this an outage costs every request a fetch timeout once the TTL expires
	if now.Before(v.nextRefreshAt) {
		if usableWhileStale {
			return key, nil
		}
		return nil, fmt.Errorf("unknown signing key ID: %s - the JWKS reload is rate limited", kid)
	}

	v.nextRefreshAt = now.Add(jwksRefreshCooldown)
	keys, err := v.fetchKeys()
	if err != nil {
		// bounded, so a revoked key cannot stay usable for the whole length of a JWKS outage
		if usableWhileStale {
			log.WithError(err).Warn("unable to refresh the JWKS - using the cached signing key")
			return key, nil
		}
		return nil, err
	}
	v.keys = keys
	v.keysExpireAt = now.Add(jwksCacheTTL)

	refreshed, ok := keys[kid]
	if !ok {
		return nil, fmt.Errorf("unknown signing key ID: %s", kid)
	}
	return refreshed, nil
}

func (v *TrustedCallerVerifier) fetchJWKS() (map[string]*rsa.PublicKey, error) {
	client := &http.Client{Timeout: jwksRequestTimeout}
	resp, err := client.Get(v.wellKnownURL) // nolint
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.WithError(closeErr).Warn("problem closing the JWKS response body")
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unable to load the JWKS from %s - status code: %d", v.wellKnownURL, resp.StatusCode)
	}

	var keySet jwks
	if err := json.NewDecoder(io.LimitReader(resp.Body, jwksResponseSizeLimit)).Decode(&keySet); err != nil {
		return nil, err
	}

	keys := make(map[string]*rsa.PublicKey, len(keySet.Keys))
	for _, webKey := range keySet.Keys {
		if webKey.Kid == "" || (webKey.Kty != "" && webKey.Kty != "RSA") || (webKey.Use != "" && webKey.Use != "sig") {
			continue
		}
		key, keyErr := parseRSAPublicKey(webKey)
		if keyErr != nil {
			log.WithError(keyErr).Warnf("unable to parse the JWKS signing key: %s", webKey.Kid)
			continue
		}
		keys[webKey.Kid] = key
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no usable RSA signing key at %s", v.wellKnownURL)
	}
	return keys, nil
}

func parseRSAPublicKey(webKey jsonWebKeys) (*rsa.PublicKey, error) {
	if webKey.N != "" && webKey.E != "" {
		modulus, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(webKey.N, "="))
		if err != nil {
			return nil, err
		}
		exponent, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(webKey.E, "="))
		if err != nil {
			return nil, err
		}
		if len(modulus) == 0 || len(exponent) == 0 || len(exponent) > 4 {
			return nil, errors.New("key carries a malformed modulus or exponent")
		}
		return &rsa.PublicKey{
			N: new(big.Int).SetBytes(modulus),
			E: int(new(big.Int).SetBytes(exponent).Int64()),
		}, nil
	}
	if len(webKey.X5c) == 0 {
		return nil, errors.New("key carries neither a modulus/exponent pair nor a certificate")
	}
	return jwt.ParseRSAPublicKeyFromPEM([]byte("-----BEGIN CERTIFICATE-----\n" + webKey.X5c[0] + "\n-----END CERTIFICATE-----"))
}
