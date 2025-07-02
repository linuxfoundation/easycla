// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	eventOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/events"
	"github.com/stretchr/testify/assert"
)

func TestEventsService(t *testing.T) {

	mockRepo := events.NewMockRepository()
	eventsMockRepo := mockRepo
	combinedMockRepo := mockRepo
	eventsService := events.NewService(eventsMockRepo, combinedMockRepo)

	eventsService.LogEvent(&events.LogEventArgs{
		EventType: events.GitHubOrganizationAdded,
		ProjectID: "project-1234",
		CompanyID: "company-1234",
		UserID:    "testUserID",
		EventData: &events.GitHubOrganizationAddedEventData{GitHubOrganizationName: "testorg"},
	})

	eventsSearch, err := eventsService.SearchEvents(&eventOps.SearchEventsParams{
		ProjectID: aws.String("project-1234"),
		CompanyID: aws.String("company-1234"),
	})
	assert.Nil(t, err, "Error is nil")
	assert.Equal(t, 1, len(eventsSearch.Events))
}
