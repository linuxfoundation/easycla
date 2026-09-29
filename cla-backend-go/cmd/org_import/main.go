// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

// org_import is the M3 organization import/sync CLI (lfx-self-serve #2750). See README.md.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/linuxfoundation/easycla/cla-backend-go/company"
	"github.com/linuxfoundation/easycla/cla-backend-go/config"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/gerrits"
	"github.com/linuxfoundation/easycla/cla-backend-go/github_organizations"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/orgimport"
	"github.com/linuxfoundation/easycla/cla-backend-go/project/repository"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/repositories"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/token"
	"github.com/linuxfoundation/easycla/cla-backend-go/users"
	acs_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/approvals"
	member_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/member-service"
	organization_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service"
)

const usageText = `usage:
  org_import audit  [--out-dir ./org-import-out] [report flags]
  org_import ingest [--apply] [--yes] [--tranche N] [--ids id1,id2] [--mapping map.csv] [--decisions decisions.csv]
                    [--shared-domains domains.txt] [--state state.jsonl] [--routes register,rewrite] [--skip-wait]
                    [--use-apex] [--out-dir ./org-import-out] [report flags]
  report flags:     [--email-to a@x,b@y] [--no-email] [--no-aws-log] [--aws-log-group /easycla/org-import/<stage>]

Environment: STAGE=dev|prod (required), AWS credentials for that account (AWS_PROFILE/AWS_SDK_LOAD_CONFIG=1
or exported keys), AWS_REGION (default us-east-1), LOG_LEVEL (default warn), ORG_IMPORT_USE_APEX=true.
Without --apply nothing is written to EasyCLA, ACS, Salesforce or member-service; every run still writes the
report files and run.log under --out-dir, copies run.log to CloudWatch Logs and e-mails the full decision
record to SSM cla-org-import-report-emails-<stage> (or --email-to).
`

type combinedRepo struct {
	users.UserRepository
	company.IRepository
	repository.ProjectRepository
	projects_cla_groups.Repository
}

type env struct {
	deps orgimport.Deps
	cfg  config.Config
	sess *session.Session
}

type reportFlags struct {
	emailTo  string
	noEmail  bool
	noAWSLog bool
	logGroup string
}

const (
	cmdAudit     = "audit"
	cmdIngest    = "ingest"
	trueString   = "true"
	unknownValue = "unknown"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin))
}

func run(args []string, stdin io.Reader) int {
	if len(args) < 1 {
		fmt.Fprint(os.Stderr, usageText)
		return 2
	}
	if os.Getenv("LOG_LEVEL") == "" {
		_ = os.Setenv("LOG_LEVEL", "warn")
	}
	cmd := args[0]
	fs := flag.NewFlagSet("org_import "+cmd, flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usageText); fs.PrintDefaults() }
	outDir := fs.String("out-dir", "./org-import-out", "directory for CSV reports and run.log (gitignored)")
	apply := fs.Bool("apply", false, "perform writes (default: dry run)")
	yes := fs.Bool("yes", false, "skip the confirmation prompt with --apply")
	tranche := fs.Int("tranche", 0, "process at most N groups (0 = all)")
	ids := fs.String("ids", "", "comma-separated company_external_id list (old or new ids)")
	mapping := fs.String("mapping", "", "Salesforce mapping CSV old_id,new_id,action,approved")
	decisions := fs.String("decisions", "", "duplicate-review decisions CSV decision,old_ids,target_sfid,reviewer,note (lfx-self-serve #3085)")
	sharedDomains := fs.String("shared-domains", "", "shared-domain list, one domain per line (default: built-in list)")
	state := fs.String("state", "", "append-only JSONL state file for rewrite tranches")
	routes := fs.String("routes", "register,rewrite", "routes to process: register,rewrite")
	skipWait := fs.Bool("skip-wait", false, "check new ids in org-service once instead of waiting up to 40 minutes")
	useApex := fs.Bool("use-apex", os.Getenv("ORG_IMPORT_USE_APEX") == trueString, "resolve new ids through the Salesforce Apex endpoint (needs cla-salesforce-apex-* SSM params)")
	waitMax := fs.Duration("wait-max", 40*time.Minute, "maximum org-service propagation wait")
	var rf reportFlags
	fs.StringVar(&rf.emailTo, "email-to", "", "report recipients (comma-separated); default SSM cla-org-import-report-emails-<stage>")
	fs.BoolVar(&rf.noEmail, "no-email", false, "do not e-mail the report")
	fs.BoolVar(&rf.noAWSLog, "no-aws-log", false, "do not copy run.log to CloudWatch Logs")
	fs.StringVar(&rf.logGroup, "aws-log-group", "", "CloudWatch Logs group (default /easycla/org-import/<stage>)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if cmd != cmdAudit && cmd != cmdIngest {
		fmt.Fprint(os.Stderr, usageText)
		return 2
	}
	stage := os.Getenv("STAGE")
	if stage == "" {
		fmt.Fprintln(os.Stderr, "STAGE is not set")
		return 2
	}
	if os.Getenv("AWS_REGION") == "" {
		_ = os.Setenv("AWS_REGION", "us-east-1")
	}
	if rf.logGroup == "" {
		rf.logGroup = "/easycla/org-import/" + stage
	}

	var err error
	opts := orgimport.Options{
		Stage: stage, Apply: *apply, Tranche: *tranche, Mapping: *mapping, Decisions: *decisions, SharedDomains: *sharedDomains,
		State: *state, SkipWait: *skipWait, UseApex: *useApex, OutDir: *outDir, WaitMax: *waitMax,
	}
	if *ids != "" {
		opts.IDs = strings.Split(*ids, ",")
	}
	if opts.Routes, err = parseRoutes(*routes); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if !*apply && *yes {
		fmt.Fprintln(os.Stderr, "note: --yes has no effect without --apply")
	}
	if *apply && *state == "" && cmd == cmdIngest && hasRoute(opts.Routes, orgimport.RouteRewrite) {
		fmt.Fprintf(os.Stderr, "%v; use --routes register for a register-only apply\n", orgimport.ErrStateRequired)
		return 2
	}

	if err = os.MkdirAll(*outDir, 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "cannot create %s: %v\n", *outDir, err)
		return 2
	}
	logPath := filepath.Join(*outDir, "run.log")
	logFile, err := os.Create(filepath.Clean(logPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot create %s: %v\n", logPath, err)
		return 2
	}
	out := io.MultiWriter(os.Stdout, logFile)
	errOut := io.MultiWriter(os.Stderr, logFile)
	log.GetLogger().SetOutput(io.MultiWriter(os.Stderr, logFile))
	snapshotInputs(*outDir, opts, out, errOut)

	info := orgimport.RunInfo{
		Stage: stage, Command: cmd, Apply: *apply, Args: args, Start: time.Now().UTC(), Runner: runnerName(),
		Repository: os.Getenv("GITHUB_REPOSITORY"), Revision: buildRevision(), OutDir: *outDir,
		Workflow: orgimport.WorkflowInputs{Routes: *routes, Tranche: fmt.Sprint(*tranche), IDs: *ids, Mapping: *mapping, Decisions: *decisions, SharedDomains: *sharedDomains},
	}
	if runID := os.Getenv("GITHUB_RUN_ID"); runID != "" {
		info.RunURL = fmt.Sprintf("%s/%s/actions/runs/%s", os.Getenv("GITHUB_SERVER_URL"), os.Getenv("GITHUB_REPOSITORY"), runID)
		info.Artifact = fmt.Sprintf("org-import-out-%s-%s", stage, runID)
	}
	fmt.Fprintf(out, "org-import %s %s stage=%s start=%s runner=%s revision=%s args=%s\n", cmd, info.Mode(), stage, info.Start.Format(time.RFC3339), info.Runner, info.Revision, strings.Join(args, " "))

	ctx := context.Background()
	var plan *orgimport.Plan
	var audit *orgimport.AuditResult
	var e env
	code := func() int {
		var err error
		e, err = wire(ctx, stage, opts.UseApex)
		if err != nil {
			fmt.Fprintf(errOut, "setup failed: %v\n", err)
			info.Err = err.Error()
			return 2
		}
		e.deps.Out = out
		if cmd == cmdAudit {
			if audit, err = orgimport.Audit(ctx, e.deps, opts); err != nil {
				fmt.Fprintf(errOut, "audit failed: %v\n", err)
				info.Err = err.Error()
				return 1
			}
			info.Summary = fmt.Sprintf("%d company rows audited", len(audit.Rows))
			fmt.Fprintf(out, "reports written to %s (audit.csv, unresolvable.csv, possible_duplicates.csv, run.log)\n", *outDir)
			return 0
		}
		if plan, err = orgimport.BuildPlan(ctx, e.deps, opts); err != nil {
			fmt.Fprintf(errOut, "planning failed: %v\n", err)
			info.Err = err.Error()
			return 1
		}
		plan.Print(out)
		if *apply && !*yes && !confirm(stage, out, stdin) {
			fmt.Fprintf(out, "aborted: nothing was written\n")
			info.Err = "aborted at the confirmation prompt"
			return 2
		}
		summary, err := orgimport.Execute(ctx, e.deps, opts, plan)
		fmt.Fprintf(out, "%s\n", summary.String())
		info.Summary = summary.String()
		if err != nil {
			fmt.Fprintf(errOut, "ingest finished with errors: %v\n", err)
			info.Err = err.Error()
			return 1
		}
		fmt.Fprintf(out, "reports written to %s (plan.csv, manual_actions.csv, targets.csv, to_salesforce.csv, run.log)\n", *outDir)
		return 0
	}()

	info.End = time.Now().UTC()
	report(ctx, e, info, plan, audit, rf, logPath, out, errOut)
	_ = logFile.Close()
	return code
}

// report ships run.log to CloudWatch Logs and e-mails the decision record; failures never change the exit code.
func report(ctx context.Context, e env, info orgimport.RunInfo, plan *orgimport.Plan, audit *orgimport.AuditResult, rf reportFlags, logPath string, out, errOut io.Writer) {
	runLog, err := os.ReadFile(filepath.Clean(logPath))
	if err != nil {
		fmt.Fprintf(errOut, "report: cannot read %s: %v\n", logPath, err)
	}
	if e.sess == nil {
		fmt.Fprintf(errOut, "report: AWS session unavailable, skipping CloudWatch Logs and e-mail\n")
		return
	}
	if !rf.noAWSLog {
		info.LogGroup = rf.logGroup
		info.LogStream = strings.NewReplacer(":", "-", "*", "-").Replace(fmt.Sprintf("%s-%s-%s-%s", info.Start.Format("2006-01-02T15-04-05Z"), info.Command, info.Mode(), info.Runner))
		shipper := &orgimport.LogShipper{Client: cloudwatchlogs.New(e.sess), Group: info.LogGroup, Stream: info.LogStream}
		n, shipErr := shipper.Ship(ctx, runLog)
		if shipErr != nil {
			fmt.Fprintf(errOut, "report: CloudWatch Logs failed (%s/%s): %v\n", info.LogGroup, info.LogStream, shipErr)
			info.LogGroup, info.LogStream = "", ""
		} else {
			fmt.Fprintf(out, "run.log copied to CloudWatch Logs %s stream %s (%d events)\n", info.LogGroup, info.LogStream, n)
		}
	}
	if rf.noEmail {
		fmt.Fprintf(out, "report e-mail disabled (--no-email)\n")
		return
	}
	recipients := orgimport.ParseRecipients(rf.emailTo)
	if len(recipients) == 0 {
		value, found, ssmErr := readSSM(ctx, ssm.New(e.sess), fmt.Sprintf("cla-org-import-report-emails-%s", info.Stage), false)
		if ssmErr != nil {
			fmt.Fprintf(errOut, "report: %v\n", ssmErr)
		}
		if !found {
			fmt.Fprintf(out, "report e-mail skipped: SSM cla-org-import-report-emails-%s is not set and --email-to is empty\n", info.Stage)
			return
		}
		recipients = orgimport.ParseRecipients(value)
	}
	if len(recipients) == 0 || e.cfg.SenderEmailAddress == "" {
		fmt.Fprintf(out, "report e-mail skipped: recipients=%d sender=%q\n", len(recipients), e.cfg.SenderEmailAddress)
		return
	}
	rep := orgimport.BuildReport(info, plan, audit, runLog)
	raw, notice, err := rep.Deliverable(e.cfg.SenderEmailAddress, recipients)
	if err != nil {
		fmt.Fprintf(errOut, "report: building e-mail failed: %v\n", err)
		return
	}
	if notice != "" {
		fmt.Fprintf(errOut, "report: %s\n", notice)
	}
	id, err := orgimport.Mailer{Client: ses.New(e.sess)}.Send(ctx, e.cfg.SenderEmailAddress, recipients, raw)
	if err != nil {
		fmt.Fprintf(errOut, "report: SES send failed: %v\n", err)
		return
	}
	fmt.Fprintf(out, "report e-mailed to %s (%d bytes, %d attachments built, SES message id %s)\n", strings.Join(recipients, ","), len(raw), len(rep.Attachments), id)
}

// snapshotInputs copies the effective non-secret inputs (mapping, decisions, shared domains and the
// state file as it was before the run) into outDir so the artifact and the report zip are a complete record.
func parseRoutes(spec string) ([]orgimport.Route, error) {
	var routes []orgimport.Route
	for _, r := range strings.Split(spec, ",") {
		switch strings.TrimSpace(r) {
		case "register":
			routes = append(routes, orgimport.RouteRegister)
		case "rewrite":
			routes = append(routes, orgimport.RouteRewrite)
		case "":
		default:
			return nil, fmt.Errorf("unknown route %q (register|rewrite)", r)
		}
	}
	return routes, nil
}

func hasRoute(routes []orgimport.Route, want orgimport.Route) bool {
	for _, r := range routes {
		if r == want {
			return true
		}
	}
	return false
}

func snapshotInputs(outDir string, opts orgimport.Options, out, errOut io.Writer) {
	for _, in := range []struct{ name, src string }{
		{"input-mapping.csv", opts.Mapping}, {"input-decisions.csv", opts.Decisions},
		{"input-shared_domains.txt", opts.SharedDomains}, {"input-state.jsonl", opts.State},
	} {
		if in.src == "" {
			continue
		}
		dst := filepath.Join(outDir, in.name)
		data, err := os.ReadFile(filepath.Clean(in.src))
		if err != nil {
			if os.IsNotExist(err) && in.name == "input-state.jsonl" {
				fmt.Fprintf(out, "input %s: %s does not exist yet (fresh state)\n", in.name, in.src)
				continue
			}
			fmt.Fprintf(errOut, "input %s: cannot read %s: %v\n", in.name, in.src, err)
			continue
		}
		if err = os.WriteFile(dst, data, 0o600); err != nil {
			fmt.Fprintf(errOut, "input %s: cannot write %s: %v\n", in.name, dst, err)
			continue
		}
		fmt.Fprintf(out, "input %s: copied %s (%d bytes) to %s\n", in.name, in.src, len(data), dst)
	}
}

func runnerName() string {
	if id := os.Getenv("GITHUB_RUN_ID"); id != "" {
		return "gha-" + id
	}
	user := os.Getenv("USER")
	if user == "" {
		user = unknownValue
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "localhost"
	}
	return user + "@" + host
}

// set by the Makefile (-X main.commit=… -X main.branch=…); a package build stamps VCS info instead
var (
	commit string
	branch string
)

func buildRevision() string {
	if commit != "" {
		if branch != "" {
			return commit + " (" + branch + ")"
		}
		return commit
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		rev, modified := "", ""
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				if s.Value == trueString {
					modified = "-dirty"
				}
			}
		}
		if rev != "" {
			return rev + modified
		}
	}
	if sha := os.Getenv("GITHUB_SHA"); sha != "" {
		return sha
	}
	return unknownValue
}

func confirm(stage string, out io.Writer, stdin io.Reader) bool {
	fmt.Fprintf(out, "APPLY mode: the plan above will be executed against %s. Type the stage name to continue: ", stage)
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	return strings.TrimSpace(line) == stage
}

func wire(ctx context.Context, stage string, useApex bool) (env, error) {
	awsSession := session.Must(session.NewSession(&aws.Config{}))
	e := env{sess: awsSession}
	cfg, err := config.LoadConfig("", awsSession, stage)
	if err != nil {
		return e, fmt.Errorf("loading SSM config: %w", err)
	}
	e.cfg = cfg
	token.Init(cfg.Auth0Platform.ClientID, cfg.Auth0Platform.ClientSecret, cfg.Auth0Platform.URL, cfg.Auth0Platform.Audience)

	usersRepo := users.NewRepository(awsSession, stage)
	companyRepo := company.NewRepository(awsSession, stage)
	projectClaGroupRepo := projects_cla_groups.NewRepository(awsSession, stage)
	repositoriesRepo := repositories.NewRepository(awsSession, stage)
	gerritRepo := gerrits.NewRepository(awsSession, stage)
	projectRepo := repository.NewRepository(awsSession, stage, repositoriesRepo, gerritRepo, projectClaGroupRepo)
	eventsRepo := events.NewRepository(awsSession, stage)
	githubOrganizationsRepo := github_organizations.NewRepository(awsSession, stage)
	approvalRepo := approvals.NewRepository(stage, awsSession, fmt.Sprintf("cla-%s-approvals", stage))
	eventsService := events.NewService(eventsRepo, combinedRepo{usersRepo, companyRepo, projectRepo, projectClaGroupRepo})
	signaturesRepo := signatures.NewRepository(awsSession, stage, companyRepo, usersRepo, eventsService, repositoriesRepo, githubOrganizationsRepo, gerrits.NewService(gerritRepo), approvalRepo)

	organization_service.InitClient(cfg.APIGatewayURL, eventsService)
	acs_service.InitClient(cfg.APIGatewayURL, cfg.AcsAPIKey)

	e.deps = orgimport.Deps{
		Companies:  companyRepo,
		Signatures: signaturesRepo,
		ECLAs:      orgimport.DynamoECLACounter{DB: dynamodb.New(awsSession), Table: fmt.Sprintf("cla-%s-signatures", stage)},
		Events:     events.NewRekeyRepository(awsSession, stage),
		Orgs:       orgimport.OrgServiceAdapter{Client: organization_service.GetClient()},
		ACS:        acs_service.GetClient(),
	}

	members, err := member_service.NewClient(member_service.Config{
		BaseURL:       cfg.MemberService.BaseURL,
		Audience:      cfg.MemberService.Audience,
		OAuthTokenURL: cfg.Auth0Platform.URL,
		ClientID:      cfg.Auth0Platform.ClientID,
		ClientSecret:  cfg.Auth0Platform.ClientSecret,
	})
	switch {
	case err == nil:
		e.deps.Members = members
	case errors.Is(err, member_service.ErrNotConfigured):
		log.Warnf("member-service not configured (cla-member-service-base-url-%s / cla-member-service-auth0-audience-%s): liveness falls back to org-service, register steps will fail", stage, stage)
	default:
		return e, err
	}

	if useApex {
		ssmClient := ssm.New(awsSession)
		baseURL, err := requireSSM(ctx, ssmClient, fmt.Sprintf("cla-salesforce-apex-base-url-%s", stage), false)
		if err != nil {
			return e, err
		}
		apexToken, err := requireSSM(ctx, ssmClient, fmt.Sprintf("cla-salesforce-apex-token-%s", stage), true)
		if err != nil {
			return e, err
		}
		apex, err := orgimport.NewApexClient(baseURL, apexToken)
		if err != nil {
			return e, err
		}
		e.deps.Apex = apex
	}
	return e, nil
}

func requireSSM(ctx context.Context, client *ssm.SSM, key string, decrypt bool) (string, error) {
	value, found, err := readSSM(ctx, client, key, decrypt)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("%w: %s", orgimport.ErrApexUnavailable, key)
	}
	return value, nil
}

func readSSM(ctx context.Context, client *ssm.SSM, key string, decrypt bool) (string, bool, error) {
	out, err := client.GetParameterWithContext(ctx, &ssm.GetParameterInput{Name: aws.String(key), WithDecryption: aws.Bool(decrypt)})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == ssm.ErrCodeParameterNotFound {
			return "", false, nil
		}
		return "", false, fmt.Errorf("reading SSM %s: %w", key, err)
	}
	return strings.TrimSpace(aws.StringValue(out.Parameter.Value)), true, nil
}
