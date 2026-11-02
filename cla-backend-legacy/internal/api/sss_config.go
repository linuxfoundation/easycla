// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmTypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/logging"
	sss "github.com/linuxfoundation/easycla/cla-sss-base"
)

const sssConfigRetryInterval = 30 * time.Second

type sssParameterClient interface {
	GetParameter(context.Context, *ssm.GetParameterInput, ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

type sssConfiguration struct {
	baseURL  string
	audience string
	required bool
	enabled  bool
}

func loadSSSConfig(ctx context.Context, client sssParameterClient, stage string) (sssConfiguration, error) {
	keys := []string{
		fmt.Sprintf("cla-sss-base-url-%s", stage),
		fmt.Sprintf("cla-sss-auth0-audience-%s", stage),
		fmt.Sprintf("cla-sss-required-%s", stage),
		fmt.Sprintf("cla-sss-enabled-%s", stage),
	}
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		out, err := client.GetParameter(ctx, &ssm.GetParameterInput{Name: &key})
		if err != nil {
			var notFound *ssmTypes.ParameterNotFound
			if errors.As(err, &notFound) {
				logging.Debugf("optional SSM key %s not provisioned", key)
				continue
			}
			return sssConfiguration{}, fmt.Errorf("unable to read SSS parameter %s: %w", key, err)
		}
		if out == nil || out.Parameter == nil || out.Parameter.Value == nil {
			return sssConfiguration{}, fmt.Errorf("SSM returned an incomplete SSS parameter %s", key)
		}
		values[key] = strings.TrimSpace(*out.Parameter.Value)
	}
	boolValue := func(key string, fallback bool) bool {
		value := values[key]
		if value == "" {
			return fallback
		}
		parsed, parseErr := strconv.ParseBool(value)
		if parseErr != nil {
			logging.WithError(parseErr).Warnf("unable to parse optional SSM key %s as boolean - using default value %t", key, fallback)
			return fallback
		}
		return parsed
	}
	return sssConfiguration{
		baseURL:  values[keys[0]],
		audience: values[keys[1]],
		required: boolValue(keys[2], false),
		enabled:  boolValue(keys[3], true),
	}, nil
}

func (h *Handlers) initializeSSS(ctx context.Context) error {
	cfg, err := loadSSSConfig(ctx, h.sssConfigClient, h.sssConfigStage)
	if err != nil {
		h.sssConfigErr = err
		h.sssRequired = true
		h.sssEnabled = true
		return err
	}
	client, err := sss.NewClientFromPlatformCredentials(
		cfg.baseURL, cfg.audience,
		strings.TrimSpace(os.Getenv("PLATFORM_AUTH0_URL")),
		strings.TrimSpace(os.Getenv("PLATFORM_AUTH0_CLIENT_ID")),
		strings.TrimSpace(os.Getenv("PLATFORM_AUTH0_CLIENT_SECRET")),
	)
	if err != nil {
		if cfg.enabled && cfg.required {
			return fmt.Errorf("failed to initialize required SSS client: %w", err)
		}
		logging.Warnf("failed to initialize optional SSS client, screening will be unavailable: %v", err)
	}
	if cfg.enabled && cfg.required && client == nil {
		return errors.New("SSS is required but not configured (base URL or audience missing in SSM)")
	}
	h.sssClient = client
	h.sssRequired = cfg.required
	h.sssEnabled = cfg.enabled
	h.sssConfigErr = nil
	return nil
}

func (h *Handlers) retrySSSConfig(ctx context.Context) error {
	h.sssConfigMu.Lock()
	defer h.sssConfigMu.Unlock()
	// After recovery these fields are immutable; every SSS consumer passes this
	// lock before reading them, including requests that waited for recovery.
	if h.sssConfigErr == nil {
		return nil
	}
	if time.Now().Before(h.sssConfigRetryAfter) {
		return h.sssConfigErr
	}
	if err := h.initializeSSS(ctx); err != nil {
		h.sssConfigErr = err
		if ctx.Err() == nil {
			h.sssConfigRetryAfter = time.Now().Add(sssConfigRetryInterval)
		}
		logging.WithError(err).Error("SSS configuration recovery failed; signing remains blocked")
		return err
	}
	logging.Info("SSS configuration recovered")
	return nil
}
