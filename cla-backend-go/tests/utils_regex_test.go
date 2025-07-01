// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package tests

import (
	"fmt"
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
)

// TestValidCompanyName is a collection of unit tests for the ValidCompanyName utility function
func TestValidCompanyName(t *testing.T) {
	// Valid company names
	validInput := []string{
		"This is a company name",
		"23 Air Jordan",
		"sdfsdfsdfááÀÁ(test)[test]0343+_7343-9(@)/the world",
		"ááÀÁ test",
		"世界",
		"?ááÀÁ test",
		"!valid name",
		".test",
	}

	for _, str := range validInput {
		utils.ValidCompanyName(str)
		assert.True(t, utils.ValidCompanyName(str), fmt.Sprintf("ValidCompanyName - %s", str))
	}

	var longString string
	for i := 1; i <= 256; i++ {
		longString += "a"
	}
	//log.Printf("longString = %d", len(longString))

	// Invalid company names
	inValidInput := []string{
		"<html>",
		"<p>test</p>",
		"<javascript src=\"harold.js\"/>",
		"",  // min 2 chars
		"1", // min 2 chars
		longString,
	}

	for _, str := range inValidInput {
		assert.False(t, utils.ValidCompanyName(str), fmt.Sprintf("ValidCompanyName - %s", str))
	}
}

// TestValidWebsite is a collection of unit tests for the ValidWebsite utility function
func TestValidWebsite(t *testing.T) {
	// Valid websites
	validInput := []string{
		"http://www.Augnewcompanynirupamav.com",
		"https://Augnewcompanynirupamav.com",
		"https://www.Augnewcompanynirupamav.com",
		"http://www.Augnewcompanynirupamav.com?input=value&foo=bar",
		"Augnewcompanynirupamav.com",
		"Augnewcompanynirupamav.io",
		"Augnewcompanynirupamav.edu",
		"Augnewcompanynirupamav.in",
		"http://www.my-domain.com",
		"http://www-2.my-domain.com",
		"https://www.my-domain.com",
		"https://www-2.my-domain.com",
		"https://www-2.test.my-domain.com",
	}

	for _, str := range validInput {
		utils.ValidWebsite(str)
		assert.True(t, utils.ValidWebsite(str), fmt.Sprintf("ValidWebsite - %s", str))
	}

	var longString string
	for i := 1; i <= 256; i++ {
		longString += "a"
	}
	//log.Printf("longString = %d", len(longString))

	// Invalid company names
	inValidInput := []string{
		"Augnewcompanynirupamav",
		"yahoo",
		"!invalid name",
		".test",
		"?ááÀÁ test",
		"",         // min 5 chars
		"1",        // min 5 chars
		"12",       // min 5 chars
		"123",      // min 5 chars
		"1234",     // min 5 chars
		"a.ai",     // min 5.chars
		longString, // max 255 chars
	}

	for _, str := range inValidInput {
		assert.False(t, utils.ValidWebsite(str), fmt.Sprintf("ValidWebsite - %s", str))
	}
}
