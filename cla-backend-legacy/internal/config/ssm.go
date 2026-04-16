// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/store"
)

// Thin SSM helper used to fetch a small number of secrets that are not safe/feasible
// to inject into Lambda environment variables (e.g. multiline private keys).
//
// This intentionally mirrors legacy Python behavior (load_ssm_keys) and existing
// v3/v4 Go behavior (config/ssm.go) conceptually.

var (
	ssmOnce   sync.Once
	ssmClient *ssm.Client
	ssmErr    error

	cacheMu sync.Mutex
	cache   = map[string]string{}
)

func regionFromEnv() string {
	// Prefer explicit AWS_REGION, then fall back to the convention used elsewhere
	// in this repo (REGION / DYNAMODB_AWS_REGION).
	for _, k := range []string{"AWS_REGION", "AWS_DEFAULT_REGION", "REGION", "DYNAMODB_AWS_REGION"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return "us-east-1"
}

func getSSMClient(ctx context.Context) (*ssm.Client, error) {
	ssmOnce.Do(func() {
		cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(regionFromEnv()))
		if err != nil {
			ssmErr = err
			return
		}
		ssmClient = ssm.NewFromConfig(cfg)
	})
	return ssmClient, ssmErr
}

// GetEnvOrSSM returns the environment variable if set, otherwise fetches the value
// from SSM parameter store using the base key suffixed by the current STAGE.
//
// Example:
//
//	envName="GITHUB_PRIVATE_KEY"
//	ssmBaseKey="cla-gh-app-private-key"
//	stage="dev" -> parameter name "/cla-gh-app-private-key-dev"
func GetEnvOrSSM(ctx context.Context, envName, ssmBaseKey string) (string, error) {
	if v := strings.TrimSpace(os.Getenv(envName)); v != "" {
		return v, nil
	}
	stage := store.StageFromEnv()
	if stage == "" {
		return "", fmt.Errorf("%s not set and STAGE is empty; cannot resolve SSM parameter", envName)
	}
	paramName := fmt.Sprintf("/%s-%s", strings.TrimPrefix(ssmBaseKey, "/"), stage)
	return GetSSMParameter(ctx, paramName)
}

// GetSSMParameter fetches a parameter by name. It also tries an alternate name
// with/without a leading slash to match historical inconsistencies.
func GetSSMParameter(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("empty SSM parameter name")
	}

	cacheMu.Lock()
	if v, ok := cache[name]; ok {
		cacheMu.Unlock()
		return v, nil
	}
	cacheMu.Unlock()

	client, err := getSSMClient(ctx)
	if err != nil {
		return "", err
	}

	tryNames := []string{name}
	if strings.HasPrefix(name, "/") {
		tryNames = append(tryNames, strings.TrimPrefix(name, "/"))
	} else {
		tryNames = append(tryNames, "/"+name)
	}

	var lastErr error
	for _, n := range tryNames {
		out, e := client.GetParameter(ctx, &ssm.GetParameterInput{
			Name:           aws.String(n),
			WithDecryption: aws.Bool(true),
		})
		if e != nil {
			lastErr = e
			continue
		}
		if out.Parameter == nil || out.Parameter.Value == nil {
			lastErr = fmt.Errorf("ssm parameter %q returned empty value", n)
			continue
		}
		val := aws.ToString(out.Parameter.Value)
		cacheMu.Lock()
		cache[name] = val
		cacheMu.Unlock()
		return val, nil
	}
	return "", lastErr
}
