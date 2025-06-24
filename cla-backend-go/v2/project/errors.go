// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package project

import (
	"errors"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
)

// errors
var (
	ErrCLAGroupDoesNotExist = errors.New("cla group does not exist")
	ErrCLAGroupIDMissing    = errors.New("cla group id is missing")
)

// codedResponse interface
type codedResponse interface {
	Code() string
}

// errorResponse is a helper to wrap the specified error into an error response model
func errorResponse(reqID string, err error) *models.ErrorResponse {
	code := ""
	if e, ok := err.(codedResponse); ok {
		code = e.Code()
	}

	e := models.ErrorResponse{
		Code:       code,
		Message:    err.Error(),
		XRequestID: reqID,
	}

	return &e
}
