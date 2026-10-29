// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"errors"
	"testing"

	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	v1User "github.com/linuxfoundation/easycla/cla-backend-go/user"
	mock_users "github.com/linuxfoundation/easycla/cla-backend-go/v2/signatures/mock_users"
	v2UserModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service/models"
)

// CreateCLAManager provisions the EasyCLA record of a new CLA manager without the admin flag and leaves
// an existing record alone (lfx-self-serve-ops#978)
func TestEnsureCLAManagerUser(t *testing.T) {
	company := &v1Models.Company{CompanyID: "company-1", CompanyExternalID: "0014100000Te0G7AAJ"}
	platformUser := &v2UserModels.User{Username: "jdoe", Emails: []*v2UserModels.Email{{EmailAddress: swag.String("jdoe@example.com")}}}

	t.Run("a missing record is created without the admin flag", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		users := mock_users.NewMockService(ctrl)
		users.EXPECT().GetUserByLFUserName("jdoe").Return(nil, nil)
		var created *v1Models.User
		users.EXPECT().CreateUser(gomock.Any(), gomock.Nil()).DoAndReturn(func(u *v1Models.User, _ *v1User.CLAUser) (*v1Models.User, error) {
			created = u
			return u, nil
		})
		got, err := (&service{easyCLAUserService: users}).ensureCLAManagerUser(logrus.Fields{}, company, platformUser, "John", "Doe")
		require.NoError(t, err)
		require.NotNil(t, created)
		assert.False(t, created.Admin, "a CLA manager is not an EasyCLA administrator")
		assert.Equal(t, "jdoe", created.LfUsername)
		assert.Equal(t, strfmt.Email("jdoe@example.com"), created.LfEmail)
		assert.Equal(t, "John Doe", created.Username)
		assert.Equal(t, "company-1", created.CompanyID)
		assert.Equal(t, "0014100000Te0G7AAJ", created.UserExternalID)
		assert.Same(t, created, got)
	})
	t.Run("an existing administrator is neither recreated nor demoted", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		users := mock_users.NewMockService(ctrl)
		existing := &v1Models.User{UserID: "u-1", LfUsername: "jdoe", Admin: true}
		users.EXPECT().GetUserByLFUserName("jdoe").Return(existing, nil)
		got, err := (&service{easyCLAUserService: users}).ensureCLAManagerUser(logrus.Fields{}, company, platformUser, "John", "Doe")
		require.NoError(t, err)
		assert.Same(t, existing, got)
		assert.True(t, got.Admin)
	})
	t.Run("a lookup failure is reported", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		users := mock_users.NewMockService(ctrl)
		users.EXPECT().GetUserByLFUserName("jdoe").Return(nil, errors.New("dynamo down"))
		_, err := (&service{easyCLAUserService: users}).ensureCLAManagerUser(logrus.Fields{}, company, platformUser, "John", "Doe")
		assert.EqualError(t, err, "Problem getting claUser by :jdoe, error: dynamo down ")
	})
	t.Run("a creation failure is reported", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		users := mock_users.NewMockService(ctrl)
		users.EXPECT().GetUserByLFUserName("jdoe").Return(nil, nil)
		users.EXPECT().CreateUser(gomock.Any(), gomock.Nil()).Return(nil, errors.New("dynamo down"))
		_, err := (&service{easyCLAUserService: users}).ensureCLAManagerUser(logrus.Fields{}, company, platformUser, "John", "Doe")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Failed to create user : ")
	})
}
