// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"time"

	emailevent "github.com/LF-Engineering/lfx-models/models/email"
	"github.com/LF-Engineering/lfx-models/models/event"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
)

// Service interface defines methods of event service
type Service interface {
}

type service struct {
}

// NewService creates new instance of event service
func NewService() Service {
	return &service{}
}

// CreateEventWrapper creates a new event with the specified event type
func CreateEventWrapper(eventType string) *event.Event {
	now := time.Now().UTC()
	openAPITime := getCurrentDateTime()
	err := openAPITime.Scan(now)
	if err != nil {
		log.Warnf("error while converting/scanning strfmt time: %v", err)
		return nil
	}

	return &event.Event{
		ID:      uuid.New().String(),
		Version: "0.1.0",
		Type:    eventType,
		Created: openAPITime,
		SourceID: &event.Source{
			Name:        "EasyCLA Service",
			Description: "EasyCLA Service",
			ClientID:    "easycla-service",
		},
	}
}

// ToEmailEvent creates an email event model from the email details
func ToEmailEvent(sender *string, recipients []string, subject *string, emailBody *string) *emailevent.EmailEvent {

	// Convert the string array to an array of strfmt email recipients
	var emailRecipients = make([]strfmt.Email, len(recipients))
	for i, recipient := range recipients {
		emailRecipients[i] = strfmt.Email(recipient)
	}

	log.Debug("Generating email event...")
	_, nowAsString := CurrentTime()
	from := strfmt.Email(*sender)
	return &emailevent.EmailEvent{
		From:       &from,
		Recipients: emailRecipients,
		Subject:    subject,
		Body:       emailBody,
		Type:       "cla-email-event",
		CreatedOn:  nowAsString,
	}
}

// ToEmailTemplateEvent creates an email event model from the email details
func ToEmailTemplateEvent(sender *string, recipients []string, subject *string, emailBody *string, templateName string) *emailevent.EmailEvent {

	// Convert the string array to an array of strfmt email recipients
	var emailRecipients = make([]strfmt.Email, len(recipients))
	for i, recipient := range recipients {
		emailRecipients[i] = strfmt.Email(recipient)
	}

	// log.Debug("Generating email template event...")
	_, nowAsString := CurrentTime()
	from := strfmt.Email(*sender)
	return &emailevent.EmailEvent{
		From:         &from,
		Recipients:   emailRecipients,
		Subject:      subject,
		Body:         emailBody,
		Type:         "cla-email-event",
		Parameters:   map[string]interface{}{"BODY": emailBody},
		TemplateName: templateName,
		CreatedOn:    nowAsString,
	}
}

// getCurrentDateTime returns the current date time as a strfmt.DateTime object
func getCurrentDateTime() strfmt.DateTime {
	// Grab the current time as a strfmt date/time
	now := time.Now().UTC()
	openAPITime := strfmt.NewDateTime()
	err := openAPITime.Scan(now)
	if err != nil {
		log.Warnf("error while converting/scanning strfmt time: %v", err)
	}

	return openAPITime
}
