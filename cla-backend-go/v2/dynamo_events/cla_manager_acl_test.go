// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package dynamo_events

import (
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// a mixed ACL edit adds only the added manager and removes only the removed one (lfx-self-serve-ops#346 item 3)
func TestUpdateCLAPermissionsAppliesEachACLEntrySeparately(t *testing.T) {
	type call struct {
		managers []string
		action   string
	}
	var calls []call
	previous := claManagerPermissionUpdater
	claManagerPermissionUpdater = func(_ *service, _ Signature, managers []string, action string) error {
		calls = append(calls, call{managers: append([]string(nil), managers...), action: action})
		return nil
	}
	defer func() { claManagerPermissionUpdater = previous }()

	image := func(acl []string) map[string]events.DynamoDBAttributeValue {
		return map[string]events.DynamoDBAttributeValue{
			"signature_id":             events.NewStringAttribute("sig-ccla"),
			"signature_type":           events.NewStringAttribute("ccla"),
			"signature_reference_type": events.NewStringAttribute("company"),
			"signature_reference_id":   events.NewStringAttribute("company-1"),
			"signature_project_id":     events.NewStringAttribute("cla-group-1"),
			"signature_acl":            events.NewStringSetAttribute(acl),
		}
	}
	s := &service{}
	err := s.UpdateCLAPermissions(events.DynamoDBEventRecord{Change: events.DynamoDBStreamRecord{OldImage: image([]string{"keep", "removed"}), NewImage: image([]string{"keep", "added"})}})
	require.NoError(t, err)
	assert.ElementsMatch(t, []call{{managers: []string{"added"}, action: AddCLAManager}, {managers: []string{"removed"}, action: DeleteCLAManager}}, calls, "every ACL difference is applied on its own, exactly once")
}

// a failing entry is reported but never stops the remaining entries (mlehotskylf review on #5253)
func TestUpdateCLAPermissionsAppliesEveryEntryDespiteAFailure(t *testing.T) {
	var seen []string
	previous := claManagerPermissionUpdater
	claManagerPermissionUpdater = func(_ *service, _ Signature, managers []string, _ string) error {
		seen = append(seen, managers...)
		if managers[0] == "removed-first" {
			return errors.New("no grant for removed-first")
		}
		return nil
	}
	defer func() { claManagerPermissionUpdater = previous }()

	image := func(acl []string) map[string]events.DynamoDBAttributeValue {
		return map[string]events.DynamoDBAttributeValue{
			"signature_id":             events.NewStringAttribute("sig-ccla"),
			"signature_type":           events.NewStringAttribute("ccla"),
			"signature_reference_type": events.NewStringAttribute("company"),
			"signature_reference_id":   events.NewStringAttribute("company-1"),
			"signature_project_id":     events.NewStringAttribute("cla-group-1"),
			"signature_acl":            events.NewStringSetAttribute(acl),
		}
	}
	s := &service{}
	err := s.UpdateCLAPermissions(events.DynamoDBEventRecord{Change: events.DynamoDBStreamRecord{OldImage: image([]string{"keep", "removed-first", "removed-second"}), NewImage: image([]string{"keep", "added"})}})
	require.ErrorContains(t, err, "no grant for removed-first")
	assert.ElementsMatch(t, []string{"removed-first", "removed-second", "added"}, seen, "the entries after the failing one are still applied")
}
