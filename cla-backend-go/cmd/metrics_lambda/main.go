// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"os"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/linuxfoundation/easycla/cla-backend-go/token"

	"github.com/linuxfoundation/easycla/cla-backend-go/config"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/metrics"
	project_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service"
)

var (
	// version the application version
	version string

	// build/Commit the application build number
	commit string

	// branch the build branch
	branch string

	// build date
	buildDate string
)

var awsSession = session.Must(session.NewSession(&aws.Config{}))
var metricsRepo metrics.Repository
var stage string

func init() {
	stage = os.Getenv("STAGE")
	if stage == "" {
		log.Fatal("stage not set")
	}
	log.Infof("STAGE set to %s\n", stage)
	configFile, err := config.LoadConfig("", awsSession, stage)
	if err != nil {
		log.Panicf("Unable to load config - Error: %v", err)
	}
	pcgRepo := projects_cla_groups.NewRepository(awsSession, stage)
	metricsRepo = metrics.NewRepository(awsSession, stage, configFile.APIGatewayURL, pcgRepo)
	token.Init(configFile.Auth0Platform.ClientID, configFile.Auth0Platform.ClientSecret, configFile.Auth0Platform.URL, configFile.Auth0Platform.Audience)
	project_service.InitClient(configFile.APIGatewayURL)
}

func handler(ctx context.Context, event events.CloudWatchEvent) {
	err := metricsRepo.CalculateAndSaveMetrics()
	if err != nil {
		log.Fatalf("Unable to save metrics in dynamodb. error = %s", err)
	}
}

func printBuildInfo() {
	log.Infof("Version                 : %s", version)
	log.Infof("Git commit hash         : %s", commit)
	log.Infof("Branch                  : %s", branch)
	log.Infof("Build date              : %s", buildDate)
}

func main() {
	log.Info("Lambda server starting...")
	printBuildInfo()
	if os.Getenv("LOCAL_MODE") == "true" {
		handler(utils.NewContext(), events.CloudWatchEvent{})
	} else {
		lambda.Start(handler)
	}
	log.Infof("Lambda shutting down...")
}
