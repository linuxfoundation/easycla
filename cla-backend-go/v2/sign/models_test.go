// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package sign

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDocuSignEventNotificationUsesRecipientEvents(t *testing.T) {
	notification := DocuSignEventNotification{
		URL:            "https://example.com/v4/signed/individual/1/2/3",
		LoggingEnabled: true,
		RecipientEvents: []DocuSignRecipientEvent{
			{RecipientEventStatusCode: "Completed"},
		},
	}

	data, err := json.Marshal(notification)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	payload := string(data)
	if !strings.Contains(payload, `"recipientEvents":[{"recipientEventStatusCode":"Completed"}]`) {
		t.Fatalf("expected DocuSign recipient completed event in payload, got: %s", payload)
	}
	if strings.Contains(payload, "envelopeEvents") || strings.Contains(payload, "envelopeEventStatusCode") {
		t.Fatalf("unexpected envelope event fields in payload: %s", payload)
	}
}
