// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package email

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

// SESService sends an email using Amazon SES.
//
// This mirrors cla/models/email_service.py::SesEmailService.
type SESService struct {
	client *ses.Client
	sender string
}

func NewSESFromEnv(ctx context.Context) (*SESService, error) {
	sender := strings.TrimSpace(os.Getenv("SES_SENDER_EMAIL_ADDRESS"))
	if sender == "" {
		return nil, errors.New("SES_SENDER_EMAIL_ADDRESS is required for EMAIL_SERVICE=SES")
	}
	client, err := newSESClientFromEnv(ctx)
	if err != nil {
		return nil, err
	}
	return &SESService{client: client, sender: sender}, nil
}

func (s *SESService) Send(ctx context.Context, subject, body string, recipients []string) error {
	if len(recipients) == 0 {
		return nil
	}
	_, err := s.client.SendEmail(ctx, &ses.SendEmailInput{
		Source: aws.String(s.sender),
		Destination: &types.Destination{
			ToAddresses: recipients,
		},
		Message: &types.Message{
			Subject: &types.Content{Data: aws.String(subject)},
			Body: &types.Body{
				Text: &types.Content{Data: aws.String(body)},
			},
		},
	})
	return err
}
