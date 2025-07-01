// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package docraptor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
)

var (
	errInvalidKey = errors.New("invalid key")
)

const (
	docraptorURL = "https://%s@docraptor.com/docs"
)

// Client structure model
type Client struct {
	apiKey   string
	url      string
	testMode bool
}

// NewDocraptorClient creates a new docraptor client instance
func NewDocraptorClient(key string, testMode bool) (Client, error) {
	if key == "" {
		return Client{}, errInvalidKey
	}

	url := fmt.Sprintf(docraptorURL, key)

	return Client{
		apiKey:   key,
		url:      url,
		testMode: testMode,
	}, nil
}

// CreatePDF accepts an HTML document and returns a PDF
func (dc Client) CreatePDF(html string, claType string) (io.ReadCloser, error) {
	f := logrus.Fields{
		"functionName": "v1.docraptor.client.CreatePDF",
		"claType":      claType,
		"testMode":     dc.testMode,
		"url":          dc.url,
	}

	document := map[string]interface{}{
		"document_type":    "pdf",
		"document_content": html,
		"name":             "docraptor-go.pdf",
		"test":             dc.testMode,
	}

	documentBytes, err := json.Marshal(document)
	if err != nil {
		log.WithFields(f).Warnf("unable to encode docraptor payload for request, error: %+v", err)
		return nil, err
	}

	log.WithFields(f).Debug("Generating PDF using docraptor...")
	resp, err := http.Post(dc.url, "application/json", bytes.NewBuffer(documentBytes))
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem with API call to docraptor url: %s", dc.url)
		return nil, err
	}
	// Do not close - rely on the caller to close the reader otherwise we will get the read from Response.Body after Close error
	//defer func() {
	//	closeErr := resp.Body.Close()
	//	if closeErr != nil {
	//		log.WithFields(f).WithError(closeErr).Warn("problem closing docraptor response")
	//	}
	//}()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		msg := fmt.Sprintf("unexpected http response code from docraptor url: %s, status code: %d", dc.url, resp.StatusCode)
		log.WithFields(f).Warn(msg)
		return nil, errors.New(msg)
	}
	log.WithFields(f).Debugf("successful response from docraptor url: %s, status code: %d", dc.url, resp.StatusCode)

	return resp.Body, nil
}
