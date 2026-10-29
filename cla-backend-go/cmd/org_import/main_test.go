// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/easycla/cla-backend-go/company"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/orgimport"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	member_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/member-service"
)

func TestRunUsageErrors(t *testing.T) {
	t.Setenv("STAGE", "")
	assert.Equal(t, 2, run(nil, strings.NewReader("")))
	assert.Equal(t, 2, run([]string{"bogus"}, strings.NewReader("")))
	assert.Equal(t, 2, run([]string{"ingest", "--nope"}, strings.NewReader("")))
	assert.Equal(t, 2, run([]string{"ingest"}, strings.NewReader("")), "STAGE is required")
	t.Setenv("STAGE", "dev")
	assert.Equal(t, 2, run([]string{"ingest", "--routes", "delete"}, strings.NewReader("")))
	assert.Equal(t, 2, run([]string{"ingest", "--apply", "--yes", "--out-dir", t.TempDir()}, strings.NewReader("")), "rewrite apply needs --state")
	assert.Equal(t, 2, run([]string{"ingest", "--apply", "--yes", "--routes", "rewrite", "--out-dir", t.TempDir()}, strings.NewReader("")))
}

func TestSnapshotInputs(t *testing.T) {
	dir := t.TempDir()
	mapping := filepath.Join(dir, "m.csv")
	require.NoError(t, os.WriteFile(mapping, []byte("old_id,new_id,action,approved\n"), 0o600))
	outDir := filepath.Join(dir, "out")
	require.NoError(t, os.MkdirAll(outDir, 0o750))
	var out, errOut bytes.Buffer
	notices := snapshotInputs(outDir, orgimport.Options{Mapping: mapping, Decisions: filepath.Join(dir, "missing.csv"), State: filepath.Join(dir, "state.jsonl")}, &out, &errOut)
	data, err := os.ReadFile(filepath.Clean(filepath.Join(outDir, "input-mapping.csv")))
	require.NoError(t, err)
	assert.Equal(t, "old_id,new_id,action,approved\n", string(data))
	assert.Contains(t, errOut.String(), "input-decisions.csv: cannot read")
	require.Len(t, notices, 1, "an uncaptured input is a report notice")
	assert.Contains(t, notices[0], "input input-decisions.csv: cannot read")
	assert.Contains(t, out.String(), "input-state.jsonl")
	assert.Contains(t, out.String(), "does not exist yet")
	_, err = os.Stat(filepath.Join(outDir, "input-state.jsonl"))
	assert.True(t, os.IsNotExist(err))
}

func TestCaptureJournal(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "out")
	require.NoError(t, os.MkdirAll(outDir, 0o750))
	var out, errOut bytes.Buffer

	assert.Nil(t, captureJournal(outDir, "", &out, &errOut), "no journal configured")
	assert.Nil(t, captureJournal(outDir, filepath.Join(dir, "never.jsonl"), &out, &errOut))
	assert.Contains(t, out.String(), "nothing was journaled")
	_, err := os.Stat(filepath.Join(outDir, "state.jsonl"))
	assert.True(t, os.IsNotExist(err))

	// an external journal is copied in full as state.jsonl after the run
	external := filepath.Join(dir, "journal.jsonl")
	require.NoError(t, os.WriteFile(external, []byte("{\"step\":\"start\"}\n{\"step\":\"done\"}\n"), 0o600))
	assert.Nil(t, captureJournal(outDir, external, &out, &errOut))
	data, err := os.ReadFile(filepath.Clean(filepath.Join(outDir, "state.jsonl")))
	require.NoError(t, err)
	assert.Equal(t, "{\"step\":\"start\"}\n{\"step\":\"done\"}\n", string(data))
	assert.Contains(t, out.String(), "copied (33 bytes)")

	// the record's own journal is not copied onto itself
	inRecord := filepath.Join(outDir, "state.jsonl")
	before, err := os.Stat(inRecord)
	require.NoError(t, err)
	assert.Nil(t, captureJournal(outDir, filepath.Join(outDir, "..", "out", "state.jsonl"), &out, &errOut))
	after, err := os.Stat(inRecord)
	require.NoError(t, err)
	assert.Equal(t, before.ModTime(), after.ModTime())

	// an unreadable journal is a report notice
	if os.Geteuid() != 0 {
		locked := filepath.Join(dir, "locked.jsonl")
		require.NoError(t, os.WriteFile(locked, []byte("{}\n"), 0o000))
		notices := captureJournal(outDir, locked, &out, &errOut)
		require.Len(t, notices, 1)
		assert.Contains(t, notices[0], "journal "+locked+": cannot read")
		assert.Contains(t, errOut.String(), notices[0])
	}
}

func TestConfirm(t *testing.T) {
	var out bytes.Buffer
	assert.True(t, confirm("dev", &out, strings.NewReader("dev\n")))
	assert.Contains(t, out.String(), "Type the stage name")
	assert.False(t, confirm("dev", &out, strings.NewReader("prod\n")))
	assert.False(t, confirm("dev", &out, strings.NewReader("")))
	assert.True(t, confirm("prod", &out, strings.NewReader("prod")), "last line without newline")
}

func TestRunnerAndRevision(t *testing.T) {
	t.Setenv("GITHUB_RUN_ID", "42")
	assert.Equal(t, "gha-42", runnerName())
	t.Setenv("GITHUB_RUN_ID", "")
	t.Setenv("USER", "lukasz")
	assert.True(t, strings.HasPrefix(runnerName(), "lukasz@"))
	assert.NotEmpty(t, buildRevision())
	commit, branch = "abc123", "unicron-x"
	defer func() { commit, branch = "", "" }()
	assert.Equal(t, "abc123 (unicron-x)", buildRevision(), "Makefile -X values win over VCS stamping")
	branch = ""
	assert.Equal(t, "abc123", buildRevision())
}

// a dry run only reads DynamoDB; the member-service client and the stage switch gate --apply alone
func TestRunCLAAdminsGates(t *testing.T) {
	t.Run("apply needs the member-service client", func(t *testing.T) {
		var out, errOut bytes.Buffer
		info := &orgimport.RunInfo{}
		code := runCLAAdmins(context.Background(), env{}, "dev", true, true, nil, t.TempDir(), info, &out, &errOut, strings.NewReader(""))
		assert.Equal(t, 2, code)
		assert.Equal(t, orgimport.ErrNotConfigured.Error(), info.Err)
	})
	t.Run("apply is refused while the stage switch is off", func(t *testing.T) {
		var out, errOut bytes.Buffer
		info := &orgimport.RunInfo{}
		code := runCLAAdmins(context.Background(), env{members: &member_service.Client{}}, "dev", true, true, nil, t.TempDir(), info, &out, &errOut, strings.NewReader(""))
		assert.Equal(t, 2, code)
		assert.Contains(t, info.Err, "cla-member-service-cla-admins-enabled-dev is not true")
	})
	t.Run("apply is refused while the stage switch is unreadable", func(t *testing.T) {
		var out, errOut bytes.Buffer
		info := &orgimport.RunInfo{}
		e := env{members: &member_service.Client{}}
		e.cfg.MemberService.CLAAdminsFlagError = "optional SSM key cla-member-service-cla-admins-enabled-dev must be true or false"
		code := runCLAAdmins(context.Background(), e, "dev", true, true, nil, t.TempDir(), info, &out, &errOut, strings.NewReader(""))
		assert.Equal(t, 2, code)
		assert.Contains(t, info.Err, "unreadable or not true/false")
		assert.Contains(t, info.Err, "must be true or false")
	})
}

func sampledClock(date string, err error) *awsClock {
	c := &awsClock{}
	r := &request.Request{HTTPResponse: &http.Response{Header: http.Header{}}, Error: err}
	if date != "" {
		r.HTTPResponse.Header.Set("Date", date)
	}
	c.observe(r)
	return c
}

// the roster timestamp of cla-admins --apply is AWS time (PR 5277 review): the first valid response Date
// plus the monotonic time elapsed since, never this machine's wall clock; without a sample the apply is refused
func TestAWSClock(t *testing.T) {
	want := time.Date(2026, 10, 27, 16, 58, 8, 0, time.UTC)
	t.Run("the first valid Date wins and advances with the monotonic clock", func(t *testing.T) {
		c := sampledClock("Tue, 27 Oct 2026 16:58:08 GMT", nil)
		c.observe(&request.Request{HTTPResponse: &http.Response{Header: http.Header{"Date": {"Thu, 29 Oct 2026 10:00:00 GMT"}}}})
		got, ok := c.now()
		require.True(t, ok)
		assert.WithinDuration(t, want, got, 2*time.Second)
		assert.Greater(t, time.Since(got), time.Hour, "AWS time, not this machine's clock")
		now, err := applyClock(c)
		require.NoError(t, err)
		assert.WithinDuration(t, want, now(), 2*time.Second)
	})
	t.Run("missing, malformed or failed responses leave the clock empty", func(t *testing.T) {
		for name, c := range map[string]*awsClock{
			"empty":     {},
			"missing":   sampledClock("", nil),
			"malformed": sampledClock("yesterday", nil),
			"failed":    sampledClock("Tue, 27 Oct 2026 16:58:08 GMT", errors.New("boom")),
		} {
			_, ok := c.now()
			assert.False(t, ok, name)
			_, err := applyClock(c)
			assert.ErrorIs(t, err, errNoAWSTime, name)
		}
		_, err := applyClock(nil)
		assert.ErrorIs(t, err, errNoAWSTime)
	})
	t.Run("apply is refused without an AWS time sample and runs with one", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			clock *awsClock
			code  int
		}{
			{name: "no sample", clock: &awsClock{}, code: 2},
			{name: "sample", clock: sampledClock("Tue, 27 Oct 2026 16:58:08 GMT", nil), code: 0},
		} {
			t.Run(tc.name, func(t *testing.T) {
				var out, errOut bytes.Buffer
				info := &orgimport.RunInfo{}
				e := env{members: &member_service.Client{}, clock: tc.clock, sess: offlineSession(t),
					deps: orgimport.Deps{Companies: claAdminsTestCompanies{}, Signatures: claAdminsTestSignatures{}}}
				e.cfg.MemberService.CLAAdminsEnabled = true
				code := runCLAAdmins(context.Background(), e, "dev", true, true, nil, t.TempDir(), info, &out, &errOut, strings.NewReader(""))
				assert.Equal(t, tc.code, code, errOut.String())
				if tc.code == 2 {
					assert.Contains(t, info.Err, "no AWS time sample")
					assert.NotContains(t, out.String(), "APPLY:")
				} else {
					assert.Empty(t, info.Err)
				}
			})
		}
	})
}

// an apply with one signed CCLA, through fake DynamoDB and member-service endpoints: the roster's
// source_updated_at is the AWS sample plus the time elapsed since it arrived, never this machine's clock
func TestRunCLAAdminsApplyStampsRostersWithAWSTime(t *testing.T) {
	const sfid = "0014100000Te0G7AAJ"
	reply := func(w http.ResponseWriter, s string) {
		if _, err := w.Write([]byte(s)); err != nil {
			t.Error(err)
		}
	}
	dynamo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		var resp string
		switch r.Header.Get("X-Amz-Target") {
		case "DynamoDB_20120810.UpdateItem":
			resp = `{}`
		case "DynamoDB_20120810.GetItem":
			resp = `{"Item":{"signature_ids":{"SS":["s1"]}}}`
		case "DynamoDB_20120810.Query":
			resp = `{"Items":[{"signature_id":{"S":"s1"}}],"Count":1}`
		case "DynamoDB_20120810.BatchGetItem":
			if strings.Contains(string(body), "cla-dev-companies") {
				resp = `{"Responses":{"cla-dev-companies":[{"company_id":{"S":"c-1"},"company_external_id":{"S":"` + sfid + `"}}]},"UnprocessedKeys":{}}`
			} else {
				resp = `{"Responses":{"cla-dev-signatures":[{"signature_id":{"S":"s1"},"signature_reference_id":{"S":"c-1"},"signature_type":{"S":"ccla"},"signature_reference_type":{"S":"company"},"signature_signed":{"BOOL":true},"signature_acl":{"L":[{"S":"amy"}]}}]},"UnprocessedKeys":{}}`
			}
		default:
			http.Error(w, "unexpected "+r.Header.Get("X-Amz-Target"), http.StatusBadRequest)
			return
		}
		reply(w, resp)
	}))
	t.Cleanup(dynamo.Close)
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String(dynamo.URL),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		DisableSSL:  aws.Bool(true),
		MaxRetries:  aws.Int(0),
	})
	require.NoError(t, err)

	var mu sync.Mutex
	var stamped []string
	members := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/oauth/token":
			reply(w, `{"access_token":"t","expires_in":3600}`)
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/settings/cla_admins"):
			var body struct {
				SourceUpdatedAt string `json:"source_updated_at"`
			}
			if decodeErr := json.NewDecoder(r.Body).Decode(&body); decodeErr != nil {
				http.Error(w, decodeErr.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			stamped = append(stamped, body.SourceUpdatedAt)
			mu.Unlock()
			reply(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(members.Close)
	client, err := member_service.NewClient(member_service.Config{BaseURL: members.URL, Audience: "aud", OAuthTokenURL: members.URL, ClientID: "id", ClientSecret: "secret"})
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	companies := mock_company.NewMockIRepository(ctrl)
	companies.EXPECT().GetCompaniesByExternalID(gomock.Any(), gomock.Any(), true).Return([]*models.Company{{CompanyID: "c-1", CompanyExternalID: sfid}}, nil).AnyTimes()

	sample := time.Date(2026, 10, 27, 16, 58, 8, 0, time.UTC)
	e := env{
		sess:      sess,
		members:   client,
		clock:     &awsClock{date: sample, receivedAt: time.Now().Add(-10 * time.Minute)},
		companies: companies,
		deps: orgimport.Deps{
			Companies:  claAdminsTestCompanies{companies: []models.Company{{CompanyID: "c-1", CompanyExternalID: sfid}}},
			Signatures: claAdminsTestSignatures{rows: []*signatures.ItemSignature{{SignatureID: "s1", SignatureReferenceID: "c-1", SignatureReferenceType: "company", SignatureType: "ccla", SignatureSigned: true}}},
		},
	}
	e.cfg.MemberService.CLAAdminsEnabled = true
	var out, errOut bytes.Buffer
	info := &orgimport.RunInfo{}
	code := runCLAAdmins(context.Background(), e, "dev", true, true, nil, t.TempDir(), info, &out, &errOut, strings.NewReader(""))
	require.Equal(t, 0, code, errOut.String()+out.String())
	assert.Contains(t, out.String(), "published=1")

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, stamped, 1)
	got, err := time.Parse(time.RFC3339Nano, stamped[0])
	require.NoError(t, err)
	assert.WithinDuration(t, sample.Add(10*time.Minute), got, 5*time.Second, "the AWS sample plus the elapsed time since it arrived")
	assert.Greater(t, time.Since(got), 12*time.Hour, "never this machine's clock")
}

type claAdminsTestCompanies struct{ companies []models.Company }

func (c claAdminsTestCompanies) GetCompanies(context.Context) (*models.Companies, error) {
	return &models.Companies{Companies: c.companies}, nil
}
func (c claAdminsTestCompanies) GetCompanyRecord(context.Context, string) (*company.DBModel, error) {
	return nil, errors.New("unused")
}
func (c claAdminsTestCompanies) UpdateCompanyExternalID(context.Context, string, string, string) error {
	return errors.New("unused")
}

type claAdminsTestSignatures struct{ rows []*signatures.ItemSignature }

func (s claAdminsTestSignatures) GetCCLASignatures(context.Context, *bool, *bool) ([]*signatures.ItemSignature, error) {
	return s.rows, nil
}

// an SDK session that can never reach DynamoDB: the cases below must finish before any table read
func offlineSession(t *testing.T) *session.Session {
	t.Helper()
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String("http://127.0.0.1:9"),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		DisableSSL:  aws.Bool(true),
		MaxRetries:  aws.Int(0),
	})
	require.NoError(t, err)
	return sess
}

// a dry run needs no member-service client, and per-organization failures reach the run record
func TestRunCLAAdminsDryRunAndFailures(t *testing.T) {
	t.Run("dry run without a member-service client succeeds", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		e := env{
			sess:      offlineSession(t),
			companies: mock_company.NewMockIRepository(ctrl),
			deps:      orgimport.Deps{Companies: claAdminsTestCompanies{}, Signatures: claAdminsTestSignatures{}},
		}
		var out, errOut bytes.Buffer
		info := &orgimport.RunInfo{}
		code := runCLAAdmins(context.Background(), e, "dev", false, true, nil, t.TempDir(), info, &out, &errOut, strings.NewReader(""))
		assert.Equal(t, 0, code, errOut.String())
		assert.Empty(t, info.Err)
		assert.Contains(t, out.String(), "cla-admins orgs=0 published=0 dry_run=0")
	})
	t.Run("a failing organization sets the run record and the exit code", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		companies := mock_company.NewMockIRepository(ctrl)
		companies.EXPECT().GetCompaniesByExternalID(gomock.Any(), gomock.Any(), true).Return(nil, errors.New("dynamo down")).AnyTimes()
		e := env{
			sess:      offlineSession(t),
			companies: companies,
			deps: orgimport.Deps{
				Companies:  claAdminsTestCompanies{companies: []models.Company{{CompanyID: "c-1", CompanyExternalID: "0014100000Te0G7AAJ"}}},
				Signatures: claAdminsTestSignatures{rows: []*signatures.ItemSignature{{SignatureID: "s1", SignatureReferenceID: "c-1", SignatureReferenceType: "company", SignatureType: "ccla", SignatureSigned: true}}},
			},
		}
		var out, errOut bytes.Buffer
		e.deps.Out = &out
		info := &orgimport.RunInfo{}
		code := runCLAAdmins(context.Background(), e, "dev", false, true, nil, t.TempDir(), info, &out, &errOut, strings.NewReader(""))
		assert.Equal(t, 1, code)
		assert.Equal(t, "cla-admins: 1 organization(s) failed", info.Err)
		assert.Contains(t, out.String(), "errors=1")
		assert.Contains(t, out.String(), "dynamo down")
	})
}
