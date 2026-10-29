// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api

import (
	"strings"
	"testing"
)

// the signature-cancel page renders the stored return URL inside an href, so it goes through html/template's
// contextual escaping (ops#829, ops#992): a javascript: URL is neutralised and markup in the title is escaped,
// while a normal https return URL and the sign URL keep working
func TestCanceledSignaturePageEscapesStoredValues(t *testing.T) {
	type data struct {
		SignatureTypeTitle string
		SignURL            string
		ReturnURL          string
	}
	render := func(d data) string {
		var sb strings.Builder
		if err := canceledSignatureHTML.Execute(&sb, d); err != nil {
			t.Fatalf("render: %v", err)
		}
		return sb.String()
	}

	hostile := render(data{
		SignatureTypeTitle: `Ccla<script>alert(1)</script>`,
		SignURL:            "https://sign.example/envelope?e=1&u=2",
		ReturnURL:          `javascript:alert(document.cookie)`,
	})
	if strings.Contains(hostile, "<script>alert(1)</script>") || !strings.Contains(hostile, "Ccla&lt;script&gt;alert(1)&lt;/script&gt; CLA.") {
		t.Errorf("title not escaped:\n%s", hostile)
	}
	if strings.Contains(hostile, `href="javascript:`) || !strings.Contains(hostile, `href="#ZgotmplZ"`) {
		t.Errorf("javascript: return URL not neutralised:\n%s", hostile)
	}
	if !strings.Contains(hostile, `href="https://sign.example/envelope?e=1&amp;u=2"`) {
		t.Errorf("sign URL not preserved as an escaped attribute:\n%s", hostile)
	}

	benign := render(data{SignatureTypeTitle: "Icla", SignURL: "https://sign.example/s", ReturnURL: "https://github.com/org/repo/pull/7?x=1&y=2#review"})
	for _, want := range []string{`href="https://sign.example/s"`, `href="https://github.com/org/repo/pull/7?x=1&amp;y=2#review"`, "Restart Authorization", "Icla CLA."} {
		if !strings.Contains(benign, want) {
			t.Errorf("benign page lacks %q:\n%s", want, benign)
		}
	}

	// the stored https return URL tries to break out of the quoted href (the ops#829 / ops#992 shape)
	breakout := render(data{SignatureTypeTitle: "Icla", SignURL: "https://sign.example/s", ReturnURL: `https://github.com/org/repo/pull/7?x=1" onmouseover="alert(1)#frag`})
	if strings.Contains(breakout, `onmouseover="alert(1)"`) || strings.Contains(breakout, ` onmouseover=`) {
		t.Errorf("attribute breakout not neutralised:\n%s", breakout)
	}
	if !strings.Contains(breakout, `href="https://github.com/org/repo/pull/7?x=1%22%20onmouseover=%22alert%281%29#frag"`) {
		t.Errorf("breakout quote not percent-encoded inside the attribute:\n%s", breakout)
	}
	if n := strings.Count(breakout, "<a "); n != 2 {
		t.Errorf("expected exactly 2 anchors (sign + return), got %d:\n%s", n, breakout)
	}

	empty := render(data{SignatureTypeTitle: "Icla", SignURL: "https://sign.example/s", ReturnURL: ""})
	if strings.Contains(empty, "Restart Authorization") || strings.Count(empty, "<a ") != 1 {
		t.Errorf("empty return URL must render only the sign button:\n%s", empty)
	}
}
