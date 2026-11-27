// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-openapi/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
