// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// configLookupResponse is a channel response model for the configuration lookup
type configLookupResponse struct {
	key   string
	value string
}

// getSSMString is a generic routine to fetch the specified key value
func getSSMString(ssmClient *ssm.SSM, key string) (string, error) {
	value, err := ssmClient.GetParameter(&ssm.GetParameterInput{
		Name:           aws.String(key),
		WithDecryption: aws.Bool(false),
	})
	if err != nil {
		log.Warnf("unable to read SSM parameter %s - error: %+v", key, err)
		return "", err
	}

	return strings.TrimSpace(*value.Parameter.Value), nil
}

// loadSSMConfig fetches all the configuration values and populates the response Config model
func loadSSMConfig(awsSession *session.Session, stage string) Config { //nolint
	f := logrus.Fields{
		"functionName": "loadSSMConfig",
		"stage":        stage,
	}
	config := Config{}
	config.SignatureQueryDefaultValue = "all"

	ssmClient := ssm.New(awsSession)

	// AWS Region
	config.AWS.Region = *awsSession.Config.Region

	// A channel for the responses from the go routines
	responseChannel := make(chan configLookupResponse)

	// Our list of keys to lookup
	ssmKeys := []string{
		fmt.Sprintf("cla-auth0-domain-%s", stage),
		fmt.Sprintf("cla-auth0-clientId-%s", stage),
		fmt.Sprintf("cla-auth0-username-claim-%s", stage),
		fmt.Sprintf("cla-auth0-algorithm-%s", stage),
		fmt.Sprintf("cla-gh-oauth-client-id-go-backend-%s", stage),
		fmt.Sprintf("cla-gh-oauth-secret-go-backend-%s", stage),
		fmt.Sprintf("cla-gh-access-token-%s", stage),
		fmt.Sprintf("cla-gh-app-id-%s", stage),
		fmt.Sprintf("cla-gh-app-private-key-%s", stage),
		fmt.Sprintf("cla-gh-test-organization-%s", stage),
		fmt.Sprintf("cla-gh-test-organization-installation-id-%s", stage),
		fmt.Sprintf("cla-gh-test-repository-%s", stage),
		fmt.Sprintf("cla-gh-test-repository-id-%s", stage),
		//fmt.Sprintf("cla-gitlab-oauth-secret-go-backend-%s", stage),
		fmt.Sprintf("cla-gitlab-app-id-%s", stage),
		fmt.Sprintf("cla-gitlab-app-secret-%s", stage),
		fmt.Sprintf("cla-gitlab-app-private-key-%s", stage),
		fmt.Sprintf("cla-gitlab-app-redirect-uri-%s", stage),
		fmt.Sprintf("cla-gitlab-app-web-hook-uri-%s", stage),
		fmt.Sprintf("cla-corporate-base-%s", stage),
		fmt.Sprintf("cla-corporate-v1-base-%s", stage),
		fmt.Sprintf("cla-corporate-v2-base-%s", stage),
		fmt.Sprintf("cla-contributor-v2-base-%s", stage),
		fmt.Sprintf("cla-doc-raptor-api-key-%s", stage),
		fmt.Sprintf("cla-session-store-table-%s", stage),
		fmt.Sprintf("cla-ses-sender-email-address-%s", stage),
		fmt.Sprintf("cla-allowed-origins-%s", stage),
		fmt.Sprintf("cla-sns-event-topic-arn-%s", stage),
		fmt.Sprintf("cla-signature-files-bucket-%s", stage),
		fmt.Sprintf("cla-auth0-platform-client-id-%s", stage),
		fmt.Sprintf("cla-auth0-platform-client-secret-%s", stage),
		fmt.Sprintf("cla-auth0-platform-audience-%s", stage),
		fmt.Sprintf("cla-auth0-platform-url-%s", stage),
		fmt.Sprintf("cla-auth0-platform-api-gw-%s", stage),
		fmt.Sprintf("cla-lf-group-client-id-%s", stage),
		fmt.Sprintf("cla-lf-group-client-secret-%s", stage),
		fmt.Sprintf("cla-lf-group-client-url-%s", stage),
		fmt.Sprintf("cla-lf-group-refresh-token-%s", stage),
		fmt.Sprintf("cla-v1-api-url-%s", stage),
		fmt.Sprintf("cla-acs-api-key-%s", stage),
		fmt.Sprintf("cla-lfx-portal-url-%s", stage),
		fmt.Sprintf("cla-lfx-metrics-report-sqs-region-%s", stage),
		fmt.Sprintf("cla-lfx-metrics-report-sqs-url-%s", stage),
		fmt.Sprintf("cla-lfx-metrics-report-enabled-%s", stage),
		fmt.Sprintf("cla-enable-services-for-parent-%s", stage),
		fmt.Sprintf("cla-signature-query-default-%s", stage),
		fmt.Sprintf("cla-platform-api-gw-%s", stage),
		fmt.Sprintf("cla-api-v4-base-%s", stage),
		fmt.Sprintf("cla-landing-page-%s", stage),
		fmt.Sprintf("cla-logo-url-%s", stage),
		fmt.Sprintf("cla-docusign-private-key-%s", stage),
	}

	// For each key to lookup
	for _, key := range ssmKeys {
		// Create a go routine to this concurrently
		go func(theKey string) {
			theValue, err := getSSMString(ssmClient, theKey)
			if err != nil {
				log.WithFields(f).WithError(err).Fatalf("error looking up key: %s", theKey)
			}
			// Send the response back through the channel
			responseChannel <- configLookupResponse{
				key:   theKey,
				value: theValue,
			}
		}(key)
	}

	for i := 0; i < len(ssmKeys); i++ {
		resp := <-responseChannel
		switch resp.key {
		case fmt.Sprintf("cla-auth0-domain-%s", stage):
			config.Auth0.Domain = resp.value
		case fmt.Sprintf("cla-auth0-clientId-%s", stage):
			config.Auth0.ClientID = resp.value
		case fmt.Sprintf("cla-auth0-username-claim-%s", stage):
			config.Auth0.UsernameClaim = resp.value
		case fmt.Sprintf("cla-auth0-algorithm-%s", stage):
			config.Auth0.Algorithm = resp.value
		case fmt.Sprintf("cla-gh-oauth-client-id-go-backend-%s", stage):
			config.GitHub.ClientID = resp.value
		case fmt.Sprintf("cla-gh-oauth-secret-go-backend-%s", stage):
			config.GitHub.ClientSecret = resp.value
		case fmt.Sprintf("cla-gh-access-token-%s", stage):
			config.GitHub.AccessToken = resp.value
		case fmt.Sprintf("cla-gh-app-id-%s", stage):
			githubAppID, err := strconv.Atoi(resp.value)
			if err != nil {
				errMsg := fmt.Sprintf("invalid value of key: %s", fmt.Sprintf("cla-gh-app-id-%s", stage))
				log.WithFields(f).WithError(err).Fatal(errMsg)
			}
			config.GitHub.AppID = githubAppID
		case fmt.Sprintf("cla-gh-app-private-key-%s", stage):
			config.GitHub.AppPrivateKey = resp.value
		case fmt.Sprintf("cla-gh-test-organization-%s", stage):
			config.GitHub.TestOrganization = resp.value
		case fmt.Sprintf("cla-gh-test-organization-installation-id-%s", stage):
			config.GitHub.TestOrganizationInstallationID = resp.value
		case fmt.Sprintf("cla-gh-test-repository-%s", stage):
			config.GitHub.TestRepository = resp.value
		case fmt.Sprintf("cla-gh-test-repository-id-%s", stage):
			config.GitHub.TestRepositoryID = resp.value

		//	gitlab ssm
		case fmt.Sprintf("cla-gitlab-app-id-%s", stage):
			config.Gitlab.AppClientID = resp.value
			// DEBUG
			log.WithFields(f).Debugf("CLA GitLab App ID: %s...%s", resp.value[0:4], resp.value[len(resp.value)-4:])
		case fmt.Sprintf("cla-gitlab-app-secret-%s", stage):
			config.Gitlab.AppClientSecret = resp.value
			// DEBUG
			log.WithFields(f).Debugf("CLA GitLab App Secret: %s...%s", resp.value[0:4], resp.value[len(resp.value)-4:])
		case fmt.Sprintf("cla-gitlab-app-private-key-%s", stage):
			config.Gitlab.AppPrivateKey = resp.value
			// DEBUG
			log.WithFields(f).Debugf("CLA GitLab App Private Key: %s...%s", resp.value[0:4], resp.value[len(resp.value)-4:])
		case fmt.Sprintf("cla-gitlab-app-redirect-uri-%s", stage):
			config.Gitlab.RedirectURI = resp.value
		case fmt.Sprintf("cla-gitlab-app-web-hook-uri-%s", stage):
			config.Gitlab.WebHookURI = resp.value
		case fmt.Sprintf("cla-contributor-v2-base-%s", stage):
			config.CLAContributorv2Base = resp.value
		case fmt.Sprintf("cla-api-v4-base-%s", stage):
			config.ClaAPIV4Base = resp.value
		case fmt.Sprintf("cla-landing-page-%s", stage):
			config.CLALandingPage = resp.value
		case fmt.Sprintf("cla-logo-url-%s", stage):
			config.CLALogoURL = resp.value
		case fmt.Sprintf("cla-corporate-base-%s", stage):
			config.CorporateConsoleURL = resp.value
		case fmt.Sprintf("cla-corporate-v1-base-%s", stage):
			config.CorporateConsoleV1URL = resp.value
		case fmt.Sprintf("cla-corporate-v2-base-%s", stage):
			config.CorporateConsoleV2URL = resp.value
		case fmt.Sprintf("cla-doc-raptor-api-key-%s", stage):
			config.Docraptor.APIKey = resp.value
			// Docraptor adds a watermark for generated PDFs that have the test mode flag set to true
			// We don't want a bunch of test documents generated in DEV to count against our quota, so we generally
			// set this flag to true for DEV, false for STAGING and PROD
			// Commenting this out for now as we are evaluating various templates and QA is unable to verify with the
			// watermark.  Restore this to just staging and prod after the testing phase is done.
			config.Docraptor.TestMode = stage == "dev"
			//config.Docraptor.TestMode = false // disable test mode while we evaluate various templates
		case fmt.Sprintf("cla-session-store-table-%s", stage):
			config.SessionStoreTableName = resp.value
		case fmt.Sprintf("cla-ses-sender-email-address-%s", stage):
			config.SenderEmailAddress = resp.value
		case fmt.Sprintf("cla-allowed-origins-%s", stage):
			config.AllowedOriginsCommaSeparated = resp.value
		case fmt.Sprintf("cla-sns-event-topic-arn-%s", stage):
			config.SNSEventTopicARN = resp.value
		case fmt.Sprintf("cla-signature-files-bucket-%s", stage):
			config.SignatureFilesBucket = resp.value
		case fmt.Sprintf("cla-auth0-platform-client-id-%s", stage):
			config.Auth0Platform.ClientID = resp.value
		case fmt.Sprintf("cla-auth0-platform-client-secret-%s", stage):
			config.Auth0Platform.ClientSecret = resp.value
		case fmt.Sprintf("cla-auth0-platform-audience-%s", stage):
			config.Auth0Platform.Audience = resp.value
		case fmt.Sprintf("cla-auth0-platform-url-%s", stage):
			config.Auth0Platform.URL = resp.value
		case fmt.Sprintf("cla-auth0-platform-api-gw-%s", stage):
			config.APIGatewayURL = resp.value
		case fmt.Sprintf("cla-platform-api-gw-%s", stage):
			config.PlatformAPIGatewayURL = resp.value
		case fmt.Sprintf("cla-lf-group-client-id-%s", stage):
			config.LFGroup.ClientID = resp.value
		case fmt.Sprintf("cla-lf-group-client-secret-%s", stage):
			config.LFGroup.ClientSecret = resp.value
		case fmt.Sprintf("cla-lf-group-client-url-%s", stage):
			config.LFGroup.ClientURL = resp.value
		case fmt.Sprintf("cla-lf-group-refresh-token-%s", stage):
			config.LFGroup.RefreshToken = resp.value
		case fmt.Sprintf("cla-v1-api-url-%s", stage):
			config.ClaV1ApiURL = resp.value
		case fmt.Sprintf("cla-acs-api-key-%s", stage):
			config.AcsAPIKey = resp.value
		case fmt.Sprintf("cla-lfx-portal-url-%s", stage):
			config.LFXPortalURL = resp.value
		case fmt.Sprintf("cla-lfx-metrics-report-sqs-region-%s", stage):
			config.MetricsReport.AwsSQSRegion = resp.value
		case fmt.Sprintf("cla-lfx-metrics-report-sqs-url-%s", stage):
			config.MetricsReport.AwsSQSQueueURL = resp.value
		case fmt.Sprintf("cla-lfx-metrics-report-enabled-%s", stage):
			boolVal, err := strconv.ParseBool(resp.value)
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("unable to convert %s value to a boolean - setting value to false in the configuration",
					fmt.Sprintf("cla-lfx-metrics-report-enabled-%s", stage))
				config.MetricsReport.Enabled = false
			} else {
				config.MetricsReport.Enabled = boolVal
			}
		case fmt.Sprintf("cla-enable-services-for-parent-%s", stage):
			boolVal, err := strconv.ParseBool(resp.value)
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("unable to convert %s value to a boolean - setting value to false in the configuration",
					fmt.Sprintf("cla-enable-services-for-parent-%s", stage))
				config.EnableCLAServiceForParent = false
			} else {
				config.EnableCLAServiceForParent = boolVal
			}
		case fmt.Sprintf("cla-signature-query-default-%s", stage):
			if resp.value == "" {
				config.SignatureQueryDefault = config.SignatureQueryDefaultValue
			} else {
				config.SignatureQueryDefault = resp.value
			}
		case fmt.Sprintf("cla-docusign-private-key-%s", stage):
			config.DocuSignPrivateKey = resp.value
		}
	}

	return config
}
