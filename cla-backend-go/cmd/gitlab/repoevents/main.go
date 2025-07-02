// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"

	"github.com/aws/aws-lambda-go/events"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
)

// This function is useful for generating dynamodb events for Repository testing, the output of this function is used as
// ./bin/dynamo-events-lambda-mac {OUTPUT_OF_THIS_FUNCTION}
func main() {
	event := events.DynamoDBEvent{
		Records: []events.DynamoDBEventRecord{
			{
				EventSourceArn: "aws:dynamodb/cla-dev-repositories",
				EventName:      "MODIFY",
				EventSource:    "aws:dynamodb",
				Change: events.DynamoDBStreamRecord{
					OldImage: map[string]events.DynamoDBAttributeValue{
						"repository_id":                events.NewStringAttribute("1fa3de39-8274-4750-ba7c-242d5d659dd1"),
						"repository_name":              events.NewStringAttribute("easycla-gitlab-test"),
						"repository_organization_name": events.NewStringAttribute("penguinsoft"),
						"repository_project_id":        events.NewStringAttribute("40af3652-e8bf-489d-a917-cb2214a89640"),
						"repository_sfdc_id":           events.NewStringAttribute("a092M00001If9uZQAR"),
						"project_sfid":                 events.NewStringAttribute("a092M00001If9uZQAR"),
						"repository_external_id":       events.NewNumberAttribute("28893091"),
						"repository_type":              events.NewStringAttribute("gitlab"),
						"enabled":                      events.NewBooleanAttribute(false),
					},
					NewImage: map[string]events.DynamoDBAttributeValue{
						"repository_id":                events.NewStringAttribute("1fa3de39-8274-4750-ba7c-242d5d659dd1"),
						"repository_name":              events.NewStringAttribute("easycla-gitlab-test"),
						"repository_organization_name": events.NewStringAttribute("penguinsoft"),
						"repository_project_id":        events.NewStringAttribute("40af3652-e8bf-489d-a917-cb2214a89640"),
						"repository_sfdc_id":           events.NewStringAttribute("a092M00001If9uZQAR"),
						"project_sfid":                 events.NewStringAttribute("a092M00001If9uZQAR"),
						"repository_external_id":       events.NewNumberAttribute("28893091"),
						"repository_type":              events.NewStringAttribute("gitlab"),
						"enabled":                      events.NewBooleanAttribute(true),
					},
				}},
		},
	}

	b, err := json.Marshal(event)
	if err != nil {
		log.Fatalf("marshall : %v", err)
	}

	log.Println(string(b))
}
