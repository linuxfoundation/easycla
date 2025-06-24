// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	// "context"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	eventOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/events"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	// "github.com/linuxfoundation/easycla/cla-backend-go/users"
)

var awsSession = session.Must(session.NewSession(&aws.Config{}))
var stage string

func main() {
	stage = os.Getenv("STAGE")
	if stage == "" {
		log.Fatal("stage not set")
	}
	log.Infof("STAGE set to %s\n", stage)

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Initialize the events repository
	eventsRepo := events.NewRepository(awsSession, stage)
	eventService := events.NewService(eventsRepo, nil)

	// Initialize the users repository
	// usersRepo := users.NewRepository(awsSession, stage)

	inputFilename := flag.String("input-file", "", "Input with a given list of lf usernames")
	claGroup := flag.String("cla-group-id", "", "The ID of the CLA group")
	claGroupName := flag.String("cla-group-name", "", "The name of the CLA group")
	flag.Parse()

	if *inputFilename == "" || *claGroup == "" {
		log.Fatalf("Both input-file and cla-group are required")
	}

	log.Debugf("Input file: %s", *inputFilename)

	file, err := os.Open(*inputFilename)
	if err != nil {
		log.Fatalf("Unable to read input file: %s", *inputFilename)
	}

	defer func() {
		if err = file.Close(); err != nil {
			log.Fatalf("Error closing file: %v", err)
		}
	}()

	reader := csv.NewReader(file)

	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("Unable to read file")
	}

	log.Debugf("CLA Group Name: %s", *claGroup)

	type Report struct {
		Username string
		Events   []*models.Event
	}

	projectReport := make([]Report, 0)

	for i, record := range records {
		if i == 0 {
			continue
		}
		lfUsername := record[0]
		log.Debugf("Processing record: %s", lfUsername)
		report := Report{
			Username: lfUsername,
		}

		// Increment the wait group
		wg.Add(1)

		go func(lfusername string) {
			defer wg.Done()
			log.Debugf("Processing record: %s", lfusername)
			searchParams := eventOps.SearchEventsParams{
				SearchTerm: &lfusername,
				ProjectID:  claGroup,
			}
			events, eventErr := eventService.SearchEvents(&searchParams)
			if eventErr != nil {
				log.Debugf("Error getting events: %v", eventErr)
				report.Events = nil
			}

			if len(events.Events) == 0 {
				log.Warnf("No events found for user: %s", lfusername)
				report.Events = nil
			} else {
				log.Debugf("Events found for user: %s", lfusername)
				report.Events = events.Events
			}

			mu.Lock()
			projectReport = append(projectReport, report)
			defer mu.Unlock()

		}(lfUsername)
	}

	// Wait for all the go routines to finish
	wg.Wait()

	// Create a csv file with the results
	outputFilename := fmt.Sprintf("ldap-%s-%s.csv", *claGroupName, time.Now().Format("2006-01-02-15-04-05"))
	outputFile, err := os.Create(filepath.Clean(outputFilename))

	if err != nil {
		log.Fatalf("Unable to create output file: %s", outputFilename)
	}

	defer func() {
		if err = outputFile.Close(); err != nil {
			log.Fatalf("Error closing file: %v", err)
		}
	}()

	writer := csv.NewWriter(outputFile)

	err = writer.Write([]string{"Username", "Event ID", "Event Data", "Event Type", "Event Date"})
	if err != nil {
		log.Fatalf("Error writing csv: %v", err)
	}

	for _, report := range projectReport {
		if report.Events == nil {
			err = writer.Write([]string{report.Username, "No events found", "", "", ""})
			if err != nil {
				log.Fatalf("Error writing csv: %v", err)
			}
			continue
		}
		for _, event := range report.Events {
			err = writer.Write([]string{report.Username, event.EventID, event.EventData, event.EventType, event.EventTime})
			if err != nil {
				log.Fatalf("Error writing csv: %v", err)
			}

		}
	}

	writer.Flush()

	if err := writer.Error(); err != nil {
		log.Fatalf("Error writing csv: %v", err)
	}

	log.Infof("Output written to: %s", outputFilename)

}
