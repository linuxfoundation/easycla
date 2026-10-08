// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sign

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestRollbackSignatoryGrants(t *testing.T) {
	type removal struct{ email, companySFID, projectSFID string }
	cases := []struct {
		name             string
		authorityEmail   string
		authorityGranted bool
		directGranted    bool
		removeErr        error
		want             []removal
	}{
		{name: "email flow removes the authority grant it created", authorityEmail: "signatory@example.com", authorityGranted: true, want: []removal{{"signatory@example.com", "comp-sfid", "proj-sfid"}}},
		{name: "email flow leaves a pre-existing authority grant alone", authorityEmail: "signatory@example.com", authorityGranted: false, want: nil},
		{name: "direct flow removes the caller grant it created", directGranted: true, want: []removal{{"caller@example.com", "comp-sfid", "proj-sfid"}}},
		{name: "direct flow leaves a pre-existing caller grant alone", directGranted: false, want: nil},
		{name: "caller who is also the authority is removed once", authorityEmail: "Caller@example.com", authorityGranted: true, directGranted: true, want: []removal{{"Caller@example.com", "comp-sfid", "proj-sfid"}}},
		{name: "direct grant is removed even when a matching authority email was not granted", authorityEmail: "Caller@example.com", authorityGranted: false, directGranted: true, want: []removal{{"caller@example.com", "comp-sfid", "proj-sfid"}}},
		{name: "removal errors are logged and do not stop the rollback", authorityEmail: "signatory@example.com", authorityGranted: true, directGranted: true, removeErr: errors.New("acs down"),
			want: []removal{{"signatory@example.com", "comp-sfid", "proj-sfid"}, {"caller@example.com", "comp-sfid", "proj-sfid"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []removal
			original := removeSignatoryRoleFn
			removeSignatoryRoleFn = func(ctx context.Context, email, companySFID, projectSFID string) error {
				assert.NoError(t, ctx.Err(), "the rollback must not run on the canceled request context")
				_, hasDeadline := ctx.Deadline()
				assert.True(t, hasDeadline, "the rollback context must be bounded")
				got = append(got, removal{email, companySFID, projectSFID})
				return tc.removeErr
			}
			defer func() { removeSignatoryRoleFn = original }()

			canceled, cancel := context.WithCancel(context.Background())
			cancel()
			rollbackSignatoryGrants(canceled, logrus.Fields{}, tc.authorityEmail, "caller@example.com", "comp-sfid", "proj-sfid", tc.authorityGranted, tc.directGranted)

			assert.Equal(t, tc.want, got)
		})
	}
}
