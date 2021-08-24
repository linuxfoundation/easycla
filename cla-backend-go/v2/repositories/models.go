// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package repositories

// GitLabAddRepoModel data model for GitLab add repository
type GitLabAddRepoModel struct {
	ClaGroupID    string
	GroupName     string
	ExternalID    int64
	GroupFullPath string
	ProjectIDList []int64
}
