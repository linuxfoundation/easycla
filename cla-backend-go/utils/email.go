// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"bytes"
	"errors"
	"html/template"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/communitybridge/easycla/cla-backend-go/config"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

// EmailSender contains method to send email
type EmailSender interface {
	SendEmail(subject string, body string, recipients []string) error
}

var emailSender EmailSender

// SetEmailSender sets up default email sender
func SetEmailSender(es EmailSender) {
	emailSender = es
}

// GetEmailSender returns back the current email sender
func GetEmailSender() EmailSender {
	return emailSender
}

// MockEmailSender useful when working with tests
type MockEmailSender struct{}

// SendEmail does nothing
func (m *MockEmailSender) SendEmail(subject string, body string, recipients []string) error {
	return nil
}

type snsEmail struct {
	snsClient          *sns.SNS
	snsEventTopicARN   string
	senderEmailAddress string
}

// SetSnsEmailSender set sns as mechanism to send email
func SetSnsEmailSender(awsSession *session.Session, snsEventTopicARN string, senderEmailAddress string) {
	emailSender = &snsEmail{
		snsClient:          sns.New(awsSession),
		snsEventTopicARN:   snsEventTopicARN,
		senderEmailAddress: senderEmailAddress,
	}
}

// SendEmail sends an email to the specified recipients
func (s *snsEmail) SendEmail(subject string, body string, recipients []string) error {
	f := logrus.Fields{
		"functionName": "utils.SendEmail",
		"subject":      subject,
		"recipients":   strings.Join(recipients, ","),
	}
	event := CreateEventWrapper("cla-email-event")
	event.Data = ToEmailTemplateEvent(&s.senderEmailAddress, recipients, &subject, &body, "EasyCLA System Email Template")

	b, err := event.MarshalBinary()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("Unable to marshal event model")
		return err
	}

	log.Debugf("Sending SNS message to topic: '%s'", s.snsEventTopicARN)
	input := &sns.PublishInput{
		Message:  aws.String(string(b)),          // Required
		TopicArn: aws.String(s.snsEventTopicARN), // Required
	}

	sendResp, err := s.snsClient.Publish(input)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("Error publishing message to topic: %s", s.snsEventTopicARN)
		return err
	}

	log.WithFields(f).Debugf("Successfully sent SNS message. Response ID: %s", StringValue(sendResp.MessageId))
	return nil
}

// SendEmail function send email. It uses emailSender interface.
func SendEmail(subject string, body string, recipients []string) error {
	if emailSender == nil {
		return errors.New("email sender not set")
	}
	return emailSender.SendEmail(subject, body, recipients)
}

// GetCorporateURL returns the corporate URL based on the specified flag
func GetCorporateURL(isV2Project bool) string {
	if isV2Project {
		return config.GetConfig().CorporateConsoleV2URL
	}
	return config.GetConfig().CorporateConsoleV1URL
}

// GetEmailHelpContent returns the standard email help paragraph details.
func GetEmailHelpContent(showV2HelpLink bool) string {
	// We only support v2 help links as of late 2021/early2022
	helpLinkInfo := `<p>If you need help or have questions about EasyCLA, you can
<a href="https://docs.linuxfoundation.org/lfx/easycla" target="_blank">read the documentation</a> or
<a href="https://jira.linuxfoundation.org/servicedesk/customer/portal/4/create/143" target="_blank">reach out to us for
support</a>.</p>`
	if showV2HelpLink {
		return helpLinkInfo
	}

	return helpLinkInfo
}

// GetEmailSignOffContent returns the standard email sign-off details
func GetEmailSignOffContent() string {
	return `<p>EasyCLA Support Team</p>`
}

// RenderTemplate renders the template for given template with given params
func RenderTemplate(claGroupVersion, templateName, templateStr string, params interface{}) (string, error) {
	tmpl := template.New(templateName)
	t, err := tmpl.Parse(templateStr)
	if err != nil {
		return "", err
	}

	var tpl bytes.Buffer
	if err := t.Execute(&tpl, params); err != nil {
		return "", err
	}

	result := tpl.String()
	result = result + GetEmailHelpContent(claGroupVersion == V2)
	result = result + GetEmailSignOffContent()
	return result, nil
}
