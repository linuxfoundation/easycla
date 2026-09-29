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
	notices := snapshotInputs(outDir, orgimport.Options{Mapping: mapping, Decisions: filepath.Join(dir, "missing.csv"), State: filepath.Join(dir, "state.jsonl")}, &out, &errOut)
	data, err := os.ReadFile(filepath.Clean(filepath.Join(outDir, "input-mapping.csv")))
	require.NoError(t, err)
	assert.Equal(t, "old_id,new_id,action,approved\n", string(data))
	assert.Contains(t, errOut.String(), "input-decisions.csv: cannot read")
	require.Len(t, notices, 1, "an uncaptured input is a report notice")
	assert.Contains(t, notices[0], "input input-decisions.csv: cannot read")
	assert.Contains(t, out.String(), "input-state.jsonl")
	assert.Contains(t, out.String(), "does not exist yet")
	_, err = os.Stat(filepath.Join(outDir, "input-state.jsonl"))
	assert.True(t, os.IsNotExist(err))
}

func TestCaptureJournal(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "out")
	require.NoError(t, os.MkdirAll(outDir, 0o750))
	var out, errOut bytes.Buffer

	assert.Nil(t, captureJournal(outDir, "", &out, &errOut), "no journal configured")
	assert.Nil(t, captureJournal(outDir, filepath.Join(dir, "never.jsonl"), &out, &errOut))
	assert.Contains(t, out.String(), "nothing was journaled")
	_, err := os.Stat(filepath.Join(outDir, "state.jsonl"))
	assert.True(t, os.IsNotExist(err))

	// an external journal is copied in full as state.jsonl after the run
	external := filepath.Join(dir, "journal.jsonl")
	require.NoError(t, os.WriteFile(external, []byte("{\"step\":\"start\"}\n{\"step\":\"done\"}\n"), 0o600))
	assert.Nil(t, captureJournal(outDir, external, &out, &errOut))
	data, err := os.ReadFile(filepath.Clean(filepath.Join(outDir, "state.jsonl")))
	require.NoError(t, err)
	assert.Equal(t, "{\"step\":\"start\"}\n{\"step\":\"done\"}\n", string(data))
	assert.Contains(t, out.String(), "copied (33 bytes)")

	// the record's own journal is not copied onto itself
	inRecord := filepath.Join(outDir, "state.jsonl")
	before, err := os.Stat(inRecord)
	require.NoError(t, err)
	assert.Nil(t, captureJournal(outDir, filepath.Join(outDir, "..", "out", "state.jsonl"), &out, &errOut))
	after, err := os.Stat(inRecord)
	require.NoError(t, err)
	assert.Equal(t, before.ModTime(), after.ModTime())

	// an unreadable journal is a report notice
	if os.Geteuid() != 0 {
		locked := filepath.Join(dir, "locked.jsonl")
		require.NoError(t, os.WriteFile(locked, []byte("{}\n"), 0o000))
		notices := captureJournal(outDir, locked, &out, &errOut)
		require.Len(t, notices, 1)
		assert.Contains(t, notices[0], "journal "+locked+": cannot read")
		assert.Contains(t, errOut.String(), notices[0])
	}
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
