// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/go-openapi/swag"

	"github.com/davecgh/go-spew/spew"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/linuxfoundation/easycla/cla-backend-go/token"

	"github.com/linuxfoundation/easycla/cla-backend-go/config"

	"github.com/LF-Engineering/lfx-models/models/stats"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	awsqs "github.com/aws/aws-sdk-go/service/sqs"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/metrics"
	v2ProjectService "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service"
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
	v2ProjectService.InitClient(configFile.APIGatewayURL)
}

func handler(ctx context.Context, event events.CloudWatchEvent) {
	f := logrus.Fields{
		"functionName": "handler",
		"eventID":      event.ID,
		"eventVersion": event.Version,
	}

	totalCountMetrics, err := metricsRepo.GetTotalCountMetrics()
	if err != nil {
		log.WithFields(f).WithError(err).Fatal("unable to get totalCount metrics from dynamodb.")
	}

	req := stats.Request{
		Products: &stats.Products{
			EasyCLA: stats.Stats{
				"cla_contributors": stats.Stat{
					Action:    swag.String(stats.StatActionReplace),
					Frequency: swag.String(stats.StatFrequencyAllTime),
					Type:      swag.String(stats.StatTypeNumber),
					Value:     float64(totalCountMetrics.ContributorsCount),
				},
				"clas_signed": stats.Stat{
					Action:    swag.String(stats.StatActionReplace),
					Frequency: swag.String(stats.StatFrequencyAllTime),
					Type:      swag.String(stats.StatTypeNumber),
					Value:     float64(totalCountMetrics.CLAsSignedCount),
				},
				"live_projects": stats.Stat{
					Action:    swag.String(stats.StatActionReplace),
					Frequency: swag.String(stats.StatFrequencyAllTime),
					Type:      swag.String(stats.StatTypeNumber),
					Value:     float64(totalCountMetrics.ProjectsLiveCount),
				},
				// repositories = GitHub repositories + Gerrit Instances (not repos) <--- under-counting gerrit repos
				"repositories_covered": stats.Stat{
					Action:    swag.String(stats.StatActionReplace),
					Frequency: swag.String(stats.StatFrequencyAllTime),
					Type:      swag.String(stats.StatTypeNumber),
					Value:     float64(totalCountMetrics.GerritRepositoriesEnabledCount + totalCountMetrics.GithubRepositoriesEnabledCount),
				},
			},
		},
		Project: "",
	}

	dataInBytes, err := req.MarshalBinary()
	if err != nil {
		log.WithFields(f).WithError(err).Fatal("marshall the sqs event failed")
	}

	log.WithFields(f).Debugf("going to send total count metrics to sqs queue %s", spew.Sdump(req))
	conf := config.GetConfig()

	var region, queueURL string
	if conf.MetricsReport.AwsSQSRegion == "" {
		log.WithFields(f).Fatal("aws sqs region config is missing")
	}
	region = conf.MetricsReport.AwsSQSRegion

	if conf.MetricsReport.AwsSQSQueueURL == "" {
		log.WithFields(f).Fatal("aws sqs queue url config is missing")
	}
	queueURL = conf.MetricsReport.AwsSQSQueueURL

	sqsSession := awsqs.New(session.Must(session.NewSession(&aws.Config{
		Region: aws.String(region),
	})))

	input := &awsqs.SendMessageInput{
		MessageBody: aws.String(string(dataInBytes)),
		QueueUrl:    aws.String(queueURL),
	}

	if conf.MetricsReport.Enabled {
		_, err = sqsSession.SendMessage(input)
		if err != nil {
			log.WithFields(f).WithError(err).Fatal("sending the message to sqs failed")
			return
		}
		log.WithFields(f).Infof("metrics report sent successfully to queue %s", queueURL)
	} else {
		log.WithFields(f).Info("metrics report not sent - disabled in the configuration")
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
