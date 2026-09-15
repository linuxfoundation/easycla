// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
)

// CompanySanctionedCode is the machine-readable error code returned when a write is blocked for a sanctioned company
const CompanySanctionedCode = "company_sanctioned"

// CompanySanctionedSigningGuidance is the guidance appended to the message of the typed 403 returned by the CCLA signing gates
const CompanySanctionedSigningGuidance = "We're sorry, but this organization requires additional trade compliance review, so the Contributor License Agreement (CLA) cannot be completed at this time. If you believe this is an error, please contact EasyCLA Support via the chat widget."

// SanctionedCompanyError indicates a write was blocked because the company is flagged as sanctioned
type SanctionedCompanyError struct {
	CompanyID   string
	CompanySFID string
	CompanyName string
	// Guidance is optional user-facing text appended (after a newline) to the response message, not to Error()
	Guidance string
}

// Error returns the error message
func (e *SanctionedCompanyError) Error() string {
	return fmt.Sprintf("company %s requires further review for trade compliance", e.CompanyName)
}

// ResponseMessage returns the message carried by the typed 403 response: the error text plus the optional guidance
func (e *SanctionedCompanyError) ResponseMessage() string {
	if e.Guidance == "" {
		return e.Error()
	}
	return e.Error() + "\n" + e.Guidance
}

// CheckCompanySanctioned returns a SanctionedCompanyError when the company's stored sanctions flag is set, nil otherwise
func CheckCompanySanctioned(company *v1Models.Company) *SanctionedCompanyError {
	if company == nil || !company.IsSanctioned {
		return nil
	}
	return &SanctionedCompanyError{
		CompanyID:   company.CompanyID,
		CompanySFID: company.CompanyExternalID,
		CompanyName: company.CompanyName,
	}
}

// RejectIfCompanySanctioned returns a 403 company_sanctioned responder when the company's stored sanctions flag is set, nil otherwise
func RejectIfCompanySanctioned(ctx context.Context, company *v1Models.Company) middleware.Responder {
	sanctionedErr := CheckCompanySanctioned(company)
	if sanctionedErr == nil {
		return nil
	}
	var reqID string
	if v, ok := ctx.Value(XREQUESTID).(string); ok {
		reqID = v
	}
	return CompanySanctionedResponder(reqID, sanctionedErr)
}

// CompanySanctionedResponder builds the typed 403 company_sanctioned JSON response
func CompanySanctionedResponder(reqID string, sanctionedErr *SanctionedCompanyError) middleware.Responder {
	return middleware.ResponderFunc(func(rw http.ResponseWriter, _ runtime.Producer) {
		rw.Header().Set("Content-Type", "application/json")
		if reqID != "" {
			rw.Header().Set(XREQUESTID, reqID)
		}
		rw.WriteHeader(http.StatusForbidden)
		body := struct {
			Code        string `json:"code"`
			Message     string `json:"message"`
			CompanyID   string `json:"company_id"`
			CompanySFID string `json:"company_sfid"`
			XRequestID  string `json:"x-request-id,omitempty"`
		}{
			Code:        CompanySanctionedCode,
			Message:     sanctionedErr.ResponseMessage(),
			CompanyID:   sanctionedErr.CompanyID,
			CompanySFID: sanctionedErr.CompanySFID,
			XRequestID:  reqID,
		}
		if encodeErr := json.NewEncoder(rw).Encode(body); encodeErr != nil {
			log.WithField("functionName", "utils.CompanySanctionedResponder").WithError(encodeErr).Warn("unable to encode company_sanctioned response body")
		}
	})
}
