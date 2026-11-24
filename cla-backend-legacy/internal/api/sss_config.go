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
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/logging"
	sss "github.com/linuxfoundation/easycla/cla-sss-base"
)

const sssConfigRetryInterval = 30 * time.Second

type sssParameterClient interface {
	GetParameters(context.Context, *ssm.GetParametersInput, ...func(*ssm.Options)) (*ssm.GetParametersOutput, error)
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
	out, err := client.GetParameters(ctx, &ssm.GetParametersInput{Names: keys})
	if err != nil {
		return sssConfiguration{}, fmt.Errorf("unable to read SSS parameters: %w", err)
	}
	values := make(map[string]string, len(keys))
	notProvisioned := make(map[string]bool, len(keys))
	if out != nil {
		for _, parameter := range out.Parameters {
			if parameter.Name != nil && parameter.Value != nil {
				values[strings.TrimPrefix(*parameter.Name, "/")] = strings.TrimSpace(*parameter.Value)
			}
		}
		for _, name := range out.InvalidParameters {
			notProvisioned[strings.TrimPrefix(name, "/")] = true
		}
	}
	for _, key := range keys {
		if _, ok := values[key]; ok {
			continue
		}
		if !notProvisioned[key] {
			return sssConfiguration{}, fmt.Errorf("SSM returned an incomplete SSS parameter %s", key)
		}
		logging.Debugf("optional SSM key %s not provisioned", key)
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
