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

## `GET /v4/company/external/{companySFID}/cla-groups` ([lfx-self-serve#2149](https://github.com/linuxfoundation/lfx-self-serve/issues/2149))

Read-only organization CLA landing list: one entry per **(signing entity × CLA group)**,
derived from the signed+approved CCLA signatures of every signing entity (company record)
carrying the given external `companySFID`. Each entry carries all its ids
(`companyID`, `companySFID`, `claGroupID`, `foundationSFID`, `signatureID`, project
`projectSFID`, manager `userID`) plus `companyName`,
`signingEntityName`, `claGroupName`, `foundationName`, the covered `projects`,
`signed`/`signedOn`/`signedBy`, the stored `sanctioned` flag, `approvedContributorsCount` (employee
acknowledgements under the CCLA, same count as the corporate-console contributors list),
`approvalCriteriaCount`, `claManagers` (from the signature ACL) with `claManagersCount`,
`needsClaManager` (= signed with zero managers) and `autoCreateECLA`.

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
the endpoint never auto-creates the company record. Auth: LF admin, `organization`
scope for the `companySFID`, or any `project|organization` scope whose organization half
matches (ACS resource `company_cla_groups`, action `view_all`). Probe:
`utils/company_cla_groups.sh`.

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
before delegation. Response echoes all ids: `signature_id`, `sign_url`
(empty for `send_as_email`), `cla_group_id`, `project_sfid`, `company_id` (the signing
entity's EasyCLA company record), `company_sfid`. Auth mirrors the console:
`project|organization` tree scope for the (`project_sfid`, `company_sfid`) pair, LF admin
disallowed (ACS resource `self_serve_request_corporate_signature`, action `create`).
Probe: `utils/self_serve_request_corporate_signature.sh` — **a 200 creates a real
DocuSign envelope**; attestation/auth probes (400/403) are side-effect free.

## Managers & acknowledgments write ops ([lfx-self-serve#2151](https://github.com/linuxfoundation/lfx-self-serve/issues/2151))

CLA-manager request lifecycle under
`/v4/company/{companyID}/project/{projectSFID}/cla-manager/requests`: `GET` (list — HTTP
200 with an empty `requests` array when none), `GET .../{requestID}`,
`PUT .../{requestID}/approve` and `PUT .../{requestID}/deny`. The v4 surface wraps the v1
request service verbatim: approve flips the request to `approved`, adds the requester to
the CCLA signature ACL and emails the CLA managers + requester; deny flips it to `denied`
and emails without touching the ACL. Responses reuse the id-complete
`cla-manager-request` shape (`requestID`, `companyID`/`companyExternalID`,
`projectID`/`projectExternalID`, `userID`/`userExternalID`, names, emails, `status`,
`created`/`updated`). A request belonging to another company or CLA group returns 404.

`PUT /v4/cla-group/{claGroupID}/ecla/{signatureID}/invalidate` invalidates one employee
acknowledgement, mirroring the ICLA invalidate internals: sets
`signature_approved=false` with invalidation metadata from the optional body — `reason`
(enum: `signed-in-error`, `should-be-corporate`, `compliance`, `other`) and `note`
(≤2048 chars) — logs the event and emails the employee. 400 when the signature is not an ECLA or
belongs to another CLA group, 409 when already invalidated; the response echoes
`signature_id`, `cla_group_id`, `company_id`, `user_id`.

Auth for all five: `project|organization` tree scope for the project/company pair, LF
admin disallowed (ACS resources `cla_manager_request_admin`,
`cla_manager_request_approve`, `cla_manager_request_deny`, `ecla_invalidate`). The
existing last-CLA-manager guard is unchanged: a signed CCLA always keeps at least one
manager. Probe: `utils/cla_manager_requests_ops.sh` — approve/deny/invalidate mutate;
list/get and the 400/403/404/409 probes are side-effect free.

## Sanctioned-company write gating ([lfx-self-serve#2153](https://github.com/linuxfoundation/lfx-self-serve/issues/2153))

Every M3-surface write op targeting a company whose stored `is_sanctioned` flag is set —
regardless of sanction origin (SSS or manual); no live SSS call is made — is rejected with
HTTP 403 and a typed body: `code: "company_sanctioned"`, a trade-compliance `message`,
`company_id`, `company_sfid`. Gated ops (11): `updateApprovalList`, `eclaAutoCreate` (both
enable **and** disable — supersedes the M2 handler-local 400 on enable), `invalidateECLA`,
`createCLAManager`, `deleteCLAManager`, `createCLAManagerDesignee`,
`createCLAManagerDesigneeByGroup`, `inviteCompanyAdmin`, `createCLAManagerRequest`,
`approveCLAManagerRequest`, `denyCLAManagerRequest`. Reads are unchanged and the CCLA
signing path was already gated in M2. Invalidating a company's **existing** ECLAs when it
becomes sanctioned stays blocked pending the product decision in
[lfx-self-serve#2051](https://github.com/linuxfoundation/lfx-self-serve/issues/2051).
Related fix ([lfx-self-serve#2186](https://github.com/linuxfoundation/lfx-self-serve/issues/2186)):
removing an email from the approval list now invalidates **all** matching employee
acknowledgements instead of paging by 10. Probe: `utils/sanctioned_write_gate.sh` —
PASS = 403 `company_sanctioned` per op; payloads use bogus targets so probes are
side-effect free even where the gate is broken, except eclaAutoCreate (no bogus
placeholder exists for it, so it is skipped unless `ECLA_AUTO_CREATE_OK=1` — a broken
gate then persists `auto_create_ecla=false` on the real CCLA) and updateApprovalList
(targets the real CCLA — a broken gate rewrites its email approval-list column
unchanged and adds an inactive approval-history row for the bogus email; junk only,
no approval-state change).

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
  `company_cla_groups` → type 2 (organization) plus a type-1 twin resource wired into
  `ViewCompanyClaGroups` (`view_all`) so `project|organization` pair holders pass too,
  and an extra `CLAManagerRequestAdmin` statement binding the type-1
  `cla_manager_request` row. After edits, flush: `POST /warden/invalidate/cache
  {"type":"resource"}` + `POST /cache/flush`. **Repeat the same surgery on prod ACS
  before M3 goes live**, then verify with warden v1
  `GET /acs/v1/api/warden/subjects/authorize?resource=<path>&actions=<action>` (returns
  computed scopes per resource row). Quirk: statements POST works only **without** the
  trailing slash (`/policies/{id}/statements`).
- **acs-cli deploy workflows have no `concurrency:` group**: concurrent merge-triggered
  runs race on the S3 `bundle-data/` upload (last writer wins with its own checkout's
  snapshot — the two 2026-09-04 dev runs collided; the complete checkout won by ~3 s).
  Run prod syncs one at a time, or add a concurrency group first.
- **LF-admin tokens always 403 on the DISALLOW_ADMIN ops** (2150 + all five 2151 ops):
  warden short-circuits admins with `{"allowed":true,"isAdmin":true}` and **no scopes**,
  and `DISALLOW_ADMIN_SCOPE` checks ignore the admin bit — identical to the long-standing
  approval-list endpoint, so this is platform-standard, not a bug. Testing/FE integration
  needs a real non-admin CLA-manager login; 2149 (`ALLOW_ADMIN`) works with admin tokens.
- **Do not rely on the `sigtype_signed_approved_id` GSI for ECLAs**: the dynamo-events
  stream handler (`v2/dynamo_events/signatures.go`) only handles `signature_type`
  `ccla`/`cla`, so auto-created ECLAs (type `ecla`) never receive the GSI attribute
  (pre-existing gap, not an M3 regression). M3 is unaffected —
  `approvedContributorsCount` filters `project-signature-index` on
  `signature_user_ccla_company_id` + `approved` + `signed`, and invalidate is PK-based.
- **Known shape gaps** (documented in swagger): `userExternalID` is always empty in
  manager-request responses (v1 read projection); `invalidateECLA` looks up the signature
  before authz, so a bogus `signatureID` returns 404 (not 403) to any authenticated
  caller.
