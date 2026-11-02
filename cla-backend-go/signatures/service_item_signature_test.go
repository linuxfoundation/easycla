// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type consistencyProbeRepo struct {
	SignatureRepository
	row        *ItemSignature
	consistent int
	eventual   int
}

func (r *consistencyProbeRepo) GetItemSignatureConsistent(_ context.Context, _ string) (*ItemSignature, error) {
	r.consistent++
	return r.row, nil
}

func (r *consistencyProbeRepo) GetItemSignature(_ context.Context, _ string) (*ItemSignature, error) {
	r.eventual++
	return nil, errors.New("eventually consistent read must not be used here")
}

// the raw-row read behind the DocuSign callback guards must be strongly consistent: it has to see the envelope just persisted
func TestGetItemSignatureReadsConsistently(t *testing.T) {
	repo := &consistencyProbeRepo{row: &ItemSignature{SignatureID: "s1", SignatureEnvelopeID: "e1"}}
	got, err := service{repo: repo}.GetItemSignature(context.Background(), "s1")
	require.NoError(t, err)
	assert.Same(t, repo.row, got)
	assert.Equal(t, 1, repo.consistent)
	assert.Zero(t, repo.eventual)
}
