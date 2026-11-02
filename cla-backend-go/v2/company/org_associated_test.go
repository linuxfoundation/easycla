// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsOrgAssociatedMatchesBothACSWordings(t *testing.T) {
	assert.True(t, isOrgAssociated(errors.New("[400] createOrgUserRoleOrgScope: users are already associated with other organization")))
	assert.True(t, isOrgAssociated(errors.New("[400] createOrgUserRoleOrgScope: user is already associated with other organization")))
	assert.False(t, isOrgAssociated(errors.New("[409] createOrgUsrRoleScopesConflict")))
	assert.False(t, isOrgAssociated(nil))
}
