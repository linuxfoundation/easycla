// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"fmt"
	"strings"
	"sync"
	"time"

	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/restapi/operations/events"
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

func (s *Service) v2SignaturesToCorporateSignatures(src models.Signatures, projectSFID string) (*models.CorporateSignatures, error) {
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

// TransformSignatureToCorporateSignature transforms a Signature model into a CorporateSignature model
func (s *Service) TransformSignatureToCorporateSignature(signature *models.Signature, corporateSignature *models.CorporateSignature, projectSFID string) error {
	f := logrus.Fields{
		"functionName": "TransformSignatureToCorporateSignature",
		"signatureID":  signature.SignatureID,
	}

	var wg sync.WaitGroup
	var errMutex sync.Mutex
	var err error

	transformApprovalList := func(approvalList []string, listType string, destinationList *[]*models.ApprovalItem) {
		defer wg.Done()
		for _, item := range approvalList {
			searchTerm := fmt.Sprintf("%s was added to the approval list", item)

			pageSize := int64(10000)
			result, eventErr := s.eventService.SearchEvents(&events.SearchEventsParams{
				SearchTerm: &searchTerm,
				ProjectID:  &projectSFID,
				PageSize:   &pageSize,
			})
			if eventErr != nil {
				errMutex.Lock()
				err = eventErr
				errMutex.Unlock()
				return
			}
			approvalItem := &models.ApprovalItem{
				ApprovalItem: item,
			}
			if len(result.Events) > 0 {
				event := getLatestEvent(result.Events)
				approvalItem.DateAdded = event.EventTime
			} else {
				log.WithFields(f).Debugf("no events found for %s: %s", listType, item)
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

func getLatestEvent(events []*v1Models.Event) *v1Models.Event {
	var latest *v1Models.Event
	var latestTime time.Time

	for _, item := range events {
		t, err := utils.ParseDateTime(item.EventTime)
		if err != nil {
			log.Debugf("Error parsing time: %+v ", err)
			continue
		}

		if latest == nil || t.After(latestTime) {
			latest = item
			latestTime = t
		}
	}

	return latest
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
