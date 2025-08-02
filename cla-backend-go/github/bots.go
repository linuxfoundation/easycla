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
// - "" matches empty value
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
	if pattern == "" && value == "" {
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

// isActorSkipped returns true if the actor should be skipped according to ANY pattern in config.
// Each config entry is "<login_pattern>;<email_pattern>;<name_pattern>"
// Any missing pattern defaults to "" which is special and matches missing property, null property value or empty string property value
func isActorSkipped(actor *UserCommitSummary, config []string) bool {
	for _, pattern := range config {
		parts := strings.Split(pattern, ";")
		for len(parts) < 3 {
			parts = append(parts, "")
		}
		loginPattern, emailPattern, namePattern := parts[0], parts[1], parts[2]

		var login, email, name string
		if actor != nil && actor.CommitAuthor != nil {
			if actor.CommitAuthor.Login != nil {
				login = *actor.CommitAuthor.Login
			}
			if actor.CommitAuthor.Email != nil {
				email = *actor.CommitAuthor.Email
			}
			if actor.CommitAuthor.Name != nil {
				name = *actor.CommitAuthor.Name
			}
		}

		if propertyMatches(loginPattern, login) &&
			propertyMatches(emailPattern, email) &&
			propertyMatches(namePattern, name) {
			return true
		}
	}
	return false
}

// actorToString converts a UserCommitSummary actor to a string representation.
func actorToString(actor *UserCommitSummary) string {
	const nullStr = "(null)"
	if actor == nil {
		return nullStr
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
	return fmt.Sprintf("id='%v',login='%v',username='%v',email='%v'", id, login, username, email)
}

// parseConfigPatterns takes a config string and returns a slice of pattern strings.
// If the config starts with '[' and ends with ']', splits by '||' inside; else returns []string{config}.
// Trims whitespace from each pattern.
func parseConfigPatterns(config string) []string {
	config = strings.TrimSpace(config)
	if len(config) >= 2 && strings.HasPrefix(config, "[") && strings.HasSuffix(config, "]") {
		inner := config[1 : len(config)-1]
		parts := strings.Split(inner, "||")
		for i, p := range parts {
			parts[i] = strings.TrimSpace(p)
		}
		return parts
	}
	return []string{config}
}

// SkipAllowlistedBots- check if the actors are allowlisted based on the skip_cla configuration.
// Returns two lists:
// - actors still missing cla: actors who still need to sign the CLA after checking skip_cla
// - allowlisted actors: actors who are skipped due to skip_cla configuration
// :param orgModel: The GitHub organization model instance.
// :param orgRepo: The repository name in the format 'org/repo'.
// :param actorsMissingCla: List of UserCommitSummary objects representing actors who are missing CLA.
// :return: two arrays (actors still missing CLA, allowlisted actors)
// : in cla-{stage}-github-orgs table there can be a skip_cla field which is a dict with the following structure:
//
//	{
//	    "repo-name": "<login_pattern>;<email_pattern>;<name_pattern>",
//	    "re:repo-regexp": "[<login_pattern>;<email_pattern>;<name_pattern>||...]",
//	    "*": "<login_pattern>"
//	}
//
// where:
//   - repo-name is the exact repository name under given org (e.g., "my-repo" not "my-org/my-repo")
//   - re:repo-regexp is a regex pattern to match repository names
//   - * is a wildcard that applies to all repositories
//   - <login_pattern> is a GitHub login pattern (exact match or regex prefixed by re: or match all '*') if not specified defaults to ""
//   - <email_pattern> is a GitHub email pattern (exact match or regex prefixed by re: or match all '*') if not specified defaults to ""
//   - <name_pattern> is a GitHub name pattern (exact match or regex prefixed by re: or match all '*') if not specified defaults to ""
//     "" matches empty value, null value or missing property
//     The login, email and name patterns are separated by a semicolon (;). Email and name parts are optional.
//     There can be an array of patterns for a single repository, separated by ||. It must start with a '[' and end with a ']': "[...||...||...]"
//     If the skip_cla is not set, it will skip the allowlisted bots check.
func SkipAllowlistedBots(ev events.Service, orgModel *models.GithubOrganization, orgRepo, projectID string, actorsMissingCLA []*UserCommitSummary) ([]*UserCommitSummary, []*UserCommitSummary) {
	repo := stripOrg(orgRepo)
	f := logrus.Fields{
		"functionName": "github.SkipAllowlistedBots",
		"orgRepo":      orgRepo,
		"repo":         repo,
		"projectID":    projectID,
	}
	outActorsMissingCLA := []*UserCommitSummary{}
	allowlistedActors := []*UserCommitSummary{}

	skipCLA := orgModel.SkipCla
	if skipCLA == nil {
		log.WithFields(f).Debug("skip_cla is not set, skipping allowlisted bots check")
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
		log.WithFields(f).Debug("No skip_cla config found for repo, skipping allowlisted bots check")
		return actorsMissingCLA, []*UserCommitSummary{}
	}

	configArray := parseConfigPatterns(config)

	// Log full configuration
	actorDebugData := make([]string, 0, len(actorsMissingCLA))
	for _, a := range actorsMissingCLA {
		actorDebugData = append(actorDebugData, actorToString(a))
	}
	log.WithFields(f).Debugf("final skip_cla config for repo %s is %+v; actorsMissingCLA: [%s]", orgRepo, configArray, strings.Join(actorDebugData, ", "))

	for _, actor := range actorsMissingCLA {
		if actor == nil {
			continue
		}
		actorData := actorToString(actor)
		log.WithFields(f).Debugf("Checking actor: %s for skip_cla config: %+v", actorData, configArray)
		if isActorSkipped(actor, configArray) {
			msg := fmt.Sprintf(
				"Skipping CLA check for repo='%s', actor: %s due to skip_cla config: %+v",
				orgRepo, actorData, configArray,
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
				ProjectID: projectID,
			})
			log.WithFields(f).Debugf("event logged")
			actor.Authorized = true
			allowlistedActors = append(allowlistedActors, actor)
		} else {
			outActorsMissingCLA = append(outActorsMissingCLA, actor)
		}
	}

	return outActorsMissingCLA, allowlistedActors
}
