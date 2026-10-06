// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StateRecord is one append-only JSONL line: a group's progress through the rewrite steps.
type StateRecord struct {
	OldID      string   `json:"old_id"`
	Key        string   `json:"key,omitempty"`
	NewID      string   `json:"new_id"`
	CompanyIDs []string `json:"company_ids"`
	Step       string   `json:"step"`
	TS         string   `json:"ts"`
	Status     string   `json:"status"`
	Err        string   `json:"err,omitempty"`
}

// key is the group selector: the company id of a row-targeted group, otherwise the old external id.
func (rec StateRecord) key() string {
	if rec.Key != "" {
		return rec.Key
	}
	return rec.OldID
}

// State is the parsed state file; Last holds the newest record per group key.
type State struct {
	path string
	Last map[string]StateRecord
}

// LoadState reads the state file (a missing file is an empty state).
func LoadState(path string) (*State, error) {
	st := &State{path: path, Last: map[string]StateRecord{}}
	if path == "" {
		return st, nil
	}
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		var rec StateRecord
		if err := json.Unmarshal([]byte(text), &rec); err != nil {
			return nil, fmt.Errorf("state line %d: %w", line, err)
		}
		if rec.key() == "" {
			return nil, fmt.Errorf("state line %d: empty old_id", line)
		}
		st.Last[rec.key()] = rec
	}
	return st, sc.Err()
}

// Unfinished returns the groups that were started but never reached the done step.
func (st *State) Unfinished() []StateRecord {
	var out []StateRecord
	for _, rec := range st.Last {
		if rec.Step != StepDone || rec.Status != statusOK {
			out = append(out, rec)
		}
	}
	sortRecords(out)
	return out
}

// Done reports whether the group (old external id, or company id of a row-targeted group) completed all steps.
func (st *State) Done(key string) bool {
	rec, ok := st.Last[key]
	return ok && rec.Step == StepDone && rec.Status == statusOK
}

// Append writes one record durably (write, fsync, close); Last is updated only after success.
// It is a no-op when the state file is not set.
func (st *State) Append(rec StateRecord, now time.Time) error {
	if st.path == "" {
		return nil
	}
	rec.TS = now.UTC().Format(time.RFC3339)
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(st.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err = f.Write(append(b, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	st.Last[rec.key()] = rec
	return nil
}

func sortRecords(recs []StateRecord) {
	for i := 1; i < len(recs); i++ {
		for j := i; j > 0 && recs[j].key() < recs[j-1].key(); j-- {
			recs[j], recs[j-1] = recs[j-1], recs[j]
		}
	}
}
