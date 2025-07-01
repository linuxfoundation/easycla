// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package events

import "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"

// IndividualSignedEvent represntative of ICLA signatures
const IndividualSignedEvent = "IndividualSignatureSigned"

// Event data model
type Event struct {
	EventID   string `dynamodbav:"event_id"`
	EventType string `dynamodbav:"event_type"`

	EventUserID     string `dynamodbav:"event_user_id"`
	EventUserName   string `dynamodbav:"event_user_name"`
	EventLfUsername string `dynamodbav:"event_lf_username"`

	EventCLAGroupID        string `dynamodbav:"event_cla_group_id"`
	EventCLAGroupName      string `dynamodbav:"event_cla_group_name"`
	EventCLAGroupNameLower string `dynamodbav:"event_cla_group_name_lower"`

	EventProjectID         string `dynamodbav:"event_project_id"` // legacy, same as the SFID
	EventProjectSFID       string `dynamodbav:"event_project_sfid"`
	EventProjectName       string `dynamodbav:"event_project_name"`
	EventParentProjectSFID string `dynamodbav:"event_parent_project_sfid"`
	EventParentProjectName string `dynamodbav:"event_parent_project_name"`

	EventCompanyID   string `dynamodbav:"event_company_id"`
	EventCompanySFID string `dynamodbav:"event_company_sfid"`
	EventCompanyName string `dynamodbav:"event_company_name"`

	EventData    string `dynamodbav:"event_data"`
	EventSummary string `dynamodbav:"event_summary"`

	EventTime      string `dynamodbav:"event_time"`
	EventTimeEpoch int64  `dynamodbav:"event_time_epoch"`
}

// DBUser data model
type DBUser struct {
	UserID             string   `json:"user_id"`
	UserExternalID     string   `json:"user_external_id"`
	LFEmail            string   `json:"lf_email"`
	Admin              bool     `json:"admin"`
	LFUsername         string   `json:"lf_username"`
	DateCreated        string   `json:"date_created"`
	DateModified       string   `json:"date_modified"`
	UserName           string   `json:"user_name"`
	Version            string   `json:"version"`
	UserEmails         []string `json:"user_emails"`
	UserGithubID       string   `json:"user_github_id"`
	UserCompanyID      string   `json:"user_company_id"`
	UserGithubUsername string   `json:"user_github_username"`
	Note               string   `json:"note"`
}

func (e *Event) toEvent() *models.Event { //nolint
	event := &models.Event{
		EventID:   e.EventID,
		EventType: e.EventType,

		UserID:     e.EventUserID,
		UserName:   e.EventUserName,
		LfUsername: e.EventLfUsername,

		EventCLAGroupID:        e.EventCLAGroupID,
		EventCLAGroupName:      e.EventCLAGroupName,
		EventCLAGroupNameLower: e.EventCLAGroupNameLower,

		EventProjectID:         e.EventProjectID,
		EventProjectSFID:       e.EventProjectSFID,
		EventProjectName:       e.EventProjectName,
		EventParentProjectSFID: e.EventParentProjectSFID,
		EventParentProjectName: e.EventParentProjectName,

		EventCompanyID:   e.EventCompanyID,
		EventCompanySFID: e.EventCompanySFID,
		EventCompanyName: e.EventCompanyName,

		EventTime:      e.EventTime,
		EventTimeEpoch: e.EventTimeEpoch,

		EventData:    e.EventData,
		EventSummary: e.EventSummary,
	}
	// Disregard Company details for ICLA event
	if event.EventType != IndividualSignedEvent {
		event.EventCompanyID = e.EventCompanyID
		event.EventCompanyName = e.EventCompanyName
	}

	return event
}

// DBProjectModel data model
type DBProjectModel struct {
	DateCreated                      string                   `dynamodbav:"date_created"`
	DateModified                     string                   `dynamodbav:"date_modified"`
	ProjectExternalID                string                   `dynamodbav:"project_external_id"`
	ProjectID                        string                   `dynamodbav:"project_id"`
	ProjectName                      string                   `dynamodbav:"project_name"`
	Version                          string                   `dynamodbav:"version"`
	ProjectCclaEnabled               bool                     `dynamodbav:"project_ccla_enabled"`
	ProjectCclaRequiresIclaSignature bool                     `dynamodbav:"project_ccla_requires_icla_signature"`
	ProjectIclaEnabled               bool                     `dynamodbav:"project_icla_enabled"`
	ProjectCorporateDocuments        []DBProjectDocumentModel `dynamodbav:"project_corporate_documents"`
	ProjectIndividualDocuments       []DBProjectDocumentModel `dynamodbav:"project_individual_documents"`
	ProjectMemberDocuments           []DBProjectDocumentModel `dynamodbav:"project_member_documents"`
	ProjectACL                       []string                 `dynamodbav:"project_acl"`
}

// DBProjectDocumentModel is a data model for the CLA Group Project documents
type DBProjectDocumentModel struct {
	DocumentName            string `dynamodbav:"document_name"`
	DocumentFileID          string `dynamodbav:"document_file_id"`
	DocumentPreamble        string `dynamodbav:"document_preamble"`
	DocumentLegalEntityName string `dynamodbav:"document_legal_entity_name"`
	DocumentAuthorName      string `dynamodbav:"document_author_name"`
	DocumentContentType     string `dynamodbav:"document_content_type"`
	DocumentS3URL           string `dynamodbav:"document_s3_url"`
	DocumentMajorVersion    string `dynamodbav:"document_major_version"`
	DocumentMinorVersion    string `dynamodbav:"document_minor_version"`
	DocumentCreationDate    string `dynamodbav:"document_creation_date"`
}
