// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"fmt"
	"strings"

	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/jinzhu/copier"
	"github.com/sirupsen/logrus"
)

func v2Signature(src *v1Models.Signature) (*models.Signature, error) {
	var dst models.Signature
	err := copier.Copy(&dst, src)
	if err != nil {
		return nil, err
	}
	return &dst, nil
}

func v2Signatures(src *v1Models.Signatures) (*models.Signatures, error) {
	var dst models.Signatures
	err := copier.Copy(&dst, src)
	if err != nil {
		return nil, err
	}
	return &dst, nil
}

func v2SignaturesReplaceCompanyID(src *v1Models.Signatures, internalID, externalID string) (*models.Signatures, error) {
	var dst models.Signatures
	err := copier.Copy(&dst, src)
	if err != nil {
		return nil, err
	}

	// Replace the internal ID with the External ID
	for _, sig := range dst.Signatures {
		if sig.SignatureReferenceID == internalID {
			sig.SignatureReferenceID = externalID
		}
	}

	return &dst, nil
}

func iclaSigCsvHeader() string {
	return `Name,GitHub Username,GitLab Username,LF_ID,Email,Signed Date,Approved,Signed`
}

func iclaSigCsvLine(sig *v1Models.IclaSignature) string {
	var dateTime string
	t, err := utils.ParseDateTime(sig.SignedOn)
	if err != nil {
		log.WithFields(logrus.Fields{"signature_id": sig.SignatureID, "signature_created": sig.SignedOn}).
			Error("invalid time format present for signatures")
	} else {
		dateTime = t.Format("Jan 2,2006")
	}
	return fmt.Sprintf("\n\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",%t,%t", sig.UserName, sig.GithubUsername, sig.GitlabUsername, sig.LfUsername, sig.UserEmail, dateTime, sig.SignatureApproved, sig.SignatureSigned)
}

func cclaSigCsvHeader() string {
	return `Company Name,Signed,Approved,DomainApprovalList,EmailApprovalList,GitHubOrgApprovalList,GitHubUsernameApprovalList,Date Signed,Approved,Signed`
}

func cclaSigCsvLine(sig *v1Models.Signature) string {
	var dateTime string
	t, err := utils.ParseDateTime(sig.SignedOn)
	if err != nil {
		log.WithFields(logrus.Fields{"signature_id": sig.SignatureID, "signature_created": sig.SignedOn}).
			Error("invalid time format present for signatures")
	} else {
		dateTime = t.Format("Jan 2,2006")
	}
	return fmt.Sprintf("\n\"%s\",%t,%t,\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",%t,%t",
		sig.CompanyName,
		sig.SignatureSigned,
		sig.SignatureApproved,
		strings.Join(sig.DomainApprovalList, ","),
		strings.Join(sig.EmailApprovalList, ","),
		strings.Join(sig.GithubOrgApprovalList, ","),
		strings.Join(sig.GithubUsernameApprovalList, ","),
		dateTime,
		sig.SignatureApproved,
		sig.SignatureApproved)
}
