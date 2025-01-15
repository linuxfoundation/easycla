// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gerrits

import (
	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/go-openapi/strfmt"
)

// Gerrit represent gerrit instances table
type Gerrit struct {
	DateCreated   string `json:"date_created,omitempty"`
	DateModified  string `json:"date_modified,omitempty"`
	GerritID      string `json:"gerrit_id,omitempty"`
	GerritName    string `json:"gerrit_name,omitempty"`
	GerritURL     string `json:"gerrit_url,omitempty"`
	GroupIDCcla   string `json:"group_id_ccla,omitempty"`
	GroupIDIcla   string `json:"group_id_icla,omitempty"`
	GroupNameCcla string `json:"group_name_ccla,omitempty"`
	GroupNameIcla string `json:"group_name_icla,omitempty"`
	ProjectSFID   string `json:"project_sfid,omitempty"`
	ProjectID     string `json:"project_id,omitempty"`
	Version       string `json:"version,omitempty"`
}

// toModel converts the gerrit structure into a response model
func (g *Gerrit) toModel() *models.Gerrit {
	return &models.Gerrit{
		DateCreated:  g.DateCreated,
		DateModified: g.DateModified,
		GerritID:     strfmt.UUID4(g.GerritID),
		GerritName:   g.GerritName,
		GerritURL:    strfmt.URI(g.GerritURL),
		GroupIDCcla:  g.GroupIDCcla,
		ProjectID:    g.ProjectID,
		Version:      g.Version,
		ProjectSFID:  g.ProjectSFID,
	}
}

// WebLink contains the name and url
type WebLink struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// GerritRepoInfo a simplified gerrit repo information data model
type GerritRepoInfo struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	State       string    `json:"state"`
	WebLinks    []WebLink `json:"web_links"`
}

// AccountsConfigInfo entity contains information about Gerrit configuration from the accounts section. https://gerrit.linuxfoundation.org/infra/Documentation/rest-api-config.html#accounts-config-info
type AccountsConfigInfo struct {
	Visibility         string `json:"visibility"`
	DefaultDisplayName string `json:"default_display_name"`
}

// GroupInfo entity contains information about a group. This can be a Gerrit internal group, or an external group that is known to Gerrit. https://gerrit.linuxfoundation.org/infra/Documentation/rest-api-groups.html#group-info
type GroupInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Options     string `json:"options"`
	Description string `json:"description"`
	GroupID     string `json:"group_id"`
	Owner       string `json:"owner"`
	OwnerID     string `json:"owner_id"`
	CreatedOn   string `json:"created_on"`
	// _more_groups
	// members
	// includes
}

// ContributorAgreementInfo entity contains information about a contributor agreement. https://gerrit.linuxfoundation.org/infra/Documentation/rest-api-accounts.html#contributor-agreement-info
type ContributorAgreementInfo struct {
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	URL             string    `json:"url"`
	AutoVerifyGroup GroupInfo `json:"auto_verify_group"`
}

// AuthInfo entity contains information about the authentication configuration of the Gerrit server. https://gerrit.linuxfoundation.org/infra/Documentation/rest-api-config.html#auth-info
type AuthInfo struct {
	Type                     string                     `json:"type"`
	UseContributorAgreements bool                       `json:"use_contributor_agreements"`
	ContributorAgreements    []ContributorAgreementInfo `json:"contributor_agreements"`
	EditableAccountFields    []string                   `json:"editable_account_fields"`
	LoginURL                 string                     `json:"login_url"`
	LoginText                string                     `json:"login_text"`
	SwitchAccountURL         string                     `json:"switch_account_url"`
	RegisterURL              string                     `json:"register_url"`
	RegisterText             string                     `json:"register_text"`
	EditFullNameURL          string                     `json:"edit_full_name_url"`
	HTTPPasswordURL          string                     `json:"http_password_url"`
	GitBasicAuthPolicy       string                     `json:"git_bacic_auth_policy"`
}

// ChangeInfo entity contains information about Gerrit configuration from the change section.- https://gerrit.linuxfoundation.org/infra/Documentation/rest-api-config.html#change-config-info
type ChangeInfo struct {
	AllowBlame  bool   `json:"allow_blame"`
	LargeChange int    `json:"large_change"`
	ReplyLabel  string `json:"reply_label"`
	// reply_tooltip
	// update_delay
	// submit_whole_topic
	// disable_private_changes
	// mergeability_computation_behavior
	// enable_attention_set
	// enable_assignee
}

// DownloadSchemesInfo entity contains information about a supported download scheme and its commands. https://gerrit.linuxfoundation.org/infra/Documentation/rest-api-config.html#download-scheme-info
type DownloadSchemesInfo struct {
	URL             string `json:"url"`
	IsAuthRequired  bool   `json:"is_auth_required"`
	IsAuthSupported bool   `json:"is_auth_supported"`
	// Commands `json:"commands"`
	// CloneCommands `json:"clone_commands"`
}

// DownloadInfo entity contains information about supported download options. - https://gerrit.linuxfoundation.org/infra/Documentation/rest-api-config.html#download-info
type DownloadInfo struct {
	Schemes  DownloadSchemesInfo `json:"schemes"`
	Archives []string            `json:"archives"`
}

// GerritInfo entity contains information about Gerrit configuration from the gerrit section. https://gerrit.linuxfoundation.org/infra/Documentation/rest-api-config.html#gerrit-info
type GerritInfo struct {
	AllProjectsName string `json:"all_projects_name"`
	AllUsersName    string `json:"all_users_name"`
	DocURL          string `json:"doc_url"`
	ReportBugURL    string `json:"report_bug_url"`
	DocSearch       bool   `json:"doc_search"`
	EditPGPKeys     bool   `json:"edit_pgp_keys"`
}

// IndexConfigInfo data model
type IndexConfigInfo struct {
	// Finish this build-out if needed
}

// PluginConfigInfo data model
type PluginConfigInfo struct {
	// Finish this build-out if needed
}

// ReceiveInfo data model
type ReceiveInfo struct {
	// Finish this build-out if needed
}

// SshdInfo data model
type SshdInfo struct {
	// Finish this build-out if needed
}

// SuggestInfo data model
type SuggestInfo struct {
	// Finish this build-out if needed
}

// UserConfigInfo data model
type UserConfigInfo struct {
	// Finish this build-out if needed
}

// ServerInfo is the response model returned from the server config query
type ServerInfo struct {
	Accounts     AccountsConfigInfo `json:"accounts"`
	Auth         AuthInfo           `json:"auth"`
	Change       ChangeInfo         `json:"change"`
	Download     DownloadInfo       `json:"download"`
	Gerrit       GerritInfo         `json:"gerrit"`
	Index        IndexConfigInfo    `json:"index"`
	Plugin       PluginConfigInfo   `json:"plugin"`
	Receive      ReceiveInfo        `json:"receive"`
	SSHD         SshdInfo           `json:"sshd"`
	Suggest      SuggestInfo        `json:"suggest"`
	User         UserConfigInfo     `json:"user"`
	DefaultTheme string             `json:"default_theme"`
}
