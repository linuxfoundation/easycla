// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
)

func TestGithubRepositoryDisabledTemplate(t *testing.T) {
	params := GithubRepositoryDisabledTemplateParams{
		GithubRepositoryActionTemplateParams: GithubRepositoryActionTemplateParams{
			CommonEmailParams: CommonEmailParams{
				RecipientName: "CLA Manager",
			},
			CLAGroupTemplateParams: CLAGroupTemplateParams{
				CLAGroupName: "JohnsProject",
			},
			RepositoryName: "johnsRepository",
		},
		GithubAction: "deleted",
	}

	result, err := RenderTemplate(utils.V2, GithubRepositoryDisabledTemplateName, GithubRepositoryDisabledTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello CLA Manager")
	assert.Contains(t, result, "regarding the Github Repository johnsRepository")
	assert.Contains(t, result, "associated with the CLA Group JohnsProject")
	assert.Contains(t, result, "Github Repository johnsRepository was deleted")
}

func TestGithubRepositoryArchivedTemplate(t *testing.T) {
	params := GithubRepositoryArchivedTemplateParams{
		GithubRepositoryActionTemplateParams: GithubRepositoryActionTemplateParams{
			CommonEmailParams: CommonEmailParams{
				RecipientName: "CLA Manager",
			},
			CLAGroupTemplateParams: CLAGroupTemplateParams{
				CLAGroupName: "JohnsProject",
			},
			RepositoryName: "johnsRepository",
		},
	}

	result, err := RenderTemplate(utils.V2, GithubRepositoryArchivedTemplateName, GithubRepositoryArchivedTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello CLA Manager")
	assert.Contains(t, result, "regarding the Github Repository johnsRepository")
	assert.Contains(t, result, "associated with the CLA Group JohnsProject")
	assert.Contains(t, result, "Github Repository johnsRepository was archived")
}

func TestGithubRepositoryRenamedTemplate(t *testing.T) {
	params := GithubRepositoryRenamedTemplateParams{
		GithubRepositoryActionTemplateParams: GithubRepositoryActionTemplateParams{
			CommonEmailParams: CommonEmailParams{
				RecipientName: "CLA Manager",
			},
			CLAGroupTemplateParams: CLAGroupTemplateParams{
				CLAGroupName: "JohnsProject",
			},
			RepositoryName: "johnsNewRepository",
		},
		OldRepositoryName: "johnsOldRepository",
		NewRepositoryName: "johnsNewRepository",
	}

	result, err := RenderTemplate(utils.V2, GithubRepositoryRenamedTemplateName, GithubRepositoryRenamedTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello CLA Manager")
	assert.Contains(t, result, "regarding the Github Repository johnsNewRepository")
	assert.Contains(t, result, "associated with the CLA Group JohnsProject")
	assert.Contains(t, result, "Github Repository johnsOldRepository was renamed to johnsNewRepository")
}

func TestGithubRepositoryTransferredTemplate(t *testing.T) {
	params := GithubRepositoryTransferredTemplateParams{
		GithubRepositoryActionTemplateParams: GithubRepositoryActionTemplateParams{
			CommonEmailParams: CommonEmailParams{
				RecipientName: "CLA Manager",
			},
			CLAGroupTemplateParams: CLAGroupTemplateParams{
				CLAGroupName: "JohnsProject",
			},
			RepositoryName: "johnsNewRepository",
		},
		OldGithubOrgName: "johnsOldGithubOrg",
		NewGithubOrgName: "johnsNewGithubOrg",
	}

	result, err := RenderTemplate(utils.V2, GithubRepositoryTransferredTemplateName, GithubRepositoryTransferredTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello CLA Manager")
	assert.Contains(t, result, "regarding the Github Repository johnsNewRepository")
	assert.Contains(t, result, "associated with the CLA Group JohnsProject")
	assert.Contains(t, result, "Github Repository johnsNewRepository was transferred from johnsOldGithubOrg Organization to johnsNewGithubOrg Organization")
}

func TestGithubRepositoryTransferredFailedTemplate(t *testing.T) {
	params := GithubRepositoryTransferredTemplateParams{
		GithubRepositoryActionTemplateParams: GithubRepositoryActionTemplateParams{
			CommonEmailParams: CommonEmailParams{
				RecipientName: "CLA Manager",
			},
			CLAGroupTemplateParams: CLAGroupTemplateParams{
				CLAGroupName: "JohnsProject",
			},
			RepositoryName: "johnsNewRepository",
		},
		OldGithubOrgName: "johnsOldGithubOrg",
		NewGithubOrgName: "johnsNewGithubOrg",
	}

	result, err := RenderTemplate(utils.V2, GithubRepositoryTransferredFailedTemplateName, GithubRepositoryTransferredFailedTemplate,
		params)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello CLA Manager")
	assert.Contains(t, result, "regarding the Github Repository johnsNewRepository")
	assert.Contains(t, result, "associated with the CLA Group JohnsProject")
	assert.Contains(t, result, "Github Repository johnsNewRepository was transferred from johnsOldGithubOrg Organization to johnsNewGithubOrg Organization")
	assert.Contains(t, result, "EasyCLA is not enabled for the new Github Organization johnsNewGithubOrg")
	assert.Contains(t, result, "The Github Repository johnsNewRepository is now disabled")
}
