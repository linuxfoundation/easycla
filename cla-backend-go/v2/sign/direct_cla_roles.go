// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sign

import (
	"context"
	"errors"
	"fmt"
	"strings"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	acsService "github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service"
	memberService "github.com/linuxfoundation/easycla/cla-backend-go/v2/member-service"
	orgModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/models"
	userService "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service"
	"github.com/sirupsen/logrus"
)

var (
	prepareDirectUserForSigningFn = prepareDirectUserForSigning
	deleteDirectSignatoryGrantFn  = func(ctx context.Context, roleID, grantID string) error {
		return acsService.GetClient().DeleteUserGrant(ctx, roleID, grantID)
	}
)

func (s *service) SetMemberServiceClient(client *memberService.Client) {
	s.memberClient = client
}

func (s *service) memberOrganizationForScreening(ctx context.Context, companySFID string) (*orgModels.Organization, error) {
	if s.memberClient == nil {
		return nil, memberService.ErrNotConfigured
	}
	sfid, valid := memberService.SFID18(companySFID)
	if !valid || !strings.HasPrefix(sfid, "001") {
		return nil, memberService.ErrInvalidSFID
	}
	org, err := s.memberClient.GetB2BOrg(ctx, sfid)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, fmt.Errorf("member-service returned no organization for sanctions screening")
	}
	returnedSFID, valid := memberService.SFID18(org.UID)
	if !valid || returnedSFID != sfid {
		return nil, fmt.Errorf("member-service returned a different organization for sanctions screening")
	}
	return &orgModels.Organization{
		ID:      returnedSFID,
		Name:    org.Name,
		Domains: org.PrimaryDomain,
		Link:    org.Website,
	}, nil
}

func prepareDirectUserForSigning(ctx context.Context, email, companySFID, projectSFID string) (*acsService.OrgGrant, error) {
	user, err := userService.GetClient().SearchUsersByEmailContext(ctx, email)
	if errors.Is(err, userService.ErrUserNotFound) {
		log.WithField("userEmail", email).Debug("signatory has no LFID - no ACS grant required")
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if user == nil || strings.TrimSpace(user.Username) == "" {
		log.WithField("userEmail", email).Debug("signatory has no LFID - no ACS grant required")
		return nil, nil
	}
	acs := acsService.GetClient()
	roleID, err := acs.GetRoleID(utils.CLASignatoryRole)
	if err != nil {
		return nil, err
	}
	return acs.CreateProjectRoleGrant(ctx, companySFID, projectSFID, roleID, user.Username)
}

func rollbackDirectSignatoryGrant(ctx context.Context, fields logrus.Fields, grant *acsService.OrgGrant) {
	if grant == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), signatoryRollbackTimeout)
	defer cancel()
	if err := deleteDirectSignatoryGrantFn(ctx, grant.RoleID, grant.GrantID); err != nil {
		log.WithFields(fields).WithError(err).Warnf("failed to remove the signatory grant created by this request: %s", grant.GrantID)
	}
}
