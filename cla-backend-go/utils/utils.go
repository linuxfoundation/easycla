// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofrs/uuid"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

const (
	// V1 is the version 1 identifier
	V1 = "v1"

	// V2 is the version 2 identifier
	V2 = "v2"
)

// FmtDuration is a helper function to format a duration in the HH:MM:SS.sss format
func FmtDuration(d time.Duration) string {
	//days := int64(d.Hours() / 24)
	hours := int64(math.Mod(d.Hours(), 24))
	minutes := int64(math.Mod(d.Minutes(), 60))
	seconds := int64(math.Mod(d.Seconds(), 60))
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, d.Milliseconds())
}

// TimeToString returns time in the RFC3339 format
func TimeToString(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// CurrentTime returns the current UTC time and current Time in the RFC3339 format
func CurrentTime() (time.Time, string) {
	t := time.Now().UTC()
	return t, TimeToString(t)
}

// CurrentSimpleDateTimeString returns the current UTC time and current Time in the "2006-01-02T15:04:05Z" format
func CurrentSimpleDateTimeString() string {
	t := time.Now().UTC()
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

// ParseDateTime attempts to convert the string to one of our supported date time formats
func ParseDateTime(dateTimeStr string) (time.Time, error) {
	dateTimeStrTrimmed := strings.TrimSpace(dateTimeStr)

	supportedFormats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05Z-07:00",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05.000000-0700",
		time.RFC850,
		time.RFC1123,
		time.RFC1123Z,
	}

	for _, format := range supportedFormats {
		dateTimeValue, err := time.Parse(format, dateTimeStrTrimmed)
		if err == nil {
			return dateTimeValue, err
		}
	}

	return time.Unix(0, 0), fmt.Errorf("unsupported date/time format: %s", dateTimeStrTrimmed)
}

// AddStringAttribute adds string attribute to dynamodb input map
func AddStringAttribute(item map[string]*dynamodb.AttributeValue, key string, value string) {
	if value != "" {
		item[key] = &dynamodb.AttributeValue{S: aws.String(value)}
	}
}

// AddNumberAttribute adds number attribute to dynamodb input map
func AddNumberAttribute(item map[string]*dynamodb.AttributeValue, key string, value int64) {
	numString := strconv.FormatInt(value, 10)
	item[key] = &dynamodb.AttributeValue{N: aws.String(numString)}
}

// StringInSlice returns true if the specified string value exists in the slice, otherwise returns false
func StringInSlice(a string, list []string) bool {
	if list == nil {
		return false
	}

	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// RemoveItemsFromList given a list, removes the specified entries from that list
func RemoveItemsFromList(existingList, removeEntries []string) []string {
	if existingList == nil {
		return nil
	}

	if removeEntries == nil {
		return existingList
	}

	for _, value := range removeEntries {
		idx := -1
		for i, item := range existingList {
			if value == item {
				idx = i
				break
			}
		}
		if idx != -1 {
			existingList = append(existingList[:idx], existingList[idx+1:]...)
		}
	}

	return existingList
}

// RemoveDuplicates removes any duplicate entries in the provided list and returns a new list
func RemoveDuplicates(list []string) []string {
	if list == nil {
		return nil
	}

	var newList []string

	for _, v := range list {
		if !StringInSlice(v, newList) {
			newList = append(newList, v)
		}
	}

	return newList
}

// HostInSlice returns true if the specified host value exists in the slice, otherwise returns false
func HostInSlice(a string, list []string) bool {
	if list == nil {
		return false
	}

	for _, b := range list {
		b = strings.Split(b, ":")[0]
		if b == a {
			return true
		}
	}
	return false
}

// SliceDifference returns the entries that are different between the two slices
func SliceDifference(a, b []string) []string {
	var diff []string
	for _, x := range a {
		if !StringInSlice(x, b) {
			diff = append(diff, x)
		}
	}
	for _, x := range b {
		if !StringInSlice(x, a) {
			diff = append(diff, x)
		}
	}
	return diff
}

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

	// For now, we only allow alpha numeric values
	re := regexp.MustCompile("^[a-zA-Z0-9_-]*$")
	valid := re.MatchString(strings.TrimSpace(githubUsername))
	if !valid {
		return fmt.Sprintf("invalid GitHub username: %s", githubUsername), false
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

// IsUUIDv4 returns true if the specified ID is in the UUIDv4 format, otherwise returns false
func IsUUIDv4(id string) bool {
	value, err := uuid.FromString(id)
	if err != nil {
		return false
	}

	return value.Version() == uuid.V4
}
