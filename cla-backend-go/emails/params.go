// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"fmt"
	"html/template"
	"net/url"
	"path"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

// ClaManagerInfoParams represents the CLAManagerInfo used inside of the Email Templates
type ClaManagerInfoParams struct {
	LfUsername string
	Email      string
}

// CLAProjectParams is useful struct which keeps cla group project info and also
// know how to render it's corporate console url
type CLAProjectParams struct {
	ExternalProjectName     string
	ProjectSFID             string
	FoundationName          string
	FoundationSFID          string
	SignedAtFoundationLevel bool
	CorporateConsole        string
}

// GetProjectFullURL has the logic how to return back it's full url in corporate console
// it checks at SignedAtFoundationLevel flag as well for this specific kind of projects
func (p CLAProjectParams) GetProjectFullURL() template.HTML {
	if p.CorporateConsole == "" {
		return template.HTML(p.ExternalProjectName)
	}

	u, err := url.Parse(p.CorporateConsole)
	if err != nil {
		log.Warnf("couldn't parse the console url, probably wrong configuration used : %s : %v", p.CorporateConsole, err)
		// at least return the project name so we don't have broken email
		return template.HTML(p.ExternalProjectName)
	}

	var projectConsolePathURL string
	fullURLHtml := `<a href="%s" target="_blank">%s</a>`
	if p.SignedAtFoundationLevel {
		u.Path = path.Join(u.Path, "foundation", p.FoundationSFID, "cla")
		projectConsolePathURL = u.String()
	} else {
		u.Path = path.Join(u.Path, "foundation", p.FoundationSFID, "project", p.ProjectSFID, "cla")
		projectConsolePathURL = u.String()
	}

	return template.HTML(fmt.Sprintf(fullURLHtml, projectConsolePathURL, p.ExternalProjectName))
}

// CLAManagerTemplateParams includes the params for the CLAManagerTemplateParams
type CLAManagerTemplateParams struct {
	RecipientName string
	CompanyName   string
	CLAGroupName  string
	Project       CLAProjectParams
	CLAManagers   []ClaManagerInfoParams
	// ChildProjectCount indicates how many childProjects are under this CLAGroup
	// this is important for some of the email rendering knowing if claGroup has
	// multiple children
	ChildProjectCount int
}

// ApprovalTemplateParams details approval fields for contributor
type ApprovalTemplateParams struct {
	RecipientName string
	CompanyName   string
	CLAGroupName  string
	Approver      string
	Projects      []CLAProjectParams
}

// GetProjectNameOrFoundation returns if the foundationName is set it gets back
// the foundation Name otherwise the ProjectName is  returned
func (claParams CLAManagerTemplateParams) GetProjectNameOrFoundation() string {
	if claParams.ChildProjectCount == 0 {
		return claParams.Project.ExternalProjectName
	}

	// if multiple return the foundation if present
	if claParams.Project.FoundationName != "" {
		return claParams.Project.FoundationName
	}
	//default to project name if nothing works
	return claParams.Project.ExternalProjectName
}
