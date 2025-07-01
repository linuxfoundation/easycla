// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cmd

import (
	"time"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

// responseMetrics is a small structure for keeping track of the request metrics
type responseMetrics struct {
	reqID   string
	method  string
	start   time.Time
	elapsed time.Duration
	expire  time.Time
}

var reqMap = make(map[string]*responseMetrics, 5)

// requestStart holds the request ID, method and timing information in a small structure
func requestStart(reqID, method string) {
	now, _ := utils.CurrentTime()
	reqMap[reqID] = &responseMetrics{
		reqID:   reqID,
		method:  method,
		start:   now,
		elapsed: 0,
		expire:  now.Add(time.Minute * 5),
	}
}

// getRequestMetrics returns the response metrics based on the request id value
func getRequestMetrics(reqID string) *responseMetrics {
	if x, found := reqMap[reqID]; found {
		now, _ := utils.CurrentTime()
		x.elapsed = now.Sub(x.start)
		return x
	}

	return nil
}

// clearRequestMetrics removes the request from the map
func clearRequestMetrics(reqID string) {
	delete(reqMap, reqID)
}
