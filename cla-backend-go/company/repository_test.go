// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSanctionUpdate(t *testing.T) {
	const now = "2026-08-20T10:11:12Z"

	tests := []struct {
		name        string
		sanctioned  bool
		origin      string
		expression  string
		condition   string
		stampedDate bool
	}{
		{
			name:        "sss flags the company",
			sanctioned:  true,
			origin:      sanctionOriginSSS,
			expression:  "SET #S = :s, #M = :m, #D = :d, #O = :o",
			condition:   "attribute_not_exists(#S) OR #S = :false OR #O = :o",
			stampedDate: true,
		},
		{
			name:       "sss clears the company",
			sanctioned: false,
			origin:     sanctionOriginSSS,
			expression: "SET #S = :s, #M = :m, #O = :o",
		},
		{
			name:        "admin flags the company",
			sanctioned:  true,
			origin:      "",
			expression:  "SET #S = :s, #M = :m, #D = :d REMOVE #O",
			stampedDate: true,
		},
		{
			name:       "admin clears the company",
			sanctioned: false,
			origin:     "",
			expression: "SET #S = :s, #M = :m REMOVE #O",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			update := buildSanctionUpdate(tc.sanctioned, tc.origin, now)

			assert.Equal(t, tc.expression, update.expression)
			if tc.condition == "" {
				assert.Nil(t, update.condition, "only an SSS-set flag is conditional")
			} else {
				require.NotNil(t, update.condition)
				assert.Equal(t, tc.condition, *update.condition, "the manual/admin block must stay protected")
			}

			if tc.stampedDate {
				require.Contains(t, update.names, "#D")
				assert.Equal(t, "sanctioned_date", *update.names["#D"])
				require.Contains(t, update.values, ":d")
				assert.Equal(t, now, *update.values[":d"].S, "the flag and the date are stamped with the same time")
			} else {
				assert.NotContains(t, update.names, "#D", "clearing the flag leaves the stored date alone")
				assert.NotContains(t, update.values, ":d")
			}

			assert.Equal(t, tc.sanctioned, *update.values[":s"].BOOL)
			assert.Equal(t, now, *update.values[":m"].S)

			// Every declared name and value has to be referenced, or DynamoDB rejects the update.
			for name := range update.names {
				assert.Contains(t, update.expression+condition(update), name)
			}
			for value := range update.values {
				assert.Contains(t, update.expression+condition(update), value)
			}
		})
	}
}

func condition(update sanctionUpdate) string {
	if update.condition == nil {
		return ""
	}
	return " " + *update.condition
}
