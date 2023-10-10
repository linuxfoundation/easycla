// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package sign

import (
	"context"
	"log"
)

func (s *service) getAccessToken(ctx context.Context) (string, error) {
	f := logrus.Fields{
		"functionName": "sign.getAccessToken",
	}

	// Get the access token
	jwtAssertion, jwterr := jwtToken()
}