// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// GetProperty is a common routine to bind and return the specified environment variable
func GetProperty(property string) string {
	f := logrus.Fields{
		"functionName": "utils.properties.GetProperty",
	}
	err := viper.BindEnv(property)
	if err != nil {
		log.WithFields(f).WithError(err).Fatalf("unable to load property: %s - value not defined or empty", property)
	}

	value := viper.GetString(property)
	if value == "" {
		log.WithFields(f).WithError(err).Fatalf("property: %s cannot be empty", property)
	}

	return value
}
