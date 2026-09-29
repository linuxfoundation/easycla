// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

// Package orgimport implements the M3 organization import/sync tool (lfx-self-serve #2750):
// registration of EasyCLA companies with an active CCLA as B2B orgs and the rewrite of dead or
// legacy (lf…) external IDs to live Salesforce account IDs. It never creates, merges or deletes
// EasyCLA rows.
package orgimport

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/linuxfoundation/easycla/cla-backend-go/company"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	acs_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service"
	member_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/member-service"
)

// Route is the import route of a company group.
type Route string

// Routes.
const (
	RouteRegister Route = "register"
	RouteRewrite  Route = "rewrite"
	RouteManual   Route = "manual"
)

// IDShape classifies a company_external_id.
type IDShape string

// ID shapes.
const (
	ShapeSFID  IDShape = "001"
	ShapeLF    IDShape = "lf"
	ShapeEmpty IDShape = "empty"
	ShapeOther IDShape = "other"
)

// Liveness values (Group.Live), run modes and the manual/pending reasons shared by several files.
const (
	LiveLive       = "live"
	LiveDead       = "dead"
	LiveError      = "error"
	LiveUnverified = "unverified"

	ModeApply  = "apply"
	ModeDryRun = "dry-run"

	ReasonNoMapping        = "no_mapping"
	ReasonDeadAccount      = "dead_account"
	ReasonMappingAmbiguous = "mapping_ambiguous"
	ReasonMappingSameID    = "mapping_same_id"
	ReasonSharedDomain     = "shared_domain"
	ReasonDistinctConflict = "distinct_conflict"
	ReasonCRMUnverified    = "crm_unverified"

	orgStatusErr = "err"
)

// Step names recorded in the state file.
const (
	StepStart    = "start"
	StepResolve  = "resolve"
	StepWait     = "wait"
	StepGrants   = "copy_grants"
	StepRows     = "rewrite_rows"
	StepEvents   = "rekey_events"
	StepCleanup  = "delete_old_grants"
	StepRegister = "register"
	StepDone     = "done"
)

// Sentinel errors.
var (
	ErrOrgNotFound     = errors.New("organization not found")
	ErrNotConfigured   = errors.New("member-service is not configured (cla-member-service-* SSM parameters missing)")
	ErrApexUnavailable = errors.New("apex endpoint is not configured (cla-salesforce-apex-* SSM parameters missing)")
	ErrStateRequired   = errors.New("rewrite apply requires --state <file> (the resume journal; keep it across runs)")
)

// CompanyStore is the subset of company.IRepository the tool uses.
type CompanyStore interface {
	GetCompanies(ctx context.Context) (*models.Companies, error)
	GetCompanyRecord(ctx context.Context, companyID string) (*company.DBModel, error)
	UpdateCompanyExternalID(ctx context.Context, companyID, oldExternalID, newExternalID string) error
}

// CCLALister lists CCLA signature rows (signatures.SignatureRepository satisfies it).
type CCLALister interface {
	GetCCLASignatures(ctx context.Context, signed, approved *bool) ([]*signatures.ItemSignature, error)
}

// ECLACounter counts employee acknowledgements referencing a company (audit only).
type ECLACounter interface {
	CountECLAs(ctx context.Context, companyID string) (int, error)
}

// EventRekeyer re-keys company-scoped events (events.RekeyRepository satisfies it).
type EventRekeyer interface {
	ListEventIDsByCompanySFID(ctx context.Context, companySFID string) ([]string, error)
	ListEventIDsByCompanySFIDCLAGroup(ctx context.Context, companySFID, claGroupID string) ([]string, error)
	RekeyEventCompanySFID(ctx context.Context, eventID, oldSFID, newSFID string) (bool, error)
}

// Org is the org-service view of an organization.
type Org struct {
	ID                 string
	Name               string
	Website            string
	SigningEntityNames []string
}

// OrgService is the org-service surface the tool uses; GetOrganization returns ErrOrgNotFound on 404.
type OrgService interface {
	GetOrganization(ctx context.Context, orgID string) (*Org, error)
	CreateUserRoleScope(ctx context.Context, username, organizationID, objectType, objectID, roleID string) error
	DeleteUserRoleScope(ctx context.Context, organizationID, roleID, grantID, username string) error
}

// ACSService lists every grant scoped to an organization.
type ACSService interface {
	ListOrgGrants(ctx context.Context, orgSFID string) ([]acs_service.OrgGrant, error)
}

// MemberService registers B2B orgs; nil when the SSM parameters are absent.
type MemberService interface {
	GetB2BOrg(ctx context.Context, uid string) (*member_service.B2BOrg, error)
	RegisterB2BOrg(ctx context.Context, sfid string) (*member_service.B2BOrg, error)
}

// ApexService is the Salesforce find-or-create contract (design §4), used only with --use-apex.
type ApexService interface {
	FindOrCreate(ctx context.Context, req ApexRequest) (*ApexResult, error)
}

// Deps are the external dependencies; Members, Events, ECLAs and Apex may be nil.
type Deps struct {
	Companies  CompanyStore
	Signatures CCLALister
	ECLAs      ECLACounter
	Events     EventRekeyer
	Orgs       OrgService
	ACS        ACSService
	Members    MemberService
	Apex       ApexService
	Out        io.Writer
	Now        func() time.Time
	Sleep      func(context.Context, time.Duration) error
}

// Options are the ingest/audit flags.
type Options struct {
	Stage         string
	Apply         bool
	Tranche       int
	IDs           []string
	Mapping       string
	Decisions     string
	SharedDomains string
	State         string
	Routes        []Route
	SkipWait      bool
	UseApex       bool
	OutDir        string
	WaitMax       time.Duration
	WaitPoll      time.Duration
}

// Row is one companies-table row; ExternalID is trimmed for classification, RawExternalID is the
// exact stored value pinned by the conditional rewrite.
type Row struct {
	CompanyID         string
	CompanyName       string
	SigningEntityName string
	ExternalID        string
	RawExternalID     string
	ActiveCCLA        bool
	CCLACount         int
	CLAGroupIDs       []string
	CCLASignedOn      string
}

// Group is the import unit: every row sharing one company_external_id.
type Group struct {
	OldID        string
	Key          string
	Shape        IDShape
	Rows         []*Row
	Route        Route
	ManualReason string
	Duplicate    bool
	NewID        string
	Action       string
	Org          *Org
	OrgStatus    string
	Live         string
	Decision     *Decision
	Replayed     bool
	ViaApex      bool
	Err          error
}

// Summary is the one-line run result.
type Summary struct {
	Stage      string
	Mode       string
	Eligible   int
	Registered int
	Rewritten  int
	Pending    int
	Manual     int
	Failed     int
}

func (s Summary) String() string {
	return "stage=" + s.Stage + " mode=" + s.Mode +
		" eligible=" + strconv.Itoa(s.Eligible) + " registered=" + strconv.Itoa(s.Registered) +
		" rewritten=" + strconv.Itoa(s.Rewritten) + " pending=" + strconv.Itoa(s.Pending) + " manual=" + strconv.Itoa(s.Manual) + " failed=" + strconv.Itoa(s.Failed)
}

// Pending is a rewrite candidate without a resolved new id (mapping row missing or account found
// dead) or a register candidate whose Account could not be verified in the CRM; a group whose
// Account is created by Apex at apply time is not pending.
func (g *Group) Pending() bool {
	if g.Route == RouteRegister {
		return g.ManualReason == ReasonCRMUnverified
	}
	return g.Route == RouteRewrite && g.NewID == "" && !g.ApexCreates()
}

// ApexCreates reports a rewrite whose new Account does not exist yet: Apex creates it (and assigns
// the id) during apply.
func (g *Group) ApexCreates() bool {
	return g.ViaApex && g.Action == ActionCreated && g.NewID == ""
}

// RowTargeted is a single-row group whose company_external_id is empty or malformed: it is selected
// by its company id (Key) and nothing in ACS, org-service or the events table is keyed by the old value.
func (g *Group) RowTargeted() bool {
	return g.Key != g.OldID
}

// pinnedOld is the exact stored value the row's conditional rewrite pins: the raw value while it
// still carries the group's old id, otherwise the old id itself (an already rewritten replayed row
// then fails the condition and is verified by the read-back).
func (g *Group) pinnedOld(row *Row) string {
	if strings.TrimSpace(row.RawExternalID) == strings.TrimSpace(g.OldID) {
		return row.RawExternalID
	}
	return g.OldID
}

// Website is the org-service website of the group's organization ("" when unknown).
func (g *Group) Website() string {
	if g.Org == nil {
		return ""
	}
	return strings.TrimSpace(g.Org.Website)
}

// Domain is the normalized website domain ("" when unknown).
func (g *Group) Domain() string {
	return Domain(g.Website())
}

// Names returns the signing entity (or company) name of each row.
func (g *Group) Names() []string {
	names := make([]string, 0, len(g.Rows))
	for _, r := range g.Rows {
		names = append(names, firstNonEmpty(r.SigningEntityName, r.CompanyName))
	}
	return names
}

// CompanyIDs returns the internal ids of the group's rows.
func (g *Group) CompanyIDs() []string {
	ids := make([]string, 0, len(g.Rows))
	for _, r := range g.Rows {
		ids = append(ids, r.CompanyID)
	}
	return ids
}

// CLAGroupIDs returns the distinct CLA group ids of the group's active CCLAs.
func (g *Group) CLAGroupIDs() []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range g.Rows {
		for _, id := range r.CLAGroupIDs {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}
