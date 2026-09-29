// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const trueString = "true"

// Mapping actions produced by Salesforce (mapping file or Apex).
const (
	ActionMatched   = "matched"
	ActionCreated   = "created"
	ActionAmbiguous = "ambiguous"
)

// MappingRow is one old_id,new_id,action,approved line.
type MappingRow struct {
	OldID    string
	NewID    string
	Action   string
	Approved bool
}

// Mapping is the validated Salesforce mapping file; Targets counts old ids per new id (feeds the
// approval-aware collision pass).
type Mapping struct {
	Rows    map[string]MappingRow
	Targets map[string]int
}

// LoadMapping reads and validates a mapping CSV (header required).
func LoadMapping(path string) (*Mapping, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return ParseMapping(f)
}

// ParseMapping parses a mapping CSV.
func ParseMapping(r io.Reader) (*Mapping, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("mapping: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("mapping: empty file")
	}
	header := records[0]
	col := map[string]int{}
	for i, h := range header {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, name := range []string{"old_id", "new_id", "action", "approved"} {
		if _, ok := col[name]; !ok {
			return nil, fmt.Errorf("mapping: missing column %q (need old_id,new_id,action,approved)", name)
		}
	}
	m := &Mapping{Rows: map[string]MappingRow{}, Targets: map[string]int{}}
	for n, rec := range records[1:] {
		line := n + 2
		if len(rec) < len(header) {
			return nil, fmt.Errorf("mapping line %d: expected %d columns", line, len(header))
		}
		row := MappingRow{
			OldID:  strings.TrimSpace(rec[col["old_id"]]),
			NewID:  strings.TrimSpace(rec[col["new_id"]]),
			Action: strings.ToLower(strings.TrimSpace(rec[col["action"]])),
		}
		switch strings.ToLower(strings.TrimSpace(rec[col["approved"]])) {
		case trueString, "yes", "1":
			row.Approved = true
		case "false", "no", "0", "":
		default:
			return nil, fmt.Errorf("mapping line %d: approved must be true|false", line)
		}
		if row.OldID == "" {
			return nil, fmt.Errorf("mapping line %d: empty old_id", line)
		}
		if _, dup := m.Rows[row.OldID]; dup {
			return nil, fmt.Errorf("mapping line %d: duplicate old_id %s", line, row.OldID)
		}
		switch row.Action {
		case ActionMatched, ActionCreated:
			if !IsSFID(row.NewID) {
				return nil, fmt.Errorf("mapping line %d: new_id %q is not a Salesforce account id", line, row.NewID)
			}
			m.Targets[row.NewID]++
		case ActionAmbiguous:
		default:
			return nil, fmt.Errorf("mapping line %d: action must be matched|created|ambiguous", line)
		}
		m.Rows[row.OldID] = row
	}
	return m, nil
}

// Resolve returns the new id for oldID or the manual reason why it cannot be used.
func (m *Mapping) Resolve(oldID string) (newID, action, reason string) {
	if m == nil {
		return "", "", ReasonNoMapping
	}
	row, ok := m.Rows[oldID]
	if !ok {
		return "", "", ReasonNoMapping
	}
	switch {
	case row.Action == ActionAmbiguous:
		return "", row.Action, ReasonMappingAmbiguous
	case row.Action == ActionMatched && !row.Approved:
		return "", row.Action, "mapping_not_approved"
	case row.NewID == oldID:
		return "", row.Action, ReasonMappingSameID
	}
	return row.NewID, row.Action, ""
}
