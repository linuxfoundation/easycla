// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizePlainText(t *testing.T) {
	assert.Equal(t, "", SanitizePlainText(""))
	assert.Equal(t, "", SanitizePlainText(" \r\n \x07\x1b \t "))
	assert.Equal(t, "one\ntwo\nthree", SanitizePlainText("one\r\ntwo\rthree"), "CR and CRLF normalize to LF")
	assert.Equal(t, "keep\ttabs\nand lines", SanitizePlainText("keep\ttabs\nand lines"))
	assert.Equal(t, "bell stripped", SanitizePlainText("bell\x07 stripped\x00"))
	assert.Equal(t, "trimmed", SanitizePlainText("  trimmed \n"))
}

func TestSanitizeSingleLine(t *testing.T) {
	assert.Equal(t, "", SanitizeSingleLine(""))
	assert.Equal(t, "Subject line", SanitizeSingleLine("Subject\r\n line\x07"))
	assert.Equal(t, "no tabs", SanitizeSingleLine("no\t tabs"))
}
