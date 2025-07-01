// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package init

import (
	"fmt"

	"github.com/linuxfoundation/easycla/cla-backend-go/config"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/spf13/viper"
)

const (
	// ServiceName is the name of the service - this is used in logs and as a environment prefix
	ServiceName = "CLA_SERVICE"
)

var (
	stage      string
	configFile = ""
	configVars config.Config
)

// CommonInit initializes the common properties
func CommonInit() {
	stage = GetProperty("STAGE")
}

// GetProperty is a common routine to bind and return the specified environment variable
func GetProperty(property string) string {
	err := viper.BindEnv(property)
	if err != nil {
		log.Fatalf("Unable to load property: %s - value not defined or empty", property)
	}

	value := viper.GetString(property)
	if value == "" {
		err := fmt.Errorf("%s environment variable cannot be empty", property)
		log.Fatal(err.Error())
	}

	return value
}

// Init initialization logic for all the handlers
func Init() {
	CommonInit()
	AWSInit()
}

// ConfigVariable loads all the SSM values based on stage.
func ConfigVariable() {
	var err error
	configVars, err = config.LoadConfig(configFile, awsSession, stage)
	if err != nil {
		log.Panicf("Unable to load config - Error: %v", err)
	}
}

// GetStage returns the deployment stage, e.g. dev, test, stage or prod
func GetStage() string {
	return stage
}

// GetConfig returns the configuration SSM based on stage, e.g. dev, test, stage or prod
func GetConfig() config.Config {
	return configVars
}
