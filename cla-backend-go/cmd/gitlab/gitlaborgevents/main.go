// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"os"

	"github.com/aws/aws-lambda-go/events"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
)

// This function is useful for generating dynamodb events for GitlabOrg testing, the output of this function is used as
// ./bin/dynamo-events-lambda-mac {OUTPUT_OF_THIS_FUNCTION}
func main() {
	// authInfo needed for creating the gitlab client
	authInfo := os.Getenv("GITLAB_AUTH_INFO")

	event := events.DynamoDBEvent{
		Records: []events.DynamoDBEventRecord{
			{
				EventSourceArn: "aws:dynamodb/cla-dev-gitlab-orgs",
				EventName:      "MODIFY",
				EventSource:    "aws:dynamodb",
				Change: events.DynamoDBStreamRecord{
					OldImage: map[string]events.DynamoDBAttributeValue{
						"organization_id":           events.NewStringAttribute("4ace6f9f-0518-4621-ae86-d0dacc75af83"),
						"organization_full_path":    events.NewStringAttribute("penguinsoft"),
						"organization_sfid":         events.NewStringAttribute("a092M00001If9uZQAR"),
						"auto_enabled_cla_group_id": events.NewStringAttribute("40af3652-e8bf-489d-a917-cb2214a89640"),
						"external_gitlab_group_id":  events.NewNumberAttribute("12700028"),
						"auth_state":                events.NewStringAttribute("18eb90d1-8c36-4962-ba91-e264ccbcab3a"),
						"organization_url":          events.NewStringAttribute("https://gitlab.com/groups/penguinsoft"),
						"auth_info":                 events.NewStringAttribute(authInfo),
						"organization_name_lower":   events.NewStringAttribute("penguinsoft"),
						"project_sfid":              events.NewStringAttribute("a092M00001If9uZQAR"),
						"organization_name":         events.NewStringAttribute("penguinsoft"),
						"enabled":                   events.NewBooleanAttribute(false),
						"auto_enabled":              events.NewBooleanAttribute(false),
						"branch_protection_enabled": events.NewBooleanAttribute(false),
					},
					NewImage: map[string]events.DynamoDBAttributeValue{
						"organization_id":           events.NewStringAttribute("4ace6f9f-0518-4621-ae86-d0dacc75af83"),
						"organization_full_path":    events.NewStringAttribute("penguinsoft"),
						"organization_sfid":         events.NewStringAttribute("a092M00001If9uZQAR"),
						"auto_enabled_cla_group_id": events.NewStringAttribute("40af3652-e8bf-489d-a917-cb2214a89640"),
						"external_gitlab_group_id":  events.NewNumberAttribute("12700028"),
						"auth_state":                events.NewStringAttribute("18eb90d1-8c36-4962-ba91-e264ccbcab3a"),
						"organization_url":          events.NewStringAttribute("https://gitlab.com/groups/penguinsoft"),
						"auth_info":                 events.NewStringAttribute(authInfo),
						"organization_name_lower":   events.NewStringAttribute("penguinsoft"),
						"project_sfid":              events.NewStringAttribute("a092M00001If9uZQAR"),
						"organization_name":         events.NewStringAttribute("penguinsoft"),
						"enabled":                   events.NewBooleanAttribute(true),
						"auto_enabled":              events.NewBooleanAttribute(false),
						"branch_protection_enabled": events.NewBooleanAttribute(true),
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
