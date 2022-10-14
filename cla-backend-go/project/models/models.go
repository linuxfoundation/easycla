// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package models

// DBProjectModel data model
type DBProjectModel struct {
	DateCreated                      string                   `dynamodbav:"date_created"`
	DateModified                     string                   `dynamodbav:"date_modified"`
	ProjectExternalID                string                   `dynamodbav:"project_external_id"`
	ProjectID                        string                   `dynamodbav:"project_id"`
	FoundationSFID                   string                   `dynamodbav:"foundation_sfid"`
	RootProjectRepositoriesCount     int64                    `dynamodbav:"root_project_repositories_count"`
	ProjectName                      string                   `dynamodbav:"project_name"`
	ProjectNameLower                 string                   `dynamodbav:"project_name_lower"`
	ProjectDescription               string                   `dynamodbav:"project_description"`
	Version                          string                   `dynamodbav:"version"`
	ProjectTemplateID                string                   `dynamodbav:"project_template_id"`
	ProjectCclaEnabled               bool                     `dynamodbav:"project_ccla_enabled"`
	ProjectCclaRequiresIclaSignature bool                     `dynamodbav:"project_ccla_requires_icla_signature"`
	ProjectIclaEnabled               bool                     `dynamodbav:"project_icla_enabled"`
	ProjectLive                      bool                     `dynamodbav:"project_live"`
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
