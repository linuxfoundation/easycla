// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/aws/aws-sdk-go/service/ses/sesiface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reportFixture(t *testing.T) (RunInfo, *Plan, string) {
	t.Helper()
	fx, dir := collisionFixture(t)
	mapping := writeMapping(t, dir, "lf-a,"+targetSFID+",matched,true", "lf-b,"+targetSFID+",created,true", "lf-c,"+targetSFID2+",created,true")
	outDir := filepath.Join(dir, "out")
	opts := Options{Stage: "dev", Mapping: mapping, OutDir: outDir, Routes: []Route{RouteRewrite}}
	plan, err := BuildPlan(context.Background(), fx.deps(), opts)
	require.NoError(t, err)
	sum, err := Execute(context.Background(), fx.deps(), opts, plan)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(outDir, "run.log"), []byte("line 1\nline 2\n"), 0o600))
	start := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	info := RunInfo{
		Stage: "dev", Command: "ingest", Args: []string{"ingest", "--mapping", mapping, "--routes", "rewrite", "--out-dir", outDir, "--no-aws-log"},
		Start: start, End: start.Add(90 * time.Second), Runner: "lukasz@dockaws", Repository: "linuxfoundation/easycla",
		RunURL: "https://github.com/linuxfoundation/easycla/actions/runs/42", Artifact: "org-import-out-dev-42", Revision: "abc123-dirty",
		OutDir: outDir, LogGroup: "/easycla/org-import/dev", LogStream: "s1", Summary: sum.String(),
		Workflow: WorkflowInputs{Routes: "rewrite", Tranche: "0", Mapping: mapping},
	}
	return info, plan, outDir
}

func TestBuildReport(t *testing.T) {
	info, plan, outDir := reportFixture(t)
	rep := BuildReport(info, plan, nil, []byte("line 1\nline 2\n"))
	assert.Equal(t, "[EasyCLA org-import][dev] ingest dry-run: "+info.Summary+" (OK)", rep.Subject)
	for _, want := range []string{
		"Manual actions (2)", "target_collision", "decisions file", "Targets (2)", "needs_decision", "Plan (3 groups: 0 register, 1 rewrite)",
		"How to apply this plan", "STAGE=dev ./bin/org-import ingest --mapping", "--apply --yes", "gh workflow run org-import-sweep.yml -R linuxfoundation/easycla -f stage=dev -f mode=apply -f routes=rewrite",
		"-f mapping=&#34;$(tr &#39;\\n&#39; &#39;|&#39; &lt; ", "https://github.com/linuxfoundation/easycla/actions/runs/42", "org-import-out-dev-42", "abc123-dirty", "/easycla/org-import/dev / s1",
		"1m30s", "line 2", "Attachments", "manual_actions.csv", "plan.csv", "targets.csv", "run.log", "out.zip", "acme.example",
	} {
		assert.Contains(t, rep.HTML, want, want)
	}
	assert.NotContains(t, rep.HTML, "--no-aws-log --apply", "report-only flags are dropped from the apply command")
	for _, want := range []string{"== Manual actions (2) ==", "== Targets (2) ==", "== Plan", "== How to apply this plan ==", "== run.log ==", "line 1", `-f mapping="$(tr '\n' '|' < `} {
		assert.Contains(t, rep.Text, want, want)
	}
	names := []string{}
	for _, a := range rep.Attachments {
		names = append(names, a.Name)
	}
	assert.Equal(t, []string{"manual_actions.csv", "plan.csv", "targets.csv", "to_salesforce.csv", "run.log", "out.zip"}, names)
	zr, err := zip.NewReader(bytes.NewReader(rep.Attachments[5].Data), int64(len(rep.Attachments[5].Data)))
	require.NoError(t, err)
	var zipped []string
	for _, f := range zr.File {
		zipped = append(zipped, f.Name)
	}
	assert.ElementsMatch(t, []string{"plan.csv", "manual_actions.csv", "targets.csv", "to_salesforce.csv", "run.log"}, zipped)
	_, err = ZipDir(filepath.Join(outDir, "missing"))
	assert.Error(t, err)

	// apply run: no "how to apply", error → FAILED subject and Error row
	info.Apply, info.Err, info.Summary = true, "2 group(s) failed", ""
	rep = BuildReport(info, plan, nil, nil)
	assert.Equal(t, "[EasyCLA org-import][dev] ingest apply: no summary (FAILED)", rep.Subject)
	assert.NotContains(t, rep.HTML, "How to apply")
	assert.Contains(t, rep.HTML, "<th align=\"left\">Error</th><td>2 group(s) failed</td>")
}

func TestBuildReportEscapesAndTruncates(t *testing.T) {
	info, plan, _ := reportFixture(t)
	info.Args = append(info.Args, "<script>")
	big := bytes.Repeat([]byte("0123456789abcdef0123456789abcde\n"), 4*1024)
	rep := BuildReport(info, plan, nil, big)
	assert.Contains(t, rep.HTML, "&lt;script&gt;")
	assert.NotContains(t, rep.HTML, "<script>")
	assert.Contains(t, rep.HTML, "full log attached")
	assert.Contains(t, rep.Text, "(last ")
	assert.Less(t, len(rep.HTML), len(big))

	// audit report
	fx := newFixture()
	fx.company("c-live", "Live Corp", "", liveSFID, "cg-1")
	fx.company("c-empty", "Empty Inc", "", "", "cg-1")
	fx.platform.sfAccounts[liveSFID] = true
	fx.platform.orgs[liveSFID] = &Org{ID: liveSFID}
	dir := t.TempDir()
	res, err := Audit(context.Background(), fx.deps(), Options{Stage: "dev", OutDir: dir})
	require.NoError(t, err)
	rep = BuildReport(RunInfo{Stage: "dev", Command: "audit", OutDir: dir, Summary: "2 company rows audited"}, nil, res, nil)
	assert.Equal(t, "[EasyCLA org-import][dev] audit dry-run: 2 company rows audited (OK)", rep.Subject)
	assert.Contains(t, rep.HTML, "Audit (2 company rows)")
	assert.Contains(t, rep.HTML, "<td>tier</td><td>"+TierOK+"</td><td>1</td>")
	assert.Contains(t, rep.Text, "route | register | 1")
	assert.NotContains(t, rep.HTML, "How to apply")
	names := []string{}
	for _, a := range rep.Attachments {
		names = append(names, a.Name)
	}
	assert.Equal(t, []string{"audit.csv", "unresolvable.csv", "possible_duplicates.csv", filepath.Base(dir) + ".zip"}, names)
}

func TestApplyCommands(t *testing.T) {
	cmds := ApplyCommands(RunInfo{Stage: "prod", Args: []string{"ingest", "--apply", "--yes", "--ids", "a,b", "--mapping", "my map.csv", "--no-email"},
		Workflow: WorkflowInputs{Routes: "register,rewrite", Tranche: "10", IDs: "a,b", Mapping: "my map.csv", Decisions: "d.csv", SharedDomains: "s.txt"}})
	require.Len(t, cmds, 2)
	assert.Equal(t, "STAGE=prod ./bin/org-import ingest --ids a,b --mapping 'my map.csv' --apply --yes", cmds[0])
	assert.Equal(t, `gh workflow run org-import-sweep.yml -R linuxfoundation/easycla -f stage=prod -f mode=apply -f routes=register,rewrite -f tranche=10 -f ids=a,b -f mapping="$(tr '\n' '|' < 'my map.csv')" -f decisions="$(tr '\n' '|' < d.csv)" -f shared_domains="$(tr '\n' '|' < s.txt)"`, cmds[1])
	assert.Equal(t, "''", shellQuote(""))
	assert.Equal(t, `'it'\''s'`, shellQuote("it's"))
	assert.Equal(t, []string{"a@x.org", "b@y.org", "c@z.org"}, ParseRecipients(" a@x.org, b@y.org;c@z.org\n"))
	assert.Empty(t, ParseRecipients(" , "))
}

func TestReportMIME(t *testing.T) {
	info, plan, _ := reportFixture(t)
	rep := BuildReport(info, plan, nil, []byte("log\n"))
	raw, err := rep.MIME("admin@lfx.linuxfoundation.org", []string{"a@example.org", "b@example.org"})
	require.NoError(t, err)
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	require.NoError(t, err)
	assert.Equal(t, "admin@lfx.linuxfoundation.org", msg.Header.Get("From"))
	assert.Equal(t, "a@example.org, b@example.org", msg.Header.Get("To"))
	subject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	require.NoError(t, err)
	assert.Equal(t, rep.Subject, subject)
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	require.NoError(t, err)
	assert.Equal(t, "multipart/mixed", mediaType)
	mr := multipart.NewReader(msg.Body, params["boundary"])
	var parts []string
	var bodies [][]byte
	for {
		p, partErr := mr.NextPart()
		if errors.Is(partErr, io.EOF) {
			break
		}
		require.NoError(t, partErr)
		data, readErr := io.ReadAll(p)
		require.NoError(t, readErr)
		parts = append(parts, p.Header.Get("Content-Type"))
		bodies = append(bodies, data)
	}
	require.Len(t, parts, 1+len(rep.Attachments))
	assert.True(t, strings.HasPrefix(parts[0], "multipart/alternative"))
	assert.Contains(t, parts[1], `name="manual_actions.csv"`)
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(string(bodies[1]), "\r\n", ""))
	require.NoError(t, err)
	assert.Equal(t, rep.Attachments[0].Data, decoded)
	altType, altParams, err := mime.ParseMediaType(parts[0])
	require.NoError(t, err)
	assert.Equal(t, "multipart/alternative", altType)
	ar := multipart.NewReader(bytes.NewReader(bodies[0]), altParams["boundary"])
	var altTypes []string
	for {
		p, partErr := ar.NextPart()
		if errors.Is(partErr, io.EOF) {
			break
		}
		require.NoError(t, partErr)
		altTypes = append(altTypes, p.Header.Get("Content-Type"))
		data, readErr := io.ReadAll(p)
		require.NoError(t, readErr)
		decoded, decodeErr := base64.StdEncoding.DecodeString(strings.ReplaceAll(string(data), "\r\n", ""))
		require.NoError(t, decodeErr)
		assert.Contains(t, string(decoded), "Manual actions")
	}
	assert.Equal(t, []string{"text/plain; charset=utf-8", "text/html; charset=utf-8"}, altTypes)

	// oversized attachments are dropped largest-first until the message fits
	rep.Attachments = append(rep.Attachments, Attachment{Name: "huge.bin", ContentType: "application/octet-stream", Data: bytes.Repeat([]byte{1}, MaxRawEmailBytes)})
	raw, err = rep.MIME("admin@lfx.linuxfoundation.org", []string{"a@example.org"})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(raw), MaxRawEmailBytes)
	assert.NotContains(t, string(raw), "huge.bin")
	assert.Contains(t, string(raw), "manual_actions.csv")
}

type fakeCloudWatch struct {
	cloudwatchlogsiface.CloudWatchLogsAPI
	groupExists bool
	groups      []string
	streams     []string
	batches     []*cloudwatchlogs.PutLogEventsInput
	failPut     bool
}

func (f *fakeCloudWatch) CreateLogGroupWithContext(_ aws.Context, in *cloudwatchlogs.CreateLogGroupInput, _ ...request.Option) (*cloudwatchlogs.CreateLogGroupOutput, error) {
	if f.groupExists {
		return nil, awserr.New(cloudwatchlogs.ErrCodeResourceAlreadyExistsException, "exists", nil)
	}
	f.groups = append(f.groups, *in.LogGroupName)
	return &cloudwatchlogs.CreateLogGroupOutput{}, nil
}

func (f *fakeCloudWatch) CreateLogStreamWithContext(_ aws.Context, in *cloudwatchlogs.CreateLogStreamInput, _ ...request.Option) (*cloudwatchlogs.CreateLogStreamOutput, error) {
	f.streams = append(f.streams, *in.LogStreamName)
	return &cloudwatchlogs.CreateLogStreamOutput{}, nil
}

func (f *fakeCloudWatch) PutLogEventsWithContext(_ aws.Context, in *cloudwatchlogs.PutLogEventsInput, _ ...request.Option) (*cloudwatchlogs.PutLogEventsOutput, error) {
	if f.failPut {
		return nil, errors.New("throttled")
	}
	f.batches = append(f.batches, in)
	return &cloudwatchlogs.PutLogEventsOutput{NextSequenceToken: aws.String("tok" + string(rune('0'+len(f.batches))))}, nil
}

func TestLogShipper(t *testing.T) {
	cw := &fakeCloudWatch{groupExists: true}
	s := &LogShipper{Client: cw, Group: "/easycla/org-import/dev", Stream: "s1", Now: func() time.Time { return time.Unix(1_700_000_000, 0) }}
	var data bytes.Buffer
	for i := 0; i < 12000; i++ {
		data.WriteString("line\n")
	}
	data.WriteString("\n")
	data.WriteString(strings.Repeat("x", cwMaxEventBytes+10) + "\n")
	n, err := s.Ship(context.Background(), data.Bytes())
	require.NoError(t, err)
	assert.Equal(t, 12000+1+2, n)
	assert.Empty(t, cw.groups, "existing group is fine")
	assert.Equal(t, []string{"s1"}, cw.streams)
	require.Len(t, cw.batches, 2, "10,000 events per batch")
	assert.Nil(t, cw.batches[0].SequenceToken)
	assert.Equal(t, "tok1", *cw.batches[1].SequenceToken)
	assert.Len(t, cw.batches[0].LogEvents, cwMaxBatchCount)
	assert.Equal(t, int64(1_700_000_000_000), *cw.batches[0].LogEvents[0].Timestamp)
	last := cw.batches[1].LogEvents
	assert.Equal(t, " ", *last[len(last)-3].Message, "empty lines are kept")
	assert.Len(t, *last[len(last)-2].Message, cwMaxEventBytes)
	assert.Len(t, *last[len(last)-1].Message, 10)

	cw = &fakeCloudWatch{}
	s = &LogShipper{Client: cw, Group: "g", Stream: "s"}
	n, err = s.Ship(context.Background(), []byte("a\nb"))
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, []string{"g"}, cw.groups)

	cw = &fakeCloudWatch{failPut: true}
	_, err = (&LogShipper{Client: cw, Group: "g", Stream: "s"}).Ship(context.Background(), []byte("a\n"))
	assert.ErrorContains(t, err, "put log events")
	_, err = (&LogShipper{}).Ship(context.Background(), []byte("a\n"))
	assert.ErrorIs(t, err, ErrNotConfigured)

	// a batch never exceeds the byte limit
	cw = &fakeCloudWatch{}
	big := bytes.Repeat(append(bytes.Repeat([]byte("y"), 200*1024), '\n'), 12)
	_, err = (&LogShipper{Client: cw, Group: "g", Stream: "s"}).Ship(context.Background(), big)
	require.NoError(t, err)
	assert.Greater(t, len(cw.batches), 1)
	for _, b := range cw.batches {
		size := 0
		for _, e := range b.LogEvents {
			size += len(*e.Message) + cwEventOverhead
		}
		assert.LessOrEqual(t, size, 1024*1024)
	}
}

type fakeSES struct {
	sesiface.SESAPI
	sent []*ses.SendRawEmailInput
}

func (f *fakeSES) SendRawEmailWithContext(_ aws.Context, in *ses.SendRawEmailInput, _ ...request.Option) (*ses.SendRawEmailOutput, error) {
	f.sent = append(f.sent, in)
	return &ses.SendRawEmailOutput{MessageId: aws.String("mid-1")}, nil
}

func TestMailer(t *testing.T) {
	f := &fakeSES{}
	id, err := Mailer{Client: f}.Send(context.Background(), "from@x.org", []string{"a@x.org", "b@x.org"}, []byte("raw"))
	require.NoError(t, err)
	assert.Equal(t, "mid-1", id)
	require.Len(t, f.sent, 1)
	assert.Equal(t, "from@x.org", *f.sent[0].Source)
	assert.Equal(t, []string{"a@x.org", "b@x.org"}, aws.StringValueSlice(f.sent[0].Destinations))
	assert.Equal(t, []byte("raw"), f.sent[0].RawMessage.Data)
	_, err = Mailer{Client: f}.Send(context.Background(), "from@x.org", nil, []byte("raw"))
	assert.ErrorContains(t, err, "no recipients")
	_, err = Mailer{Client: f}.Send(context.Background(), "from@x.org", []string{"a@x.org"}, make([]byte, MaxRawEmailBytes+1))
	assert.ErrorContains(t, err, "SES limit")
	_, err = Mailer{}.Send(context.Background(), "from@x.org", []string{"a@x.org"}, []byte("raw"))
	assert.ErrorIs(t, err, ErrNotConfigured)
	assert.Len(t, f.sent, 1)
}
