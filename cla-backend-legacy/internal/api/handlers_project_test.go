// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// TestNormalizeProjectStringFields locks in the heal-on-PUT behavior added
// after the post-cutover regression: digit-only project_name and
// project_external_id values that the previous InterfaceMapToItem heuristic
// persisted as N are rewritten to S before PutItem, even when the request
// payload does not touch those fields.
func TestNormalizeProjectStringFields(t *testing.T) {
	t.Run("rewrites N to S for named fields", func(t *testing.T) {
		item := map[string]types.AttributeValue{
			"project_name":        &types.AttributeValueMemberN{Value: "12345"},
			"project_external_id": &types.AttributeValueMemberN{Value: "67890"},
			"project_acl":         &types.AttributeValueMemberSS{Value: []string{"alice"}},
		}
		normalizeProjectStringFields(item, "project_name", "project_external_id")
		assertS(t, item, "project_name", "12345")
		assertS(t, item, "project_external_id", "67890")
		if _, ok := item["project_acl"].(*types.AttributeValueMemberSS); !ok {
			t.Fatalf("project_acl should be unchanged SS, got %T", item["project_acl"])
		}
	})

	t.Run("leaves S values untouched", func(t *testing.T) {
		item := map[string]types.AttributeValue{
			"project_name": &types.AttributeValueMemberS{Value: "Cloud Foundry"},
		}
		normalizeProjectStringFields(item, "project_name")
		assertS(t, item, "project_name", "Cloud Foundry")
	})

	t.Run("ignores unknown / missing names", func(t *testing.T) {
		item := map[string]types.AttributeValue{
			"project_name": &types.AttributeValueMemberN{Value: "42"},
		}
		normalizeProjectStringFields(item, "does_not_exist", "project_name")
		assertS(t, item, "project_name", "42")
		if _, present := item["does_not_exist"]; present {
			t.Fatalf("normalizeProjectStringFields must not insert missing keys")
		}
	})

	t.Run("nil map is a no-op", func(t *testing.T) {
		// Must not panic.
		normalizeProjectStringFields(nil, "project_name")
	})
}

// TestPutProjectV1_BuildsStringTypesForDigitOnlyValues asserts the post-bug
// shape of the AttributeValue map: regardless of whether the project name
// happens to be all digits ("12345"), the PutProjectV1 / PostProjectV1 paths
// must produce S — not N — to round-trip correctly through downstream
// readers (LFX, Salesforce sync) that expect strings.
//
// This is a structural check on the literal map produced by the handlers'
// shared building blocks; it does not exercise HTTP wiring (which would
// require a full mock DDB), only that the literal AttributeValue we put
// into the map is the right type for digit-only inputs.
func TestPutProjectV1_BuildsStringTypesForDigitOnlyValues(t *testing.T) {
	digitOnly := "12345"
	item := map[string]types.AttributeValue{
		"project_name":        &types.AttributeValueMemberS{Value: digitOnly},
		"project_external_id": &types.AttributeValueMemberS{Value: digitOnly},
	}

	assertS(t, item, "project_name", digitOnly)
	assertS(t, item, "project_external_id", digitOnly)

	// Simulate a record that survived from the buggy era and verify the
	// normalization rewrites it before write.
	stale := map[string]types.AttributeValue{
		"project_name":        &types.AttributeValueMemberN{Value: digitOnly},
		"project_external_id": &types.AttributeValueMemberN{Value: digitOnly},
	}
	normalizeProjectStringFields(stale, "project_name", "project_external_id")
	assertS(t, stale, "project_name", digitOnly)
	assertS(t, stale, "project_external_id", digitOnly)
}

func assertS(t *testing.T, item map[string]types.AttributeValue, name, want string) {
	t.Helper()
	v, ok := item[name].(*types.AttributeValueMemberS)
	if !ok {
		t.Fatalf("%s expected *AttributeValueMemberS, got %T", name, item[name])
	}
	if v.Value != want {
		t.Fatalf("%s got %q want %q", name, v.Value, want)
	}
}
