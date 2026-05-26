// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package githublegacy

import (
	"crypto/hmac"
	"crypto/sha1" // #nosec G505 -- GitHub's legacy X-Hub-Signature webhook format uses SHA-1.
	"encoding/hex"
	"errors"
	"os"
	"strings"
)

// ValidateWebhookSignature validates GitHub webhook payload signatures.
//
// Legacy Python uses X-HUB-SIGNATURE with the format: "sha1=<hex>".
func ValidateWebhookSignature(payload []byte, signatureHeader string) (bool, error) {
	secret := strings.TrimSpace(os.Getenv("GITHUB_APP_WEBHOOK_SECRET"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("GH_APP_WEBHOOK_SECRET"))
	}
	if secret == "" {
		return false, errors.New("GITHUB_APP_WEBHOOK_SECRET is empty")
	}

	signatureHeader = strings.TrimSpace(signatureHeader)
	if signatureHeader == "" {
		return false, nil
	}
	parts := strings.SplitN(signatureHeader, "=", 2)
	if len(parts) != 2 {
		return false, nil
	}
	shaName := parts[0]
	sig := parts[1]
	if shaName != "sha1" {
		return false, nil
	}

	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write(payload)
	expected := mac.Sum(nil)

	got, err := hex.DecodeString(strings.TrimSpace(sig))
	if err != nil {
		return false, nil
	}
	return hmac.Equal(expected, got), nil
}

// SignWebhookPayload signs a payload using the same SHA1 GitHub App webhook
// secret format used by the legacy Python webhook handler.
func SignWebhookPayload(payload []byte) (string, error) {
	secret := strings.TrimSpace(os.Getenv("GITHUB_APP_WEBHOOK_SECRET"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("GH_APP_WEBHOOK_SECRET"))
	}
	if secret == "" {
		return "", errors.New("GITHUB_APP_WEBHOOK_SECRET is empty")
	}

	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write(payload)
	return "sha1=" + hex.EncodeToString(mac.Sum(nil)), nil
}
