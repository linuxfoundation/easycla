// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package approvals

type ApprovalItem struct {
	ApprovalID          string `dynamodbav:"approval_id"`
	SignatureID         string `dynamodbav:"signature_id"`
	DateAdded           string `dynamodbav:"date_added"`
	DateRemoved         string `dynamodbav:"date_removed"`
	DateCreated         string `dynamodbav:"date_created"`
	DateModified        string `dynamodbav:"date_modified"`
	ApprovalName        string `dynamodbav:"approval_name"`
	ApprovalCriteria    string `dynamodbav:"approval_criteria"`
	CompanyID           string `dynamodbav:"company_id"`
	ProjectID           string `dynamodbav:"project_id"`
	ApprovalCompanyName string `dynamodbav:"approval_company_name"`
	Note                string `dynamodbav:"note"`
	Active              bool   `dynamodbav:"active"`
}
