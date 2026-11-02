// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestColumnsLegendCoversEveryReportColumn(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, writeAuditReports(dir, &AuditResult{}))
	require.NoError(t, writeIngestReports(dir, &Plan{}))

	legend := map[string]map[string]bool{}
	for _, row := range readCSV(t, filepath.Join(dir, ColumnsFile))[1:] {
		require.Len(t, row, 5, row)
		for _, cell := range row {
			assert.NotEmpty(t, cell, row)
		}
		if legend[row[0]] == nil {
			legend[row[0]] = map[string]bool{}
		}
		assert.False(t, legend[row[0]][row[1]], "duplicate legend row %v", row)
		legend[row[0]][row[1]] = true
	}
	for _, name := range []string{"audit.csv", "unresolvable.csv", "possible_duplicates.csv", "plan.csv", "to_salesforce.csv", "manual_actions.csv", "targets.csv"} {
		header := readCSV(t, filepath.Join(dir, name))[0]
		for _, col := range header {
			assert.True(t, legend[name][col], "%s: column %q has no legend row", name, col)
		}
		assert.Len(t, legend[name], len(header), "%s: legend rows for columns the file does not have", name)
	}
	assert.Len(t, legend, 7)

	names := []string{}
	for _, a := range attachmentsOf(dir) {
		names = append(names, a.Name)
	}
	assert.Contains(t, names, ColumnsFile)
}

func attachmentsOf(dir string) []Attachment {
	out, _ := attachments(dir, nil)
	return out
}
