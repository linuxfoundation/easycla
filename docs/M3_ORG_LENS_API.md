# Org lens API — EasyCLA backend for LFX Self Serve M3

Copyright The Linux Foundation and each contributor to CommunityBridge.

SPDX-License-Identifier: CC-BY-4.0

Backend endpoints for the **M3 "Org lens"** milestone of the EasyCLA → LFX Self Serve
program. Design and delivery rules: `~/M3-BE-plan.md` appendices embedded in the
lfx-self-serve issues below.

All endpoints are served through the LFX API gateway under the `/cla-service` prefix
(dev: `https://api-gw.dev.platform.linuxfoundation.org/cla-service`) with an Auth0
bearer token. Exact request/response schemas: `cla-backend-go/swagger/cla.v2.yaml`
(+ `swagger/common/`); malformed path/body values return HTTP 422.

### Terminology

Two objects, and the names this document uses for them:

| Object | This document | Backend / code | Do not use |
|---|---|---|---|
| A signing entity's agreement with one CLA group | **CLA entry**, **corporate agreement** | CCLA, `signature_signed`, `corporate-signature` | — |
| One employee's coverage under that agreement | **employee acknowledgment**, **acknowledgment** | ECLA, `employee_signature`, `autoCreateECLA` | **"Employee CLA"**, **"ECLA"** in prose |

An employee does not sign a separate agreement — they **acknowledge** the company's corporate
agreement. "Employee CLA" implies otherwise and is not used in user-facing text: documentation,
emails and UI all say *employee acknowledgment*. **ECLA** survives only as an internal
abbreviation in code identifiers, URL paths and schema names (`autoCreateECLA`,
`/cla-group/{claGroupID}/ecla/{signatureID}/invalidate`), which this document quotes verbatim where it cites
them. Spelling is American throughout — *acknowledgment*, not *acknowledgement*
([lfx-self-serve#2435](https://github.com/linuxfoundation/lfx-self-serve/issues/2435)).

## `GET /v4/company/external/{companySFID}/cla-groups` ([lfx-self-serve#2149](https://github.com/linuxfoundation/lfx-self-serve/issues/2149))

Read-only organization CLA landing list: one entry per **(signing entity × CLA group)**,
derived from the signed+approved CCLA signatures of every signing entity (company record)
carrying the given external `companySFID`. Each entry carries all its ids
(`companyID`, `companySFID`, `claGroupID`, `foundationSFID`, `signatureID`, project
`projectSFID`, manager `userID`) plus `companyName`,
`signingEntityName`, `claGroupName`, `foundationName`, the covered `projects`,
`signed`/`signedOn`/`signedBy`, the stored `sanctioned` flag with `sanctionedAt`,
`approvedContributorsCount` (approved + signed employee acknowledgments under the CCLA — the
corporate-contributors list below also carries the invalidated ones, so its `totalCount` can be
larger), `approvalCriteriaCount`, `claManagers` (from the signature ACL) with
`claManagersCount`, `needsClaManager` (= signed with zero managers) and `autoCreateECLA`.

`signedOn` is the CCLA's **stored** signing date (`signed_on`) and is omitted when the record
carries none — legacy signatures predating that column get no creation-date substitute here
(the v1 signature converters still substitute `date_created` for the consoles). `sanctionedAt`
is the company's stored `sanctioned_date` — stamped at the first live detection or by the
admin block — and is present only while `sanctioned=true` and a date exists.

`approvalCriteriaCount` ([lfx-self-serve#2222](https://github.com/linuxfoundation/lfx-self-serve/issues/2222))
sums the CCLA's six approval lists (email, email domain, GitHub username, GitHub org,
GitLab username, GitLab group) off the signature already loaded for this response, so it
costs no extra query. It counts **rules**, not people, and is therefore unrelated to
`approvedContributorsCount` — one domain rule can cover an entire company. The Org Lens
CLA Group card renders it as its own stat alongside `claManagersCount`.

`signedBy` ([lfx-self-serve#2231](https://github.com/linuxfoundation/lfx-self-serve/issues/2231))
is the CCLA `SignatoryName` already on the signature this handler loads. It is omitted
when that name is empty; there is no CLA-manager fallback. The Org Lens overview can
then render `Signed by {name} on {date}`.

An unknown company or a company with no CCLAs returns HTTP 200 with an empty `list` —
the endpoint never auto-creates the company record. Limitation: an empty list carries no
company-level data, so the sanctions state of a company that never signed anything cannot
be read from this endpoint. Optional `pageSize`/`offset` query
parameters page the sorted list (`totalCount` = size before paging, `resultCount` = rows
returned); when omitted the full list is returned. Auth: LF admin, `organization`
scope for the `companySFID`, or any `project|organization` scope whose organization half
matches (ACS resource `company_cla_groups`, action `view_all`). Probe:
`utils/company_cla_groups.sh`.

## `GET /v4/company/external/{companySFID}/cla-group/{claGroupID}/corporate-contributors` ([lfx-self-serve#1978](https://github.com/linuxfoundation/lfx-self-serve/issues/1978))

Organization-scoped alias of the pre-existing `GET /v4/cla-group/{claGroupID}/corporate-contributors`
(whose path has no SFID segment, so `organization`/`project|organization` scopes can never
match at the gateway — effectively admin-only; the EasyCLA `azp` trusted-caller allow-list
covers identity verification on my-clas/prepare-sign only, never gateway scope matching).
This is the M3 route for the **acknowledgment table**. Resolves the company by SFID with a
read-only lookup — no record auto-creation; an unknown SFID is a 404 — selecting the
parent signing-entity record by default, or a specific one via the optional `companyID`
query parameter (an ID outside that SFID's records is a 404). Enforces the same
in-handler project/organization access check as the original, then delegates to the same
service; `searchTerm`, `pageSize` and `nextKey` pass through unchanged.

Row set and shape (shared with the original path): every **signed** employee acknowledgment
of the signing entity under the CLA group — `signature_signed=true`, in **any** approval
state, so invalidated acknowledgments (`signature_approved=false`) stay in the table and
`totalCount` counts the same set (it is therefore ≥ the CLA entry's
`approvedContributorsCount`, which counts approved + signed only). `searchTerm` is ANDed on
both the page and the count — case-insensitive substring over the acknowledging user's
name, lowercased reference name, LF username, email, GitHub/GitLab usernames and DocuSign
name. Each row carries the identity fields (`signatureID`, `name`, `linux_foundation_id`,
`github_id`, `gitlab_id`, `email`, `signature_version`, `userDocusignName`,
`userDocusignDateSigned`), `signatureApproved`/`signatureSigned`, `timestamp`,
`signatureModified`, and — when the acknowledgment was invalidated — `invalidatedAt`,
`invalidatedBy`, `invalidationReason`, `invalidationNote` (the M2 attribution attributes,
whichever exist; pre-M2 invalidations carry only the free-text `note`, which is always
echoed as `note`). Paging: `pageSize` (default 10) + `nextKey`; rows are read in
`project-signature-index` order and a page is filled until `pageSize` matching rows or the
end of the key range (matching happens in the backend, not in DynamoDB's case-sensitive
`contains`). `nextKey` is returned while unread rows remain; with a `searchTerm` the page
after the last match may come back empty with no `nextKey`.
The new path needs the same ACS resource registration (organization object on `{companySFID}`) as the
other org-lens paths before gateway scope-matching works. Done on dev: resource
`company_cla_group_corporate_contributors` (type-2 + type-1 twin, `view_all`) bound into
`ViewCompanyClaGroups`, plus the OPA bundle-data refresh described below — verified: a
non-admin cla-manager token now passes the gateway (pre-deploy proof = lambda 404 instead
of gateway 403). Declared in acs-cli `services/11-cla-service.yaml` for prod. Probe
(read-only, non-admin cla-manager token):

```bash
BASE="https://api-gw.dev.platform.linuxfoundation.org/cla-service"
curl -s -H "Authorization: Bearer $TOKEN" \
  "$BASE/v4/company/external/$COMPANY_SFID/cla-group/$CLA_GROUP_ID/corporate-contributors"
curl -s -H "Authorization: Bearer $TOKEN" \
  "$BASE/v4/company/external/$COMPANY_SFID/cla-group/$CLA_GROUP_ID/corporate-contributors?companyID=$COMPANY_ID"
# gateway-shaped 403 = ACS resource not registered; JSON list or lambda 404 = authorized
```

## `GET /v4/cla-group/search` (M3 "Sign a CLA" picker; `projects[]` from [easycla#5209](https://github.com/linuxfoundation/easycla/pull/5209))

Unscoped, read-only typeahead behind the Org lens "Sign a CLA" picker: `searchTerm`
(≥ 3 non-whitespace characters — shorter is a 422 from validation or a 400 after trimming)
and `limit` (1–100, default 20). Matches case-insensitively, as a substring, against the
CLA Group name, the Salesforce project/foundation name and the names of the linked GitHub
organizations, GitLab groups and Gerrit instances; a pasted repository URL or an
`owner/repo` path is resolved to the CLA Group owning that repository (falling back to the
owning organization). Results are deduplicated by `claGroupID` — the CLA Group is the
signing unit — sorted best match first and capped at `limit` (`truncated=true` asks the
user to refine). Each `cla-search-result` carries `claGroupID`, `claGroupName`,
`projectName`/`projectSFID` (a foundation-level CLA Group resolves to its foundation;
omitted when the group maps to several projects with no foundation marker),
`foundationSFID`, `projectExternalID`, `iclaEnabled`/`cclaEnabled`, `matchTypes`
(`claGroup`, `project`, `organization`, `repository`), `organizations[]` (`name`,
`source` ∈ github/gitlab/gerrit, `url`), `matchedRepositoryName`/`matchedRepositoryURL`
(repository matches only) and — since #5209 — **`projects[]`**: every Salesforce project the
CLA Group covers (`company-cla-group-project` items, sorted by `projectName`, including the
foundation-marker mapping of a foundation-level CLA Group), so a consumer can render a
project-level catalog (one row per covered project, "+N more" collapsed) while still signing
the one CLA Group. Presentation data only — `claGroupID` remains the hand-off key. Auth:
any authenticated principal carrying a username (or an admin machine token); no ACS
resource. Reference data (CLA Groups, project mappings, organizations, repositories) is
served from an in-process cache with a 30-minute TTL (`CLA_SEARCH_CACHE_TTL` overrides,
`0` disables), so a newly added mapping can take up to the TTL to become searchable. The
result is company-agnostic by design — a sanctioned company's state is resolved through
the cla-groups endpoint above (which only carries it once the company has a CCLA), not here.

## `POST /v4/self-serve/request-corporate-signature` ([lfx-self-serve#2150](https://github.com/linuxfoundation/lfx-self-serve/issues/2150))

Self Serve front door for starting a CCLA signing session. Input = the corporate-console
`corporate-signature-input` fields (`project_sfid`, `company_sfid`, optional
`signing_entity_name`, `send_as_email`, `authority_name`, `authority_email`,
`return_url`) plus two required-true attestation booleans `authority_acked` and
`embargo_acked` — HTTP 400 before any DocuSign work unless both are true. With both true
the request is delegated **verbatim** to the same service method behind
`/v4/request-corporate-signature` (company/signing-entity resolution, sanctions gate,
DocuSign envelope, send-by-email signatory flow all unchanged), so behavior and error
statuses match the console endpoint — with one hardening on top: a `signing_entity_name`
resolving to a company whose SFID differs from `company_sfid` is rejected with HTTP 403
before delegation. A company blocked for trade compliance is rejected — on this endpoint
and on `/v4/request-corporate-signature` alike — with the typed 403 body described under
"Sanctioned-company write gating" below (`code: "company_sanctioned"`); its `message`
additionally carries the contributor-facing guidance sentence after a newline, so existing
consumers that display `message` keep the same text while new ones key off `code`. Response echoes all ids: `signature_id`, `sign_url`
(empty for `send_as_email`), `cla_group_id`, `project_sfid`, `company_id` (the signing
entity's EasyCLA company record), `company_sfid`. Auth mirrors the console:
`project|organization` tree scope for the (`project_sfid`, `company_sfid`) pair, LF admin
disallowed (ACS resource `self_serve_request_corporate_signature`, action `create`).
Probe: `utils/self_serve_request_corporate_signature.sh` — **a 200 creates a real
DocuSign envelope**; attestation/auth probes expect 400/403 and were verified against the
dev tables, but a failure status alone does not prove the absence of side effects.

## Managers & acknowledgments write ops ([lfx-self-serve#2151](https://github.com/linuxfoundation/lfx-self-serve/issues/2151))

CLA-manager request lifecycle under
`/v4/company/{companyID}/project/{projectSFID}/cla-manager/requests`: `GET` (list — HTTP
200 with an empty `requests` array when none; optional `pageSize`/`offset` query
parameters page the created-date-sorted list with `totalCount` set to the pre-paging
size), `GET .../{requestID}`,
`PUT .../{requestID}/approve` and `PUT .../{requestID}/deny`. The v4 surface wraps the v1
request service verbatim: approve flips the request to `approved`, adds the requester to
the CCLA signature ACL and emails the CLA managers + requester; deny flips it to `denied`
and emails without touching the ACL. Responses reuse the id-complete
`cla-manager-request` shape (`requestID`, `companyID`/`companyExternalID`,
`projectID`/`projectExternalID`, `userID`/`userExternalID`, names, emails, `status`,
`created`/`updated`). A request belonging to another company or CLA group returns 404.

`PUT /v4/cla-group/{claGroupID}/ecla/{signatureID}/invalidate` invalidates one employee
acknowledgment, mirroring the ICLA invalidate internals: sets
`signature_approved=false` with invalidation metadata from the optional body — `reason`
(enum: `signed-in-error`, `should-be-corporate`, `compliance`, `other`) and `note`
(≤2048 chars) — logs the event and emails the employee. 400 when the signature is not an employee
acknowledgment or belongs to another CLA group, 409 when already invalidated; the response echoes
`signature_id`, `cla_group_id`, `company_id`, `user_id`.

Auth for all five: `project|organization` tree scope for the project/company pair, LF
admin disallowed (ACS resources `cla_manager_request_admin`,
`cla_manager_request_approve`, `cla_manager_request_deny`, `ecla_invalidate`). The
existing last-CLA-manager guard is unchanged: a signed CCLA always keeps at least one
manager. Probe: `utils/cla_manager_requests_ops.sh` — approve/deny/invalidate mutate;
list/get do not write; the 400/403/404/409 probes were verified against the dev tables,
but a failure status alone does not prove the absence of side effects.

## Sanctioned-company write gating ([lfx-self-serve#2153](https://github.com/linuxfoundation/lfx-self-serve/issues/2153))

Every M3-surface write op targeting a company whose stored `is_sanctioned` flag is set —
regardless of sanction origin (SSS or manual); no live SSS call is made — is rejected with
HTTP 403 and a typed body: `code: "company_sanctioned"`, a trade-compliance `message`,
`company_id`, `company_sfid`. Gated ops (11): `updateApprovalList`, `eclaAutoCreate` (both
enable **and** disable — supersedes the M2 handler-local 400 on enable), `invalidateECLA`,
`createCLAManager`, `deleteCLAManager`, `createCLAManagerDesignee`,
`createCLAManagerDesigneeByGroup`, `inviteCompanyAdmin`, `createCLAManagerRequest`,
`approveCLAManagerRequest`, `denyCLAManagerRequest`. Reads are unchanged; the CCLA
signing path was already gated in M2 and now returns the same typed body (see the
request-corporate-signature section). Invalidating a company's **existing** employee
acknowledgments when it becomes sanctioned stays blocked pending the product decision in
[lfx-self-serve#2051](https://github.com/linuxfoundation/lfx-self-serve/issues/2051).
Related fix ([lfx-self-serve#2186](https://github.com/linuxfoundation/lfx-self-serve/issues/2186)):
removing an email from the approval list now invalidates **all** matching employee
acknowledgments instead of paging by 10. Related M3 fixes on the same path: an
email/GitHub-username/GitHub-org removal leaves an acknowledgment approved while another
list entry still covers the user (`userStillApproved`), and the auto-create flow
(`autoCreateECLA`) never re-approves an invalidated acknowledgment — a record carrying
invalidation evidence (any attribution attribute, or the legacy "Signature invalidated"
note) is skipped, so re-adding someone to a list does not silently undo a CLA-manager or
admin invalidation; the contributor re-acknowledges through the console instead, which
creates a fresh record. Probe: `utils/sanctioned_write_gate.sh` —
PASS = 403 `company_sanctioned` per op; payloads use bogus targets to limit the damage
where the gate is broken, but a broken gate still runs the real write path: eclaAutoCreate
has no bogus placeholder (skipped unless `ECLA_AUTO_CREATE_OK=1` — a broken gate persists
`auto_create_ecla=false` on the real CCLA) and updateApprovalList targets the real CCLA (a
broken gate rewrites its email approval-list column, adds an inactive approval-history row
for the bogus email and keeps the approval edit's user, event, notification and cache side
effects). Re-check the tables after any non-403 result.

## Deployment & validation notes (dev validated 2026-09-04, Githash 1979904)

All endpoints above were validated end-to-end on dev (status codes, response shapes,
DynamoDB side effects, invalidation metadata, cross-feature count consistency). Findings
that matter again at **prod rollout**:

- **ACS resource object types — manual surgery required (or a fixed acs-cli sync).**
  The platform ACS `POST /v1/api/resources` **ignores `object_type_id`** and always
  creates resources as type 8 (community), which forwards no scopes — every scope-checked
  M3 endpoint 403s for everyone. The acs-cli `services/11-cla-service.yaml` declares the
  correct `objectTypeIDs`, but the sync (POST-based) cannot apply them. Dev was
  hand-fixed via `PUT /resources/{id}` (honors the field; body needs `name`, `path`,
  `object_type_id`, `any_role`): `cla_manager_request_approve`/`_deny`/`_admin`,
  `ecla_invalidate`, `self_serve_request_corporate_signature` → type 1 (project);
  `company_cla_groups` and `company_cla_group_corporate_contributors` → type 2
  (organization), each plus a type-1 twin resource wired into `ViewCompanyClaGroups`
  (`view_all`) so `project|organization` pair holders pass too,
  and an extra `CLAManagerRequestAdmin` statement binding the type-1
  `cla_manager_request` row. **DB edits alone do not propagate**: the live gateway
  authorizer is OPA reading a bundle from S3 (`s3://lf-opa-bundle-{stage}`), not the
  warden DB — `POST /warden/invalidate/cache` + `/cache/flush` are NOT sufficient. To
  propagate: refresh `bundle-data/resources.json` (= `GET /opa/resources`) and
  `bundle-data/role_permissions.json` (= `GET /opa/role/permission?limit=100&offset=0`)
  in that bucket — either via an acs-cli deploy run (`make opa-bundle`) or by fetching
  both with an admin M2M token, diffing against the current S3 objects (expect only your
  additions), and uploading. A bundler lambda re-tars `bundle.tar.gz` within seconds of
  upload; OPA picks it up in ~1–2 min. **Repeat the surgery + bundle refresh on prod ACS
  before M3 goes live.** Verification that works pre-deploy: call a to-be-added path via
  the real gateway with a non-admin token — gateway 403 = not registered, lambda-style
  404 = authorized. (Direct warden v1/v2 probes do not reflect gateway decisions for
  non-admin users.) Quirks: statements POST works only **without** the trailing slash
  (`/policies/{id}/statements`) and wants action **UUIDs**, not names; actions attach via
  `PUT /resources/{id}/actions` (POST = 405).
- **acs-cli deploy workflows have no `concurrency:` group**: concurrent merge-triggered
  runs race on the S3 `bundle-data/` upload (last writer wins with its own checkout's
  snapshot — the two 2026-09-04 dev runs collided; the complete checkout won by ~3 s).
  Run prod syncs one at a time, or add a concurrency group first.
- **LF-admin tokens always 403 on the DISALLOW_ADMIN ops** (2150 + all five 2151 ops):
  warden short-circuits admins with `{"allowed":true,"isAdmin":true}` and **no scopes**,
  and `DISALLOW_ADMIN_SCOPE` checks ignore the admin bit — identical to the long-standing
  approval-list endpoint, so this is platform-standard, not a bug. Testing/FE integration
  needs a real non-admin CLA-manager login; 2149 (`ALLOW_ADMIN`) works with admin tokens.
- **Do not rely on the `sigtype_signed_approved_id` GSI for employee acknowledgments**: the
  dynamo-events stream handler (`v2/dynamo_events/signatures.go`) stamps the key on new
  writes, including literal `ecla` rows (#5199), but acknowledgments written before that
  fix may lack it and no backfill has been run. M3 is unaffected —
  `approvedContributorsCount` and the corporate-contributors list both filter
  `project-signature-index` on `signature_user_ccla_company_id` + `signed`
  (+ `approved` for the count), and invalidate is PK-based.
- **Known shape gaps** (documented in swagger): `userExternalID` is always empty in
  manager-request responses (v1 read projection); `invalidateECLA` looks up the signature
  before authz, so a bogus `signatureID` returns 404 (not 403) to any authenticated
  caller.
