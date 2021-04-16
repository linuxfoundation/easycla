// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testUser = "john"
)

func TestCLAGroupUpdatedEventData_GetEventSummaryString(t *testing.T) {

	testCases := []struct {
		name       string
		eventData  *CLAGroupUpdatedEventData
		summaryStr string
	}{
		{
			name:       "empty",
			eventData:  &CLAGroupUpdatedEventData{},
			summaryStr: "The CLA Group was updated by the user john.",
		},
		{
			name: "only name updated",
			eventData: &CLAGroupUpdatedEventData{
				NewClaGroupName: "updatedNameValue",
			},
			summaryStr: "The CLA Group name was updated to : updatedNameValue by the user john.",
		},
		{
			name: "only name updated but old description still passed",
			eventData: &CLAGroupUpdatedEventData{
				NewClaGroupName:        "updatedNameValue",
				NewClaGroupDescription: "oldDescriptionValue",
				OldClaGroupDescription: "oldDescriptionValue",
			},
			summaryStr: "The CLA Group name was updated to : updatedNameValue by the user john.",
		},
		{
			name: "only description updated",
			eventData: &CLAGroupUpdatedEventData{
				NewClaGroupDescription: "updatedDescriptionValue",
			},
			summaryStr: "The CLA Group description was updated to : updatedDescriptionValue by the user john.",
		},
		{
			name: "only description updated but old name still passed",
			eventData: &CLAGroupUpdatedEventData{
				NewClaGroupDescription: "updatedDescriptionValue",
				NewClaGroupName:        "oldNameValue",
				OldClaGroupName:        "oldNameValue",
			},
			summaryStr: "The CLA Group description was updated to : updatedDescriptionValue by the user john.",
		},
		{
			name: "name and description updated",
			eventData: &CLAGroupUpdatedEventData{
				NewClaGroupName:        "updatedNameValue",
				NewClaGroupDescription: "updatedDescriptionValue",
			},
			summaryStr: "The CLA Group name was updated to : updatedNameValue and the description was updated to : updatedDescriptionValue by the user john.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			summary, _ := tc.eventData.GetEventSummaryString(&LogEventArgs{UserName: testUser})
			assert.Equal(tt, tc.summaryStr, summary)
		})
	}
}

func TestCLAGroupUpdatedEventData_GetEventDetailsString(t *testing.T) {
	projectID := "projectIDValue"

	testCases := []struct {
		name      string
		eventData *CLAGroupUpdatedEventData
		detailStr string
	}{
		{
			name:      "empty",
			eventData: &CLAGroupUpdatedEventData{},
			detailStr: "CLA Group ID: projectIDValue was updated by the user john.",
		},
		{
			name: "only name updated",
			eventData: &CLAGroupUpdatedEventData{
				NewClaGroupName: "updatedNameValue",
				OldClaGroupName: "oldNameValue",
			},
			detailStr: "CLA Group ID: projectIDValue was updated with Name from : oldNameValue to : updatedNameValue by the user john.",
		},
		{
			name: "only name updated but old description still passed",
			eventData: &CLAGroupUpdatedEventData{
				NewClaGroupName:        "updatedNameValue",
				OldClaGroupName:        "oldNameValue",
				NewClaGroupDescription: "oldDescriptionValue",
				OldClaGroupDescription: "oldDescriptionValue",
			},
			detailStr: "CLA Group ID: projectIDValue was updated with Name from : oldNameValue to : updatedNameValue by the user john.",
		},
		{
			name: "only description updated",
			eventData: &CLAGroupUpdatedEventData{
				NewClaGroupDescription: "updatedDescriptionValue",
				OldClaGroupDescription: "oldDescriptionValue",
			},
			detailStr: "CLA Group ID: projectIDValue was updated with Description from : oldDescriptionValue to : updatedDescriptionValue by the user john.",
		},
		{
			name: "only description updated but old name still passed",
			eventData: &CLAGroupUpdatedEventData{
				NewClaGroupDescription: "updatedDescriptionValue",
				OldClaGroupDescription: "oldDescriptionValue",
				NewClaGroupName:        "oldNameValue",
				OldClaGroupName:        "oldNameValue",
			},
			detailStr: "CLA Group ID: projectIDValue was updated with Description from : oldDescriptionValue to : updatedDescriptionValue by the user john.",
		},
		{
			name: "name and description updated",
			eventData: &CLAGroupUpdatedEventData{
				NewClaGroupName:        "updatedNameValue",
				OldClaGroupName:        "oldNameValue",
				NewClaGroupDescription: "updatedDescriptionValue",
				OldClaGroupDescription: "oldDescriptionValue",
			},
			detailStr: "CLA Group ID: projectIDValue was updated with Name from : oldNameValue to : updatedNameValue, Description from : oldDescriptionValue to : updatedDescriptionValue by the user john.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			summary, _ := tc.eventData.GetEventDetailsString(&LogEventArgs{
				UserName:  testUser,
				ProjectID: projectID,
			})
			assert.Equal(tt, tc.detailStr, summary)
		})
	}
}
