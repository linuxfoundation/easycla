// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package store

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// TestBuildSanctionUpdate locks in the sanctioned_date semantics: the date is stamped on every
// flag-set and never touched when the flag is cleared.
func TestBuildSanctionUpdate(t *testing.T) {
	const now = "2026-08-20T10:11:12.000000+0000"

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
			origin:      "sss",
			expression:  "SET #S = :s, #M = :m, #D = :d, #O = :o",
			condition:   "attribute_not_exists(#S) OR #S = :false OR #O = :o",
			stampedDate: true,
		},
		{
			name:       "sss clears the company",
			sanctioned: false,
			origin:     "sss",
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

			if update.expression != tc.expression {
				t.Fatalf("expression = %q, want %q", update.expression, tc.expression)
			}
			switch {
			case tc.condition == "" && update.condition != nil:
				t.Fatalf("condition = %q, want none: only an SSS-set flag is conditional", *update.condition)
			case tc.condition != "" && update.condition == nil:
				t.Fatal("missing condition: the manual/admin block must stay protected")
			case tc.condition != "" && *update.condition != tc.condition:
				t.Fatalf("condition = %q, want %q", *update.condition, tc.condition)
			}

			_, hasName := update.names["#D"]
			date, hasValue := update.values[":d"]
			if hasName != tc.stampedDate || hasValue != tc.stampedDate {
				t.Fatalf("sanctioned_date stamped = %v/%v, want %v", hasName, hasValue, tc.stampedDate)
			}
			if tc.stampedDate {
				if update.names["#D"] != "sanctioned_date" {
					t.Fatalf("#D = %q, want sanctioned_date", update.names["#D"])
				}
				if got := date.(*types.AttributeValueMemberS).Value; got != now {
					t.Fatalf("sanctioned_date = %q, want %q: stamped with the same time as the flag", got, now)
				}
			}

			// Every declared name and value has to be referenced, or DynamoDB rejects the update.
			full := update.expression
			if update.condition != nil {
				full += " " + *update.condition
			}
			for name := range update.names {
				if !strings.Contains(full, name) {
					t.Errorf("name %s declared but never referenced", name)
				}
			}
			for value := range update.values {
				if !strings.Contains(full, value) {
					t.Errorf("value %s declared but never referenced", value)
				}
			}
		})
	}
}

func companyRow(id, name, entity, created string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"company_id":          &types.AttributeValueMemberS{Value: id},
		"company_name":        &types.AttributeValueMemberS{Value: name},
		"signing_entity_name": &types.AttributeValueMemberS{Value: entity},
		"date_created":        &types.AttributeValueMemberS{Value: created},
	}
}

func TestPickParentCompany(t *testing.T) {
	parent := companyRow("b", "Acme", "Acme", "2021-01-01T00:00:00Z")
	bare := companyRow("c", "Acme", "", "2022-01-01T00:00:00Z")
	entity := companyRow("a", "Acme", "Acme Labs", "2020-01-01T00:00:00Z")
	entity2 := companyRow("d", "Acme", "Acme GmbH", "2020-06-01T00:00:00Z")

	if got := PickParentCompany(nil); got != nil {
		t.Fatalf("expected nil for no rows, got %v", got)
	}
	if got := attrString(PickParentCompany([]map[string]types.AttributeValue{entity, parent, bare}), "company_id"); got != "b" {
		t.Fatalf("expected the oldest parent-like row b, got %s", got)
	}
	if got := attrString(PickParentCompany([]map[string]types.AttributeValue{entity2, entity}), "company_id"); got != "a" {
		t.Fatalf("expected the oldest signing-entity row a when no parent exists, got %s", got)
	}
	tie := companyRow("a", "Acme", "", "2021-01-01T00:00:00Z")
	if got := attrString(PickParentCompany([]map[string]types.AttributeValue{parent, tie}), "company_id"); got != "a" {
		t.Fatalf("expected the smallest company_id on a date tie, got %s", got)
	}
	pynamo := companyRow("e", "Acme", "", "2021-01-01T00:00:00.100000+0000")
	if got := attrString(PickParentCompany([]map[string]types.AttributeValue{pynamo, parent}), "company_id"); got != "b" {
		t.Fatalf("expected the older RFC3339 row b over the later pynamo-formatted row, got %s", got)
	}
	sameInstant := companyRow("f", "Acme", "", "2021-01-01T00:00:00.000000+0000")
	if got := attrString(PickParentCompany([]map[string]types.AttributeValue{sameInstant, parent}), "company_id"); got != "b" {
		t.Fatalf("expected the smallest company_id when differently formatted dates are the same instant, got %s", got)
	}
}
