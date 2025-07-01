// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"

	"github.com/stretchr/testify/assert"
)

func TestFmtDuration(t *testing.T) {
	now := time.Now()
	duration, err := time.ParseDuration("2h45m35s")
	assert.Nil(t, err, fmt.Sprintf("Time parse error: %+v", err))
	future := now.Add(duration)
	strDuration := utils.FmtDuration(future.Sub(now))
	assert.True(t, strings.HasPrefix(strDuration, "02:45:35"))
}

func TestParseDateTimeDefault(t *testing.T) {
	validInput := []string{
		"2020-03-27T20:19:07+08:00",
		"2020-03-27T12:00:17+00:00",
		"2013-01-31T04:00:17-05:00",
	}

	for _, dateTimeStr := range validInput {
		dateTimeValue, err := utils.ParseDateTime(dateTimeStr)
		assert.Nil(t, err, fmt.Sprintf("Check for Valid Date Time: %s - %+v", dateTimeStr, dateTimeValue))
		assert.True(t, dateTimeValue.After(time.Unix(0, 0)))
	}

	inValidInput := []string{
		// Note: Formatter doesn't seem to catch invalid time zone values
		"20201-11-27T12:00:17+00:00", // Invalid Year
		"2020-13-27T12:00:17+00:00",  // Invalid Month
		"2013-01-32T04:00:17-05:00",  // Invalid Day
		"2013-01-02T24:01:17-05:00",  // Invalid Hour
		"2013-01-02T14:71:17-05:00",  // Invalid Minute
		"2013-01-02T14:21:87-05:00",  // Invalid Second
	}

	for _, dateTimeStr := range inValidInput {
		dateTimeValue, err := utils.ParseDateTime(dateTimeStr)
		assert.NotNil(t, err, fmt.Sprintf("Check for Invalid Date Time: %s - %v", dateTimeStr, dateTimeValue))
	}
}

func TestParseDateTimeZulu(t *testing.T) {
	validInput := []string{
		"2020-05-05T16:09:37Z",
		"2020-03-27T15:04:05Z+07:00",
		"2020-09-02T15:04:05Z-07:00",
		"2016-01-02T15:04:05Z-07:00",
	}

	for _, dateTimeStr := range validInput {
		dateTimeValue, err := utils.ParseDateTime(dateTimeStr)
		assert.Nil(t, err, fmt.Sprintf("Check for Valid Date Time: %s - %+v", dateTimeStr, dateTimeValue))
		assert.True(t, dateTimeValue.After(time.Unix(0, 0)))
	}

	inValidInput := []string{
		// Note: Formatter doesn't seem to catch invalid time zone values
		"20016-01-02T15:04:05Z-07:00", // Invalid Year
		"2006-21-02T15:04:05Z-07:00",  // Invalid Month
		"22006-01-92T15:04:05Z-07:00", // Invalid Day
		"22006-01-92T25:04:05Z-07:00", // Invalid Hour
		"22006-01-92T15:74:05Z-07:00", // Invalid Minute
		"22006-01-92T25:04:85Z-07:00", // Invalid Second
	}

	for _, dateTimeStr := range inValidInput {
		dateTimeValue, err := utils.ParseDateTime(dateTimeStr)
		assert.NotNil(t, err, fmt.Sprintf("Check for Invalid Date Time: %s - %+v", dateTimeStr, dateTimeValue))
	}
}

func TestParseDateTimeMS(t *testing.T) {
	validInput := []string{
		"2020-03-27T15:04:05.000000+0000",
		"2016-12-02T05:14:05.000000+0800",
		"2006-08-31T10:24:05.000000-1000",
		"2019-04-15T20:30:12.13589",
		"2019-04-15T20:30:19.321645",
	}

	for _, dateTimeStr := range validInput {
		dateTimeValue, err := utils.ParseDateTime(dateTimeStr)
		assert.Nil(t, err, fmt.Sprintf("Check for Valid Date Time: %s - %+v", dateTimeStr, dateTimeValue))
		assert.True(t, dateTimeValue.After(time.Unix(0, 0)))
	}

	inValidInput := []string{
		// Note: Formatter doesn't seem to catch invalid time zone values
		"20062-01-02T15:04:05.000000+0000", // Invalid Year
		"2006-21-02T15:04:05.000000+0000",  // Invalid Month
		"2006-01-92T15:04:05.000000+0000",  // Invalid Day
		"2020-03-27T35:04:05.000000+0000",  // Invalid Hour
		"2020-03-27T15:94:05.000000+0000",  // Invalid Minute
		"2020-03-27T15:04:85.000000+0000",  // Invalid Second
	}

	for _, dateTimeStr := range inValidInput {
		dateTimeValue, err := utils.ParseDateTime(dateTimeStr)
		assert.NotNil(t, err, fmt.Sprintf("Check for Invalid Date Time: %s - %+v", dateTimeStr, dateTimeValue))
	}
}

func TestStringInSlice(t *testing.T) {
	mySlice := []string{"aaaa", "bbbb", "cccc"}
	assert.True(t, utils.StringInSlice("aaaa", mySlice))
	assert.True(t, utils.StringInSlice("bbbb", mySlice))
	assert.True(t, utils.StringInSlice("cccc", mySlice))
	assert.False(t, utils.StringInSlice("aaaa1", mySlice))
	assert.False(t, utils.StringInSlice("aaaa", nil))
	assert.False(t, utils.StringInSlice("aaaa", []string{}))
}

func TestRemoveEntries(t *testing.T) {
	expected := []string{"aaaa", "bbbb", "cccc"}
	assert.Equal(t, expected, utils.RemoveItemsFromList(expected, nil))
	assert.Equal(t, expected, utils.RemoveItemsFromList([]string{"aaaa", "bbbb", "cccc", "dddd"}, []string{"dddd"}))
	assert.Equal(t, expected, utils.RemoveItemsFromList([]string{"aa", "aaaa", "bbbb", "cccc"}, []string{"aa"}))
	assert.Equal(t, expected, utils.RemoveItemsFromList([]string{"aa", "aaaa", "bbbb", "cccc", "fff"}, []string{"aa", "fff"}))
	assert.Equal(t, expected, utils.RemoveItemsFromList([]string{"aa", "aaaa", "bbbb", "cccc", "fff", "dddd"}, []string{"aa", "dddd", "fff"}))
	assert.Equal(t, expected, utils.RemoveItemsFromList([]string{"aaaa", "bbbb", "cccc", "dddd", "eeee"}, []string{"dddd", "eeee"}))
	assert.Nil(t, utils.RemoveItemsFromList(nil, []string{"dddd"}))
	assert.Nil(t, utils.RemoveItemsFromList(nil, nil))
}

func TestRemoveDuplicates(t *testing.T) {
	expected := []string{"aaaa", "bbbb", "cccc"}
	assert.Equal(t, expected, utils.RemoveDuplicates([]string{"aaaa", "bbbb", "cccc"}))
	assert.Equal(t, expected, utils.RemoveDuplicates([]string{"aaaa", "bbbb", "bbbb", "cccc"}))
	assert.Equal(t, expected, utils.RemoveDuplicates([]string{"aaaa", "bbbb", "cccc", "bbbb"}))
	assert.Equal(t, expected, utils.RemoveDuplicates([]string{"aaaa", "bbbb", "cccc", "cccc"}))
	assert.Equal(t, expected, utils.RemoveDuplicates([]string{"aaaa", "bbbb", "cccc", "aaaa"}))
	assert.Nil(t, utils.RemoveDuplicates(nil))
}

func TestHostInSlice(t *testing.T) {
	mySlice := []string{"project.dev.lfcla.com", "corporate.dev.lfcla.com", "contributor.dev.lfcla.com", "api.dev.lfcla.com", "dev.lfcla.com", "localhost", "localhost:8100", "localhost:8101"}
	assert.True(t, utils.HostInSlice("project.dev.lfcla.com", mySlice))
	assert.False(t, utils.HostInSlice("*.dev.lfcla.com", mySlice))
	assert.True(t, utils.HostInSlice("corporate.dev.lfcla.com", mySlice))
	assert.True(t, utils.HostInSlice("contributor.dev.lfcla.com", mySlice))
	assert.True(t, utils.HostInSlice("api.dev.lfcla.com", mySlice))
	assert.False(t, utils.HostInSlice("https://api.dev.lfcla.com", mySlice))
	assert.True(t, utils.HostInSlice("dev.lfcla.com", mySlice))
	assert.False(t, utils.HostInSlice("https://dev.lfcla.com", mySlice))
	assert.True(t, utils.HostInSlice("localhost", []string{"localhost", "foo", "bar"}))
	assert.True(t, utils.HostInSlice("localhost", []string{"foo", "localhost:8100", "localhost:8101"}))
}

func TestSliceDifferenceDiff1(t *testing.T) {
	a := []string{"a", "b", "c"}
	b := []string{"a", "d", "c"}
	assert.Equal(t, []string{"b", "d"}, utils.SliceDifference(a, b), "SliceDifference")
}
func TestSliceDifferenceDiff2(t *testing.T) {
	a := []string{"a", "c"}
	b := []string{"a", "c"}
	var c []string
	assert.Equal(t, c, utils.SliceDifference(a, b), "No Differences")
}
func TestSliceDifferenceDiff3(t *testing.T) {
	a := []string{"a", "b"}
	b := []string{"a", "b", "c"}
	assert.Equal(t, []string{"c"}, utils.SliceDifference(a, b), "B is Different")
}
func TestSliceDifferenceDiff4(t *testing.T) {
	a := []string{"a", "b", "c"}
	b := []string{"a", "b"}
	assert.Equal(t, []string{"c"}, utils.SliceDifference(a, b), "A is Different")
}
func TestSliceDifferenceDiff5(t *testing.T) {
	var a []string
	b := []string{"a", "b"}
	assert.Equal(t, []string{"a", "b"}, utils.SliceDifference(a, b), "A is empty")
}
func TestSliceDifferenceDiff6(t *testing.T) {
	a := []string{"a", "b"}
	var b []string
	assert.Equal(t, []string{"a", "b"}, utils.SliceDifference(a, b), "A is empty")
}

func TestTrimRemoveTrailingSpace(t *testing.T) {
	type KeyValue struct {
		input    string
		expected string
	}
	testValues := []KeyValue{
		{"SET #A = :a,", "SET #A = :a"},
		{"  SET #A = :a, ", "SET #A = :a"},
		{"  SET #A = :a ", "SET #A = :a"},
		{"SET #A = :a", "SET #A = :a"},
		{"SET #A = :a, #B = :b,", "SET #A = :a, #B = :b"},
		{"SET #A = :a, #B = :b, #C = :c,", "SET #A = :a, #B = :b, #C = :c"},
		{"SET #A = :a, #B = :b, #C = :c", "SET #A = :a, #B = :b, #C = :c"},
	}

	for _, testValue := range testValues {
		assert.Equal(t, testValue.expected, utils.TrimRemoveTrailingComma(testValue.input))
	}

}

// TestGetPathFromURL tests for getting the path for a URL
func TestGetPathFromURL(t *testing.T) {
	input := "https://cla-signature-files-dev.s3.amazonaws.com/contract-group/66b97366-a298-4625-965e-0c292c39f9a2/template/ccla-2020-09-25T22-37-51Z.pdf"
	expected := "/contract-group/66b97366-a298-4625-965e-0c292c39f9a2/template/ccla-2020-09-25T22-37-51Z.pdf"
	result, err := utils.GetPathFromURL(input)
	assert.Nil(t, err, "GetPathFromURL error is nil")
	assert.Equal(t, expected, result)
}
