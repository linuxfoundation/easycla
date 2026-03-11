package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// AuthError mirrors the legacy Python behavior where handlers return the `response`
// field directly as JSON (sometimes a string, sometimes an object).
type AuthError struct {
	Response any
}

func (e *AuthError) Error() string {
	if e == nil {
		return "auth error"
	}
	return fmt.Sprintf("auth error: %v", e.Response)
}

type AuthUser struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Sub      string `json:"sub"`
}

type jwksResponse struct {
	Keys []struct {
		Kty string `json:"kty"`
		Kid string `json:"kid"`
		Use string `json:"use"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

// Auth0Validator validates user access tokens against Auth0 JWKS.
//
// Environment variables expected (same as legacy Python):
//   - AUTH0_DOMAIN
//   - AUTH0_ALGORITHM
//   - AUTH0_USERNAME_CLAIM
//   - AUTH0_USERNAME_CLAIM_CLI (optional fallback)
type Auth0Validator struct {
	Domain           string
	Algorithm        string
	UsernameClaim    string
	UsernameClaimAlt string
	httpClient       *http.Client
}

func NewAuth0ValidatorFromEnv(httpClient *http.Client) *Auth0Validator {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Auth0Validator{
		Domain:           strings.TrimSpace(os.Getenv("AUTH0_DOMAIN")),
		Algorithm:        strings.TrimSpace(os.Getenv("AUTH0_ALGORITHM")),
		UsernameClaim:    strings.TrimSpace(os.Getenv("AUTH0_USERNAME_CLAIM")),
		UsernameClaimAlt: strings.TrimSpace(os.Getenv("AUTH0_USERNAME_CLAIM_CLI")),
		httpClient:       httpClient,
	}
}

func GetBearerToken(headers http.Header) (string, *AuthError) {
	auth := strings.TrimSpace(headers.Get("Authorization"))
	if auth == "" {
		auth = strings.TrimSpace(headers.Get("AUTHORIZATION"))
	}
	if auth == "" {
		return "", &AuthError{Response: "missing authorization header"}
	}

	parts := strings.Fields(auth)
	if len(parts) == 0 {
		return "", &AuthError{Response: "missing authorization header"}
	}
	if strings.ToLower(parts[0]) != "bearer" {
		return "", &AuthError{Response: "authorization header must begin with \"Bearer\""}
	}
	if len(parts) == 1 {
		return "", &AuthError{Response: "token not found"}
	}
	if len(parts) > 2 {
		return "", &AuthError{Response: "authorization header must be of the form \"Bearer token\""}
	}
	return parts[1], nil
}

// Authenticate matches legacy handler expectations:
//   - user: decoded Auth0 user
//   - errResp: response payload mirroring the Python implementation
//   - err: non-nil when authentication failed
//
// This wrapper keeps the internal implementation returning *AuthError while also
// providing the handler-friendly (payload, error) split.
func (v *Auth0Validator) Authenticate(headers http.Header) (*AuthUser, any, error) {
	user, aerr := v.authenticate(headers)
	if aerr != nil {
		return nil, aerr.Response, aerr
	}
	return user, nil, nil
}

func (v *Auth0Validator) authenticate(headers http.Header) (*AuthUser, *AuthError) {
	if v == nil {
		return nil, &AuthError{Response: "auth validator not configured"}
	}
	if v.Domain == "" {
		return nil, &AuthError{Response: "AUTH0_DOMAIN is empty"}
	}
	if v.UsernameClaim == "" {
		return nil, &AuthError{Response: "AUTH0_USERNAME_CLAIM is empty"}
	}
	// Default to RS256 if not set. The Python code reads this from env and passes
	// it through; in practice it's RS256.
	if v.Algorithm == "" {
		v.Algorithm = "RS256"
	}

	tokenString, aerr := GetBearerToken(headers)
	if aerr != nil {
		return nil, aerr
	}

	jwks, aerr := v.fetchJWKS()
	if aerr != nil {
		return nil, aerr
	}

	parsed, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if t == nil {
			return nil, errors.New("token is nil")
		}
		if alg := t.Method.Alg(); alg != v.Algorithm {
			return nil, fmt.Errorf("unexpected signing method: %s", alg)
		}

		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("kid not found")
		}
		pk, ok := jwksKeyToRSAPublicKey(jwks, kid)
		if !ok {
			return nil, errNoMatchingKey
		}
		return pk, nil
	})
	if err != nil {
		if errors.Is(err, errNoMatchingKey) {
			// Mirror Python's AuthError payload.
			return nil, &AuthError{Response: map[string]any{"code": "invalid_header", "description": "Unable to find appropriate key"}}
		}

		// Try to map validation errors to legacy Python strings.
		var ve *jwt.ValidationError
		if errors.As(err, &ve) {
			if ve.Errors&jwt.ValidationErrorExpired != 0 {
				return nil, &AuthError{Response: "token is expired"}
			}
			if ve.Errors&jwt.ValidationErrorClaimsInvalid != 0 {
				return nil, &AuthError{Response: "incorrect claims"}
			}
		}

		return nil, &AuthError{Response: "unable to parse authentication"}
	}
	if parsed == nil || !parsed.Valid {
		return nil, &AuthError{Response: "unable to parse authentication"}
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, &AuthError{Response: "unable to decode claims"}
	}

	username := claimString(claims, v.UsernameClaim)
	if username == "" && v.UsernameClaimAlt != "" {
		username = claimString(claims, v.UsernameClaimAlt)
	}
	if username == "" {
		return nil, &AuthError{Response: "username claim not found"}
	}

	user := &AuthUser{
		Name:     claimString(claims, "name"),
		Email:    claimString(claims, "email"),
		Username: username,
		Sub:      claimString(claims, "sub"),
	}
	return user, nil
}

var errNoMatchingKey = errors.New("no matching jwks key")

func (v *Auth0Validator) fetchJWKS() (*jwksResponse, *AuthError) {
	url := "https://" + strings.TrimSuffix(v.Domain, "/") + "/.well-known/jwks.json"
	req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, &AuthError{Response: "unable to fetch well known jwks"}
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, &AuthError{Response: "unable to fetch well known jwks"}
	}
	defer resp.Body.Close()

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, &AuthError{Response: "unable to fetch well known jwks"}
	}
	return &jwks, nil
}

func jwksKeyToRSAPublicKey(jwks *jwksResponse, kid string) (*rsa.PublicKey, bool) {
	if jwks == nil {
		return nil, false
	}
	for _, k := range jwks.Keys {
		if k.Kid != kid {
			continue
		}
		// Convert base64url encoded modulus and exponent.
		nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			return nil, false
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			return nil, false
		}
		n := new(big.Int).SetBytes(nBytes)
		e := new(big.Int).SetBytes(eBytes)
		if n.Sign() <= 0 || e.Sign() <= 0 {
			return nil, false
		}
		return &rsa.PublicKey{N: n, E: int(e.Int64())}, true
	}
	return nil, false
}

func claimString(claims jwt.MapClaims, key string) string {
	v, ok := claims[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}
