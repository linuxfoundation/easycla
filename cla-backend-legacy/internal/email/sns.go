// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package email

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/google/uuid"
)

// SNSService publishes an email event to an SNS topic.
//
// This is a 1:1 port of cla/models/email_service.py::SnsEmailService.
type SNSService struct {
	client   *sns.Client
	topicARN string
	sender   string
}

func NewSNSFromEnv(ctx context.Context) (*SNSService, error) {
	topicARN := strings.TrimSpace(os.Getenv("SNS_EVENT_TOPIC_ARN"))
	if topicARN == "" {
		return nil, errors.New("SNS_EVENT_TOPIC_ARN is required for EMAIL_SERVICE=SNS")
	}
	sender := strings.TrimSpace(os.Getenv("SES_SENDER_EMAIL_ADDRESS"))
	if sender == "" {
		// Keep parity with Python which expects SES_SENDER_EMAIL_ADDRESS even when using SNS.
		return nil, errors.New("SES_SENDER_EMAIL_ADDRESS is required for EMAIL_SERVICE=SNS")
	}
	client, err := newSNSClientFromEnv(ctx)
	if err != nil {
		return nil, err
	}
	return &SNSService{client: client, topicARN: topicARN, sender: sender}, nil
}

func (s *SNSService) Send(ctx context.Context, subject, body string, recipients []string) error {
	msg, err := buildSNSEmailMessage(subject, body, s.sender, recipients)
	if err != nil {
		return err
	}
	_, err = s.client.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(s.topicARN),
		Message:  aws.String(msg),
	})
	return err
}

func buildSNSEmailMessage(subject, body, sender string, recipients []string) (string, error) {
	// Match legacy payload exactly (field names + version/type).
	// Python source: cla/models/email_service.py::SnsEmailService.get_email_message
	msg := map[string]any{}
	source := map[string]any{}
	data := map[string]any{}

	data["body"] = body
	data["from"] = sender
	data["subject"] = subject
	data["type"] = "cla-email-event"
	data["recipients"] = recipients
	data["template_name"] = "EasyCLA System Email Template"
	data["parameters"] = map[string]any{"BODY": body}

	msg["data"] = data
	source["client_id"] = "easycla-service"
	source["description"] = "EasyCLA Service"
	source["name"] = "EasyCLA Service"
	msg["source_id"] = source
	msg["id"] = uuid.NewString()
	msg["type"] = "cla-email-event"
	msg["version"] = "0.1.0"

	b, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("marshal sns email message: %w", err)
	}
	return string(b), nil
}
