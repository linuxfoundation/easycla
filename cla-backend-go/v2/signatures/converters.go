// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"fmt"
	"strings"
	"sync"

	"github.com/jinzhu/copier"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/approvals"
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

	signatureApprovals := map[string][]string{
		"domain":         signature.DomainApprovalList,
		"email":          signature.EmailApprovalList,
		"githubOrg":      signature.GithubOrgApprovalList,
		"githubUsername": signature.GithubUsernameApprovalList,
		"gitlabOrg":      signature.GitlabOrgApprovalList,
		"gitlabUsername": signature.GitlabUsernameApprovalList,
	}

	// Transform the approval list items
	for key, value := range signatureApprovals {
		wg.Add(1)
		go func(key string, value []string) {
			defer wg.Done()
			for _, item := range value {
				// Default to the signature modified date
				// log.WithFields(f).Debugf("searching for approval for %s: %s", key, item)
				approvalItem := models.ApprovalItem{
					ApprovalItem: item,
					DateAdded:    signature.SignatureModified,
				}
				foundApprovals := searchSignatureApprovals(signature.SignatureID, key, item, approvals)

				if len(foundApprovals) > 0 {
					// ideally this should be one record
					approvalItem.DateAdded = foundApprovals[0].DateAdded
					log.WithFields(f).Debugf("found approval for %s: %s :%s", key, item, approvalItem.DateAdded)
				}

				switch key {
				case "domain":
					corporateSignature.DomainApprovalList = append(corporateSignature.DomainApprovalList, &approvalItem)
				case "email":
					corporateSignature.EmailApprovalList = append(corporateSignature.EmailApprovalList, &approvalItem)
				case "githubOrg":
					corporateSignature.GithubOrgApprovalList = append(corporateSignature.GithubOrgApprovalList, &approvalItem)
				case "githubUsername":
					corporateSignature.GithubUsernameApprovalList = append(corporateSignature.GithubUsernameApprovalList, &approvalItem)
				case "gitlabOrg":
					corporateSignature.GitlabOrgApprovalList = append(corporateSignature.GitlabOrgApprovalList, &approvalItem)
				case "gitlabUsername":
					corporateSignature.GitlabUsernameApprovalList = append(corporateSignature.GitlabUsernameApprovalList, &approvalItem)
				}
			}
		}(key, value)

	}

	log.WithFields(f).Debug("waiting for approval list items to be processed")
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
