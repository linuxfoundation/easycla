// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"testing"

	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/assert"

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
)

func TestClaManagerUserRecordIsNotAnAdmin(t *testing.T) {
	record := claManagerUserRecord(&v1Models.Company{CompanyID: "company-1", CompanyExternalID: "0014100000Te0G7AAJ"}, "jdoe", "jdoe@example.com", "J Doe", "2026-10-28T08:00:00Z")
	assert.Equal(t, &v1Models.User{
		CompanyID:      "company-1",
		UserExternalID: "0014100000Te0G7AAJ",
		LfEmail:        strfmt.Email("jdoe@example.com"),
		Admin:          false,
		LfUsername:     "jdoe",
		DateCreated:    "2026-10-28T08:00:00Z",
		DateModified:   "2026-10-28T08:00:00Z",
		Username:       "J Doe",
		Version:        "v1",
	}, record)
}
