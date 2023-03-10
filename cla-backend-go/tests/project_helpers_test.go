// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"testing"

	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/communitybridge/easycla/cla-backend-go/v2/project-service/models"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/assert"
)

const (
	testProjectID   = "def13456"
	testProjectLogo = "testlogurl.com"
)

func TestIsProjectHasRootParentNoParent(t *testing.T) {
	project := &models.ProjectOutputDetailed{}
	project.Foundation = nil
	assert.True(t, utils.IsProjectHasRootParent(project), "Project Has Root Parent - Empty Parent")
}

func TestIsProjectHasRootParentLF(t *testing.T) {
	project := &models.ProjectOutputDetailed{}
	project.Foundation = &models.Foundation{
		ID:      testProjectID,
		LogoURL: testProjectLogo,
		Name:    utils.TheLinuxFoundation,
	}
	assert.True(t, utils.IsProjectHasRootParent(project), "Project Has Root Parent - LF Parent")
}

func TestIsProjectHasRootParentLFProjectsLLCFalse(t *testing.T) {
	project := &models.ProjectOutputDetailed{}
	project.Foundation = &models.Foundation{
		ID:      testProjectID,
		LogoURL: testProjectLogo,
		Name:    utils.LFProjectsLLC,
	}
	assert.False(t, utils.IsProjectHasRootParent(project), "Project Has Root Parent - LF Projects LLC Parent")
}

func TestIsProjectHasRootParentNonLF(t *testing.T) {
	project := &models.ProjectOutputDetailed{}
	project.Foundation = &models.Foundation{
		ID:      testProjectID,
		LogoURL: testProjectLogo,
		Name:    "other",
	}
	assert.False(t, utils.IsProjectHasRootParent(project), "Project Should not have Root Parent as - LF Project Parent")
}

func TestIsStandaloneProject(t *testing.T) {
	project := &models.ProjectOutputDetailed{}
	project.Foundation = nil
	project.Projects = []*models.ProjectOutput{}
	assert.True(t, utils.IsStandaloneProject(project), "Standalone Project with No Parent with No Children")
}

func TestLFParent(t *testing.T) {
	project := &models.ProjectOutputDetailed{}
	project.Foundation = &models.Foundation{
		ID:      testProjectID,
		LogoURL: testProjectLogo,
		Name:    utils.TheLinuxFoundation,
	}
	project.Projects = []*models.ProjectOutput{}
	assert.True(t, utils.IsStandaloneProject(project), "Standalone Project with LF Parent with No Children")
}

func TestLFProjectsLLCParent(t *testing.T) {
	project := &models.ProjectOutputDetailed{}
	project.Foundation = &models.Foundation{
		ID:      testProjectID,
		LogoURL: testProjectLogo,
		Name:    utils.LFProjectsLLC,
	}
	project.Projects = []*models.ProjectOutput{}
	assert.False(t, utils.IsStandaloneProject(project), "Should not be a standalone Project with LF Projects LLC parent with No Children")
}

func TestLFParentWithChildren(t *testing.T) {
	project := &models.ProjectOutputDetailed{}
	project.Foundation = &models.Foundation{
		ID:      testProjectID,
		LogoURL: testProjectLogo,
		Name:    utils.TheLinuxFoundation,
	}
	project.Projects = []*models.ProjectOutput{}
	assert.True(t, utils.IsStandaloneProject(project), "Standalone Project with LF Parent with Children")
}

func TestLFProjectsLLCParentWithChildren(t *testing.T) {
	project := &models.ProjectOutputDetailed{}
	project.Foundation = &models.Foundation{
		ID:      testProjectID,
		LogoURL: testProjectLogo,
		Name:    utils.LFProjectsLLC,
	}
	project.Projects = []*models.ProjectOutput{}
	child := &models.ProjectOutput{
		ProjectCommon:     models.ProjectCommon{},
		CreatedDate:       nil,
		DocuSignStatus:    nil,
		EndDate:           nil,
		EntityType:        "",
		ExecutiveDirector: nil,
		Foundation:        nil,
		HerokuConnectID:   "",
		ID:                testProjectID,
		IsDeleted:         false,
		LFSponsored:       false,
		LegalParent:       nil,
		ModifiedDate:      nil,
		OpportunityOwner:  nil,
		Owner:             nil,
		ProgramManager:    nil,
		ProjectType:       "SubProject",
		RenewalOwner:      nil,
		Slug:              "another-slug",
		SystemModStamp:    strfmt.DateTime{},
		Type:              "",
	}
	project.Projects = []*models.ProjectOutput{child}
	assert.False(t, utils.IsStandaloneProject(project), "Standalone Project with LF Projects LLC parent with Children")
}

func TestIsProjectHaveChildrenNoChildren(t *testing.T) {
	project := &models.ProjectOutputDetailed{}
	project.Foundation = nil
	project.Projects = []*models.ProjectOutput{}
	assert.False(t, utils.IsProjectHaveChildren(project), "Project has no children")
}

func TestIsProjectHaveChildrenWithChildren(t *testing.T) {
	project := &models.ProjectOutputDetailed{}
	project.Foundation = nil
	child := &models.ProjectOutput{
		ProjectCommon:     models.ProjectCommon{},
		CreatedDate:       nil,
		DocuSignStatus:    nil,
		EndDate:           nil,
		EntityType:        "",
		ExecutiveDirector: nil,
		Foundation:        nil,
		HerokuConnectID:   "",
		ID:                testProjectID,
		IsDeleted:         false,
		LFSponsored:       false,
		LegalParent:       nil,
		ModifiedDate:      nil,
		OpportunityOwner:  nil,
		Owner:             nil,
		ProgramManager:    nil,
		ProjectType:       "SubProject",
		RenewalOwner:      nil,
		Slug:              "random-slug",
		SystemModStamp:    strfmt.DateTime{},
		Type:              "",
	}
	project.Projects = []*models.ProjectOutput{child}
	assert.True(t, utils.IsProjectHaveChildren(project), "Project has Children")
}
