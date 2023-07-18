// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/gofrs/uuid"
)

// ValidEmail tests the specified email string, returns true if email is valid, returns false otherwise
func ValidEmail(email string) bool {
	emailRegexp := regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
	return emailRegexp.MatchString(strings.TrimSpace(email))
}

// ValidDomain tests the specified domain string, returns true if domain is valid, returns false otherwise
func ValidDomain(domain string, allowWildcard bool) (string, bool) { // nolint
	domain = strings.TrimSpace(domain)

	switch {
	case len(domain) == 0:
		return "domain is empty", false
	case len(domain) > 255:
		return fmt.Sprintf("domain name length is %d, can't exceed 255", len(domain)), false
	}
	var l int
	for i := 0; i < len(domain); i++ {
		b := domain[i]
		if b == '.' {
			// check domain labels validity
			switch {
			case i == l:
				return fmt.Sprintf("invalid character '%c' at offset %d: label can't begin with a period", b, i), false
			case i-l > 63:
				return fmt.Sprintf("byte length of label '%s' is %d, can't exceed 63", domain[l:i], i-l), false
			case domain[l] == '-':
				return fmt.Sprintf("label '%s' at offset %d begins with a hyphen", domain[l:i], l), false
			case domain[i-1] == '-':
				return fmt.Sprintf("label '%s' at offset %d ends with a hyphen", domain[l:i], l), false
			}
			l = i + 1
			continue
		}

		// If wildcard domains are allowed, e.g. *.linuxfoundation.org
		if allowWildcard {
			// test label character validity, note: tests are ordered by decreasing validity frequency
			if !(b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '-' || b == '*' || b >= 'A' && b <= 'Z') {
				// show the printable unicode character starting at byte offset i
				c, _ := utf8.DecodeRuneInString(domain[i:])
				if c == utf8.RuneError {
					return fmt.Sprintf("invalid character at offset %d", i), false
				}
				return fmt.Sprintf("invalid character '%c' at offset %d", c, i), false
			}
		} else {
			// test label character validity, note: tests are ordered by decreasing validity frequency
			if !(b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '-' || b >= 'A' && b <= 'Z') {
				// show the printable unicode character starting at byte offset i
				c, _ := utf8.DecodeRuneInString(domain[i:])
				if c == utf8.RuneError {
					return fmt.Sprintf("invalid character at offset %d", i), false
				}
				return fmt.Sprintf("invalid character '%c' at offset %d", c, i), false
			}
		}
	}

	// check top level domain validity
	switch {
	case l == len(domain):
		return "missing top level domain, domain can't end with a period", false
	case len(domain)-l > 63:
		return fmt.Sprintf("byte length of top level domain '%s' is %d, can't exceed 63", domain[l:], len(domain)-l), false
	case domain[l] == '-':
		return fmt.Sprintf("top level domain '%s' at offset %d begins with a hyphen", domain[l:], l), false
	case domain[len(domain)-1] == '-':
		return fmt.Sprintf("top level domain '%s' at offset %d ends with a hyphen", domain[l:], l), false
	case domain[l] >= '0' && domain[l] <= '9':
		return fmt.Sprintf("top level domain '%s' at offset %d begins with a digit", domain[l:], l), false
	}

	return "", true
}

// ValidGitHubUsername tests the specified GitHub username string, returns true if valid, returns false otherwise
func ValidGitHubUsername(githubUsername string) (string, bool) {

	if len(strings.TrimSpace(githubUsername)) <= 2 {
		return "github username must be 3 or more characters", false
	}

	// For now, we only allow alphanumeric values
	re := regexp.MustCompile("^[a-zA-Z0-9._-]*$")
	valid := re.MatchString(strings.TrimSpace(githubUsername))
	if !valid {
		return fmt.Sprintf("invalid GitHub username: %s", githubUsername), false
	}

	return "", true
}

// ValidGitlabUsername tests the specified Gitlab username string, returns true if valid, returns false otherwise
func ValidGitlabUsername(gitlabUsername string) (string, bool) {

	if len(strings.TrimSpace(gitlabUsername)) <= 2 {
		return "gitlab username must be 3 or more characters", false
	}

	// For now, we only allow alphanumeric values
	re := regexp.MustCompile("^[a-zA-Z0-9._-]*$")
	valid := re.MatchString(strings.TrimSpace(gitlabUsername))
	if !valid {
		return fmt.Sprintf("invalid Gitlab username: %s", gitlabUsername), false
	}

	return "", true
}

// ValidGitHubOrg tests the specified GitHub Organization string, returns true if valid, returns false otherwise
func ValidGitHubOrg(githubOrg string) (string, bool) {

	if len(strings.TrimSpace(githubOrg)) <= 2 {
		return "github organization must be 3 or more characters", false
	}

	re := regexp.MustCompile("^[a-zA-Z0-9._-]*$")
	valid := re.MatchString(strings.TrimSpace(githubOrg))
	if !valid {
		return fmt.Sprintf("invalid GitHub organization: %s", githubOrg), false
	}

	return "", true
}

// ValidGitlabOrg tests the specified Gitlab Organization string, returns true if valid, returns false otherwise
func ValidGitlabOrg(gitlabOrg string) (string, bool) {

	if len(strings.TrimSpace(gitlabOrg)) <= 2 {
		return "gitlab organization must be 3 or more characters", false
	}

	re := regexp.MustCompile(`^(?:http(s)?:\/\/)?(?:www\.)?(\w+[\w-]+\w+\.)?gitlab\.com[\w\-\._~:/?#[\]@!\$&'\(\)\*\+,;=.]{3,100}$`)
	valid := re.MatchString(strings.TrimSpace(gitlabOrg))
	if !valid {
		return fmt.Sprintf("invalid Gitlab organization: %s", gitlabOrg), false
	}

	return "", true
}

// IsUUIDv4 returns true if the specified ID is in the UUIDv4 format, otherwise returns false
func IsUUIDv4(id string) bool {
	value, err := uuid.FromString(id)
	if err != nil {
		return false
	}

	return value.Version() == uuid.V4
}

// IsSalesForceID returns true if the specified ID is a SalesForce formatted ID, otherwise returns false
func IsSalesForceID(id string) bool {
	regExp := regexp.MustCompile("^[a-zA-Z0-9]{18}|[a-zA-Z0-9]{15}$")
	return regExp.MatchString(strings.TrimSpace(id))
}
