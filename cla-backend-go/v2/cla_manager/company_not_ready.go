// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"
	"github.com/sirupsen/logrus"

	"github.com/linuxfoundation/easycla/cla-backend-go/config"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/client/organizations"
)

const companyNotReadyCode = "company_not_ready"

// CompanyNotReadyError identifies the company whose Salesforce Account the Organization Service does not know yet
type CompanyNotReadyError struct {
	CompanyID   string
	CompanySFID string
}

func (e *CompanyNotReadyError) Error() string {
	return fmt.Sprintf("%s (company %s, sfid %s)", ErrCompanyNotReady.Error(), e.CompanyID, e.CompanySFID)
}

// Is makes errors.Is(err, ErrCompanyNotReady) true for the typed error
func (e *CompanyNotReadyError) Is(target error) bool { return target == ErrCompanyNotReady }

// companyCreateViaMemberService is the cla-member-service-create-company-enabled switch (tests override it)
var companyCreateViaMemberService = func() bool { return config.GetConfig().MemberService.CreateCompanyEnabled }

// companyNotReady explains an Organization Service "organization not found" (wrapped or not) with the typed
// not-ready error while companies are created through member-service; any other error stays as it is
func companyNotReady(f logrus.Fields, err error, companyID, companySFID string) error {
	var notFound *organizations.GetOrgNotFound
	if !companyCreateViaMemberService() || !errors.As(err, &notFound) {
		return nil
	}
	log.WithFields(f).Warnf("the Organization Service does not know company %s (sfid %s) yet - asking the caller to retry later", companyID, companySFID)
	return &CompanyNotReadyError{CompanyID: companyID, CompanySFID: companySFID}
}

// companyNotReadyResponder builds the typed 409 company_not_ready JSON response (shape of utils.CompanySanctionedResponder)
func companyNotReadyResponder(reqID string, notReady *CompanyNotReadyError) middleware.Responder {
	return middleware.ResponderFunc(func(rw http.ResponseWriter, _ runtime.Producer) {
		rw.Header().Set("Content-Type", "application/json")
		if reqID != "" {
			rw.Header().Set(utils.XREQUESTID, reqID)
		}
		rw.WriteHeader(http.StatusConflict)
		body := struct {
			Code        string `json:"code"`
			Message     string `json:"message"`
			CompanyID   string `json:"company_id"`
			CompanySFID string `json:"company_sfid"`
			XRequestID  string `json:"x-request-id,omitempty"`
		}{Code: companyNotReadyCode, Message: ErrCompanyNotReady.Error(), CompanyID: notReady.CompanyID, CompanySFID: notReady.CompanySFID, XRequestID: reqID}
		if encodeErr := json.NewEncoder(rw).Encode(body); encodeErr != nil {
			log.WithField("functionName", "cla_manager.companyNotReadyResponder").WithError(encodeErr).Warn("unable to encode company_not_ready response body")
		}
	})
}
