// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package config

import (
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

var easyCLAConfig Config

// Config data model
type Config struct {
	// Auth0
	Auth0 Auth0 `json:"auth0"`

	// Auth0Platform config
	Auth0Platform Auth0Platform `json:"auth0_platform"`

	// APIGatewayURL is the API gateway URL - old variable which is set by the old cla-auth0-gateway SSM key
	APIGatewayURL string `json:"api_gateway_url"`
	// PlatformAPIGatewayURL is the platform API gateway URL
	PlatformAPIGatewayURL string `json:"platform_api_gateway_url"`

	// EnableCLAServiceForParent is a configuration flag to indicate if we should set the enable_services=[CLA] attribute on the parent project object in the project service when a child project is associated with a CLA group. This determines the v2 project console experience/behavior."
	EnableCLAServiceForParent bool `json:"enable_cla_service_for_parent"`

	// SignatureQueryDefault is a flag to indicate how a default signature query should return data - show only 'active' signatures or 'all' signatures when no other query signed/approved params are provided
	SignatureQueryDefault string `json:"signature_query_default"`
	// SignatureQueryDefaultValue the default value for the SignatureQueryDefault configuration value
	SignatureQueryDefaultValue string `json:"signature_query_default_value"`

	// SFDC

	// GitHub

	// Docusign

	// Docraptor
	Docraptor Docraptor `json:"docraptor"`

	// LF Identity

	// AWS
	AWS AWS `json:"aws"`

	// GitHub Application
	GitHub GitHub `json:"github"`

	// Gitlab Application
	Gitlab Gitlab `json:"gitlab"`

	// Dynamo Session Store
	SessionStoreTableName string `json:"sessionStoreTableName"`

	// Sender Email Address
	SenderEmailAddress string `json:"senderEmailAddress"`

	AllowedOriginsCommaSeparated string   `json:"allowedOriginsCommaSeparated"`
	AllowedOrigins               []string `json:"-"`

	CorporateConsoleURL   string `json:"corporateConsoleURL"`
	CorporateConsoleV1URL string `json:"corporateConsoleV1URL"`
	CorporateConsoleV2URL string `json:"corporateConsoleV2URL"`

	CLAContributorv2Base string `json:"cla-contributor-v2-base"`

	// SNSEventTopic the topic ARN for events
	SNSEventTopicARN string `json:"snsEventTopicARN"`

	// S3 bucket to store signatures
	SignatureFilesBucket string `json:"signatureFilesBucket"`

	// LF Group
	LFGroup LFGroup `json:"lf_group"`

	// CLAV1ApiURL is api url of v1. it is used in v2 sign service
	ClaV1ApiURL string `json:"cla_v1_api_url"`

	// AcsAPIKey is api key of the acs
	AcsAPIKey string `json:"acs_api_key"`

	// LFXPortalURL is url of the LFX UI for the particular environment
	LFXPortalURL string `json:"lfx_portal_url"`

	// MetricsReport has the transport config to send the metrics data
	MetricsReport MetricsReport `json:"metrics_report"`
}

// Auth0 model
type Auth0 struct {
	Domain        string `json:"auth0-domain"`
	ClientID      string `json:"auth0-clientId"`
	UsernameClaim string `json:"auth0-username-claim"`
	Algorithm     string `json:"auth0-algorithm"`
}

// Auth0Platform model
type Auth0Platform struct {
	ClientID     string `json:"auth0-clientId"`
	ClientSecret string `json:"auth0-clientSecret"`
	Audience     string `json:"audience"`
	URL          string `json:"url"`
}

// Docraptor model
type Docraptor struct {
	APIKey   string `json:"apiKey"`
	TestMode bool   `json:"testMode"`
}

// LFGroup contains LF LDAP group access information
type LFGroup struct {
	ClientURL    string `json:"client_url"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RefreshToken string `json:"refresh_token"`
}

// AWS model
type AWS struct {
	Region string `json:"region"`
}

// GitHub model
type GitHub struct {
	ClientID                       string `json:"clientId"`
	ClientSecret                   string `json:"clientSecret"`
	AccessToken                    string `json:"accessToken"`
	AppID                          int    `json:"app_id"`
	AppPrivateKey                  string `json:"app_private_key"`
	TestOrganization               string `json:"test_organization"`
	TestOrganizationInstallationID string `json:"test_organization_installation_id"`
	TestRepository                 string `json:"test_repository"`
	TestRepositoryID               string `json:"test_repository_id"`
}

// Gitlab config data model
type Gitlab struct {
	AppClientID     string `json:"app_client_id"`
	AppClientSecret string `json:"app_client_secret"`
	AppPrivateKey   string `json:"app_client_private_key"`
	RedirectURI     string `json:"app_redirect_uri"`
	WebHookURI      string `json:"app_web_hook_uri"`
}

// MetricsReport keeps the config needed to send the metrics data report
type MetricsReport struct {
	AwsSQSRegion   string `json:"aws_sqs_region"`
	AwsSQSQueueURL string `json:"aws_sqs_queue_url"`
	Enabled        bool   `json:"metrics_reporting_enabled"`
}

// GetConfig returns the current EasyCLA configuration
func GetConfig() Config {
	return easyCLAConfig
}

// LoadConfig loads the configuration
func LoadConfig(configFilePath string, awsSession *session.Session, awsStage string) (Config, error) {
	var err error

	if configFilePath != "" {
		// Read from local env.jso
		log.Info("Loading local config...")
		easyCLAConfig, err = loadLocalConfig(configFilePath)

	} else if awsSession != nil {
		// Read from SSM
		log.Info("Loading SSM config...")
		easyCLAConfig = loadSSMConfig(awsSession, awsStage)

	} else {
		return Config{}, errors.New("config not found")
	}

	if err != nil {
		return Config{}, err
	}

	// Convert the allowed origins into an array of values
	easyCLAConfig.AllowedOrigins = strings.Split(easyCLAConfig.AllowedOriginsCommaSeparated, ",")

	return easyCLAConfig, nil
}
