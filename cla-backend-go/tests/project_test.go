// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/communitybridge/easycla/cla-backend-go/project/common"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
)

func createDocument(major, minor int, date time.Time, name string) models.ClaGroupDocument {
	return models.ClaGroupDocument{
		DocumentMajorVersion: strconv.Itoa(major),
		DocumentMinorVersion: strconv.Itoa(minor),
		DocumentCreationDate: utils.TimeToString(date),
		DocumentName:         name,
	}
}

func mustParseTime(t *testing.T, layout, value string) time.Time {
	dt, err := time.Parse(layout, value)
	assert.Nil(t, err)
	return dt
}

func TestGetCurrentDocument(t *testing.T) {
	// Dates
	dt_250217 := mustParseTime(t, time.RFC3339, "2025-02-17T15:00:13Z")
	dt_240217 := mustParseTime(t, time.RFC3339, "2024-02-17T15:00:13Z")
	dt_240218 := mustParseTime(t, time.RFC3339, "2024-02-18T15:00:13Z")
	dt_230217 := mustParseTime(t, time.RFC3339, "2023-02-17T15:00:13Z")
	dt_230218 := mustParseTime(t, time.RFC3339, "2023-02-18T15:00:13Z")
	dt_220218 := mustParseTime(t, time.RFC3339, "2022-02-18T15:00:13Z")

	// Create documents
	document_15 := createDocument(1, 5, dt_250217, "document_15")
	document_29 := createDocument(2, 9, dt_240217, "document_29")
	document_29_newer := createDocument(2, 9, dt_240218, "document_29_newer")
	document_210 := createDocument(2, 10, dt_230217, "document_210")
	document_210_newer := createDocument(2, 10, dt_230218, "document_210_newer")
	document_30 := createDocument(3, 0, dt_220218, "document_30")
	document_31 := createDocument(3, 1, dt_220218, "document_31")
	document_310 := createDocument(3, 10, dt_220218, "document_310")

	// Test cases
	tests := []struct {
		docs          []models.ClaGroupDocument
		expectedName  string
		expectedMajor int
		expectedMinor int
	}{
		{[]models.ClaGroupDocument{document_29}, "document_29", 2, 9},
		{[]models.ClaGroupDocument{document_29_newer, document_29}, "document_29_newer", 2, 9},
		{[]models.ClaGroupDocument{document_29, document_29_newer}, "document_29_newer", 2, 9},
		{[]models.ClaGroupDocument{document_29, document_210}, "document_210", 2, 10},
		{[]models.ClaGroupDocument{document_29_newer, document_210}, "document_210", 2, 10},
		{[]models.ClaGroupDocument{document_29, document_210_newer}, "document_210_newer", 2, 10},
		{[]models.ClaGroupDocument{document_29_newer, document_210_newer}, "document_210_newer", 2, 10},
		{[]models.ClaGroupDocument{document_29, document_29_newer, document_210}, "document_210", 2, 10},
		{[]models.ClaGroupDocument{document_29, document_210, document_210_newer}, "document_210_newer", 2, 10},
		{[]models.ClaGroupDocument{document_29, document_29_newer, document_210, document_210_newer}, "document_210_newer", 2, 10},
		{[]models.ClaGroupDocument{document_210, document_210_newer, document_29_newer, document_29}, "document_210_newer", 2, 10},
		{[]models.ClaGroupDocument{document_210, document_15, document_210_newer, document_29_newer, document_29}, "document_210_newer", 2, 10},
		{[]models.ClaGroupDocument{document_210, document_210_newer, document_29_newer, document_30, document_29}, "document_30", 3, 0},
		{[]models.ClaGroupDocument{document_210, document_15, document_210_newer, document_29_newer, document_30, document_29}, "document_30", 3, 0},
		{[]models.ClaGroupDocument{document_15, document_30, document_29}, "document_30", 3, 0},
		{[]models.ClaGroupDocument{document_31, document_310, document_30}, "document_310", 3, 10},
	}

	for _, tt := range tests {
		doc, err := common.GetCurrentDocument(context.Background(), tt.docs)
		assert.Nil(t, err)

		// Check the document name
		assert.Equal(t, tt.expectedName, doc.DocumentName)

		// Check the major version
		major, err := strconv.Atoi(doc.DocumentMajorVersion)
		assert.Nil(t, err)
		assert.Equal(t, tt.expectedMajor, major)

		// Check the minor version
		minor, err := strconv.Atoi(doc.DocumentMinorVersion)
		assert.Nil(t, err)
		assert.Equal(t, tt.expectedMinor, minor)
	}
}

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
