// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cmd

import (
	"sync"
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

var reqMap sync.Map

// requestStart holds the request ID, method and timing information in a small structure
func requestStart(reqID, method string) {
	now, _ := utils.CurrentTime()
	rm := &responseMetrics{
		reqID:   reqID,
		method:  method,
		start:   now,
		elapsed: 0,
		expire:  now.Add(time.Minute * 5),
	}
	reqMap.Store(reqID, rm)
}

// getRequestMetrics returns the response metrics based on the request id value
func getRequestMetrics(reqID string) *responseMetrics {
	if val, found := reqMap.Load(reqID); found {
		rm, ok := val.(*responseMetrics)
		if !ok {
			return nil
		}
		now, _ := utils.CurrentTime()
		rm.elapsed = now.Sub(rm.start)
		return rm
	}
	return nil
}

// clearRequestMetrics removes the request from the map
func clearRequestMetrics(reqID string) {
	reqMap.Delete(reqID)
}
