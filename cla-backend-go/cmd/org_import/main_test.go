// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/easycla/cla-backend-go/orgimport"
)

func TestRunUsageErrors(t *testing.T) {
	t.Setenv("STAGE", "")
	assert.Equal(t, 2, run(nil, strings.NewReader("")))
	assert.Equal(t, 2, run([]string{"bogus"}, strings.NewReader("")))
	assert.Equal(t, 2, run([]string{"ingest", "--nope"}, strings.NewReader("")))
	assert.Equal(t, 2, run([]string{"ingest"}, strings.NewReader("")), "STAGE is required")
	t.Setenv("STAGE", "dev")
	assert.Equal(t, 2, run([]string{"ingest", "--routes", "delete"}, strings.NewReader("")))
	assert.Equal(t, 2, run([]string{"ingest", "--apply", "--yes", "--out-dir", t.TempDir()}, strings.NewReader("")), "rewrite apply needs --state")
	assert.Equal(t, 2, run([]string{"ingest", "--apply", "--yes", "--routes", "rewrite", "--out-dir", t.TempDir()}, strings.NewReader("")))
}

func TestSnapshotInputs(t *testing.T) {
	dir := t.TempDir()
	mapping := filepath.Join(dir, "m.csv")
	require.NoError(t, os.WriteFile(mapping, []byte("old_id,new_id,action,approved\n"), 0o600))
	outDir := filepath.Join(dir, "out")
	require.NoError(t, os.MkdirAll(outDir, 0o750))
	var out, errOut bytes.Buffer
	snapshotInputs(outDir, orgimport.Options{Mapping: mapping, Decisions: filepath.Join(dir, "missing.csv"), State: filepath.Join(dir, "state.jsonl")}, &out, &errOut)
	data, err := os.ReadFile(filepath.Clean(filepath.Join(outDir, "input-mapping.csv")))
	require.NoError(t, err)
	assert.Equal(t, "old_id,new_id,action,approved\n", string(data))
	assert.Contains(t, errOut.String(), "input-decisions.csv: cannot read")
	assert.Contains(t, out.String(), "input-state.jsonl")
	assert.Contains(t, out.String(), "does not exist yet")
	_, err = os.Stat(filepath.Join(outDir, "input-state.jsonl"))
	assert.True(t, os.IsNotExist(err))
}

func TestConfirm(t *testing.T) {
	var out bytes.Buffer
	assert.True(t, confirm("dev", &out, strings.NewReader("dev\n")))
	assert.Contains(t, out.String(), "Type the stage name")
	assert.False(t, confirm("dev", &out, strings.NewReader("prod\n")))
	assert.False(t, confirm("dev", &out, strings.NewReader("")))
	assert.True(t, confirm("prod", &out, strings.NewReader("prod")), "last line without newline")
}

func TestRunnerAndRevision(t *testing.T) {
	t.Setenv("GITHUB_RUN_ID", "42")
	assert.Equal(t, "gha-42", runnerName())
	t.Setenv("GITHUB_RUN_ID", "")
	t.Setenv("USER", "lukasz")
	assert.True(t, strings.HasPrefix(runnerName(), "lukasz@"))
	assert.NotEmpty(t, buildRevision())
	commit, branch = "abc123", "unicron-x"
	defer func() { commit, branch = "", "" }()
	assert.Equal(t, "abc123 (unicron-x)", buildRevision(), "Makefile -X values win over VCS stamping")
	branch = ""
	assert.Equal(t, "abc123", buildRevision())
}
