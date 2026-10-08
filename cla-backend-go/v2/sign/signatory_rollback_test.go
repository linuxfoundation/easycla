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
		name           string
		authorityEmail string
		directGranted  bool
		removeErr      error
		want           []removal
	}{
		{name: "email flow removes the authority grant", authorityEmail: "signatory@example.com", want: []removal{{"signatory@example.com", "comp-sfid", "proj-sfid"}}},
		{name: "direct flow removes the caller grant it created", directGranted: true, want: []removal{{"caller@example.com", "comp-sfid", "proj-sfid"}}},
		{name: "direct flow leaves a pre-existing caller grant alone", directGranted: false, want: nil},
		{name: "caller who is also the authority is removed once", authorityEmail: "Caller@example.com", directGranted: true, want: []removal{{"Caller@example.com", "comp-sfid", "proj-sfid"}}},
		{name: "removal errors are logged and do not stop the rollback", authorityEmail: "signatory@example.com", directGranted: true, removeErr: errors.New("acs down"),
			want: []removal{{"signatory@example.com", "comp-sfid", "proj-sfid"}, {"caller@example.com", "comp-sfid", "proj-sfid"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []removal
			original := removeSignatoryRoleFn
			removeSignatoryRoleFn = func(_ context.Context, email, companySFID, projectSFID string) error {
				got = append(got, removal{email, companySFID, projectSFID})
				return tc.removeErr
			}
			defer func() { removeSignatoryRoleFn = original }()

			rollbackSignatoryGrants(context.Background(), logrus.Fields{}, tc.authorityEmail, "caller@example.com", "comp-sfid", "proj-sfid", tc.directGranted)

			assert.Equal(t, tc.want, got)
		})
	}
}
