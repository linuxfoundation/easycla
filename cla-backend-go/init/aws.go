// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package init

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
)

var (
	// AWS
	awsRegion            string
	awsSession           *session.Session
	awsCloudWatchService *cloudwatch.CloudWatch
)

// AWSInit initialization logic for the AWS resources
func AWSInit() {
	awsRegion = GetProperty("DYNAMODB_AWS_REGION")

	if err := startCloudWatchSession(); err != nil {
		log.Fatalf("Error starting the AWS CloudWatch session - Error: %s", err.Error())
	}
}

// GetAWSSession returns an AWS session based on the region and credentials
func GetAWSSession() (*session.Session, error) {
	if awsSession == nil {
		log.Debugf("Creating a new AWS session for region: %s", awsRegion)
		/*
			ses, err := session.NewSession(&aws.Config{
				Region:      aws.String(awsRegion),
				Credentials: credentials.NewStaticCredentials(awsAccessKeyID, awsSecretAccessKey, ""),
				MaxRetries:  aws.Int(5),
			})
		*/
		awsSession = session.Must(session.NewSession(
			&aws.Config{
				Region:                        aws.String(awsRegion),
				CredentialsChainVerboseErrors: aws.Bool(true),
				MaxRetries:                    aws.Int(5),
			},
		))

		log.Debugf("Successfully created a new AWS session for region: %s...", awsRegion)
	}

	return awsSession, nil
}

// startCloudWatchSession creates a new AWS CloudWatch service session
func startCloudWatchSession() error {
	sess, err := GetAWSSession()
	if err != nil {
		log.Fatal("Error creating a new AWS Session", err)
		return err
	}

	awsCloudWatchService = cloudwatch.New(sess)

	log.Info("CloudWatch service started")

	return nil
}

// GetAWSCloudWatchService returns the CloudWatch service client
func GetAWSCloudWatchService() *cloudwatch.CloudWatch {
	return awsCloudWatchService
}
