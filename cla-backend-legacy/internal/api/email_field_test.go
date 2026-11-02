// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestEmailFieldEscapesAndBounds(t *testing.T) {
	cases := map[string]struct {
		in   string
		max  int
		want string
	}{
		"anchor":    {`<a href="https://evil.invalid">click</a>`, 60, "&lt;a href=&#34;https://evil.invalid&#34;&gt;click&lt;/a&gt;"},
		"img":       {" <img src=x onerror=alert(1)> ", 60, "&lt;img src=x onerror=alert(1)&gt;"},
		"truncated": {strings.Repeat("a", 61), 60, strings.Repeat("a", 60)},
		"multibyte": {strings.Repeat("é", 31), 61, strings.Repeat("é", 30)},
		"plain":     {"Jane Doe", 60, "Jane Doe"},
	}
	for name, tc := range cases {
		if got := emailField(tc.in, tc.max); got != tc.want {
			t.Errorf("%s: emailField(%q, %d) = %q, want %q", name, tc.in, tc.max, got, tc.want)
		}
	}
}

func TestPublicSignatureItemDropsTheDocuSignEnvelope(t *testing.T) {
	item := map[string]types.AttributeValue{
		"signature_id":          &types.AttributeValueMemberS{Value: "sig-1"},
		"signature_signed":      &types.AttributeValueMemberBOOL{Value: true},
		"user_docusign_raw_xml": &types.AttributeValueMemberS{Value: "<DocuSignEnvelopeInformation/>"},
		"signature_envelope_id": &types.AttributeValueMemberS{Value: "e1"},
	}
	out := publicSignatureItem(item)
	if out["signature_id"] != "sig-1" || out["signature_signed"] != true {
		t.Fatalf("unexpected serialization: %#v", out)
	}
	for _, secret := range []string{"user_docusign_raw_xml", "signature_envelope_id"} {
		if _, present := out[secret]; present {
			t.Fatalf("%s must never be returned to a client", secret)
		}
	}
}

// the anonymous /v2 approval request must not inject markup into the CLA manager's e-mail (lfx-self-serve-ops#352)
func TestRequestCompanyAllowlistBodyEscapesCallerFields(t *testing.T) {
	payload := `<img src=x onerror=alert(1)>`
	body := requestCompanyAllowlistBody("Boss"+payload, "Project X", "Mallory"+payload, "m@example.invalid"+payload, "Acme", "Acme Inc", "please"+payload, "https://corporate.example.invalid/c/1")
	if strings.Contains(body, payload) {
		t.Fatalf("caller-supplied markup reached the body: %s", body)
	}
	if strings.Count(body, "&lt;img src=x onerror=alert(1)&gt;") != 5 {
		t.Fatalf("every caller-supplied field must be escaped (name twice, recipient, e-mail, message): %s", body)
	}
	for _, keep := range []string{"Project X", "Acme Inc", `<a href="https://corporate.example.invalid/c/1" target="_blank">`} {
		if !strings.Contains(body, keep) {
			t.Fatalf("stored value %q must stay intact: %s", keep, body)
		}
	}
	stored := requestCompanyAllowlistBody("Boss", "Project <b>X</b>", "Mallory", "m@example.invalid", "Acme <i>Inc</i>", "id:1, name: Acme <i>Inc</i>", "hi", "https://corporate.example.invalid/c/1")
	if strings.Contains(stored, "<b>") || strings.Contains(stored, "<i>") || !strings.Contains(stored, "Project &lt;b&gt;X&lt;/b&gt;") {
		t.Fatalf("stored project/company values must be escaped too: %s", stored)
	}
}
