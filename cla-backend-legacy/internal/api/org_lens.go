// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"sync"

	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/config"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/logging"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/store"
)

var orgLensAccountID = regexp.MustCompile(`^001[A-Za-z0-9]{15}$`)

var (
	orgLensBaseOnce  sync.Once
	orgLensBaseValue string
)

// orgLensBase reads the optional setting cla-org-lens-base-{stage} once; unset or unreadable keeps the Corporate Console link
func orgLensBase(ctx context.Context) string {
	orgLensBaseOnce.Do(func() {
		stage := store.StageFromEnv()
		if stage == "" {
			return
		}
		value, err := config.GetSSMParameter(ctx, "cla-org-lens-base-"+stage)
		if err != nil {
			var notFound *ssmtypes.ParameterNotFound
			if !errors.As(err, &notFound) {
				logging.Warnf("unable to read cla-org-lens-base-%s, e-mails keep the Corporate Console link: %v", stage, err)
			}
			return
		}
		orgLensBaseValue = strings.TrimSpace(value)
	})
	return orgLensBaseValue
}

// orgLensURL returns the Organization lens page of the CLA Group for the company,
// or "" when no lens base is set or the company id is not a Salesforce Account id
func orgLensURL(base, companySFID, claGroupID string) string {
	base, companySFID, claGroupID = strings.TrimSpace(base), strings.TrimSpace(companySFID), strings.TrimSpace(claGroupID)
	if base == "" || !orgLensAccountID.MatchString(companySFID) {
		return ""
	}
	if !strings.Contains(base, "://") {
		base = "https://" + base
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return ""
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/org/" + companySFID + "/easycla"
	if claGroupID != "" {
		u.Path += "/" + claGroupID
	}
	u.RawPath, u.RawQuery, u.Fragment = "", "", ""
	return u.String()
}
