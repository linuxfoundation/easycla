// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_search

import (
	"context"
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
	list *models.ClaSearchList
	err  error
	term string
}

func (s *stubService) Search(_ context.Context, searchTerm string, _ int64) (*models.ClaSearchList, error) {
	s.term = searchTerm
	return s.list, s.err
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

func TestHandlerSuccess(t *testing.T) {
	svc := &stubService{list: &models.ClaSearchList{SearchTerm: "kubernetes", ResultCount: 1, Results: []models.ClaSearchResult{{ClaGroupID: "cg-kube"}}}}
	recorder := invoke(t, svc, &auth.User{UserName: "jdoe"}, "kubernetes")
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"claGroupID":"cg-kube"`)
	assert.Equal(t, "kubernetes", svc.term)
}
