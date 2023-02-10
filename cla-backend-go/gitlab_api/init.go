// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab

import (
	"sync"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

// App is a wrapper for the GitLab configuration items
type App struct {
	gitLabAppID         string
	gitLabAppSecret     string
	gitLabAppPrivateKey string
}

var gitLabAppSingleton *App

var once sync.Once

// Init initializes the required gitlab variables
func Init(glAppID, glAppSecret, glAppPrivateKey string) *App {
	if gitLabAppSingleton == nil {
		once.Do(
			func() {
				log.Debug("Creating object single instance...")
				gitLabAppSingleton = &App{
					gitLabAppID:         glAppID,
					gitLabAppSecret:     glAppSecret,
					gitLabAppPrivateKey: glAppPrivateKey,
				}
			})
	}
	return gitLabAppSingleton
}

// GetAppID returns the GitLab application ID
func (app *App) GetAppID() string {
	return app.gitLabAppID
}

// GetAppSecret returns the GitLab application secret
func (app *App) GetAppSecret() string {
	return app.gitLabAppSecret
}

// GetAppPrivateKey returns the GitLab application private key
func (app *App) GetAppPrivateKey() string {
	return app.gitLabAppPrivateKey
}
