// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"context"
	"testing"

	"github.com/communitybridge/easycla/cla-backend-go/project/common"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
)

func TestGetCurrentDocumentVersion(t *testing.T) {
	currentTime, _ := utils.CurrentTime()
	yesterday := currentTime.AddDate(0, 0, -1)
	dayBeforeYesterday := currentTime.AddDate(0, 0, -2)

	var docs []models.ClaGroupDocument
	docs = append(docs, models.ClaGroupDocument{
		DocumentCreationDate:    utils.TimeToString(dayBeforeYesterday),
		DocumentMajorVersion:    "1",
		DocumentMinorVersion:    "0",
		DocumentAuthorName:      "Test",
		DocumentContentType:     "Test",
		DocumentFileID:          "Test",
		DocumentLegalEntityName: "Test",
		DocumentName:            "Test",
		DocumentPreamble:        "Test",
		DocumentS3URL:           "document1.pdf",
	})
	docs = append(docs, models.ClaGroupDocument{
		DocumentCreationDate:    utils.TimeToString(yesterday),
		DocumentMajorVersion:    "1",
		DocumentMinorVersion:    "1",
		DocumentAuthorName:      "Test",
		DocumentContentType:     "Test",
		DocumentFileID:          "Test",
		DocumentLegalEntityName: "Test",
		DocumentName:            "Test",
		DocumentPreamble:        "Test",
		DocumentS3URL:           "document2.pdf",
	})
	docs = append(docs, models.ClaGroupDocument{
		DocumentCreationDate:    utils.TimeToString(currentTime),
		DocumentMajorVersion:    "2",
		DocumentMinorVersion:    "0",
		DocumentAuthorName:      "Test",
		DocumentContentType:     "Test",
		DocumentFileID:          "Test",
		DocumentLegalEntityName: "Test",
		DocumentName:            "Test",
		DocumentPreamble:        "Test",
		DocumentS3URL:           "document3.pdf",
	})

	currentDoc, docErr := common.GetCurrentDocument(context.Background(), docs)
	assert.Nil(t, docErr, "current document error check is nil")
	assert.NotNil(t, currentDoc, "current document not nil")
	assert.Equal(t, "document3.pdf", currentDoc.DocumentS3URL, "loaded correct document")
}

func TestGetCurrentDocumentDateTime(t *testing.T) {
	currentTime, _ := utils.CurrentTime()
	yesterday := currentTime.AddDate(0, 0, -1)
	dayBeforeYesterday := currentTime.AddDate(0, 0, -2)

	var docs []models.ClaGroupDocument
	docs = append(docs, models.ClaGroupDocument{
		DocumentCreationDate:    utils.TimeToString(dayBeforeYesterday),
		DocumentMajorVersion:    "1",
		DocumentMinorVersion:    "0",
		DocumentAuthorName:      "Test",
		DocumentContentType:     "Test",
		DocumentFileID:          "Test",
		DocumentLegalEntityName: "Test",
		DocumentName:            "Test",
		DocumentPreamble:        "Test",
		DocumentS3URL:           "document1.pdf",
	})
	docs = append(docs, models.ClaGroupDocument{
		DocumentCreationDate:    utils.TimeToString(yesterday),
		DocumentMajorVersion:    "1",
		DocumentMinorVersion:    "0",
		DocumentAuthorName:      "Test",
		DocumentContentType:     "Test",
		DocumentFileID:          "Test",
		DocumentLegalEntityName: "Test",
		DocumentName:            "Test",
		DocumentPreamble:        "Test",
		DocumentS3URL:           "document2.pdf",
	})
	docs = append(docs, models.ClaGroupDocument{
		DocumentCreationDate:    utils.TimeToString(currentTime),
		DocumentMajorVersion:    "1",
		DocumentMinorVersion:    "0",
		DocumentAuthorName:      "Test",
		DocumentContentType:     "Test",
		DocumentFileID:          "Test",
		DocumentLegalEntityName: "Test",
		DocumentName:            "Test",
		DocumentPreamble:        "Test",
		DocumentS3URL:           "document3.pdf",
	})

	currentDoc, docErr := common.GetCurrentDocument(context.Background(), docs)
	assert.Nil(t, docErr, "current document error check is nil")
	assert.NotNil(t, currentDoc, "current document not nil")
	assert.Equal(t, "document3.pdf", currentDoc.DocumentS3URL, "loaded correct document")
}

func TestGetCurrentDocumentDateTimeDiffOrder(t *testing.T) {
	currentTime, _ := utils.CurrentTime()
	yesterday := currentTime.AddDate(0, 0, -1)
	dayBeforeYesterday := currentTime.AddDate(0, 0, -2)

	var docs []models.ClaGroupDocument
	docs = append(docs, models.ClaGroupDocument{
		DocumentCreationDate:    utils.TimeToString(yesterday),
		DocumentMajorVersion:    "1",
		DocumentMinorVersion:    "0",
		DocumentAuthorName:      "Test",
		DocumentContentType:     "Test",
		DocumentFileID:          "Test",
		DocumentLegalEntityName: "Test",
		DocumentName:            "Test",
		DocumentPreamble:        "Test",
		DocumentS3URL:           "document2.pdf",
	})
	docs = append(docs, models.ClaGroupDocument{
		DocumentCreationDate:    utils.TimeToString(currentTime),
		DocumentMajorVersion:    "1",
		DocumentMinorVersion:    "0",
		DocumentAuthorName:      "Test",
		DocumentContentType:     "Test",
		DocumentFileID:          "Test",
		DocumentLegalEntityName: "Test",
		DocumentName:            "Test",
		DocumentPreamble:        "Test",
		DocumentS3URL:           "document3.pdf",
	})
	docs = append(docs, models.ClaGroupDocument{
		DocumentCreationDate:    utils.TimeToString(dayBeforeYesterday),
		DocumentMajorVersion:    "1",
		DocumentMinorVersion:    "0",
		DocumentAuthorName:      "Test",
		DocumentContentType:     "Test",
		DocumentFileID:          "Test",
		DocumentLegalEntityName: "Test",
		DocumentName:            "Test",
		DocumentPreamble:        "Test",
		DocumentS3URL:           "document1.pdf",
	})

	currentDoc, docErr := common.GetCurrentDocument(context.Background(), docs)
	assert.Nil(t, docErr, "current document error check is nil")
	assert.NotNil(t, currentDoc, "current document not nil")
	assert.Equal(t, "document3.pdf", currentDoc.DocumentS3URL, "loaded correct document")
}

func TestGetCurrentDocumentMixedUp(t *testing.T) {
	currentTime, _ := utils.CurrentTime()
	yesterday := currentTime.AddDate(0, 0, -1)
	dayBeforeYesterday := currentTime.AddDate(0, 0, -2)

	var docs []models.ClaGroupDocument
	docs = append(docs, models.ClaGroupDocument{
		DocumentCreationDate:    utils.TimeToString(yesterday),
		DocumentMajorVersion:    "1",
		DocumentMinorVersion:    "0",
		DocumentAuthorName:      "Test",
		DocumentContentType:     "Test",
		DocumentFileID:          "Test",
		DocumentLegalEntityName: "Test",
		DocumentName:            "Test",
		DocumentPreamble:        "Test",
		DocumentS3URL:           "document2.pdf",
	})
	docs = append(docs, models.ClaGroupDocument{
		DocumentCreationDate:    utils.TimeToString(dayBeforeYesterday),
		DocumentMajorVersion:    "1",
		DocumentMinorVersion:    "2",
		DocumentAuthorName:      "Test",
		DocumentContentType:     "Test",
		DocumentFileID:          "Test",
		DocumentLegalEntityName: "Test",
		DocumentName:            "Test",
		DocumentPreamble:        "Test",
		DocumentS3URL:           "document1.pdf",
	})
	docs = append(docs, models.ClaGroupDocument{
		DocumentCreationDate:    utils.TimeToString(currentTime),
		DocumentMajorVersion:    "1",
		DocumentMinorVersion:    "1",
		DocumentAuthorName:      "Test",
		DocumentContentType:     "Test",
		DocumentFileID:          "Test",
		DocumentLegalEntityName: "Test",
		DocumentName:            "Test",
		DocumentPreamble:        "Test",
		DocumentS3URL:           "document3.pdf",
	})

	currentDoc, docErr := common.GetCurrentDocument(context.Background(), docs)
	assert.Nil(t, docErr, "current document error check is nil")
	assert.NotNil(t, currentDoc, "current document not nil")
	assert.Equal(t, "document1.pdf", currentDoc.DocumentS3URL, "loaded correct document")
}
