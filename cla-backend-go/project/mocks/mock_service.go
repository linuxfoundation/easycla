// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

// Code generated by MockGen. DO NOT EDIT.
// Source: project/service/service.go

// Package mock_service is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	project "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/project"
)

// MockService is a mock of Service interface.
type MockService struct {
	ctrl     *gomock.Controller
	recorder *MockServiceMockRecorder
}

// MockServiceMockRecorder is the mock recorder for MockService.
type MockServiceMockRecorder struct {
	mock *MockService
}

// NewMockService creates a new mock instance.
func NewMockService(ctrl *gomock.Controller) *MockService {
	mock := &MockService{ctrl: ctrl}
	mock.recorder = &MockServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockService) EXPECT() *MockServiceMockRecorder {
	return m.recorder
}

// CreateCLAGroup mocks base method.
func (m *MockService) CreateCLAGroup(ctx context.Context, project *models.ClaGroup) (*models.ClaGroup, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateCLAGroup", ctx, project)
	ret0, _ := ret[0].(*models.ClaGroup)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateCLAGroup indicates an expected call of CreateCLAGroup.
func (mr *MockServiceMockRecorder) CreateCLAGroup(ctx, project interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateCLAGroup", reflect.TypeOf((*MockService)(nil).CreateCLAGroup), ctx, project)
}

// DeleteCLAGroup mocks base method.
func (m *MockService) DeleteCLAGroup(ctx context.Context, claGroupID string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteCLAGroup", ctx, claGroupID)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteCLAGroup indicates an expected call of DeleteCLAGroup.
func (mr *MockServiceMockRecorder) DeleteCLAGroup(ctx, claGroupID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteCLAGroup", reflect.TypeOf((*MockService)(nil).DeleteCLAGroup), ctx, claGroupID)
}

// GetCLAGroupByID mocks base method.
func (m *MockService) GetCLAGroupByID(ctx context.Context, claGroupID string) (*models.ClaGroup, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCLAGroupByID", ctx, claGroupID)
	ret0, _ := ret[0].(*models.ClaGroup)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCLAGroupByID indicates an expected call of GetCLAGroupByID.
func (mr *MockServiceMockRecorder) GetCLAGroupByID(ctx, claGroupID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCLAGroupByID", reflect.TypeOf((*MockService)(nil).GetCLAGroupByID), ctx, claGroupID)
}

// GetCLAGroupByIDCompat mocks base method.
func (m *MockService) GetCLAGroupByIDCompat(ctx context.Context, claGroupID string) (*models.ClaGroup, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCLAGroupByIDCompat", ctx, claGroupID)
	ret0, _ := ret[0].(*models.ClaGroup)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCLAGroupByIDCompat indicates an expected call of GetCLAGroupByIDCompat.
func (mr *MockServiceMockRecorder) GetCLAGroupByIDCompat(ctx, claGroupID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCLAGroupByIDCompat", reflect.TypeOf((*MockService)(nil).GetCLAGroupByIDCompat), ctx, claGroupID)
}

// GetCLAGroupByName mocks base method.
func (m *MockService) GetCLAGroupByName(ctx context.Context, projectName string) (*models.ClaGroup, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCLAGroupByName", ctx, projectName)
	ret0, _ := ret[0].(*models.ClaGroup)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCLAGroupByName indicates an expected call of GetCLAGroupByName.
func (mr *MockServiceMockRecorder) GetCLAGroupByName(ctx, projectName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCLAGroupByName", reflect.TypeOf((*MockService)(nil).GetCLAGroupByName), ctx, projectName)
}

// GetCLAGroupCurrentCCLATemplateURLByID mocks base method.
func (m *MockService) GetCLAGroupCurrentCCLATemplateURLByID(ctx context.Context, claGroupID string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCLAGroupCurrentCCLATemplateURLByID", ctx, claGroupID)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCLAGroupCurrentCCLATemplateURLByID indicates an expected call of GetCLAGroupCurrentCCLATemplateURLByID.
func (mr *MockServiceMockRecorder) GetCLAGroupCurrentCCLATemplateURLByID(ctx, claGroupID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCLAGroupCurrentCCLATemplateURLByID", reflect.TypeOf((*MockService)(nil).GetCLAGroupCurrentCCLATemplateURLByID), ctx, claGroupID)
}

// GetCLAGroupCurrentICLATemplateURLByID mocks base method.
func (m *MockService) GetCLAGroupCurrentICLATemplateURLByID(ctx context.Context, claGroupID string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCLAGroupCurrentICLATemplateURLByID", ctx, claGroupID)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCLAGroupCurrentICLATemplateURLByID indicates an expected call of GetCLAGroupCurrentICLATemplateURLByID.
func (mr *MockServiceMockRecorder) GetCLAGroupCurrentICLATemplateURLByID(ctx, claGroupID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCLAGroupCurrentICLATemplateURLByID", reflect.TypeOf((*MockService)(nil).GetCLAGroupCurrentICLATemplateURLByID), ctx, claGroupID)
}

// GetCLAGroups mocks base method.
func (m *MockService) GetCLAGroups(ctx context.Context, params *project.GetProjectsParams) (*models.ClaGroups, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCLAGroups", ctx, params)
	ret0, _ := ret[0].(*models.ClaGroups)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCLAGroups indicates an expected call of GetCLAGroups.
func (mr *MockServiceMockRecorder) GetCLAGroups(ctx, params interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCLAGroups", reflect.TypeOf((*MockService)(nil).GetCLAGroups), ctx, params)
}

// GetCLAGroupsByExternalID mocks base method.
func (m *MockService) GetCLAGroupsByExternalID(ctx context.Context, params *project.GetProjectsByExternalIDParams) (*models.ClaGroups, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCLAGroupsByExternalID", ctx, params)
	ret0, _ := ret[0].(*models.ClaGroups)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCLAGroupsByExternalID indicates an expected call of GetCLAGroupsByExternalID.
func (mr *MockServiceMockRecorder) GetCLAGroupsByExternalID(ctx, params interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCLAGroupsByExternalID", reflect.TypeOf((*MockService)(nil).GetCLAGroupsByExternalID), ctx, params)
}

// GetCLAGroupsByExternalSFID mocks base method.
func (m *MockService) GetCLAGroupsByExternalSFID(ctx context.Context, projectSFID string) (*models.ClaGroups, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCLAGroupsByExternalSFID", ctx, projectSFID)
	ret0, _ := ret[0].(*models.ClaGroups)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCLAGroupsByExternalSFID indicates an expected call of GetCLAGroupsByExternalSFID.
func (mr *MockServiceMockRecorder) GetCLAGroupsByExternalSFID(ctx, projectSFID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCLAGroupsByExternalSFID", reflect.TypeOf((*MockService)(nil).GetCLAGroupsByExternalSFID), ctx, projectSFID)
}

// GetCLAManagers mocks base method.
func (m *MockService) GetCLAManagers(ctx context.Context, claGroupID string) ([]*models.ClaManagerUser, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCLAManagers", ctx, claGroupID)
	ret0, _ := ret[0].([]*models.ClaManagerUser)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCLAManagers indicates an expected call of GetCLAManagers.
func (mr *MockServiceMockRecorder) GetCLAManagers(ctx, claGroupID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCLAManagers", reflect.TypeOf((*MockService)(nil).GetCLAManagers), ctx, claGroupID)
}

// GetClaGroupByProjectSFID mocks base method.
func (m *MockService) GetClaGroupByProjectSFID(ctx context.Context, projectSFID string, loadRepoDetails bool) (*models.ClaGroup, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetClaGroupByProjectSFID", ctx, projectSFID, loadRepoDetails)
	ret0, _ := ret[0].(*models.ClaGroup)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetClaGroupByProjectSFID indicates an expected call of GetClaGroupByProjectSFID.
func (mr *MockServiceMockRecorder) GetClaGroupByProjectSFID(ctx, projectSFID, loadRepoDetails interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetClaGroupByProjectSFID", reflect.TypeOf((*MockService)(nil).GetClaGroupByProjectSFID), ctx, projectSFID, loadRepoDetails)
}

// GetClaGroupsByFoundationSFID mocks base method.
func (m *MockService) GetClaGroupsByFoundationSFID(ctx context.Context, foundationSFID string, loadRepoDetails bool) (*models.ClaGroups, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetClaGroupsByFoundationSFID", ctx, foundationSFID, loadRepoDetails)
	ret0, _ := ret[0].(*models.ClaGroups)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetClaGroupsByFoundationSFID indicates an expected call of GetClaGroupsByFoundationSFID.
func (mr *MockServiceMockRecorder) GetClaGroupsByFoundationSFID(ctx, foundationSFID, loadRepoDetails interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetClaGroupsByFoundationSFID", reflect.TypeOf((*MockService)(nil).GetClaGroupsByFoundationSFID), ctx, foundationSFID, loadRepoDetails)
}

// SignedAtFoundationLevel mocks base method.
func (m *MockService) SignedAtFoundationLevel(ctx context.Context, foundationSFID string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SignedAtFoundationLevel", ctx, foundationSFID)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SignedAtFoundationLevel indicates an expected call of SignedAtFoundationLevel.
func (mr *MockServiceMockRecorder) SignedAtFoundationLevel(ctx, foundationSFID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SignedAtFoundationLevel", reflect.TypeOf((*MockService)(nil).SignedAtFoundationLevel), ctx, foundationSFID)
}

// UpdateCLAGroup mocks base method.
func (m *MockService) UpdateCLAGroup(ctx context.Context, claGroupModel *models.ClaGroup) (*models.ClaGroup, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateCLAGroup", ctx, claGroupModel)
	ret0, _ := ret[0].(*models.ClaGroup)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateCLAGroup indicates an expected call of UpdateCLAGroup.
func (mr *MockServiceMockRecorder) UpdateCLAGroup(ctx, claGroupModel interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateCLAGroup", reflect.TypeOf((*MockService)(nil).UpdateCLAGroup), ctx, claGroupModel)
}
