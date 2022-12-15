// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package emails

import (
	"fmt"
	"html/template"
	"net/url"
	"path"
	"strings"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

// CommonEmailParams are part of almost every email it's sent from the system
type CommonEmailParams struct {
	RecipientName    string
	RecipientAddress string
	CompanyName      string
}

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
	IsFoundation            bool
	CorporateConsole        string
}

// GetProjectFullURL has the logic how to return back it's full url in corporate console
// it checks at SignedAtFoundationLevel flag as well for this specific kind of projects
func (p CLAProjectParams) GetProjectFullURL() template.HTML {
	if p.CorporateConsole == "" {
		return template.HTML(url.QueryEscape(p.ExternalProjectName)) // nolint gosec auto-escape HTML
	}

	u, err := url.Parse(p.CorporateConsole)
	if err != nil {
		log.Warnf("couldn't parse the console url, probably wrong configuration used : %s : %v", p.CorporateConsole, err)
		// at least return the project name so we don't have broken email
		return template.HTML(url.QueryEscape(p.ExternalProjectName)) // nolint gosec auto-escape HTML
	}

	var projectConsolePathURL string
	fullURLHtml := `<a href="%s" target="_blank">%s</a>`
	if p.IsFoundation {
		u.Path = path.Join(u.Path, "foundation", p.FoundationSFID, "cla")
		projectConsolePathURL = u.String()
	} else {
		u.Path = path.Join(u.Path, "foundation", p.FoundationSFID, "project", p.ProjectSFID, "cla")
		projectConsolePathURL = u.String()
	}

	return template.HTML(fmt.Sprintf(fullURLHtml, projectConsolePathURL, p.ExternalProjectName)) // nolint gosec auto-escape HTML
}

// CLAGroupTemplateParams includes the params for the CLAGroupTemplateParams
type CLAGroupTemplateParams struct {
	CorporateConsole string
	CLAGroupName     string
	// ChildProjectCount indicates how many childProjects are under this CLAGroup
	// this is important for some of the email rendering knowing if claGroup has
	// multiple children
	ChildProjectCount int
	Projects          []CLAProjectParams
	Version           string
}

// GetProjectNameOrFoundation returns if the foundationName is set it gets back
// the foundation Name otherwise the ProjectName is  returned
func (claParams CLAGroupTemplateParams) GetProjectNameOrFoundation() string {
	project := claParams.Projects[0]
	if claParams.ChildProjectCount == 1 {
		return claParams.Projects[0].ExternalProjectName
	}

	// if multiple return the foundation if present
	if project.FoundationName != "" {
		return project.FoundationName
	}
	//default to project name if nothing works
	return project.ExternalProjectName
}

// Project is used generally in v1 templates because the matching there was 1:1
// it will returns the first element from the projects list
func (claParams CLAGroupTemplateParams) Project() CLAProjectParams {
	return claParams.Projects[0]
}

// GetProjectsOrProject gets the first if single or all of them comma separated
func (claParams CLAGroupTemplateParams) GetProjectsOrProject() string {
	if len(claParams.Projects) == 1 {
		return claParams.Projects[0].ExternalProjectName
	}

	var projectNames []string
	for _, p := range claParams.Projects {
		projectNames = append(projectNames, p.ExternalProjectName)
	}

	return strings.Join(projectNames, ", ")
}
