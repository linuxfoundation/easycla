// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cmd

import (
	"flag"
	"fmt"
	"os"

	ini "github.com/linuxfoundation/easycla/cla-backend-go/init"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	configFile = ""
	portFlag   *int
)

func init() {
	log.Info("Running init...")

	// cobra.OnInitialize(initConfig)
	viper.AutomaticEnv()
	defaults := map[string]interface{}{
		"PORT":               8080,
		"APP_ENV":            "local",
		"USE_MOCK":           "False",
		"DB_MAX_CONNECTIONS": 1,
		"STAGE":              "dev",

		// should we validate the user's GitHub organizations?
		"GH_ORG_VALIDATION": "true",
		// should we validate company API queries against the current authenticated user?
		"COMPANY_USER_VALIDATION": "true",
	}

	for key, value := range defaults {
		viper.SetDefault(key, value)
	}

	portFlag = flag.Int("port", viper.GetInt("PORT"), "Port to listen for web requests on")

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	// Initialize our common stuff
	ini.Init()
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cla-backend-go",
	Short: "CLA Backend v3",
	Long:  "CLA Backend supporting the /v3 endpoints",
	Run:   runServer,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
