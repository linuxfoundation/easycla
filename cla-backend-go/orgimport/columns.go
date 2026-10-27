// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import "path/filepath"

// ColumnsFile is the per-run legend of every report column: which system the value is read from, how, and what it means.
const ColumnsFile = "columns.csv"

const (
	srcCompanies  = "EasyCLA DynamoDB cla-<stage>-companies"
	srcSignatures = "EasyCLA DynamoDB cla-<stage>-signatures"
	srcDerived    = "org-import (derived)"
	srcOrg        = "LFX Org Service"
	srcMember     = "LFX member-service (B2B org registry behind the Self Serve Organization Lens)"
	srcACS        = "LFX ACS role grants, listed through the ACS user role-scopes API (GET /acs/v1/api/users/rolescopes/organization)"
	srcMapping    = "mapping file (--mapping) or Salesforce Apex (--use-apex)"
	srcDecisions  = "decisions file (--decisions)"

	opScan      = "full table scan"
	opCCLAQuery = "query signature_type=ccla, signature_reference_type=company, signature_reference_id=company_id, signature_signed=true, signature_approved=true"
	opOrgGet    = "GET /orgs/{company_external_id}"
	opLiveGet   = "GET /b2b_orgs/{company_external_id}"
)

var (
	colCompanyID  = []string{"company_id", srcCompanies, opScan, "EasyCLA's own UUID of the company row; never changes; CCLA/ECLA signature rows reference it; mapping key for rows without a usable company_external_id"}
	colName       = []string{"company_name", srcCompanies, opScan, "name entered in EasyCLA when the CCLA was requested; NOT the Salesforce Account Name"}
	colSigning    = []string{"signing_entity_name", srcCompanies, opScan, "legal signing-entity label; rows sharing one company_external_id are the signing entities of one organization"}
	colExternalID = []string{"company_external_id", srcCompanies, opScan, "organization id the row is keyed on today: 001... = Salesforce Account Id (15/18 chars), lf... = legacy Org Service id with no Salesforce Account behind it, empty, or malformed"}
	colShape      = []string{"id_shape", srcDerived, "classification of company_external_id", "001 | lf | empty | other (malformed)"}
	colActive     = []string{"active_ccla", srcSignatures, opCCLAQuery, "true = at least one such signed+approved CCLA references this company_id (the import predicate); groups without an active row are never touched"}
	colCCLACount  = []string{"ccla_count", srcSignatures, opCCLAQuery, "number of such active CCLAs = CLA groups (projects) the company has a signed+approved CCLA for"}
	colECLACount  = []string{"ecla_count", srcSignatures, "count on GSI signature-user-ccla-company-index with signature_user_ccla_company_id=company_id", "employee acknowledgements (ECLAs) under this company row; empty = not measured for this row, -1 = count failed"}
	colOrgStatus  = []string{"org_service", srcOrg, opOrgGet, "HTTP status: 200 = the id exists in Org Service (inventory synced from Salesforce), 404 = unknown there (dead/deleted Account or never synced), err = lookup failed, empty = not queried (empty id)"}
	colWebsite    = []string{"website", srcOrg, opOrgGet + " .Link", "organization website as served by Org Service; empty = none or id unknown"}
	colDomain     = []string{"domain", srcDerived, "registrable domain (public suffix list) of the Org Service website", "the domain-matching key; shared domains (github.com, gmail.com, ...) never match; empty = no website"}
	colRoute      = []string{"route", srcDerived, "id_shape + member-service liveness + mapping", "register = existing Salesforce Account, only POST /b2b_orgs; rewrite = lf or dead 001 id, needs a mapping to a target Account; manual = empty/malformed id or an unresolved mapping/collision; empty = not eligible (no active CCLA)"}
	colReason     = []string{"manual_reason", srcDerived, "why the group is manual or pending", "empty_external_id | invalid_id_shape | mapping_ambiguous | mapping_not_approved | mapping_same_id | sfid_alias_forms | target_forms_differ | target_collision | distinct_conflict | missing_website | shared_domain | apex_match_needs_approval | apex_error | no_mapping | dead_account | crm_unverified | unregistered | not_eligible"}
	colSuggested  = []string{"suggested_account", srcOrg + " + " + srcMember, "org lookup (inventory: other Accounts of this import by domain/name; crm: by registrable website domain, then by name), each candidate verified with GET /b2b_orgs/{sfid}", "up to three '<sfid> <Org Service name> [inventory:domain|inventory:name|crm:domain|crm:name]' entries separated by '; '; a trailing ? = member-service could not verify the Account (403); empty = no suggestion"}
	colLive       = []string{"live", srcMember, opLiveGet, "live = 200 (Account is in the Organization Lens today = member); unregistered = 403 while other Accounts answer 200 (Account exists but has no B2B org = non-member or deleted, M3.1); dead = 404 (GET /b2b_orgs or the register POST: no such Account in Salesforce); unverified = member-service not configured; error = call failed; empty = not an Account id (lf/empty) or not probed"}
	colShared     = []string{"shared_domain", srcDerived, "domain against the shared-domain list (--shared-domains)", "true = the website domain is shared (github.com, gmail.com, ...) and is never used for Account matching"}
	colNewID      = []string{"new_id", srcMapping, "mapping row old_id -> new_id (or Apex dry-run result)", "target Salesforce Account Id the group is rewritten to; empty = no target yet"}
	colAction     = []string{"action", srcMapping, "mapping row action", "matched = existing Account found by domain/name (needs approved=true), created = new Account created for this org, ambiguous = several candidates (stays manual); empty = no mapping row"}
	colDecision   = []string{"decision", srcDecisions, "decision row covering these old ids", "collapse = the old ids (and rows already on the target) are one organization, the Account may receive them all; distinct = different organizations; empty = no decision"}
	colReviewer   = []string{"reviewer", srcDecisions, "decision row reviewer", "LF username of the person who made the decision; required by the tool"}
	colCompanyIDs = []string{"company_ids", srcCompanies, opScan, "company_id of every row of the group, ';'-separated"}
	colNames      = []string{"company_names", srcCompanies, opScan, "company_name of every row of the group, ';'-separated"}
	colError      = []string{"error", srcDerived, "last error of the group in this run", "why the group failed or stays pending; empty = none"}
	colKey        = []string{"key", srcDerived, "group key", "company_external_id of the group, or the row's company_id when the id is empty/malformed (such a row is a group of its own); also the state.jsonl key"}
	colOldID      = []string{"old_id", srcCompanies, opScan, "company_external_id as stored (the mapping file old_id); empty for rows without an id (then key = company_id)"}
)

// columnLegend lists every column of every report file: file, column, source system, operation, meaning.
var columnLegend = [][]string{
	// audit.csv
	with("audit.csv", colCompanyID), with("audit.csv", colName), with("audit.csv", colSigning), with("audit.csv", colExternalID), with("audit.csv", colShape),
	with("audit.csv", colActive), with("audit.csv", colCCLACount),
	with("audit.csv", []string{"ecla_count", srcSignatures, colECLACount[2], colECLACount[3] + "; filled only for active rows that are manual, duplicate or unresolvable"}),
	with("audit.csv", colOrgStatus), with("audit.csv", colWebsite),
	with("audit.csv", []string{"duplicate_sfid_group", srcDerived, "rows sharing one company_external_id", "true = another row carries the same company_external_id with the same or an empty signing entity name (exact-id duplicate set)"}),
	with("audit.csv", colRoute), with("audit.csv", colReason),
	with("audit.csv", []string{"tier", srcDerived, "reachability tier, 1:1 with utils/audit_company_reachability.sh", "SFID_OK | MISSING_SFID | INVALID_SFID_FORMAT | SFID_DANGLING_OR_DELETED (Org Service 404) | UNKNOWN (lookup failed)"}),
	with("audit.csv", []string{"acs_roles", srcACS, "list grants scoped to company_external_id (organization and project|organization scopes)", "role=count;... (e.g. cla-manager, cla-signatory, cla-manager-designee, contact, contributor, company-admin, viewer); copied to the target on a rewrite, then deleted on the old id; filled for eligible 001/lf groups and duplicate-set rows; err = listing failed"}),
	with("audit.csv", colSuggested),
	// unresolvable.csv
	with("unresolvable.csv", colCompanyID), with("unresolvable.csv", colName), with("unresolvable.csv", colSigning), with("unresolvable.csv", colExternalID), with("unresolvable.csv", colShape),
	with("unresolvable.csv", colCCLACount), with("unresolvable.csv", colECLACount), with("unresolvable.csv", colOrgStatus), with("unresolvable.csv", colReason),
	// possible_duplicates.csv
	with("possible_duplicates.csv", []string{"group", srcDerived, "rows sharing one id with the same or empty signing entity name, or the same normalized company_name or the same registrable website domain under different ids (sets overlapping by name and domain are merged)", "duplicate set number; every row of the set is listed; distinct signing entities under one id are not duplicates"}),
	with("possible_duplicates.csv", colCompanyID), with("possible_duplicates.csv", colName), with("possible_duplicates.csv", colSigning), with("possible_duplicates.csv", colExternalID), with("possible_duplicates.csv", colShape),
	with("possible_duplicates.csv", colDomain), with("possible_duplicates.csv", colActive), with("possible_duplicates.csv", colCCLACount), with("possible_duplicates.csv", colECLACount),
	// plan.csv
	with("plan.csv", colKey), with("plan.csv", colOldID), with("plan.csv", colShape), with("plan.csv", colRoute), with("plan.csv", colReason), with("plan.csv", colLive),
	with("plan.csv", colOrgStatus), with("plan.csv", colWebsite), with("plan.csv", colDomain), with("plan.csv", colShared), with("plan.csv", colNewID), with("plan.csv", colAction),
	with("plan.csv", colDecision), with("plan.csv", colReviewer), with("plan.csv", colCompanyIDs), with("plan.csv", colNames), with("plan.csv", colError), with("plan.csv", colSuggested),
	// to_salesforce.csv
	with("to_salesforce.csv", colOldID),
	with("to_salesforce.csv", []string{"name", srcCompanies, opScan, "company_name of the group's first row = the name proposed for the new Salesforce Account"}),
	with("to_salesforce.csv", colWebsite),
	with("to_salesforce.csv", []string{"ccla_signed_date", srcSignatures, opCCLAQuery + " (signed date, or created date when unset)", "earliest signing date among the group's active CCLAs (ISO date); empty = unknown"}),
	with("to_salesforce.csv", colDomain), with("to_salesforce.csv", colShared), with("to_salesforce.csv", colSuggested),
	// manual_actions.csv
	with("manual_actions.csv", colKey), with("manual_actions.csv", colOldID), with("manual_actions.csv", colRoute),
	with("manual_actions.csv", []string{"reason", srcDerived, "manual or pending reason of the group (same values as manual_reason) or 'failed'", colReason[3]}),
	with("manual_actions.csv", []string{"suggested_action", srcDerived, "text for the reason", "what a human must do next for this group (mapping row to add, Account to find with sales ops, flag to use, ...)"}),
	with("manual_actions.csv", colLive), with("manual_actions.csv", colOrgStatus), with("manual_actions.csv", colWebsite), with("manual_actions.csv", colDomain), with("manual_actions.csv", colShared),
	with("manual_actions.csv", colNewID), with("manual_actions.csv", colAction), with("manual_actions.csv", colCompanyIDs), with("manual_actions.csv", colNames), with("manual_actions.csv", colError), with("manual_actions.csv", colSuggested),
	// targets.csv
	with("targets.csv", []string{"target_sfid", srcMapping, "new_id of every rewrite group landing on one Account", "Salesforce Account Id that receives the rewritten rows (compared per Account: 15- and 18-char forms are the same target)"}),
	with("targets.csv", []string{"groups", srcDerived, "count", "number of source groups (old ids) rewritten onto this Account in this run"}),
	with("targets.csv", []string{"old_ids", srcCompanies, opScan, "company_external_id (or company_id for empty/malformed ids) of every source group, ';'-separated"}),
	with("targets.csv", colCompanyIDs), with("targets.csv", colNames),
	with("targets.csv", []string{"existing_rows", srcCompanies, opScan, "number of EasyCLA rows that already carry target_sfid before the rewrite (its current signing entities)"}),
	with("targets.csv", colDecision), with("targets.csv", colReviewer),
	with("targets.csv", []string{"status", srcDerived, "collision check of the Account against --decisions", "ok | needs_decision (several old ids or existing rows land on one Account without a collapse decision = target_collision) | distinct_conflict (a distinct decision spans ids resolving to this Account) | target_forms_differ (ids landing here use both the 15- and 18-char form)"}),
}

func with(file string, col []string) []string {
	return append([]string{file}, col...)
}

// writeColumnsCSV writes the column legend next to the reports so every value can be traced to its system and operation.
func writeColumnsCSV(dir string) error {
	rows := make([][]string, 0, len(columnLegend)+1)
	rows = append(rows, []string{"file", "column", "source", "operation", "meaning"})
	rows = append(rows, columnLegend...)
	return writeCSV(filepath.Join(dir, ColumnsFile), rows)
}
