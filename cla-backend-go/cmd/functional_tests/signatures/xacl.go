// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"encoding/base64"
	"encoding/json"

	"github.com/LF-Engineering/lfx-kit/auth"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
)

// GetXACLGoogle returns the X-ACL entry for this company
func GetXACLGoogle() (string, error) {
	xACL := auth.ACL{
		Admin:    false,
		Allowed:  true,
		Resource: "company_signatures",
		Context:  "staff",
		Scopes: []auth.Scope{
			{
				Type:  "organization",
				ID:    companyExternalIDGoogle,
				Role:  "cla-manager",
				Level: "staff",
			},
		},
	}

	jsonString, marshalErr := json.Marshal(xACL)
	if marshalErr != nil {
		log.Warnf("unable to marshall X-ACL entry, error: %+v", marshalErr)
		return "", marshalErr
	}
	return base64.StdEncoding.EncodeToString(jsonString), nil
}

// GetXACLATT returns the X-ACL entry for this company
func GetXACLATT() (string, error) {
	xACL := auth.ACL{
		Admin:    false,
		Allowed:  true,
		Resource: "company_signatures",
		Context:  "staff",
		Scopes: []auth.Scope{
			{
				Type:  "organization",
				ID:    companyExternalIDATT,
				Role:  "cla-manager",
				Level: "staff",
			},
		},
	}

	jsonString, marshalErr := json.Marshal(xACL)
	if marshalErr != nil {
		log.Warnf("unable to marshall X-ACL entry, error: %+v", marshalErr)
		return "", marshalErr
	}
	return base64.StdEncoding.EncodeToString(jsonString), nil
}

// GetXACLIBM returns the X-ACL entry for this company
func GetXACLIBM() (string, error) {
	xACL := auth.ACL{
		Admin:    false,
		Allowed:  true,
		Resource: "company_signatures",
		Context:  "staff",
		Scopes: []auth.Scope{
			{
				Type:  "organization",
				ID:    companyExternalIDIBM,
				Role:  "cla-manager",
				Level: "staff",
			},
		},
	}

	jsonString, marshalErr := json.Marshal(xACL)
	if marshalErr != nil {
		log.Warnf("unable to marshall X-ACL entry, error: %+v", marshalErr)
		return "", marshalErr
	}
	return base64.StdEncoding.EncodeToString(jsonString), nil
}
