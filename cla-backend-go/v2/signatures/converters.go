// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"fmt"
	"strings"
	"sync"

	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/communitybridge/easycla/cla-backend-go/v2/approvals"
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

func (s *Service) v2SignaturesToCorporateSignatures(src models.Signatures, projectSFID string) (*models.CorporateSignatures, error) {
	f := logrus.Fields{
		"functionName": "v2SignaturesToCorporateSignatures",
		"projectSFID":  projectSFID,
	}

	// Convert the signatures
	log.WithFields(f).Debugf("converting %d signatures to corporate signatures", len(src.Signatures))

	var dst models.CorporateSignatures
	err := copier.Copy(&dst, src)
	if err != nil {
		return nil, err
	}

	// Convert the individual signatures
	for _, sigSrc := range src.Signatures {
		for _, sigDest := range dst.Signatures {
			err = s.TransformSignatureToCorporateSignature(sigSrc, sigDest, projectSFID)
			if err != nil {
				return nil, err
			}
		}
	}

	return &dst, nil
}

func searchSignatureApprovals(signatureID, criteria, name string, approvalList []approvals.ApprovalItem) []approvals.ApprovalItem {
	f := logrus.Fields{
		"functionName": "searchSignatureApprovals",
		"signatureID":  signatureID,
		"criteria":     criteria,
		"name":         name,
	}

	var result = make([]approvals.ApprovalItem, 0)
	for _, approval := range approvalList {
		if approval.SignatureID == signatureID && approval.ApprovalCriteria == criteria && approval.ApprovalName == name {
			log.WithFields(f).Debugf("found approval for %s: %s :%s", criteria, name, approval.DateAdded)
			result = append(result, approval)
		}
	}

	return result
}

// TransformSignatureToCorporateSignature transforms a Signature model into a CorporateSignature model
func (s *Service) TransformSignatureToCorporateSignature(signature *models.Signature, corporateSignature *models.CorporateSignature, projectSFID string) error {
	f := logrus.Fields{
		"functionName": "TransformSignatureToCorporateSignature",
		"signatureID":  signature.SignatureID,
	}

	var wg sync.WaitGroup
	var err error

	// fetch approval list items for signature
	approvals, approvalErr := s.approvalsRepos.GetApprovalListBySignature(signature.SignatureID)
	if approvalErr != nil {
		log.WithFields(f).WithError(approvalErr).Warnf("unable to fetch approval list items for signature")
		return approvalErr
	}

	log.WithFields(f).Debugf("Fetched %d approval list items for signature", len(approvals))

	transformApprovalList := func(approvalList []string, listType string, destinationList *[]*models.ApprovalItem) {
		defer wg.Done()
		for _, item := range approvalList {
			// Default to the signature modified date
			// log.WithFields(f).Debugf("searching for approval for %s: %s", listType, item)
			foundApprovals := searchSignatureApprovals(signature.SignatureID, listType, item, approvals)
			// Handle scenarios of records with no attached event logs
			dateAdded := signature.SignatureModified

			if len(foundApprovals) > 0 {
				// ideally this should be one record
				dateAdded = approvals[0].DateAdded
			}

			approvalItem := &models.ApprovalItem{
				ApprovalItem: item,
				DateAdded:    dateAdded,
			}
			*destinationList = append(*destinationList, approvalItem)
		}
	}

	// Transform domain approval list
	wg.Add(1)
	go transformApprovalList(signature.DomainApprovalList, "domain", &corporateSignature.DomainApprovalList)

	// Transform email approval list
	wg.Add(1)
	go transformApprovalList(signature.EmailApprovalList, "email", &corporateSignature.EmailApprovalList)

	// Transform GitHub org approval list
	wg.Add(1)
	go transformApprovalList(signature.GithubOrgApprovalList, "githubOrg", &corporateSignature.GithubOrgApprovalList)

	// Transform GitHub username approval list
	wg.Add(1)
	go transformApprovalList(signature.GithubUsernameApprovalList, "githubUsername", &corporateSignature.GithubUsernameApprovalList)

	// Transform GitLab org approval list
	wg.Add(1)
	go transformApprovalList(signature.GitlabOrgApprovalList, "gitlabOrg", &corporateSignature.GitlabOrgApprovalList)

	// Transform GitLab username approval list
	wg.Add(1)
	go transformApprovalList(signature.GitlabUsernameApprovalList, "gitlabUsername", &corporateSignature.GitlabUsernameApprovalList)

	wg.Wait()

	return err
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
