// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"strings"
	"unicode"
)

// TrimRemoveTrailingComma trims the whitespace on the specified string and removes the trailing comma
func TrimRemoveTrailingComma(input string) string {
	if input == "" {
		return input
	}

	s := strings.TrimSpace(input)
	return strings.TrimSuffix(s, ",")
}

// TrimSpaceFromItems is a helper function to trim space on array items
func TrimSpaceFromItems(arr []string) []string {
	newArr := make([]string, len(arr))
	for i := range arr {
		newArr[i] = strings.TrimSpace(arr[i])
	}

	return newArr
}

// GetFirstAndLastName parses the user's name into first and last strings
func GetFirstAndLastName(firstAndLastName string) (string, string) {
	// Parse the provided user's name
	userNames := strings.Split(firstAndLastName, " ")
	var userFirstName string
	var userLastName string
	if len(userNames) >= 2 {
		userFirstName = userNames[0]
		userLastName = userNames[len(userNames)-1]
	} else if len(userNames) == 1 {
		userFirstName = userNames[0]
	}

	return strings.TrimSpace(userFirstName), strings.TrimSpace(userLastName)
}

// SanitizePlainText normalizes user-supplied free text: CR/LF variants become newlines, other
// control characters are dropped, the result is trimmed (HTML escaping is the renderer's job)
func SanitizePlainText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	var builder strings.Builder
	builder.Grow(len(text))
	for _, r := range text {
		if r == '\n' || r == '\t' || !unicode.IsControl(r) {
			builder.WriteRune(r)
		}
	}
	return strings.TrimSpace(builder.String())
}

// SanitizeSingleLine strips every control character so user-influenced values cannot inject
// email header separators
func SanitizeSingleLine(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
}
