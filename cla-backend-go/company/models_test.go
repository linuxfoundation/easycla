// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDbModelsToResponseModels(t *testing.T) {
	ctx := context.Background()

	parent := func(id, name, created string) DBModel {
		return DBModel{
			CompanyID:         id,
			CompanyName:       name,
			SigningEntityName: name,
			CompanyExternalID: "0014100000TestSFID",
			Created:           created,
			Updated:           created,
		}
	}
	signingEntity := func(id, name, entityName, created string) DBModel {
		return DBModel{
			CompanyID:         id,
			CompanyName:       name,
			SigningEntityName: entityName,
			CompanyExternalID: "0014100000TestSFID",
			Created:           created,
			Updated:           created,
		}
	}

	t.Run("single row unchanged", func(t *testing.T) {
		result, err := dbModelsToResponseModels(ctx, []DBModel{parent("id-1", "Acme", "2020-01-01T00:00:00Z")}, false)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "id-1", result[0].CompanyID)
	})

	t.Run("multiple parent candidates picks oldest regardless of order", func(t *testing.T) {
		older := parent("id-b", "Acme", "2019-06-15T00:00:00Z")
		newer := parent("id-a", "Acme", "2021-03-01T00:00:00Z")
		variant := signingEntity("id-c", "Acme", "Acme Sub LLC", "2018-01-01T00:00:00Z")

		for _, rows := range [][]DBModel{
			{newer, older, variant},
			{variant, newer, older},
		} {
			result, err := dbModelsToResponseModels(ctx, rows, false)
			require.NoError(t, err)
			require.Len(t, result, 1)
			assert.Equal(t, "id-b", result[0].CompanyID, "oldest parent-like row must win for input order %v", rows)
		}
	})

	t.Run("tie on date_created breaks on smallest company_id", func(t *testing.T) {
		a := parent("id-aaa", "Acme", "2020-01-01T00:00:00Z")
		b := parent("id-bbb", "Acme", "2020-01-01T00:00:00Z")

		for _, rows := range [][]DBModel{{a, b}, {b, a}} {
			result, err := dbModelsToResponseModels(ctx, rows, false)
			require.NoError(t, err)
			require.Len(t, result, 1)
			assert.Equal(t, "id-aaa", result[0].CompanyID)
		}
	})

	t.Run("no parent candidate falls back across all rows without error", func(t *testing.T) {
		rows := []DBModel{
			signingEntity("id-2", "Acme", "Acme Sub LLC", "2021-01-01T00:00:00Z"),
			signingEntity("id-1", "Acme", "Acme Holdings", "2019-01-01T00:00:00Z"),
		}
		result, err := dbModelsToResponseModels(ctx, rows, false)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "id-1", result[0].CompanyID, "oldest row wins in the all-rows fallback")
	})

	t.Run("includeChildCompanies true returns all rows unchanged", func(t *testing.T) {
		rows := []DBModel{
			parent("id-1", "Acme", "2019-01-01T00:00:00Z"),
			parent("id-2", "Acme", "2020-01-01T00:00:00Z"),
			signingEntity("id-3", "Acme", "Acme Sub LLC", "2021-01-01T00:00:00Z"),
		}
		result, err := dbModelsToResponseModels(ctx, rows, true)
		require.NoError(t, err)
		require.Len(t, result, 3)
		assert.Equal(t, "id-1", result[0].CompanyID)
		assert.Equal(t, "id-2", result[1].CompanyID)
		assert.Equal(t, "id-3", result[2].CompanyID)
	})

	t.Run("unparsable date row skipped and valid rows still resolve without error", func(t *testing.T) {
		rows := []DBModel{
			parent("id-bad", "Acme", "not-a-date"),
			parent("id-2", "Acme", "2021-01-01T00:00:00Z"),
			parent("id-1", "Acme", "2019-01-01T00:00:00Z"),
		}
		result, err := dbModelsToResponseModels(ctx, rows, false)
		require.NoError(t, err, "conversion error on a skipped row must not poison a usable result")
		require.Len(t, result, 1)
		assert.Equal(t, "id-1", result[0].CompanyID)
	})

	t.Run("all rows fail conversion returns error and empty list", func(t *testing.T) {
		rows := []DBModel{
			parent("id-1", "Acme", "not-a-date"),
			parent("id-2", "Acme", "also-not-a-date"),
		}
		result, err := dbModelsToResponseModels(ctx, rows, false)
		assert.Error(t, err)
		assert.Empty(t, result)
	})
}
