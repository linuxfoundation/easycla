// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package health

import (
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/service/dynamodb"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	ini "github.com/linuxfoundation/easycla/cla-backend-go/init"
)

// Service provides an API to the health API
type Service struct {
	version   string
	commit    string
	branch    string
	buildDate string
}

// HealthService interface
type HealthService interface { // nolint
	HealthCheck(ctx context.Context) (*models.Health, error)
}

// New is a simple helper function to create a health service instance
func New(version, commit, branch, buildDate string) Service {
	return Service{
		version:   version,
		commit:    commit,
		branch:    branch,
		buildDate: buildDate,
	}
}

// HealthCheck API call returns the current health of the service
func (s Service) HealthCheck(ctx context.Context) (*models.Health, error) {

	// General health
	hs := models.HealthStatus{
		TimeStamp: time.Now().UTC().Format(time.RFC3339),
		Healthy:   true,
		Name:      "CLA",
		Duration:  time.Since(time.Now()).String(),
	}

	var allStatus []*models.HealthStatus
	allStatus = append(allStatus, &hs)
	allStatus = append(allStatus, getDynamoTableStatus()...)

	var status = "healthy"
	for _, item := range allStatus {
		// If any of our dynamodb tables are not healthy, then overall we are not healthy
		if !item.Healthy {
			status = "not healthy"
			break
		}
	}

	response := models.Health{
		Status:         status,
		TimeStamp:      time.Now().UTC().Format(time.RFC3339),
		Version:        s.version,
		Githash:        s.commit,
		Branch:         s.branch,
		BuildTimeStamp: s.buildDate,
		Healths:        allStatus,
	}

	return &response, nil
}

// getDynamoTableStatus queries the dynamodb tables and reports if it is healthy
func getDynamoTableStatus() []*models.HealthStatus {
	var allStatus []*models.HealthStatus

	tableNames := []string{
		"cla-" + ini.GetStage() + "-ccla-whitelist-requests",
		"cla-" + ini.GetStage() + "-cla-manager-requests",
		"cla-" + ini.GetStage() + "-companies",
		"cla-" + ini.GetStage() + "-company-invites",
		"cla-" + ini.GetStage() + "-events",
		"cla-" + ini.GetStage() + "-gerrit-instances",
		"cla-" + ini.GetStage() + "-github-orgs",
		"cla-" + ini.GetStage() + "-metrics",
		"cla-" + ini.GetStage() + "-projects",
		"cla-" + ini.GetStage() + "-projects-cla-groups",
		"cla-" + ini.GetStage() + "-repositories",
		"cla-" + ini.GetStage() + "-session-store",
		"cla-" + ini.GetStage() + "-signatures",
		"cla-" + ini.GetStage() + "-store",
		"cla-" + ini.GetStage() + "-user-permissions",
		"cla-" + ini.GetStage() + "-users",
	}

	var wg sync.WaitGroup
	wg.Add(len(tableNames))

	for _, tableName := range tableNames {
		go func(tableName string) {
			defer wg.Done()

			// Do a quick check to see if we have a database connection
			dynamoNow := time.Now()
			dynamoAlive := isDynamoAlive(tableName)
			dynamoDuration := time.Since(dynamoNow)
			dy := models.HealthStatus{TimeStamp: time.Now().UTC().Format(time.RFC3339),
				Healthy:  dynamoAlive,
				Name:     "EasyCLA - Dynamodb - " + tableName,
				Duration: dynamoDuration.String()}

			allStatus = append(allStatus, &dy)
		}(tableName)
	}

	wg.Wait()

	return allStatus
}

// isDynamoAlive runs a check to see if we have connectivity to the database for the given table - returns true if successful, false otherwise
func isDynamoAlive(tableName string) bool {
	// Grab the AWS session
	awsSession, err := ini.GetAWSSession()
	if err != nil {
		log.Warnf("Unable to acquire AWS session - returning failed health, error: %v", err)
		return false
	}

	// Create a client and make a query - don't worry about the result - just check the error response
	dynamoDBClient := dynamodb.New(awsSession)
	_, err = dynamoDBClient.DescribeTable(&dynamodb.DescribeTableInput{
		TableName: &tableName,
	})

	// No error is success
	return err == nil
}
