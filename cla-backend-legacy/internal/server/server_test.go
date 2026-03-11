// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package server

import (
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/api"
)

func TestServerCompilation(t *testing.T) {
	// Simple compilation smoke test
	h := &api.Handlers{}
	if h == nil {
		t.Fatal("handlers should initialize")
	}
}