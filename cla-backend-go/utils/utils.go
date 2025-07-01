// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/sirupsen/logrus"

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

// FormatTimeString converts the time string into the "standard" RFC3339 format
func FormatTimeString(timeStr string) string {
	f := logrus.Fields{
		"functionName": "utils.utils.FormatTimeString",
		"timeStr":      timeStr,
	}

	t, err := ParseDateTime(timeStr)
	if err != nil {
		log.WithFields(f).Warnf("unable to convert the time string: %s into a standard form.", timeStr)
		return timeStr
	}

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
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05Z-07:00",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05.000000-0700",
		"2006-01-02T15:04:05.0",
		"2006-01-02T15:04:05.00",
		"2006-01-02T15:04:05.000",
		"2006-01-02T15:04:05.0000",
		"2006-01-02T15:04:05.00000",
		"2006-01-02T15:04:05.000000",
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
