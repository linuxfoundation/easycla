// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"fmt"
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/gofrs/uuid"

	"github.com/stretchr/testify/assert"
)

// TestValidEmail tests the email validator
func TestValidEmail(t *testing.T) {
	validEmails := []string{
		"user@linuxfoundation.org",
		"user+test@linuxfoundation.org",
	}
	inValidEmails := []string{
		"user@linuxfoundation_org",
		"user/linuxfoundation.org",
		"userlinuxfoundation.org",
	}

	for _, email := range validEmails {
		assert.True(t, utils.ValidEmail(email), fmt.Sprintf("valid email %s", email))
	}

	for _, email := range inValidEmails {
		assert.False(t, utils.ValidEmail(email), fmt.Sprintf("invalid email %s", email))
	}
}

// TestValidDomain tests the domain validator
func TestValidDomain(t *testing.T) {
	validDomains := []string{
		"linuxfoundation.org",
		"wikipedia.org",
		"google.com",
		"slack.com",
		"slack-domain-with-dash.com",
	}

	validWildcardDomains := []string{
		"linuxfoundation.org",
		"wikipedia.org",
		"google.com",
		"slack.com",
		"slack-domain-with-dash.com",
		"*.google.com",
		"*.us.google.com",
	}

	inValidDomains := []string{
		"*.google.com", // test case with allowWildcards = false
		"linuxfoundation_org",
		"*.linuxfoundation_org", // test case with allowWildcards = false
		"/linuxfoundation.org",
		"linuxfoundation+fun.org",
		"user_linuxfoundation.org",
	}

	inWildcardValidDomains := []string{
		"linuxfoundation_org",
		"/linuxfoundation.org",
		"linuxfoundation+fun.org",
		"*.linuxfoundation+fun.org",
		"user_linuxfoundation.org",
		"*.user_linuxfoundation.org",
	}

	for _, domain := range validDomains {
		msg, valid := utils.ValidDomain(domain, false)
		assert.True(t, valid, fmt.Sprintf("valid domain %s %s", domain, msg))
	}

	for _, domain := range validWildcardDomains {
		msg, valid := utils.ValidDomain(domain, true)
		assert.True(t, valid, fmt.Sprintf("valid domain %s %s", domain, msg))
	}

	for _, domain := range inValidDomains {
		msg, valid := utils.ValidDomain(domain, false)
		assert.False(t, valid, fmt.Sprintf("invalid domain %s %s", domain, msg))
	}

	for _, domain := range inWildcardValidDomains {
		msg, valid := utils.ValidDomain(domain, true)
		assert.False(t, valid, fmt.Sprintf("invalid domain %s %s", domain, msg))
	}
}

// TestGitHubUsername tests the GitHub username validator
func TestGitHubUsername(t *testing.T) {
	validGitHubUsername := []string{
		"linuxfoundation",
		"user123",
		"user_123",
		"user_name_with_underscores",
	}
	inValidGitHubUsername := []string{
		"li", // too short
		"/linuxfoundation",
		"linuxfoundation+fun",
		"user&linuxfoundation",
		"user{linuxfoundation",
		"user}linuxfoundation",
		"user*linuxfoundation",
		"user@linuxfoundation",
		"user!linuxfoundation",
		"user^linuxfoundation",
		"++userlinuxfoundation",
		"\\userlinuxfoundation",
	}

	for _, username := range validGitHubUsername {
		msg, valid := utils.ValidGitHubUsername(username)
		assert.True(t, valid, fmt.Sprintf("valid GitHub Username %s %s", username, msg))
	}

	for _, username := range inValidGitHubUsername {
		msg, valid := utils.ValidGitHubUsername(username)
		assert.False(t, valid, fmt.Sprintf("invalid GitHub Username %s %s", username, msg))
	}
}

// TestGitHubOrg tests the GitHub username validator
func TestGitHubOrg(t *testing.T) {
	validGitHubOrg := []string{
		"linuxfoundation",
		"linuxfoundation.org",
		"user123",
		"user-123",
		"user-123.org",
		"user-123.com",
		"user_123",
		"user_name_with_underscores",
	}
	inValidGitHubOrg := []string{
		"li", // too short
		"/linuxfoundation",
		"linuxfoundation+fun",
		"user&linuxfoundation",
		"user{linuxfoundation",
		"user}linuxfoundation",
		"user*linuxfoundation",
		"user@linuxfoundation",
		"user!linuxfoundation",
		"user^linuxfoundation",
		"++userlinuxfoundation",
		"\\userlinuxfoundation",
	}

	for _, org := range validGitHubOrg {
		msg, valid := utils.ValidGitHubOrg(org)
		assert.True(t, valid, fmt.Sprintf("valid GitHub Organization %s %s", org, msg))
	}

	for _, org := range inValidGitHubOrg {
		msg, valid := utils.ValidGitHubOrg(org)
		assert.False(t, valid, fmt.Sprintf("invalid GitHub Organization %s %s", org, msg))
	}
}

// TestGitlabUsername tests the Gitlab username validator
func TestGitlabUsername(t *testing.T) {
	validGitlabUsername := []string{
		"linuxfoundationuser",
		"user1234",
		"user_1234",
		"user_name_with_underscores_gitlab",
	}
	inValidGitlabUsername := []string{
		"ii", // too short
		"/linuxfoundationuser",
		"linuxfoundationuser+fun",
		"user&linuxfoundationuser",
		"user{linuxfoundationuser",
		"user}linuxfoundationuser",
		"user*linuxfoundationuser",
		"user@linuxfoundationuser",
		"user!linuxfoundationuser",
		"user^linuxfoundationuser",
		"++userlinuxfoundationuser",
		"\\userlinuxfoundationuser",
	}

	for _, username := range validGitlabUsername {
		msg, valid := utils.ValidGitlabUsername(username)
		assert.True(t, valid, fmt.Sprintf("valid Gitlab Username %s %s", username, msg))
	}

	for _, username := range inValidGitlabUsername {
		msg, valid := utils.ValidGitlabUsername(username)
		assert.False(t, valid, fmt.Sprintf("invalid Gitlab Username %s %s", username, msg))
	}
}

// TestGitlabOrg tests the GitHub username validator
func TestGitlabOrg(t *testing.T) {
	validGitlabOrg := []string{
		"linuxfoundationgrp",
		"linuxfoundationgrp.org",
		"user1234",
		"user-1234",
		"user-1234.org",
		"user-1234.com",
		"user_1234",
		"user_name_with_underscores_gitlab",
	}
	inValidGitlabOrg := []string{
		"hi", // too short
		"/linuxfoundationgrp",
		"linuxfoundationgrp+fun",
		"user&linuxfoundationgrp",
		"user{linuxfoundationgrp",
		"user}linuxfoundationgrp",
		"user*linuxfoundationgrp",
		"user@linuxfoundationgrp",
		"user!linuxfoundationgrp",
		"user^linuxfoundationgrp",
		"++userlinuxfoundationgrp",
		"\\userlinuxfoundationgrp",
	}

	for _, org := range validGitlabOrg {
		msg, valid := utils.ValidGitHubOrg(org)
		assert.True(t, valid, fmt.Sprintf("valid GitHub Organization %s %s", org, msg))
	}

	for _, org := range inValidGitlabOrg {
		msg, valid := utils.ValidGitHubOrg(org)
		assert.False(t, valid, fmt.Sprintf("invalid GitHub Organization %s %s", org, msg))
	}
}
func TestIsUUIDv4True(t *testing.T) {
	v4, err := uuid.NewV4()
	assert.Nil(t, err, "NewV4 UUID is nil")
	assert.True(t, utils.IsUUIDv4(v4.String()), fmt.Sprintf("%s is a v4 UUID", v4.String()))
}

func TestIsUUIDv4LikeSFID(t *testing.T) {
	sfid := "0014100000TdznWAAR"
	assert.False(t, utils.IsUUIDv4(sfid), fmt.Sprintf("%s is not v4 UUID", sfid))
}

func TestIsSalesForceID(t *testing.T) {
	trueTestData := []string{
		"00117000015vpjX",
		"00117000015vpjXAAQ",
	}
	falseTestData := []string{
		"",
		"00117",
		"-00117",
		"00117000015vpj-",
		"0011700001-vpjXAAQ",
		"0011700001?vpjXAAQ",
		"0011700001&vpjXAAQ",
		"0011700001_vpjXAAQ",
	}

	for i := range trueTestData {
		assert.True(t, utils.IsSalesForceID(trueTestData[i]))
	}
	for i := range falseTestData {
		assert.False(t, utils.IsSalesForceID(falseTestData[i]))
	}
}
