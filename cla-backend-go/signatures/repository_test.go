// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInvalidationUpdateExpression(t *testing.T) {
	const now = "2024-05-06T07:08:09.000000+0000"

	names, values, expr := invalidationUpdateExpression("a note", now, &InvalidationMetadata{
		InvalidatedBy: "admin-user",
		Reason:        "compliance",
		Note:          "per legal review",
	})

	assert.Contains(t, expr, "#A = :a")
	assert.Contains(t, expr, "#S = :s")
	assert.Contains(t, expr, "#DI = if_not_exists(#DI, :di)")
	assert.Contains(t, expr, "#IB = if_not_exists(#IB, :ib)", "a re-invalidation must not overwrite the first actor")
	assert.Contains(t, expr, "#IR = if_not_exists(#IR, :ir)", "a re-invalidation must not overwrite the first reason")
	assert.Contains(t, expr, "#IN = if_not_exists(#IN, :in)", "a re-invalidation must not overwrite the first note")
	assert.Contains(t, expr, "#M = :m")

	assert.Equal(t, "invalidated_by", *names["#IB"])
	assert.Equal(t, "invalidation_reason", *names["#IR"])
	assert.Equal(t, "invalidation_note", *names["#IN"])
	assert.Equal(t, "admin-user", *values[":ib"].S)
	assert.Equal(t, "compliance", *values[":ir"].S)
	assert.Equal(t, "per legal review", *values[":in"].S)
	assert.Equal(t, now, *values[":di"].S)
	assert.False(t, *values[":a"].BOOL)
}

func TestInvalidationUpdateExpressionWithoutMetadata(t *testing.T) {
	const now = "2024-05-06T07:08:09.000000+0000"

	for _, metadata := range []*InvalidationMetadata{nil, {}} {
		names, values, expr := invalidationUpdateExpression("a note", now, metadata)

		assert.NotContains(t, expr, "#IB")
		assert.NotContains(t, expr, "#IR")
		assert.NotContains(t, expr, "#IN")
		assert.Contains(t, expr, "#DI = if_not_exists(#DI, :di)")
		assert.NotContains(t, names, "#IB")
		assert.NotContains(t, values, ":ib")
	}
}
