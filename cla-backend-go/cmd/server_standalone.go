//go:build !aws_lambda
// +build !aws_lambda

// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func runServer(cmd *cobra.Command, args []string) {
	log.Info("Staring the HTTP server in local mode...")

	handler := server(true)

	errs := make(chan error, 2)
	go func() {
		log.Infof("Running http server on port: %d - set PORT environment variable to change port", viper.GetInt("PORT"))
		errs <- http.ListenAndServe(fmt.Sprintf(":%d", viper.GetInt("PORT")), handler) // nolint gosec no support for setting timeouts
	}()
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT) // nolint
		errs <- fmt.Errorf("%s", <-c)
	}()

	log.Infof("HTTP Server terminated - errors: %v", <-errs)
}
