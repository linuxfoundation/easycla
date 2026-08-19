// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import "strings"

const (
	// Connected status
	Connected = "connected"
	// PartialConnection status
	PartialConnection = "partial_connection"
	// ConnectionFailure status
	ConnectionFailure = "connection_failure"
	// NoConnection status
	NoConnection = "no_connection"
)

// SelfServeSignatureSource is the source marker recorded on an active signature session started
// proactively from LFX Self Serve - such a session carries no pull/merge request context
const SelfServeSignatureSource = "self-serve"

// IsSelfServeActiveSignature reports whether the active signature metadata belongs to a signing
// session started proactively from LFX Self Serve
func IsSelfServeActiveSignature(metadata map[string]interface{}) bool {
	source, ok := metadata["source"].(string)
	return ok && strings.EqualFold(source, SelfServeSignatureSource)
}
