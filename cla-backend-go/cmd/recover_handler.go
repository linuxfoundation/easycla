// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cmd

import (
	"net/http"
	"runtime/debug"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// recoverHandler answers 500 for a request whose handler panics, so that one request cannot end the Lambda invocation
func recoverHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			f := logrus.Fields{
				"functionName":   "cmd.recoverHandler",
				utils.XREQUESTID: r.Header.Get(utils.XREQUESTID),
				"method":         r.Method,
				"path":           r.URL.Path,
			}
			log.WithFields(f).Errorf("panic in an API handler: %v\n%s", rec, debug.Stack())
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write([]byte(`{"Code":"500","Message":"internal server error"}`)); err != nil {
				log.WithFields(f).WithError(err).Warn("unable to write the answer for a panic")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
