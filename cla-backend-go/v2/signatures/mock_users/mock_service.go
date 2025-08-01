// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

// Code generated by MockGen. DO NOT EDIT.
// Source: users/service.go

// Package mock_users is a generated GoMock package.
package mock_users

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	user "github.com/linuxfoundation/easycla/cla-backend-go/user"
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

// CreateUser mocks base method.
func (m *MockService) CreateUser(user *models.User, claUser *user.CLAUser) (*models.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateUser", user, claUser)
	ret0, _ := ret[0].(*models.User)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateUser indicates an expected call of CreateUser.
func (mr *MockServiceMockRecorder) CreateUser(user, claUser interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateUser", reflect.TypeOf((*MockService)(nil).CreateUser), user, claUser)
}

// Delete mocks base method.
func (m *MockService) Delete(userID string, claUser *user.CLAUser) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", userID, claUser)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockServiceMockRecorder) Delete(userID, claUser interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockService)(nil).Delete), userID, claUser)
}

// GetUser mocks base method.
func (m *MockService) GetUser(userID string) (*models.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUser", userID)
	ret0, _ := ret[0].(*models.User)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetUser indicates an expected call of GetUser.
func (mr *MockServiceMockRecorder) GetUser(userID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUser", reflect.TypeOf((*MockService)(nil).GetUser), userID)
}

// GetUserByEmail mocks base method.
func (m *MockService) GetUserByEmail(userEmail string) (*models.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUserByEmail", userEmail)
	ret0, _ := ret[0].(*models.User)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetUserByEmail indicates an expected call of GetUserByEmail.
func (mr *MockServiceMockRecorder) GetUserByEmail(userEmail interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUserByEmail", reflect.TypeOf((*MockService)(nil).GetUserByEmail), userEmail)
}

// GetUserByGitHubID mocks base method.
func (m *MockService) GetUserByGitHubID(gitHubID string) (*models.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUserByGitHubID", gitHubID)
	ret0, _ := ret[0].(*models.User)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetUserByGitHubID indicates an expected call of GetUserByGitHubID.
func (mr *MockServiceMockRecorder) GetUserByGitHubID(gitHubID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUserByGitHubID", reflect.TypeOf((*MockService)(nil).GetUserByGitHubID), gitHubID)
}

// GetUserByGitHubUsername mocks base method.
func (m *MockService) GetUserByGitHubUsername(gitlabUsername string) (*models.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUserByGitHubUsername", gitlabUsername)
	ret0, _ := ret[0].(*models.User)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetUserByGitHubUsername indicates an expected call of GetUserByGitHubUsername.
func (mr *MockServiceMockRecorder) GetUserByGitHubUsername(gitlabUsername interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUserByGitHubUsername", reflect.TypeOf((*MockService)(nil).GetUserByGitHubUsername), gitlabUsername)
}

// GetUserByGitLabUsername mocks base method.
func (m *MockService) GetUserByGitLabUsername(gitlabUsername string) (*models.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUserByGitLabUsername", gitlabUsername)
	ret0, _ := ret[0].(*models.User)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetUserByGitLabUsername indicates an expected call of GetUserByGitLabUsername.
func (mr *MockServiceMockRecorder) GetUserByGitLabUsername(gitlabUsername interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUserByGitLabUsername", reflect.TypeOf((*MockService)(nil).GetUserByGitLabUsername), gitlabUsername)
}

// GetUserByGitlabID mocks base method.
func (m *MockService) GetUserByGitlabID(gitHubID int) (*models.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUserByGitlabID", gitHubID)
	ret0, _ := ret[0].(*models.User)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetUserByGitlabID indicates an expected call of GetUserByGitlabID.
func (mr *MockServiceMockRecorder) GetUserByGitlabID(gitHubID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUserByGitlabID", reflect.TypeOf((*MockService)(nil).GetUserByGitlabID), gitHubID)
}

// GetUserByLFUserName mocks base method.
func (m *MockService) GetUserByLFUserName(lfUserName string) (*models.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUserByLFUserName", lfUserName)
	ret0, _ := ret[0].(*models.User)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetUserByLFUserName indicates an expected call of GetUserByLFUserName.
func (mr *MockServiceMockRecorder) GetUserByLFUserName(lfUserName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUserByLFUserName", reflect.TypeOf((*MockService)(nil).GetUserByLFUserName), lfUserName)
}

// GetUserByUserName mocks base method.
func (m *MockService) GetUserByUserName(userName string, fullMatch bool) (*models.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUserByUserName", userName, fullMatch)
	ret0, _ := ret[0].(*models.User)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetUserByUserName indicates an expected call of GetUserByUserName.
func (mr *MockServiceMockRecorder) GetUserByUserName(userName, fullMatch interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUserByUserName", reflect.TypeOf((*MockService)(nil).GetUserByUserName), userName, fullMatch)
}

// Save mocks base method.
func (m *MockService) Save(user *models.UserUpdate, claUser *user.CLAUser) (*models.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Save", user, claUser)
	ret0, _ := ret[0].(*models.User)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Save indicates an expected call of Save.
func (mr *MockServiceMockRecorder) Save(user, claUser interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Save", reflect.TypeOf((*MockService)(nil).Save), user, claUser)
}

// SearchUsers mocks base method.
func (m *MockService) SearchUsers(field, searchTerm string, fullMatch bool) (*models.Users, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SearchUsers", field, searchTerm, fullMatch)
	ret0, _ := ret[0].(*models.Users)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SearchUsers indicates an expected call of SearchUsers.
func (mr *MockServiceMockRecorder) SearchUsers(field, searchTerm, fullMatch interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SearchUsers", reflect.TypeOf((*MockService)(nil).SearchUsers), field, searchTerm, fullMatch)
}

// UpdateUser mocks base method.
func (m *MockService) UpdateUser(userID string, updates map[string]interface{}) (*models.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateUser", userID, updates)
	ret0, _ := ret[0].(*models.User)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateUser indicates an expected call of UpdateUser.
func (mr *MockServiceMockRecorder) UpdateUser(userID, updates interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateUser", reflect.TypeOf((*MockService)(nil).UpdateUser), userID, updates)
}

// UpdateUserCompanyID mocks base method.
func (m *MockService) UpdateUserCompanyID(userID, companyID, note string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateUserCompanyID", userID, companyID, note)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateUserCompanyID indicates an expected call of UpdateUserCompanyID.
func (mr *MockServiceMockRecorder) UpdateUserCompanyID(userID, companyID, note interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateUserCompanyID", reflect.TypeOf((*MockService)(nil).UpdateUserCompanyID), userID, companyID, note)
}

func (m *MockService) ConvertUserModelToUserCompatModel(user *models.User) (*models.UserCompat, error) {
    ret := m.ctrl.Call(m, "ConvertUserModelToUserCompatModel", user)
    ret0, _ := ret[0].(*models.UserCompat)
    ret1, _ := ret[1].(error)
    return ret0, ret1
}

