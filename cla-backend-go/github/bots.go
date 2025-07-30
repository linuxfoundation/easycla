// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/sirupsen/logrus"
)

// propertyMatches returns true if value matches the pattern.
// - "*" matches anything
// - "re:..." matches regex (value must be non-empty)
// - otherwise, exact match
func propertyMatches(pattern, value string) bool {
	f := logrus.Fields{
		"functionName": "github.propertyMatches",
		"pattern":      pattern,
		"value":        value,
	}
	if pattern == "*" {
		return true
	}
	if value == "" {
		return false
	}
	if strings.HasPrefix(pattern, "re:") {
		regex := pattern[3:]
		re, err := regexp.Compile(regex)
		if err != nil {
			log.WithFields(f).Debugf("Error in propertyMatches: bad regexp: %s, error: %v", regex, err)
			return false
		}
		return re.MatchString(value)
	}
	return value == pattern
}

// stripOrg removes the organization part from the repository name.
// If input is "org/repo", returns "repo". If no "/", returns input unchanged.
func stripOrg(repoFull string) string {
	idx := strings.Index(repoFull, "/")
	if idx >= 0 && idx+1 < len(repoFull) {
		return repoFull[idx+1:]
	}
	return repoFull
}

// isActorSkipped returns true if the given actor should be skipped according to the skip_cla config pattern.
// config format: "<username_pattern>;<email_pattern>"
// Actor.CommitAuthor.Login and Actor.CommitAuthor.Email should be *string, can be nil.
func isActorSkipped(actor *UserCommitSummary, config string) bool {
	f := logrus.Fields{
		"functionName": "github.isActorSkipped",
		"config":       config,
	}
	// Defensive: must have exactly one ';'
	if !strings.Contains(config, ";") {
		log.WithFields(f).Debugf("Invalid skip_cla config format: %s, expected '<username_pattern>;<email_pattern>'", config)
		return false
	}
	parts := strings.SplitN(config, ";", 2)
	if len(parts) != 2 {
		return false
	}
	usernamePattern := parts[0]
	emailPattern := parts[1]
	var (
		username string
		email    string
	)
	if actor != nil && actor.CommitAuthor != nil && actor.CommitAuthor.Login != nil {
		username = *actor.CommitAuthor.Login
	}
	if actor != nil && actor.CommitAuthor != nil && actor.CommitAuthor.Email != nil {
		email = *actor.CommitAuthor.Email
	}

	return propertyMatches(usernamePattern, username) && propertyMatches(emailPattern, email)
}

// SkipWhitelistedBots- check if the actors are whitelisted based on the skip_cla configuration.
// Returns two lists:
// - actors still missing cla: actors who still need to sign the CLA after checking skip_cla
// - whitelisted actors: actors who are skipped due to skip_cla configuration
// :param orgModel: The GitHub organization model instance.
// :param orgRepo: The repository name in the format 'org/repo'.
// :param actorsMissingCla: List of UserCommitSummary objects representing actors who are missing CLA.
// :return: two arrays (actors still missing CLA, whitelisted actors)
// : in cla-{stage}-github-orgs table there can be a skip_cla field which is a dict with the following structure:
//
//	{
//	    "repo-name": "<username_pattern>;<email_pattern>",
//	    "re:repo-regexp": "<username_pattern>;<email_pattern>",
//	    "*": "<username_pattern>;<email_pattern>"
//	}
//
// where:
//   - repo-name is the exact repository name under given org (e.g., "my-repo" not "my-org/my-repo")
//   - re:repo-regexp is a regex pattern to match repository names
//   - * is a wildcard that applies to all repositories
//   - <username_pattern> is a GitHub username pattern (exact match or regex prefixed by re: or match all '*')
//   - <email_pattern> is a GitHub email pattern (exact match or regex prefixed by re: or match all '*')
//     The username and email patterns are separated by a semicolon (;).
//     If the skip_cla is not set, it will skip the whitelisted bots check.
func SkipWhitelistedBots(ev events.Service, orgModel *models.GithubOrganization, orgRepo, projectID string, actorsMissingCLA []*UserCommitSummary) ([]*UserCommitSummary, []*UserCommitSummary) {
	repo := stripOrg(orgRepo)
	f := logrus.Fields{
		"functionName": "github.SkipWhitelistedBots",
		"orgRepo":      orgRepo,
		"repo":         repo,
		"projectID":    projectID,
	}
	outActorsMissingCLA := []*UserCommitSummary{}
	whitelistedActors := []*UserCommitSummary{}

	skipCLA := orgModel.SkipCla
	if skipCLA == nil {
		log.WithFields(f).Debug("skip_cla is not set, skipping whitelisted bots check")
		return actorsMissingCLA, []*UserCommitSummary{}
	}

	var config string

	// 1. Exact match
	if val, ok := skipCLA[repo]; ok {
		config = val
		log.WithFields(f).Debugf("skip_cla config found for repo (exact hit): '%s'", config)
	}

	// 2. Regex match (if no exact hit)
	if config == "" {
		log.WithFields(f).Debug("No skip_cla config found for repo, checking regex patterns")
		for k, v := range skipCLA {
			if !strings.HasPrefix(k, "re:") {
				continue
			}
			pattern := k[3:]
			re, err := regexp.Compile(pattern)
			if err != nil {
				log.WithFields(f).Warnf("Invalid regex in skip_cla: '%s': %+v", pattern, err)
				continue
			}
			if re.MatchString(repo) {
				config = v
				log.WithFields(f).Debugf("Found skip_cla config for repo via regex pattern: '%s'", config)
				break
			}
		}
	}

	// 3. Wildcard fallback
	if config == "" {
		if val, ok := skipCLA["*"]; ok {
			config = val
			log.WithFields(f).Debugf("No skip_cla config found for repo, using wildcard config: '%s'", config)
		}
	}

	// 4. No match
	if config == "" {
		log.WithFields(f).Debug("No skip_cla config found for repo, skipping whitelisted bots check")
		return actorsMissingCLA, []*UserCommitSummary{}
	}
	const nullStr = "(null)"

	for _, actor := range actorsMissingCLA {
		if isActorSkipped(actor, config) {
			if actor == nil {
				continue
			}
			id, login, username, email := nullStr, nullStr, nullStr, nullStr
			if actor.CommitAuthor != nil && actor.CommitAuthor.ID != nil {
				id = fmt.Sprintf("%v", *actor.CommitAuthor.ID)
			}
			if actor.CommitAuthor != nil && actor.CommitAuthor.Login != nil {
				login = *actor.CommitAuthor.Login
			}
			if actor.CommitAuthor != nil && actor.CommitAuthor.Name != nil {
				username = *actor.CommitAuthor.Name
			}
			if actor.CommitAuthor != nil && actor.CommitAuthor.Email != nil {
				email = *actor.CommitAuthor.Email
			}
			actorData := fmt.Sprintf("id='%v',login='%v',username='%v',email='%v'", id, login, username, email)
			msg := fmt.Sprintf(
				"Skipping CLA check for repo='%s', actor: %s due to skip_cla config: '%s'",
				orgRepo, actorData, config,
			)
			log.WithFields(f).Info(msg)
			eventData := events.BypassCLAEventData{
				Repo:   orgRepo,
				Config: config,
				Actor:  actorData,
			}
			ev.LogEvent(&events.LogEventArgs{
				EventType: events.BypassCLA,
				EventData: &eventData,
				UserID:    id,
				UserName:  login,
				ProjectID: projectID,
			})
			log.WithFields(f).Debugf("event logged")
			actor.Authorized = true
			whitelistedActors = append(whitelistedActors, actor)
		} else {
			outActorsMissingCLA = append(outActorsMissingCLA, actor)
		}
	}

	return outActorsMissingCLA, whitelistedActors
}
