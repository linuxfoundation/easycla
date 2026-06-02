// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/sirupsen/logrus"
)

func loadLocalConfig(configFilePath string) (Config, error) {
	f := logrus.Fields{
		"functionName": "config.local.loadLocalConfig",
	}
	content, err := os.ReadFile(filepath.Clean(configFilePath))
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("Failed to read config file: %s", configFilePath)
		return Config{}, err
	}

	localConfig := Config{}
	err = json.Unmarshal(content, &localConfig)
	if err != nil {
		return Config{}, err
	}

	return localConfig, nil
}
