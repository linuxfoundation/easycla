// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_groups

import (
	"github.com/LF-Engineering/lfx-kit/auth"
)

// EnrollProjectsModel model to encapsulate the enroll projects request
type EnrollProjectsModel struct {
	AuthUser         *auth.User
	CLAGroupID       string
	FoundationSFID   string
	ProjectSFIDList  []string
	ProjectLevel     bool
	CLAGroupProjects []string
}

// UnenrollProjectsModel model to encapsulate the unenroll projects request
type UnenrollProjectsModel struct {
	AuthUser        *auth.User
	CLAGroupID      string
	FoundationSFID  string
	ProjectSFIDList []string
}

// AssociateCLAGroupWithProjectsModel to encapsulate the associate request
type AssociateCLAGroupWithProjectsModel struct {
	AuthUser        *auth.User
	CLAGroupID      string
	FoundationSFID  string
	ProjectSFIDList []string
}

// UnassociateCLAGroupWithProjectsModel to encapsulate the unassociate request
type UnassociateCLAGroupWithProjectsModel struct {
	AuthUser        *auth.User
	CLAGroupID      string
	FoundationSFID  string
	ProjectSFIDList []string
}

// ProjectNode representing nested projects
type ProjectNode struct {
	Parent   *ProjectNode
	ID       string
	Name     string
	Children []*ProjectNode
}

type ProjectStack []*ProjectNode

func (s *ProjectStack) Push(v *ProjectNode) {
	*s = append(*s, v)
}

func (s *ProjectStack) Pop() (ProjectStack, *ProjectNode) {
	l := len(*s)
	return (*s)[:l-1], (*s)[l-1]
}

func (s *ProjectStack) IsEmpty() bool {
	return len(*s) == 0
}

func (s *ProjectStack) Peek() *ProjectNode {
	return (*s)[len(*s)-1]
}

func (s *ProjectStack) Size() int {
	return len(*s)
}
