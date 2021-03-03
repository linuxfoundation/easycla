// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/communitybridge/easycla/cla-backend-go/gen/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// addStringAttribute adds a new string attribute to the existing map
func addStringAttribute(item map[string]*dynamodb.AttributeValue, key string, value string) {
	if value != "" {
		item[key] = &dynamodb.AttributeValue{S: aws.String(value)}
	}
}

// addBooleanAttribute adds a new boolean attribute to the existing map
func addBooleanAttribute(item map[string]*dynamodb.AttributeValue, key string, value bool) {
	item[key] = &dynamodb.AttributeValue{BOOL: aws.Bool(value)}
}

// addStringSliceAttribute adds a new string slice attribute to the existing map
func addStringSliceAttribute(item map[string]*dynamodb.AttributeValue, key string, value []string) {
	item[key] = &dynamodb.AttributeValue{SS: aws.StringSlice(value)}
}

// addListAttribute adds a list to the existing map
func addListAttribute(item map[string]*dynamodb.AttributeValue, key string, value []*dynamodb.AttributeValue) {
	item[key] = &dynamodb.AttributeValue{L: value}
}

// buildCLAGroupDocumentModels builds response models based on the array of db models
func buildCLAGroupDocumentModels(dbDocumentModels []DBProjectDocumentModel) []models.ClaGroupDocument {
	if dbDocumentModels == nil {
		return nil
	}

	// Response model
	var response []models.ClaGroupDocument

	for _, dbDocumentModel := range dbDocumentModels {
		response = append(response, models.ClaGroupDocument{
			DocumentName:            dbDocumentModel.DocumentName,
			DocumentAuthorName:      dbDocumentModel.DocumentAuthorName,
			DocumentContentType:     dbDocumentModel.DocumentContentType,
			DocumentFileID:          dbDocumentModel.DocumentFileID,
			DocumentLegalEntityName: dbDocumentModel.DocumentLegalEntityName,
			DocumentPreamble:        dbDocumentModel.DocumentPreamble,
			DocumentS3URL:           dbDocumentModel.DocumentS3URL,
			DocumentMajorVersion:    dbDocumentModel.DocumentMajorVersion,
			DocumentMinorVersion:    dbDocumentModel.DocumentMinorVersion,
			DocumentCreationDate:    dbDocumentModel.DocumentCreationDate,
		})
	}

	return response
}

func (s service) fillRepoInfo(ctx context.Context, project *models.ClaGroup) {
	f := logrus.Fields{
		"functionName":   "v1.project.helpers.fillRepoInfo",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	var wg sync.WaitGroup
	wg.Add(2)
	var ghrepos []*models.GithubRepositoriesGroupByOrgs
	var gerrits []*models.Gerrit

	go func() {
		defer wg.Done()
		var err error
		ghrepos, err = s.repositoriesRepo.GetCLAGroupRepositoriesGroupByOrgs(ctx, project.ProjectID, true)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("unable to get github repositories for cla group ID: %s", project.ProjectID)
			return
		}
	}()

	go func() {
		defer wg.Done()
		var err error
		var gerritsList *models.GerritList
		gerritsList, err = s.gerritRepo.GetClaGroupGerrits(ctx, project.ProjectID, nil)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("unable to get gerrit instances for cla group ID: %s.", project.ProjectID)
			return
		}
		gerrits = gerritsList.List
	}()

	wg.Wait()
	project.GithubRepositories = ghrepos
	project.Gerrits = gerrits
}

// GetCurrentDocument returns the current document based on the version and date/time
func GetCurrentDocument(ctx context.Context, docs []models.ClaGroupDocument) (models.ClaGroupDocument, error) {
	f := logrus.Fields{
		"functionName":   "v1.project.helpers.GetCurrentDocument",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	var currentDoc models.ClaGroupDocument
	var currentDocVersion float64
	var currentDocDateTime time.Time
	for _, doc := range docs {
		maj, err := strconv.Atoi(doc.DocumentMajorVersion)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("invalid major number in cla group: %s", doc.DocumentMajorVersion)
			continue
		}

		min, err := strconv.Atoi(doc.DocumentMinorVersion)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("invalid minor number in cla group: %s", doc.DocumentMinorVersion)
			continue
		}

		version, err := strconv.ParseFloat(fmt.Sprintf("%d.%d", maj, min), 32)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("invalid major/minor version in cla group: %s.%s", doc.DocumentMajorVersion, doc.DocumentMinorVersion)
			continue
		}

		dateTime, err := utils.ParseDateTime(doc.DocumentCreationDate)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("invalid date time in cla group: %s", doc.DocumentCreationDate)
			continue
		}

		// No previous, use the first...
		if currentDoc == (models.ClaGroupDocument{}) {
			currentDoc = doc
			currentDocVersion = version
			currentDocDateTime = dateTime
			continue
		}

		// Newer version...
		if version > currentDocVersion {
			currentDoc = doc
			currentDocVersion = version
			currentDocDateTime = dateTime
		}

		// Same version, but a later date...
		if version == currentDocVersion && dateTime.After(currentDocDateTime) {
			currentDoc = doc
			currentDocVersion = version
			currentDocDateTime = dateTime
		}
	}

	return currentDoc, nil
}
