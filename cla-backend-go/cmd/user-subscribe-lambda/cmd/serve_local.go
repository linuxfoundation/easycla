//go:build !aws_lambda
// +build !aws_lambda

// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-lambda-go/events"
	log "github.com/sirupsen/logrus"
)

func postSQSEvent(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.Method != http.MethodPost {
		log.Println("404 not found.")
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	dataByte, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println("Failed to read body")
		http.Error(w, "Failed to read body", http.StatusInternalServerError)
	}

	var sEvent events.SNSEvent
	sEvent.Records = append(sEvent.Records, events.SNSEventRecord{
		EventVersion:         "1.0",
		EventSubscriptionArn: "",
		EventSource:          "localhost",
		SNS: events.SNSEntity{
			Signature:         "",
			MessageID:         "",
			Type:              "",
			TopicArn:          "",
			MessageAttributes: nil,
			SignatureVersion:  "",
			Timestamp:         time.Now().UTC(),
			SigningCertURL:    "",
			Message:           string(dataByte),
			UnsubscribeURL:    "",
			Subject:           "",
		},
	})

	err = handler(r.Context(), sEvent)
	if err != nil {
		log.Warnf("Failed to process event in handler. Error: %v", err)
		http.Error(w, "Failed to process event", http.StatusInternalServerError)
	}
}

type fn func(ctx context.Context, sqsEvent events.SNSEvent) error

var handler fn

// Start lambda handler function
func Start(hf fn) error {
	handler = hf
	http.HandleFunc("/", postSQSEvent)

	fmt.Printf("Starting server for testing HTTP POST...\n")
	if err := http.ListenAndServe(":8080", nil); err != nil { // nolint gosec http no support for setting timeouts
		log.Fatal(err)
	}
	return nil
}
