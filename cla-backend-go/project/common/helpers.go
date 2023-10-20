// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package common

import (
	"context"
	"fmt"
	"strconv"
	"time"

	models2 "github.com/communitybridge/easycla/cla-backend-go/project/models"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// AddStringAttribute adds a new string attribute to the existing map
func AddStringAttribute(item map[string]*dynamodb.AttributeValue, key string, value string) {
	if value != "" {
		item[key] = &dynamodb.AttributeValue{S: aws.String(value)}
	}
}

// AddBooleanAttribute adds a new boolean attribute to the existing map
func AddBooleanAttribute(item map[string]*dynamodb.AttributeValue, key string, value bool) {
	item[key] = &dynamodb.AttributeValue{BOOL: aws.Bool(value)}
}

// AddStringSliceAttribute adds a new string slice attribute to the existing map
func AddStringSliceAttribute(item map[string]*dynamodb.AttributeValue, key string, value []string) {
	item[key] = &dynamodb.AttributeValue{SS: aws.StringSlice(value)}
}

// AddListAttribute adds a list to the existing map
func AddListAttribute(item map[string]*dynamodb.AttributeValue, key string, value []*dynamodb.AttributeValue) {
	item[key] = &dynamodb.AttributeValue{L: value}
}

// BuildCLAGroupDocumentModels builds response models based on the array of db models
func BuildCLAGroupDocumentModels(dbDocumentModels []models2.DBProjectDocumentModel) []models.ClaGroupDocument {
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

		// // No previous, use the first...
		// if currentDoc == (models.ClaGroupDocument{}) {
		// 	currentDoc = doc
		// 	currentDocVersion = version
		// 	currentDocDateTime = dateTime
		// 	continue
		// }

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

func AreClaGroupDocumentsEqual(doc1, doc2 models.ClaGroupDocument) bool {
	return doc1.DocumentName == doc2.DocumentName &&
		doc1.DocumentAuthorName == doc2.DocumentAuthorName &&
		doc1.DocumentContentType == doc2.DocumentContentType &&
		doc1.DocumentFileID == doc2.DocumentFileID &&
		doc1.DocumentLegalEntityName == doc2.DocumentLegalEntityName &&
		doc1.DocumentPreamble == doc2.DocumentPreamble &&
		doc1.DocumentS3URL == doc2.DocumentS3URL &&
		doc1.DocumentMajorVersion == doc2.DocumentMajorVersion &&
		doc1.DocumentMinorVersion == doc2.DocumentMinorVersion &&
		doc1.DocumentCreationDate == doc2.DocumentCreationDate
}
