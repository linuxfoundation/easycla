// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"fmt"
	"strings"
)

// ManualAction is one row of manual_actions.csv: a group the tool will not process on its own.
type ManualAction struct {
	Group     *Group
	Reason    string
	Suggested string
}

// suggestions maps a manual/pending reason to what the operator should do (README §7, m3-org-cleanup.md).
var suggestions = map[string]string{
	"empty_external_id":         "Row has no company_external_id: find or create the Account with sales ops (m3-org-cleanup.md, #2749), then add a mapping row company_id,new_id,matched,true and re-run.",
	"invalid_id_shape":          "company_external_id is neither a Salesforce id nor lf…: find the Account with sales ops (m3-org-cleanup.md, #2749), then add a mapping row company_id,new_id,matched,true and re-run.",
	"no_mapping":                "Rewrite candidate without a target: add a mapping row old_id,new_id,action,approved (data in to_salesforce.csv) or run with --use-apex.",
	"dead_account":              "Salesforce account no longer exists: treat as rewrite — add a mapping row (data in to_salesforce.csv) or run with --use-apex.",
	"mapping_ambiguous":         "Several Accounts share the domain: pick the right Account with sales ops and set action=matched,approved=true in the mapping.",
	"mapping_not_approved":      "Matched Account is not approved: confirm it is the same legal entity, then set approved=true.",
	"apex_match_needs_approval": "Apex matched an existing Account: approve it with a mapping row old_id,new_id,matched,true (or reject with a different target).",
	"mapping_same_id":           "new_id equals old_id (or is the 15/18-char form of the same Account): fix the mapping row.",
	"target_collision":          "Several old ids land on this Account (or it already has EasyCLA rows): record a collapse (one Account) or distinct (separate Accounts) decision with a reviewer in the decisions file (#3085), then re-run.",
	"distinct_conflict":         "A distinct decision separates these ids but they resolve to the same Account: fix the mapping/Apex target or the decision.",
	ReasonTargetFormsDiffer:     "Ids landing on this Account use both its 15- and 18-char form: normalize the mapping new_id (and any EasyCLA row already carrying the Account) to one form, then re-run.",
	ReasonSFIDAliasForms:        "Rows of one Account carry both its 15- and 18-char id: normalize company_external_id to one form (m3-org-cleanup.md, #2749) so the rows form one group, then re-run; nothing is registered or rewritten meanwhile.",
	"missing_website":           "Org-service has no website for the organization: automatic domain match is impossible — add a mapping row after sales ops name the Account.",
	"shared_domain":             "Website domain is shared (github.com, nowebsite.com, mail provider): never matched automatically — add a mapping row after sales ops name the Account.",
	"apex_error":                "Apex call failed: re-run; if it persists check cla-salesforce-apex-* and the endpoint.",
	ReasonUnregistered:          "Account exists in Salesforce but has no b2b_org yet (GET /b2b_orgs answered 403): re-run with --register-unregistered to POST /b2b_orgs for it, or wait for the member-service access tuple.",
	ReasonCRMUnverified:         "Account liveness cannot be verified without member-service: set SSM cla-member-service-base-url-<stage> / cla-member-service-auth0-audience-<stage> and the Auth0 client grant + Heimdall roles (README §2), then re-run.",
}

// Suggest returns the operator guidance for a reason.
func Suggest(reason string) string {
	if s, ok := suggestions[reason]; ok {
		return s
	}
	if strings.HasPrefix(reason, LiveError) {
		return "Transient error: re-run; if it persists inspect run.log."
	}
	return "Review run.log for this group."
}

// SuggestError maps a group error to an action; access errors get the ops item instead of "re-run".
func SuggestError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "client-grant"), strings.Contains(msg, "not authorized to access resource server"):
		return "EasyCLA's Auth0 M2M client has no client grant for the member-service audience (SSM cla-member-service-auth0-audience-<stage>): ops must add it (README §2), then re-run."
	case strings.Contains(msg, "(403)"), strings.Contains(msg, "(401)"):
		return "member-service refused the call: the client needs auditor (GET /b2b_orgs) and global_org_admin (POST /b2b_orgs) in the member-service Heimdall ruleset (README §2), then re-run."
	case strings.Contains(msg, ErrNotConfigured.Error()):
		return "SSM cla-member-service-base-url-<stage> / cla-member-service-auth0-audience-<stage> are missing for this stage (README §2)."
	}
	return Suggest(LiveError)
}

// ManualActions lists every group that needs a human: manual, pending and failed groups.
func (p *Plan) ManualActions() []ManualAction {
	var out []ManualAction
	for _, g := range p.Groups {
		var reason, suggested string
		switch {
		case g.Err != nil:
			reason = g.ManualReason
			if reason == "" {
				reason = LiveError
			}
			suggested = SuggestError(g.Err)
		case g.Route == RouteManual, g.Pending():
			reason = g.ManualReason
			suggested = Suggest(reason)
		default:
			continue
		}
		out = append(out, ManualAction{Group: g, Reason: reason, Suggested: suggested})
	}
	return out
}

func (a ManualAction) String() string {
	g := a.Group
	s := fmt.Sprintf("%s [%s] %s", g.Key, a.Reason, a.Suggested)
	if g.NewID != "" {
		s += " target=" + g.NewID
	}
	if d := g.Domain(); d != "" {
		s += " domain=" + d
	}
	if g.Err != nil {
		s += " error=" + g.Err.Error()
	}
	return s + " | rows=" + strings.Join(g.CompanyIDs(), ";")
}
