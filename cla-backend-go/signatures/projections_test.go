// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// the DocuSign envelope id is the completion callbacks' only binding secret: API reads must never carry it (lfx-self-serve-ops#347)
func TestSignatureProjectionNeverCarriesTheEnvelopeID(t *testing.T) {
	for name, projection := range map[string]expression.ProjectionBuilder{"signature": buildProjection(), "invalidation-aware": buildInvalidationAwareProjection()} {
		expr, err := expression.NewBuilder().WithProjection(projection).Build()
		require.NoError(t, err)
		for _, n := range expr.Names() {
			assert.NotEqual(t, "signature_envelope_id", *n, name)
		}
	}
}
