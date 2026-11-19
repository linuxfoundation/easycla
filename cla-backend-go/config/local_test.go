// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadLocalConfigScreeningSwitchDefaultsOn(t *testing.T) {
	for name, tc := range map[string]struct {
		content string
		want    bool
	}{
		"not set":      {`{"sss":{"baseURL":"https://sss.invalid"}}`, true},
		"no section":   {`{}`, true},
		"switched off": {`{"sss":{"enabled":false}}`, false},
		"switched on":  {`{"sss":{"enabled":true}}`, true},
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			require.NoError(t, os.WriteFile(path, []byte(tc.content), 0600))
			cfg, err := loadLocalConfig(path)
			require.NoError(t, err)
			assert.Equal(t, tc.want, cfg.SSS.Enabled)
		})
	}
}
