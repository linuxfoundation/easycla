// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package dynamo_events

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/github_organizations"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	mock_pcg_repo "github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups/mocks"
	repositoriesmock "github.com/linuxfoundation/easycla/cla-backend-go/repositories/mock"
)

func TestProcessEventsContainsAPanicOfOneHandler(t *testing.T) {
	const table = "cla-test-signatures"
	var mu sync.Mutex
	var ran []string
	note := func(what string) {
		mu.Lock()
		defer mu.Unlock()
		ran = append(ran, what)
	}
	s := &service{functions: map[string][]EventHandlerFunc{}}
	s.registerCallback(table, Modify, func(e events.DynamoDBEventRecord) error {
		if e.EventID == "bad" {
			panic("handler failed on this record")
		}
		note("first:" + e.EventID)
		return nil
	})
	s.registerCallback(table, Modify, func(e events.DynamoDBEventRecord) error {
		note("second:" + e.EventID)
		return nil
	})
	record := func(id string) events.DynamoDBEventRecord {
		return events.DynamoDBEventRecord{EventID: id, EventName: Modify, EventSourceArn: "arn:aws:dynamodb:us-east-1:000000000000:table/" + table + "/stream/2026-01-01T00:00:00.000"}
	}

	s.ProcessEvents(events.DynamoDBEvent{Records: []events.DynamoDBEventRecord{record("bad"), record("good")}})

	assert.ElementsMatch(t, []string{"second:bad", "first:good", "second:good"}, ran, "the other handler of the record and the next record are still processed")
}

func TestAutoEnabledForGithubOrgSkipsTheNotificationWithoutACLAService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const projectSFID, claGroupID = "a09410000182dD2AAI", "da04291f-75d1-4e84-8275-9bc008205837"
	enabled := true
	repos := repositoriesmock.NewMockService(ctrl)
	repos.EXPECT().ListProjectRepositories(gomock.Any(), projectSFID, &enabled).Return(&models.GithubListRepositories{
		List: []*models.GithubRepository{{RepositoryID: "d7c1050b-2f32-44ea-bad2-3c8ff980ccd4", RepositoryProjectSfid: projectSFID, RepositoryClaGroupID: claGroupID}},
	}, nil)

	// the provider as NewService builds it for the stream lambdas
	a := &autoEnableServiceProvider{repositoryService: repos}
	err := a.AutoEnabledForGithubOrg(logrus.Fields{"functionName": "TestAutoEnable"}, github_organizations.GithubOrganization{
		OrganizationInstallationID: 12354,
		ProjectSFID:                projectSFID,
		AutoEnabled:                true,
		AutoEnabledClaGroupID:      claGroupID,
	}, true)
	require.NoError(t, err)
}

func TestProjectMappingsOfACLAGroupWithoutOneAreAnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mapped := []*projects_cla_groups.ProjectClaGroup{{ProjectSFID: "p1", FoundationSFID: "f1"}}
	pcgRepo := mock_pcg_repo.NewMockRepository(ctrl)
	pcgRepo.EXPECT().GetProjectsIdsForClaGroup(gomock.Any(), "cla-group-none").Return([]*projects_cla_groups.ProjectClaGroup{}, nil)
	pcgRepo.EXPECT().GetProjectsIdsForClaGroup(gomock.Any(), "cla-group-1").Return(mapped, nil)
	pcgRepo.EXPECT().GetProjectsIdsForClaGroup(gomock.Any(), "cla-group-down").Return(nil, errors.New("table unavailable"))
	s := &service{projectsClaGroupRepo: pcgRepo}

	_, err := s.projectMappings(context.Background(), "cla-group-none")
	require.ErrorContains(t, err, "no project mapping for CLA Group: cla-group-none")

	mappings, err := s.projectMappings(context.Background(), "cla-group-1")
	require.NoError(t, err)
	assert.Equal(t, mapped, mappings)

	_, err = s.projectMappings(context.Background(), "cla-group-down")
	require.ErrorContains(t, err, "table unavailable")
}
