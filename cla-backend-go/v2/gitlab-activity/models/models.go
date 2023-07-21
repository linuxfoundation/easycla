// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package models

// ProcessMergeActivityInput is used to pass the data needed to trigger a gitlab mr check
type ProcessMergeActivityInput struct {
	ProjectName      string
	ProjectPath      string
	ProjectNamespace string
	ProjectID        int
	MergeID          int
	RepositoryPath   string
	LastCommitSha    string
	AuthorUserName   string
	AuthorEmail      string
}
