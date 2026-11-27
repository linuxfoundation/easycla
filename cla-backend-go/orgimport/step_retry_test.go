// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-openapi/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/easycla/cla-backend-go/token"
	acs_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service"
)

const retryUser = "bob"

func badGateway(code int) error {
	return runtime.NewAPIError("response status code does not match any response statuses defined for this endpoint in the swagger spec", struct{}{}, code)
}

func TestRewriteTriesAStepAgainAfterAGatewayError(t *testing.T) {
	for _, code := range []int{502, 503, 504} {
		fx, mapping, statePath := rewriteFixture(t)
		answers := 0
		fx.platform.createHook = func(username, orgID, _ string) error {
			if username == retryUser && orgID == targetSFID && answers == 0 {
				answers++
				return badGateway(code)
			}
			return nil
		}
		opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
		plan, err := BuildPlan(context.Background(), fx.deps(), opts)
		require.NoError(t, err)

		sum, err := Execute(context.Background(), fx.deps(), opts, plan)

		require.NoError(t, err, "one %d answer does not fail the organization", code)
		assert.Equal(t, 1, sum.Rewritten)
		assert.Zero(t, sum.Failed)
		assert.Contains(t, fx.sleeps, stepRetryDelay)
		assert.Contains(t, fx.out.String(), StepGrants+" "+lfID+": ")
		assert.Contains(t, fx.out.String(), "trying the step again (1 of 2)")
		assert.Contains(t, targetGrants(t, fx), retryUser+":cla-manager:proj-1|"+targetSFID)
		assert.Empty(t, mustListOrgGrants(t, fx, lfID))
		st, err := LoadState(statePath)
		require.NoError(t, err)
		assert.True(t, st.Done(lfID))
	}
}

func TestRewriteGivesUpAfterRepeatedGatewayErrors(t *testing.T) {
	fx, mapping, statePath := rewriteFixture(t)
	answers := 0
	fx.platform.createHook = func(username, orgID, _ string) error {
		if username == retryUser && orgID == targetSFID {
			answers++
			return badGateway(502)
		}
		return nil
	}
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)

	sum, err := Execute(context.Background(), fx.deps(), opts, plan)

	require.Error(t, err)
	assert.Equal(t, 1, sum.Failed)
	assert.Equal(t, 1+stepRetries, answers, "the step runs once and is tried again twice")
	assert.True(t, gatewayError(plan.Rewrite[0].Err))
	assert.Empty(t, fx.companies.updates, "the rows are not rewritten")
	st, err := LoadState(statePath)
	require.NoError(t, err)
	assert.Equal(t, StepGrants, st.Last[lfID].Step)
	assert.Equal(t, statusFailed, st.Last[lfID].Status)
}

func TestRewriteDoesNotTryAgainAfterAnotherError(t *testing.T) {
	for _, refusal := range []error{errors.New("org-service down"), badGateway(500), badGateway(403)} {
		fx, mapping, statePath := rewriteFixture(t)
		answers := 0
		fx.platform.createHook = func(username, orgID, _ string) error {
			if username == retryUser && orgID == targetSFID {
				answers++
				return refusal
			}
			return nil
		}
		opts := Options{Stage: "dev", Apply: true, Mapping: mapping, State: statePath, Routes: []Route{RouteRewrite}, OutDir: t.TempDir()}
		plan, err := BuildPlan(context.Background(), fx.deps(), opts)
		require.NoError(t, err)

		sum, err := Execute(context.Background(), fx.deps(), opts, plan)

		require.Error(t, err)
		assert.Equal(t, 1, sum.Failed)
		assert.Equal(t, 1, answers, "%v", refusal)
		assert.NotContains(t, fx.sleeps, stepRetryDelay)
	}
}

func TestStepRetryDelayIsShort(t *testing.T) {
	assert.LessOrEqual(t, time.Duration(stepRetries)*stepRetryDelay, time.Minute)
}

func TestCollapseTriesTheEndOfRunCopyAgainAfterAGatewayError(t *testing.T) {
	fx, dir := collisionFixture(t)
	fx.platform.oneOrg = true
	fx.platform.addGrant("lf-a", "uma", "role-contact", "")
	fx.platform.addGrant("lf-b", "uma", "role-cla-signatory", "")
	answers := 0
	fx.platform.createHook = func(username, orgID, _ string) error {
		if username == "uma" && orgID == targetSFID && answers == 0 && strings.Contains(fx.out.String(), "rewritten lf-b") {
			answers++
			return badGateway(502)
		}
		return nil
	}
	mapping := writeMapping(t, dir, "lf-a,"+targetSFID+",created,true", "lf-b,"+targetSFID+",created,true")
	decisions := writeDecisions(t, dir, "collapse,lf-a;lf-b,"+targetSFID+",michal,one legal entity")
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, Decisions: decisions, State: filepath.Join(dir, "state.jsonl"), Routes: []Route{RouteRewrite}, OutDir: filepath.Join(dir, "out")}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)

	sum, err := Execute(context.Background(), fx.deps(), opts, plan)

	require.NoError(t, err)
	assert.Equal(t, 2, sum.Rewritten)
	assert.Equal(t, 1, answers, "the end-of-run copy got the 502")
	assert.Contains(t, fx.out.String(), StepCleanup+" lf-a: ")
	assert.Contains(t, fx.out.String(), "trying again (1 of 2)")
	assert.Contains(t, fx.sleeps, stepRetryDelay)
	assert.Equal(t, []string{"uma:cla-signatory:" + targetSFID, "uma:contact:" + targetSFID}, targetGrants(t, fx))
	assert.Empty(t, droppedReport(t, opts.OutDir)[1:])
}

func TestCollapseListsTheAccountAgainAfterAGatewayErrorAtTheEndOfARun(t *testing.T) {
	fx, dir := collisionFixture(t)
	fx.platform.oneOrg = true
	fx.platform.addGrant("lf-a", "uma", "role-contact", "")
	fx.platform.addGrant("lf-b", "uma", "role-cla-signatory", "")
	lists := 0
	fx.platform.listHook = func(orgID string) {
		if orgID != targetSFID || !strings.Contains(fx.out.String(), "rewritten lf-b") {
			return
		}
		lists++
		if fx.platform.grantsErr == nil {
			fx.platform.grantsErr = map[string]error{}
		}
		delete(fx.platform.grantsErr, targetSFID)
		if lists == 1 {
			fx.platform.grantsErr[targetSFID] = fmt.Errorf(acsListFailed+"%d: %s", http.StatusServiceUnavailable, "upstream")
		}
	}
	mapping := writeMapping(t, dir, "lf-a,"+targetSFID+",created,true", "lf-b,"+targetSFID+",created,true")
	decisions := writeDecisions(t, dir, "collapse,lf-a;lf-b,"+targetSFID+",michal,one legal entity")
	opts := Options{Stage: "dev", Apply: true, Mapping: mapping, Decisions: decisions, State: filepath.Join(dir, "state.jsonl"), Routes: []Route{RouteRewrite}, OutDir: filepath.Join(dir, "out")}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)

	sum, err := Execute(context.Background(), fx.deps(), opts, plan)

	require.NoError(t, err)
	assert.Equal(t, 2, sum.Rewritten)
	assert.GreaterOrEqual(t, lists, 2, "the Account is listed again after the 503")
	assert.Contains(t, fx.out.String(), "trying again (1 of 2)")
	assert.Contains(t, fx.out.String(), "grants lf-a -> "+targetSFID+": 1 copied after the other organizations of the run")
	assert.Equal(t, []string{"uma:cla-signatory:" + targetSFID, "uma:contact:" + targetSFID}, targetGrants(t, fx))
	assert.Empty(t, droppedReport(t, opts.OutDir)[1:])
}

// The ACS grants listing reports a failed answer as a plain error: this pins its text to what gatewayError reads.
func TestGatewayErrorKnowsAFailedACSListing(t *testing.T) {
	status := http.StatusBadGateway
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/acs/") {
			if err := json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "fake-token", "token_type": "Bearer", "expires_in": 3600}); err != nil {
				t.Error(err)
			}
			return
		}
		w.WriteHeader(status)
	}))
	defer server.Close()
	token.Init("id", "secret", server.URL, "aud")
	acs_service.InitClient(server.URL, "")

	for code, want := range map[int]bool{http.StatusBadGateway: true, http.StatusServiceUnavailable: true, http.StatusGatewayTimeout: true, http.StatusInternalServerError: false, http.StatusForbidden: false} {
		status = code
		_, err := acs_service.GetClient().ListOrgGrants(context.Background(), targetSFID)
		require.Error(t, err, "status %d", code)
		assert.Equal(t, want, gatewayError(err), "status %d: %v", code, err)
		assert.Equal(t, want, gatewayError(fmt.Errorf("listing the grants of %s: %w", targetSFID, err)), "status %d, wrapped", code)
	}
	assert.False(t, gatewayError(nil))
	assert.False(t, gatewayError(errors.New("acs rolescopes/organization returned 200: fine")))
}
