// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_activity

import (
	"context"
	"errors"
	"strings"
	"testing"

	v2Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/common"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/gitlab_organizations"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

var errRefreshReached = errors.New("token refresh reached")

// webhookSecretOrgService resolves one enrolled org and records whether its OAuth token was refreshed,
// which is the first call made on the org's behalf after the secret token check
type webhookSecretOrgService struct {
	gitlab_organizations.ServiceInterface
	org       *v2Models.GitlabOrganization
	refreshed bool
}

func (s *webhookSecretOrgService) GetGitLabOrganizationByFullPath(_ context.Context, _ string) (*v2Models.GitlabOrganization, error) {
	return s.org, nil
}

func (s *webhookSecretOrgService) GetGitLabOrganization(_ context.Context, _ string) (*v2Models.GitlabOrganization, error) {
	return s.org, nil
}

func (s *webhookSecretOrgService) RefreshGitLabOrganizationAuth(_ context.Context, _ *common.GitLabOrganization) (*string, error) {
	s.refreshed = true
	return nil, errRefreshReached
}

// lfx-self-serve-ops#780 (warn-only stage): a missing or different X-Gitlab-Token is logged once, without the secret
// values, and the event is still processed exactly as before (the org token refresh is the first call on its behalf)
func TestProcessMergeActivityWarnsOnWebhookSecretMismatch(t *testing.T) {
	input := &ProcessMergeActivityInput{ProjectPath: "group/repo", ProjectNamespace: "group", ProjectID: 7, MergeID: 3, RepositoryPath: "group/repo"}
	newService := func(authState string) (*service, *webhookSecretOrgService) {
		orgService := &webhookSecretOrgService{org: &v2Models.GitlabOrganization{OrganizationID: "o1", OrganizationName: "group", OrganizationFullPath: "group", AuthState: authState}}
		return &service{gitlabOrgService: orgService}, orgService
	}
	captureWarnings := func(t *testing.T) *logtest.Hook {
		t.Helper()
		logger := log.GetLogger()
		oldHooks := logger.ReplaceHooks(make(logrus.LevelHooks))
		t.Cleanup(func() { logger.ReplaceHooks(oldHooks) })
		return logtest.NewLocal(logger)
	}
	secretWarnings := func(hook *logtest.Hook) []string {
		var msgs []string
		for _, entry := range hook.AllEntries() {
			if entry.Level == logrus.WarnLevel && strings.Contains(entry.Message, "webhook secret token") {
				msgs = append(msgs, entry.Message)
			}
		}
		return msgs
	}

	for _, tc := range []struct {
		name      string
		authState string
		token     string
		warning   string
	}{
		{name: "wrong token", authState: "hook-secret", token: "forged", warning: "mismatch"},
		{name: "empty token", authState: "hook-secret", token: "", warning: "missing"},
		{name: "org without auth state", authState: "", token: "forged", warning: "mismatch"},
	} {
		t.Run(tc.name+" is logged and still processed", func(t *testing.T) {
			hook := captureWarnings(t)
			svc, orgService := newService(tc.authState)
			err := svc.ProcessMergeActivity(context.Background(), tc.token, input)
			// the service wraps errors with %v, so the sentinel is matched by message
			if assert.Error(t, err) {
				assert.Contains(t, err.Error(), errRefreshReached.Error())
				assert.NotContains(t, err.Error(), secretTokenMismatch.Error())
			}
			assert.True(t, orgService.refreshed, "the event must still be processed in the warn-only stage")
			warnings := secretWarnings(hook)
			if assert.Len(t, warnings, 1) {
				assert.Contains(t, warnings[0], "webhook secret token "+tc.warning+" for gitlab org : o1:group")
				for _, secret := range []string{"hook-secret", "forged"} {
					assert.NotContains(t, warnings[0], secret)
				}
			}
		})
	}

	t.Run("matching token is processed without a warning", func(t *testing.T) {
		hook := captureWarnings(t)
		svc, orgService := newService("hook-secret")
		err := svc.ProcessMergeActivity(context.Background(), "hook-secret", input)
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), errRefreshReached.Error())
		}
		assert.True(t, orgService.refreshed)
		assert.Empty(t, secretWarnings(hook))
	})
}
