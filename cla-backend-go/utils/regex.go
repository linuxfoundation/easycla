// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"regexp"
)

// ValidCompanyName is a routine to indicate if the regex is a valid company name
func ValidCompanyName(name string) bool {
	// The {2,100} max limit isn't working in the regex, so, use this approach
	if len(name) < 2 || len(name) > 100 {
		return false
	}

	r := regexp.MustCompile(`^([^<>]*){2,100}$`)
	return r.MatchString(name)
}

// ValidWebsite is a routine to indicate if the website is a valid URL
func ValidWebsite(website string) bool {
	if len(website) < 5 || len(website) > 255 {
		return false
	}

	// \w represents any character from the class [A-Za-z0-9_], mnemonic: 'word'.
	r := regexp.MustCompile(`^(?:http(s)?:\/\/)?[\w.-]+(?:\.[\w\.-]+)+[\w\-\._~:/?#[\]@!\$&'\(\)\*\+,;=.]+$`)
	return r.MatchString(website)
}

// ParseString parses a string and returns group values defined in the regex
func ParseString(regEx, val string) (paramsMap map[string]string) {

	var compRegEx = regexp.MustCompile(regEx)
	match := compRegEx.FindStringSubmatch(val)

	paramsMap = make(map[string]string)
	for i, name := range compRegEx.SubexpNames() {
		if i > 0 && i <= len(match) {
			paramsMap[name] = match[i]
		}
	}
	return paramsMap
}
