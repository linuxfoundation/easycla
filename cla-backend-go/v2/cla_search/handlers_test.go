// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_search

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/runtime"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	claSearchOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/cla_search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubService struct {
	list       *models.ClaSearchList
	group      *models.ClaSearchResult
	err        error
	term       string
	claGroupID string
}

func (s *stubService) Search(_ context.Context, searchTerm string, _ int64) (*models.ClaSearchList, error) {
	s.term = searchTerm
	return s.list, s.err
}

func (s *stubService) Get(_ context.Context, claGroupID string) (*models.ClaSearchResult, error) {
	s.claGroupID = claGroupID
	return s.group, s.err
}

func invoke(t *testing.T, svc Service, authUser *auth.User, searchTerm string) *httptest.ResponseRecorder {
	t.Helper()
	api := &operations.EasyclaAPI{}
	Configure(api, svc)
	require.NotNil(t, api.ClaSearchSearchClaGroupsHandler)

	params := claSearchOps.SearchClaGroupsParams{
		HTTPRequest: httptest.NewRequest(http.MethodGet, "/v4/cla-group/search", nil),
		SearchTerm:  searchTerm,
	}
	recorder := httptest.NewRecorder()
	api.ClaSearchSearchClaGroupsHandler.Handle(params, authUser).WriteResponse(recorder, runtime.JSONProducer())
	return recorder
}

func TestHandlerUnauthorizedWithoutPrincipal(t *testing.T) {
	for name, authUser := range map[string]*auth.User{"nil": nil, "empty username": {}} {
		t.Run(name, func(t *testing.T) {
			svc := &stubService{}
			assert.Equal(t, http.StatusUnauthorized, invoke(t, svc, authUser, "kubernetes").Code)
			assert.Empty(t, svc.term)
		})
	}
}

func TestHandlerAcceptsAdminPrincipalWithoutUsername(t *testing.T) {
	svc := &stubService{list: &models.ClaSearchList{SearchTerm: "kubernetes"}}
	assert.Equal(t, http.StatusOK, invoke(t, svc, &auth.User{ACL: auth.ACL{Admin: true}}, "kubernetes").Code)
	assert.Equal(t, "kubernetes", svc.term)
}

func TestHandlerBadRequestOnWhitespaceOnlyTerm(t *testing.T) {
	svc := &stubService{}
	assert.Equal(t, http.StatusBadRequest, invoke(t, svc, &auth.User{UserName: "jdoe"}, "   ").Code)
	assert.Empty(t, svc.term)
}

func TestHandlerInternalServerErrorOnServiceFailure(t *testing.T) {
	svc := &stubService{err: errors.New("boom")}
	assert.Equal(t, http.StatusInternalServerError, invoke(t, svc, &auth.User{UserName: "jdoe"}, "kubernetes").Code)
}

func TestGetHandlerNotFoundAndUnauthorized(t *testing.T) {
	api := &operations.EasyclaAPI{}
	Configure(api, &stubService{err: ErrNotFound})
	require.NotNil(t, api.ClaSearchGetClaGroupHandler)

	params := claSearchOps.GetClaGroupParams{
		HTTPRequest: httptest.NewRequest(http.MethodGet, "/v4/cla-group/11111111-1111-4111-8111-111111111111/catalogue", nil),
		ClaGroupID:  "11111111-1111-4111-8111-111111111111",
	}
	unauthorized := httptest.NewRecorder()
	api.ClaSearchGetClaGroupHandler.Handle(params, nil).WriteResponse(unauthorized, runtime.JSONProducer())
	assert.Equal(t, http.StatusUnauthorized, unauthorized.Code)

	missing := httptest.NewRecorder()
	api.ClaSearchGetClaGroupHandler.Handle(params, &auth.User{UserName: "jdoe"}).WriteResponse(missing, runtime.JSONProducer())
	assert.Equal(t, http.StatusNotFound, missing.Code)
}

func invokeGet(t *testing.T, svc Service, claGroupID, requestID string) *httptest.ResponseRecorder {
	t.Helper()
	api := &operations.EasyclaAPI{}
	Configure(api, svc)
	require.NotNil(t, api.ClaSearchGetClaGroupHandler)

	params := claSearchOps.GetClaGroupParams{
		HTTPRequest: httptest.NewRequest(http.MethodGet, "/v4/cla-group/"+claGroupID+"/catalogue", nil),
		ClaGroupID:  claGroupID,
		XREQUESTID:  &requestID,
	}
	recorder := httptest.NewRecorder()
	api.ClaSearchGetClaGroupHandler.Handle(params, &auth.User{UserName: "jdoe"}).WriteResponse(recorder, runtime.JSONProducer())
	return recorder
}

func TestGetHandlerSuccess(t *testing.T) {
	const claGroupID = "11111111-1111-4111-8111-111111111111"
	svc := &stubService{group: &models.ClaSearchResult{ClaGroupID: claGroupID, ClaGroupName: "Kubernetes", ProjectSFID: "sfid-kube"}}
	recorder := invokeGet(t, svc, claGroupID, "req-200")

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, claGroupID, svc.claGroupID)
	assert.Equal(t, "req-200", recorder.Header().Get("x-request-id"))
	var payload models.ClaSearchResult
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, claGroupID, payload.ClaGroupID)
	assert.Equal(t, "Kubernetes", payload.ClaGroupName)
	assert.Equal(t, "sfid-kube", payload.ProjectSFID)
}

func TestGetHandlerInternalServerErrorOmitsLookupError(t *testing.T) {
	const claGroupID = "22222222-2222-4222-8222-222222222222"
	svc := &stubService{err: errors.New("dynamodb: secret internal detail")}
	recorder := invokeGet(t, svc, claGroupID, "req-500")

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Equal(t, claGroupID, svc.claGroupID)
	assert.Equal(t, "req-500", recorder.Header().Get("x-request-id"))
	var payload models.ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, "req-500", payload.XRequestID)
	assert.NotContains(t, payload.Message, "secret internal detail")
}

func TestHandlerSuccess(t *testing.T) {
	svc := &stubService{list: &models.ClaSearchList{SearchTerm: "kubernetes", ResultCount: 1, Results: []models.ClaSearchResult{{ClaGroupID: "cg-kube"}}}}
	recorder := invoke(t, svc, &auth.User{UserName: "jdoe"}, "kubernetes")
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"claGroupID":"cg-kube"`)
	assert.Equal(t, "kubernetes", svc.term)
}
