// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api_logs

import (
	"fmt"
	"time"
)

// APILog data model for DynamoDB table cla-{stage}-api-log
type APILog struct {
	URL    string `dynamodbav:"url" json:"url"`
	DT     int64  `dynamodbav:"dt" json:"dt"`
	Bucket string `dynamodbav:"bucket" json:"bucket"`
}

// String returns a string representation of the APILog
func (a *APILog) String() string {
	return fmt.Sprintf("APILog{URL: %s, DT: %d, Bucket: %s}", a.URL, a.DT, a.Bucket)
}

// NewAPILog creates a new APILog entry with current timestamp
func NewAPILog(url, bucket string) *APILog {
	return &APILog{
		URL:    url,
		DT:     time.Now().UnixMilli(), // Unix timestamp in milliseconds
		Bucket: bucket,
	}
}
