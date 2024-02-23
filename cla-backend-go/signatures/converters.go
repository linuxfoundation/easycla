// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// buildProjectSignatureModels converts the response model into a response data model
func (repo repository) buildProjectSignatureModels(ctx context.Context, results *dynamodb.QueryOutput, claGroupID string, loadACLDetails bool) ([]*models.Signature, error) {
	f := logrus.Fields{
		"functionName":   "v1.signatures.converters.buildProjectSignatureModels",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
	}
	var sigs []*models.Signature

	// The DB signature model
	var dbSignatures []ItemSignature

	err := dynamodbattribute.UnmarshalListOfMaps(results.Items, &dbSignatures)
	if err != nil {
		log.WithFields(f).Warnf("error unmarshalling signatures from database for cla group ID: %s, error: %v",
			claGroupID, err)
		return nil, err
	}

	var wg sync.WaitGroup
	wg.Add(len(dbSignatures))
	for _, dbSignature := range dbSignatures {

		// Set the signature type in the response
		var claType = ""
		// Corporate Signature
		if dbSignature.SignatureReferenceType == utils.SignatureReferenceTypeCompany && dbSignature.SignatureType == utils.SignatureTypeCCLA {
			claType = utils.ClaTypeCCLA
		}
		// Employee Signature
		if dbSignature.SignatureReferenceType == utils.SignatureReferenceTypeUser && dbSignature.SignatureType == utils.SignatureTypeCLA && dbSignature.SignatureUserCompanyID != "" {
			claType = utils.ClaTypeECLA
		}

		// Individual Signature
		if dbSignature.SignatureReferenceType == utils.SignatureReferenceTypeUser && dbSignature.SignatureType == utils.SignatureTypeCLA && dbSignature.SignatureUserCompanyID == "" {
			claType = utils.ClaTypeICLA
		}

		// Use the signedOn field if possible, for older signatures that are missing it, use the date created value as the default/fallback
		signedOn := dbSignature.DateCreated
		if dbSignature.SignedOn != "" {
			signedOn = dbSignature.SignedOn
		}
		signedOn = utils.FormatTimeString(signedOn)

		sig := &models.Signature{
			SignatureID:                   dbSignature.SignatureID,
			ClaType:                       claType,
			SignatureCreated:              dbSignature.DateCreated,
			SignatureModified:             dbSignature.DateModified,
			SignatureType:                 dbSignature.SignatureType,
			SignatureReferenceID:          dbSignature.SignatureReferenceID,
			SignatureReferenceName:        dbSignature.SignatureReferenceName,
			SignatureReferenceNameLower:   dbSignature.SignatureReferenceNameLower,
			SignatureSigned:               dbSignature.SignatureSigned,
			SignatureApproved:             dbSignature.SignatureApproved,
			SignatureDocumentMajorVersion: strconv.Itoa(dbSignature.SignatureDocumentMajorVersion),
			SignatureDocumentMinorVersion: strconv.Itoa(dbSignature.SignatureDocumentMinorVersion),
			Version:                       strconv.Itoa(dbSignature.SignatureDocumentMajorVersion) + "." + strconv.Itoa(dbSignature.SignatureDocumentMinorVersion),
			SignatureReferenceType:        dbSignature.SignatureReferenceType,
			ProjectID:                     dbSignature.SignatureProjectID,
			Created:                       dbSignature.DateCreated,
			Modified:                      dbSignature.DateModified,
			EmailApprovalList:             utils.GetNilSliceIfEmpty(dbSignature.EmailApprovalList),
			DomainApprovalList:            utils.GetNilSliceIfEmpty(dbSignature.EmailDomainApprovalList),
			GithubUsernameApprovalList:    utils.GetNilSliceIfEmpty(dbSignature.GitHubUsernameApprovalList),
			GithubOrgApprovalList:         utils.GetNilSliceIfEmpty(dbSignature.GitHubOrgApprovalList),
			GitlabUsernameApprovalList:    utils.GetNilSliceIfEmpty(dbSignature.GitlabUsernameApprovalList),
			GitlabOrgApprovalList:         utils.GetNilSliceIfEmpty(dbSignature.GitlabOrgApprovalList),
			UserName:                      dbSignature.UserName,
			UserLFID:                      dbSignature.UserLFUsername,
			UserGHID:                      dbSignature.UserGithubID,
			UserGHUsername:                dbSignature.UserGithubUsername,
			UserGitlabID:                  dbSignature.UserGitlabID,
			UserGitlabUsername:            dbSignature.UserGitlabUsername,
			SignedOn:                      signedOn,
			SignatoryName:                 dbSignature.SignatoryName,
			UserDocusignName:              dbSignature.UserDocusignName,
			UserDocusignDateSigned:        dbSignature.UserDocusignDateSigned,
			AutoCreateECLA:                dbSignature.AutoCreateECLA,
			SignatureSignURL:              dbSignature.SignatureSignURL,
			SignatureCallbackURL:          dbSignature.SignatureCallbackURL,
			SignatureReturnURL:            dbSignature.SignatureReturnURL,
			SignatureReturnURLType:        dbSignature.SignatureReturnURLType,
			SignatureEnvelopeID:           dbSignature.SignatureEnvelopeID,
		}

		sigs = append(sigs, sig)
		go func(sigModel *models.Signature, signatureUserCompanyID string, sigACL []string) {
			defer wg.Done()
			var companyName = ""
			var companySigningEntityName = ""
			var userName = ""
			var userLFID = ""
			var userGHID = ""
			var userGHUsername = ""
			var swg sync.WaitGroup
			swg.Add(2)

			go func() {
				defer swg.Done()
				if sigModel.SignatureReferenceType == utils.SignatureReferenceTypeUser {
					userModel, userErr := repo.usersRepo.GetUser(sigModel.SignatureReferenceID)
					if userErr != nil || userModel == nil {
						log.WithFields(f).WithError(userErr).Warnf("unable to lookup user for signature: %s with reference type: %s using signature reference id: %s",
							sigModel.SignatureID, sigModel.SignatureReferenceType, sigModel.SignatureReferenceID)
					} else {
						userName = userModel.Username
						userLFID = userModel.LfUsername
						userGHID = userModel.GithubID
						userGHUsername = userModel.GithubUsername
					}

					if signatureUserCompanyID != "" {
						dbCompanyModel, companyErr := repo.companyRepo.GetCompany(ctx, signatureUserCompanyID)
						if companyErr != nil {
							log.WithFields(f).WithError(companyErr).Warnf("unable to lookup company record for signature: %s with reference type: %s using signature user company id: %s",
								sigModel.SignatureID, sigModel.SignatureReferenceType, signatureUserCompanyID)
						} else {
							companyName = dbCompanyModel.CompanyName
							companySigningEntityName = dbCompanyModel.SigningEntityName
						}
					}
				} else if sigModel.SignatureReferenceType == utils.SignatureReferenceTypeCompany {
					dbCompanyModel, companyErr := repo.companyRepo.GetCompany(ctx, sigModel.SignatureReferenceID)
					if companyErr != nil {
						log.WithFields(f).WithError(companyErr).Warnf("unable to lookup company record for signature: %s with reference type: %s using signature reference id: %s",
							sigModel.SignatureID, sigModel.SignatureReferenceType, sigModel.SignatureReferenceID)
					} else {
						companyName = dbCompanyModel.CompanyName
						companySigningEntityName = dbCompanyModel.SigningEntityName
					}
				}
			}()

			var signatureACL []models.User
			go func() {
				defer swg.Done()
				for _, userName := range sigACL {
					log.WithFields(f).Debugf("looking up user by user name: %s", userName)
					if loadACLDetails {
						userModel, userErr := repo.usersRepo.GetUserByUserName(userName, true)
						if userErr != nil {
							log.WithFields(f).WithError(userErr).Warnf("unable to lookup user by username: %s in ACL for signature: %s", userName, sigModel.SignatureID)
						} else {
							if userModel == nil {
								log.WithFields(f).Warnf("unable to lookup user by username: %s in ACL for signature: %s", userName, sigModel.SignatureID)
							} else {
								signatureACL = append(signatureACL, *userModel)
							}
						}
					} else {
						signatureACL = append(signatureACL, models.User{LfUsername: userName})
					}
				}
			}()
			swg.Wait()
			sigModel.CompanyName = companyName
			sigModel.SigningEntityName = companySigningEntityName
			sigModel.UserName = userName
			sigModel.UserLFID = userLFID
			sigModel.UserGHID = userGHID
			sigModel.UserGHUsername = userGHUsername
			sigModel.SignatureACL = signatureACL
		}(sig, dbSignature.SignatureUserCompanyID, dbSignature.SignatureACL)
	}
	wg.Wait()
	return sigs, nil
}

// buildProjectSignatureSummaryModels converts the response model into a signature summary model
func (repo repository) buildProjectSignatureSummaryModels(ctx context.Context, results *dynamodb.QueryOutput, projectID string) ([]*models.SignatureSummary, error) {
	f := logrus.Fields{
		"functionName":   "v1.signatures.converters.buildProjectSignatureSummaryModels",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectID":      projectID,
	}
	var sigs []*models.SignatureSummary

	// The DB signature model
	var dbSignatures []ItemSignature

	err := dynamodbattribute.UnmarshalListOfMaps(results.Items, &dbSignatures)
	if err != nil {
		log.WithFields(f).Warnf("error unmarshalling signatures from database for project: %s, error: %v",
			projectID, err)
		return nil, err
	}

	var wg sync.WaitGroup
	wg.Add(len(dbSignatures))
	for _, dbSignature := range dbSignatures {

		// Set the signature type in the response
		var claType = ""
		// Corporate Signature
		if dbSignature.SignatureReferenceType == utils.SignatureReferenceTypeCompany && dbSignature.SignatureType == utils.SignatureTypeCCLA {
			claType = utils.ClaTypeCCLA
		}
		// Employee Signature
		if dbSignature.SignatureReferenceType == utils.SignatureReferenceTypeUser && dbSignature.SignatureType == utils.SignatureTypeCLA && dbSignature.SignatureUserCompanyID != "" {
			claType = utils.ClaTypeECLA
		}

		// Individual Signature
		if dbSignature.SignatureReferenceType == utils.SignatureReferenceTypeUser && dbSignature.SignatureType == utils.SignatureTypeCLA && dbSignature.SignatureUserCompanyID == "" {
			claType = utils.ClaTypeICLA
		}

		sig := &models.SignatureSummary{
			SignatureID:                 dbSignature.SignatureID,
			ClaType:                     claType,
			SignatureType:               dbSignature.SignatureType,
			SignatureReferenceID:        dbSignature.SignatureReferenceID,
			SignatureReferenceName:      dbSignature.SignatureReferenceName,
			SignatureReferenceNameLower: dbSignature.SignatureReferenceNameLower,
			SignatureSigned:             dbSignature.SignatureSigned,
			SignatureApproved:           dbSignature.SignatureApproved,
			SignatureReferenceType:      dbSignature.SignatureReferenceType,
			ProjectID:                   dbSignature.SignatureProjectID,
			SignedOn:                    dbSignature.SignedOn,
			SignatoryName:               dbSignature.SignatoryName,
			UserDocusignName:            dbSignature.UserDocusignName,
			UserDocusignDateSigned:      dbSignature.UserDocusignDateSigned,
		}

		sigs = append(sigs, sig)
		go func(sigModel *models.SignatureSummary, signatureUserCompanyID string) {
			defer wg.Done()
			var companyName = ""
			var companySigningEntityName = ""
			var swg sync.WaitGroup
			swg.Add(1)

			go func() {
				defer swg.Done()
				if sigModel.SignatureReferenceType == "user" {
					if signatureUserCompanyID != "" {
						dbCompanyModel, companyErr := repo.companyRepo.GetCompany(ctx, signatureUserCompanyID)
						if companyErr != nil {
							log.WithFields(f).WithError(companyErr).Warnf("unable to lookup company record for signature: %s with reference type: %s using signature user company id: %s",
								sigModel.SignatureID, sigModel.SignatureReferenceType, signatureUserCompanyID)
						} else {
							companyName = dbCompanyModel.CompanyName
							companySigningEntityName = dbCompanyModel.SigningEntityName
						}
					}
				} else if sigModel.SignatureReferenceType == "company" {
					dbCompanyModel, companyErr := repo.companyRepo.GetCompany(ctx, sigModel.SignatureReferenceID)
					if companyErr != nil {
						log.WithFields(f).WithError(companyErr).Warnf("unable to lookup company record for signature: %s with reference type: %s using signature reference id: %s",
							sigModel.SignatureID, sigModel.SignatureReferenceType, sigModel.SignatureReferenceID)
					} else {
						companyName = dbCompanyModel.CompanyName
						companySigningEntityName = dbCompanyModel.SigningEntityName
					}
				}
			}()
			swg.Wait()

			sigModel.CompanyName = companyName
			sigModel.SigningEntityName = companySigningEntityName
		}(sig, dbSignature.SignatureUserCompanyID)
	}

	wg.Wait()
	return sigs, nil
}

// buildResponse is a helper function which converts a database model to a GitHub organization response model
func buildResponse(items []*dynamodb.AttributeValue) []models.GithubOrg {
	// Convert to a response model
	var orgs []models.GithubOrg
	for _, org := range items {
		selected := true
		orgs = append(orgs, models.GithubOrg{
			ID:       org.S,
			Selected: &selected,
		})
	}

	return orgs
}

// buildApprovalAttributeList builds the updated approval list based on the added and removed values
func buildApprovalAttributeList(ctx context.Context, existingList, addEntries, removeEntries []string) *dynamodb.AttributeValue {
	f := logrus.Fields{
		"functionName":   "buildApprovalAttributeList",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	var updatedList []string
	log.WithFields(f).Debugf("buildApprovalAttributeList - existing: %+v, add entries: %+v, remove entries: %+v",
		existingList, addEntries, removeEntries)

	// Add the existing entries to our response
	for _, value := range existingList {
		// No duplicates allowed
		if !utils.StringInSlice(value, updatedList) {
			log.WithFields(f).Debugf("buildApprovalAttributeList - adding existing entry: %s", value)
			updatedList = append(updatedList, strings.TrimSpace(value))
		} else {
			log.WithFields(f).Debugf("buildApprovalAttributeList - skipping existing entry: %s", value)
		}
	}

	// For all the new values...
	for _, value := range addEntries {
		// No duplicates allowed
		if !utils.StringInSlice(value, updatedList) {
			log.WithFields(f).Debugf("buildApprovalAttributeList - adding new entry: %s", value)
			updatedList = append(updatedList, strings.TrimSpace(value))
		} else {
			log.WithFields(f).Debugf("buildApprovalAttributeList - skipping new entry: %s", value)
		}
	}

	// Remove the items
	log.WithFields(f).Debugf("buildApprovalAttributeList - before: %+v - removing entries: %+v", updatedList, removeEntries)
	updatedList = utils.RemoveItemsFromList(updatedList, removeEntries)
	log.WithFields(f).Debugf("buildApprovalAttributeList - after: %+v - removing entries: %+v", updatedList, removeEntries)

	// Remove any duplicates - shouldn't have any if checked before adding
	log.WithFields(f).Debugf("buildApprovalAttributeList - before: %+v - removing duplicates", updatedList)
	updatedList = utils.RemoveDuplicates(updatedList)
	log.WithFields(f).Debugf("buildApprovalAttributeList - after: %+v - removing duplicates", updatedList)

	// Convert to the response type
	var responseList []*dynamodb.AttributeValue
	for _, value := range updatedList {
		responseList = append(responseList, &dynamodb.AttributeValue{S: aws.String(value)})
	}

	return &dynamodb.AttributeValue{L: responseList}
}

// buildCompanyIDList is a helper function to convert the DB response models into a simple list of company IDs
func (repo repository) buildCompanyIDList(ctx context.Context, results *dynamodb.QueryOutput) ([]SignatureCompanyID, error) {
	f := logrus.Fields{
		"functionName":   "buildCompanyIDList",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	var response []SignatureCompanyID

	// The DB signature model
	var dbSignatures []ItemSignature
	err := dynamodbattribute.UnmarshalListOfMaps(results.Items, &dbSignatures)
	if err != nil {
		log.WithFields(f).Warnf("error unmarshalling signatures from database, error: %v", err)
		return nil, err
	}

	// Loop and extract the company ID (signature_reference_id) value
	for _, item := range dbSignatures {
		// Lookup the company by ID - try to get more information like the external ID and name
		companyModel, companyLookupErr := repo.companyRepo.GetCompany(ctx, item.SignatureReferenceID)
		// Start building a model for this entry in the list
		signatureCompanyID := SignatureCompanyID{
			SignatureID: item.SignatureID,
			CompanyID:   item.SignatureReferenceID,
		}

		if companyLookupErr != nil || companyModel == nil {
			log.WithFields(f).Warnf("problem looking up company using id: %s, error: %+v",
				item.SignatureReferenceID, companyLookupErr)
			response = append(response, signatureCompanyID)
		} else {
			if companyModel.CompanyExternalID != "" {
				signatureCompanyID.CompanySFID = companyModel.CompanyExternalID
			}
			if companyModel.CompanyName != "" {
				signatureCompanyID.CompanyName = companyModel.CompanyName
			}
			response = append(response, signatureCompanyID)
		}
	}

	return response, nil
}
