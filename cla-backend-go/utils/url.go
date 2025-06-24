// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"net/url"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/sirupsen/logrus"
)

// GetPathFromURL helper function to extract the path from the S3 URL
func GetPathFromURL(s3URLHost string) (string, error) {
	f := logrus.Fields{
		"functionName": "getPathFromURL",
		"s3URLHost":    s3URLHost,
	}

	u, err := url.Parse(s3URLHost)
	if err != nil {
		log.WithFields(f).Warnf("s3 template url parse error, error: %+v", err)
		return "", err
	}

	return u.Path, nil
}
