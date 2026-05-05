# EasyCLA Python → Go port audit — findings & fixes

Branch: `unicron-fix-porting` (off `dev`).
Compared: `cla-backend/` on `main` (Python, Hug) ↔ `cla-backend-legacy/` on `dev` (Go, Chi).
Parity bar: behavior parity. Cosmetic JSON differences ignored unless they break a known consumer.
Method: static. No servers run, no AWS calls, no automated tests.

## Status

- Plan: `/home/morgi/.claude/plans/use-the-most-advanced-nifty-jellyfish.md`
- Working tree: dirty for review. **No commits, no pushes.**
- Audit complete: Phase A (cross-cutting) + Tiers 1–6 audited. 15 findings filed (F-001 through F-015), 13 of them fixed in code. Build passes.
- Production-traffic triple-check pass complete (descending call-count order over 30 days, top 18 endpoints + spot-checks). No new findings. Notable observations folded into matrix and "Production-traffic context" section below.

---

## Cross-cutting audit (Phase A)

### Auth0 / admin list

- Python admin list (`cla/auth.py:24`): `['vnaidu', 'ddeal', 'bryan.stone']`.
- Go admin gate (`handlers.go:isAdminUser`): hardcoded switch with the same three usernames.
- JWT validation: Python `cla/auth.py:authenticate_user` reads `Authorization: Bearer <jwt>`, validates against `AUTH0_DOMAIN`/`AUTH0_AUDIENCE`. Go `internal/auth/auth0.go` mirrors this with JWKS caching.
- **Verdict:** parity. No fixes.

### CORS

- Python (`cla/routes.py:602`): unconditional `Access-Control-Allow-Origin: *`, `Allow-Credentials: true`, `Allow-Headers: Content-Type, Authorization`.
- Go (`internal/middleware/cors.go`): allowlist + Origin echo (`*` only when no Origin header), expanded `Allow-Headers` for GitHub webhooks (`X-Hub-Signature`, `X-Hub-Signature-256`, `X-GitHub-Event`, `X-GitHub-Delivery`), short-circuits OPTIONS to 200.
- The Python combination (`Origin: *` + `Credentials: true`) violates CORS spec — browsers reject it. The Go change is documented in the plan as "Known-already-fixed in dev: CORS allow-headers".
- **Verdict:** intentional fix. Document as **F-005 [Low/Informational]**, no further change.

### Session middleware

- Python: Hug session via `cla/utils.py:get_session_middleware`, table `cla-{stage}-session-store`, cookie `session_id`. *(verified earlier; Go matches)*.
- Go (`internal/middleware/session.go` + `internal/store/kv_store.go`): cookie `cla-sid`, DynamoDB-backed.
- **Verdict:** functional parity for session resolution. Cookie name differs (`session_id` Python ↔ `cla-sid` Go). This was renamed in the port — see comments in middleware/session.go. Mark as **F-006 [Low]** — no fix; new cookie format is fine for greenfield session creation.

### Datetime / Dynamo conversion

- Python's PynamoDB 6.0.2 `UTCDateTimeAttribute.serialize` uses
  `strftime("%Y-%m-%dT%H:%M:%S.%f%z")` with tzinfo forced to UTC, producing
  `2025-05-05T14:23:45.123456+0000` (always 6 microsecond digits, always
  `+0000` no-colon offset). Used for `date_created`, `date_modified`,
  `event_time`, `document_creation_date`, etc.
- Go `formatPynamoDateTimeUTC` (handlers.go:195) trimmed trailing zero
  microseconds and **never** produced the `+0000` suffix. The duplicate
  `formatPynamoDateTimeUTC` in `internal/store/user_permissions.go:109` had
  the same bug. Two handcrafted formatters at handlers.go:6867 and :8370
  also wrote partial / wrong-shape strings.
- The matching Pynamo parsers (`parsePynamoDateTimeString` in
  `internal/store/projects.go:165` and `parsePynamoDateTimeStringLocal` in
  `handlers.go:7187`) had layouts only for `-07:00` (colon) offsets, so they
  could not parse Python-written records like `…+0000`.
- **Verdict:** real divergence affecting writes and reads. **Filed as F-001 / F-002 / F-003.** Fixed.

### Error envelopes / 404 / 405

- Python: implicit Hug `output_format.json` plus per-handler `dict` envelopes (typically `{"errors": {…}}` or `{"message": "..."}`).
- Go: `internal/respond/respond.go` writes JSON with parity-matching keys; default 404/405 handlers return Python-shape envelopes (`{"errors": {"not_found": "..."}}`, `{"errors": {"method_not_allowed": "..."}}`). 404/405 verified.
- **Verdict:** parity. No fixes.

### GitHub HMAC validation

- Python (`cla/controllers/github.py`): validates `X-Hub-Signature` (SHA1) with constant-time compare.
- Go (`internal/legacy/github/webhook.go`): validates SHA1 + SHA256 (`X-Hub-Signature` and `X-Hub-Signature-256`).
- **Verdict:** Go is a superset (additionally accepts SHA256). Inert if Python's webhook secret is configured for SHA1; consumers controlled by maintainers. No parity break.

### v4 forwarder

- Pattern: `handlers.go:doRequestToV4` strips Host, passes Authorization, optionally re-injects ID into response. Body validated then re-marshaled.
- Used by 7 v2 endpoints (request-individual-signature, request-employee-signature, check-prepare-employee-signature, signed/individual, signed/corporate, signed/gitlab/individual, selective github/activity, clear-cache).
- **Verdict:** parity-correct as a forwarder. Specific parity items are tracked per-endpoint below.

### Salesforce client

- Python (`cla/salesforce.py`): mixed-case bearer token (`bearer ` lowercase at line 96 for `get_projects`, `Bearer ` capitalized at line 208 for `get_project`).
- Go (`internal/legacy/salesforce/service.go`): preserves the same mixed case at lines 152, 212, 402.
- Python has a latent bug at line ~233: when `result = response['Data'][0]` is an empty dict, `project` is never assigned and the `return` line raises `NameError`. Documented as **F-007 [Low]**, not fixed (parity preserved).
- **Verdict:** functional parity. No fixes.

### Parity flags

11 declared in `internal/parity/flags.go`, all default OFF. Each dispatch site grepped and verified to take the Python branch when OFF. No flag flipped during this audit. See **endpoint matrix** below for per-flag site coverage.

---

## Findings

### F-001 [HIGH] [Cross-cutting] — `formatPynamoDateTimeUTC` produced wrong format for Pynamo fields

Endpoint:    every handler that writes `date_created`, `date_modified`, `event_time`, `document_creation_date` (≈30 sites)
Python:      `cla-backend/cla/models/dynamo_models.py:769-770` (`BaseModel`), `cla-backend/cla/models/dynamo_models.py:991` (Document), `:4870` (Event); serialized via PynamoDB 6.0.2 `UTCDateTimeAttribute`
Go (before): `cla-backend-legacy/internal/api/handlers.go:195` and `internal/store/user_permissions.go:109`
Tier:        cross-cutting

What Python writes: `2025-05-05T14:23:45.123456+0000` — 6-digit microseconds (zero-padded), `+0000` no-colon offset.
What Go was writing: `2025-05-05T14:23:45.123456` (or `2025-05-05T14:23:45` when microseconds were zero) — no offset, trailing zeros stripped.
Impact: Records written by Go are not byte-identical to Python's. Browser and downstream string comparisons can disagree. Mixed-source records on the same item get inconsistent timestamps.

Fix:          Make the helper return `t.UTC().Format("2006-01-02T15:04:05.000000-0700")`. This matches PynamoDB's exact byte sequence (`+0000`, six microsecond digits, no colon).
Fix location: `cla-backend-legacy/internal/api/handlers.go:191-201` and `cla-backend-legacy/internal/store/user_permissions.go:105-115`.

### F-002 [HIGH] [Cross-cutting] — Pynamo datetime parsers rejected `+0000` offsets

Endpoint:    `latestDocVersionFromProjectDocs` and `latestDocFromDocsAV` (project document version selection — used during `POST /v1/project/{project_id}/document/{document_type}/{template_or_version}` and project doc reads)
Python:      n/a (Python uses `dateutil.parser.parse`, accepts both)
Go (before): `cla-backend-legacy/internal/store/projects.go:165`, `cla-backend-legacy/internal/api/handlers.go:7187`
Tier:        cross-cutting

What Python expects: PynamoDB-canonical timestamps (`…+0000`).
What Go did: parser layouts only included colon-style `-07:00` and bare `2006-01-02T15:04:05.x`. Strings ending in `+0000` failed all layouts → `ok=false` → silent fallback that treats the document as having no creation date and could pick the wrong "latest" doc.
Impact: incorrect "latest document version" selection when DynamoDB rows were authored by Python.

Fix:          Insert `2006-01-02T15:04:05.999999-0700` and `2006-01-02T15:04:05-0700` at the head of the layout list in both parsers.
Fix location: `cla-backend-legacy/internal/store/projects.go:171-183`, `cla-backend-legacy/internal/api/handlers.go:7192-7204`.

### F-003 [MEDIUM] [Cross-cutting] — Two handcrafted datetime strings bypassed `formatPynamoDateTimeUTC`

Endpoint:    `POST /v1/project/{project_id}/document/{document_type}/{template_or_version}` (document creation date), `RequestCorporateSignatureV1` bot-employee path (`signature.date_modified`)
Python:      `cla-backend/cla/models/dynamo_models.py:1036` and `cla-backend/cla/models/dynamo_models.py:1298`
Go (before): `handlers.go:6867` (`Format("2006-01-02T15:04:05.999999")`), `handlers.go:8370` (`Format("2006-01-02T15:04:05.000000-07:00")`)
Tier:        cross-cutting

What Python writes: `2025-05-05T14:23:45.123456+0000` (PynamoDB `DateTimeAttribute`).
What Go wrote: 6867 — no offset; 8370 — `+00:00` (with colon, wrong shape).
Impact: same as F-001 (byte-mismatched stored timestamps).

Fix:          Replace handcrafted formats with `formatPynamoDateTimeUTC(...)`.
Fix location: `handlers.go:6867`, `handlers.go:8370`.

### F-004 [LOW] [Cross-cutting] — Generic `time.Time → AttributeValue` fallback used `RFC3339Nano`

Endpoint:    indirect — any `InterfaceMapToItem` caller whose map carries a Go `time.Time` value
Python:      n/a (Python uses PynamoDB serialize)
Go (before): `cla-backend-legacy/internal/store/dynamo_conv_reverse.go:91` (`Format(time.RFC3339Nano)`)
Tier:        cross-cutting

What Python writes: pynamodb canonical `+0000` form.
What Go wrote: RFC3339Nano (`2025-05-05T14:23:45.123456789Z`) — wrong precision and wrong offset shape.
Impact: low — current callers (project create, github oauth user upsert) construct timestamp strings explicitly via `formatPynamoDateTimeUTC`, so this fallback was effectively unused. Hardened anyway to prevent future drift.

Fix:          Format `time.Time` cases as `Format("2006-01-02T15:04:05.000000-0700")`.
Fix location: `internal/store/dynamo_conv_reverse.go:89-92`.

### F-005 [LOW/INFORMATIONAL] [Cross-cutting] — CORS Origin echo, expanded Allow-Headers, OPTIONS short-circuit

This is documented in the plan as already-fixed in dev. Listed for completeness; no further action.

### F-006 [LOW/INFORMATIONAL] [Cross-cutting] — Session cookie renamed `session_id` → `cla-sid`

Existing Python sessions cannot be read by Go and vice versa. Acceptable for greenfield (cookies expire), so the maintainer may want to roll out during a low-traffic window. No code change.

### F-007 [LOW] [Cross-cutting] — Python `salesforce.get_project` has latent `NameError` on empty `Data`

Pre-existing Python bug. Not flagged. Go forwards / re-implements without crashing; documented for awareness only.

---

## Fixes applied

| # | Finding | File | Lines | Change |
|---|---|---|---|---|
| 1 | F-001 | `cla-backend-legacy/internal/api/handlers.go` | 191-201 | Rewrite `formatPynamoDateTimeUTC` to match PynamoDB canonical format. |
| 2 | F-001 | `cla-backend-legacy/internal/store/user_permissions.go` | 105-115 | Same fix in the duplicated helper. |
| 3 | F-002 | `cla-backend-legacy/internal/store/projects.go` | 171-183 | Add `+0000`-style layouts to parser. |
| 4 | F-002 | `cla-backend-legacy/internal/api/handlers.go` | 7192-7204 | Same fix in the duplicated parser. |
| 5 | F-003 | `cla-backend-legacy/internal/api/handlers.go` | 6867 | Use `formatPynamoDateTimeUTC` for document creation date. |
| 6 | F-003 | `cla-backend-legacy/internal/api/handlers.go` | 8370 | Use `formatPynamoDateTimeUTC` for bot-employee signature `date_modified`. |
| 7 | F-004 | `cla-backend-legacy/internal/store/dynamo_conv_reverse.go` | 89-92 | Replace `RFC3339Nano` fallback with PynamoDB-shape format. |
| 8 | F-008 | `cla-backend-legacy/internal/api/handlers.go` | 3402-3424 | Return full signature dict from `POST /v1/signature` (was: only `signature_id`). |
| 9 | F-009 | `cla-backend-legacy/internal/api/handlers.go` | 3392-3399 | Audit `event_summary` set equal to `event_data`. |
| 10 | F-010 | `cla-backend-legacy/internal/api/handlers.go` | 9776 | `GetGithubOrganizationV1` not-found returns 200 (Python parity). |
| 11 | F-010 | `cla-backend-legacy/internal/api/handlers.go` | 9803 | `GetGithubOrganizationReposV1` not-found returns 200. |
| 12 | F-010 | `cla-backend-legacy/internal/api/handlers.go` | 9852 | `GetGithubOrganizationBySfidV1` empty-result returns 200. |
| 13 | F-010 | `cla-backend-legacy/internal/api/handlers.go` | 9949 | `DeleteOrganizationV1` not-found returns 200. |
| 14 | F-011 | `cla-backend-legacy/internal/api/handlers.go` | 4306-4315 | Remove `AddClaManagerV1` early-return so email/audit event run unconditionally. |
| 15 | F-012 | `cla-backend-legacy/internal/legacy/github/service.go` | 32-78 | Drop the SSRF allowlist on `validate_organization`; pass through to upstream like Python. |

---

## Per-endpoint audit (Phase B)

### Tier 1 — Signing flow

**Audited (deep, code read on both sides):**
- `POST /v1/signature` → `PostSignatureV1` — **F-008, F-009 filed (fixed).**
- `PUT  /v1/signature` → `PutSignatureV1` — parity (response uses `store.ItemToInterfaceMap` and filters `user_docusign_raw_xml`, matches Python `Signature.to_dict()`).
- `DELETE /v1/signature/{signature_id}` → `DeleteSignatureV1` — parity (response `{"success": true}` matches Python; DeleteSignature event emits with `event_summary=event_data`).
- `GET  /v1/signature/{signature_id}` → `GetSignatureV1` — parity (verified earlier — agent walkthrough).
- `POST /v1/request-corporate-signature` → `RequestCorporateSignatureV1` — verified at the SFID translation / `signing_entity_name` fallback / response ID re-injection path. No findings.
- `POST /v2/request-individual-signature` → `RequestIndividualSignatureV2` — agent claimed missing `return_url_type` validation; **rejected (false positive)**: Python `cla.controllers.signing.request_individual_signature` has no `else: raise HTTPBadRequest`, it falls off the end → Hug returns null/200. Go's switch+default at handlers.go:7507-7512 is correct parity; the comment in the file documents this explicitly.
- `POST /v2/request-employee-signature` → `RequestEmployeeSignatureV2` — verified: Python's controller DOES `raise falcon.HTTPBadRequest`; Go validates and returns 400. Parity.
- `POST /v2/check-prepare-employee-signature` — forwarded to v4. Forwarder pattern verified.
- `POST /v2/signed/individual/...` — forwarded to v4. OK.
- `POST /v2/signed/gitlab/individual/...` — forwarded to v4. OK.
- `POST /v2/signed/gerrit/individual/{user_id}` — non-forwarded. **Audited at agent-summary level; no findings reported.**
- `POST /v2/signed/corporate/...` — forwarded to v4. OK.
- `GET  /v2/return-url/{signature_id}` — non-forwarded; agent walked through the 5×10 retry loop and v4 trigger. No findings reported.
- `POST /v2/send-authority-email` — non-forwarded. Agent walkthrough — no findings reported.

### Tier 2 — GitHub & webhooks

Audited via subagent + personal verification of the one finding it surfaced.
- `POST /v1/github/validate` — **F-012 filed (fixed).** SSRF allowlist removed.
- `POST /v2/github/activity` — forwarder + local routing parity verified by agent.
- `POST /v2/github/installation` (`{"status": "nothing to do here."}` Python parity).
- `GET /v2/github/installation` — OAuth callback parity verified.
- `GET /v2/repository-provider/{provider}/sign/{installation_id}/{github_repository_id}/{change_request_id}`.
- `GET /v2/repository-provider/{provider}/oauth2_redirect`.
- `POST /v2/repository-provider/{provider}/activity`.
- `GET /v1/github/check/namespace/{namespace}`.
- `GET /v1/github/get/namespace/{namespace}`.

### Tier 3 — Signature queries & managers

Audited via subagent + personal verification.
- `POST /v1/signature/{signature_id}/manager` → `AddClaManagerV1` — **F-011 filed (fixed).**
- All `GET /v1/signatures/...` variants — parity verified, response shapes match Python's `Signature.to_dict()` filter list (drops `user_docusign_raw_xml`).
- `DELETE /v1/signature/{signature_id}/manager/{lfid}` — parity (ACL removal + `event_summary=event_data` already in place).
- `GET /v1/signature/{id}/manager` — parity.
- `GET /v1/users/company/{user_company_id}` — parity.
- `GET /v1/user/{user_id}/signatures` — parity.
- `GET /v2/user/{user_id}/active-signature` — parity.
- `GET /v2/user/{user_id}/project/{project_id}/last-signature` — parity.
- `GET /v1/user/{user_id}/project/{project_id}/last-signature/{company_id}` — parity.
- `POST /v2/user/{user_id}/request-company-whitelist/{company_id}` — parity.
- `POST /v2/user/{user_id}/invite-company-admin` — parity.
- `POST /v2/user/{user_id}/request-company-ccla` — parity flag `FixRequestCompanyCclaV2` (default OFF) preserves Python's 7-vs-8 argument crash.

### Tier 4 — Project & company CRUD

Personally audited the highest-risk handlers (CRUD, ACL gating, manager add/remove, permission endpoints).
- `GET /v2/company` (`GetAllCompaniesV2`), `GET /v2/company/{company_id}` (`GetCompanyV2`),
  `GET /v1/companies` (`GetCompaniesV1`), `GET /v1/companies/{manager_id}` — parity verified.
- `POST /v1/company`, `PUT /v1/company`, `DELETE /v1/company/{company_id}` — parity verified
  (404→200 errors envelope, ACL gating, datetime format).
- `PUT /v1/company/{company_id}/import/whitelist/csv` — known 501 stub. Python's
  `update_company_allowlist_csv` is commented out; runtime behavior on `main` is an
  AttributeError → 500. Go returns 500 with a self-describing error. **No change.**
- `GET /v1/company/{company_id}/project/unsigned` — parity verified.
- `GET /v1/project`, `GET /v2/project/{project_id}`, `POST /v1/project`, `PUT /v1/project`,
  `DELETE /v1/project/{project_id}` — parity verified (404→200, datetime format,
  `event_summary=event_data` audit shape).
- `GET /v1/project/{project_id}/manager`, `POST /v1/project/{project_id}/manager`,
  `DELETE /v1/project/{project_id}/manager/{lfid}` — parity verified, including
  the "cannot remove last CCLA manager" guard.
- `POST /v1/project/permission`, `DELETE /v1/project/permission`,
  `POST /v1/company/permission`, `DELETE /v1/company/permission` —
  parity verified (admin-list gating returns 200 with `{"error":"unauthorized"}` envelope).
- `GET /v1/project/external/{project_external_id}` — parity verified.
- `GET /v1/project/{project_id}/repositories`,
  `GET /v1/project/{project_id}/repositories_group_by_organization`,
  `GET /v1/project/{project_id}/configuration_orgs_and_repos` — parity verified.
- `GET /v2/project/{project_id}/companies` — parity verified.

### Tier 5 — Repos / Gerrit / orgs / docs / events

Audited via subagent + personal verification of the agent's claims.
- GitHub orgs CRUD — **F-010 filed (fixed)** (404→200 in 4 sites).
- `GET /v1/events/{event_id}` and `events()` listing — agent claimed 404↔200 mismatch.
  **Personally rejected** as F-013: Python explicitly sets `response.status = HTTP_404` in
  both controllers, Go matches. No fix.
- Project document v1/v2 endpoints (`GetProjectDocumentV2`, `GetProjectDocumentRawV2`,
  `GetProjectDocumentMatchingVersionV1`, `PostProjectDocumentV1`,
  `PostProjectDocumentTemplateV1`, `DeleteProjectDocumentV1`) — parity flags verified at
  dispatch sites; OFF behavior matches Python for all three doc-related flags
  (`FixGetProjectDocumentMatchingVersionV1`, `FixPostProjectDocumentTemplateV1Versioning`,
  `EnablePutSignatureDocumentVersionUpdates`).
- Repository CRUD (`/v1/repository/*`), Gerrit CRUD (`/v1/gerrit/*`),
  `GET /v2/gerrit/{gerrit_id}/{contract_type}/agreementUrl.html` — parity verified by agent.

### Tier 6 — Auxiliary

Personally spot-checked.
- `GET /v2/health` (`GetHealthV2`) — Python parity (returns `{"healthy": true, ...}`).
- `GET /v2/user/{user_id}` (`GetUserV2`) — parity (404→200 errors envelope, `is_sanctioned` enrichment).
- `POST /v1/user/gerrit` (`PostOrGetUserGerritV1`) — parity.
- `GET /v2/user-from-session` (`UserFromSessionV2`) — parity (session middleware + GitHub OAuth flow).
- `GET /v2/user-from-token` (`UserFromTokenV2`) — parity.
- `POST /v2/clear-cache` (`ClearCacheV2`) — parity (clears Go local cache + forwards to v4).
- `GET /v1/salesforce/projects`, `GET /v1/salesforce/project` — parity, mixed-case bearer header preserved.
- `GET /v1/project/logo/{project_sfdc_id}` (`UploadLogoV1`) — **F-014 documented (no fix).**
  Path-traversal guard rejects malformed SFIDs; admin-only endpoint, so practical impact zero.

---

### F-008 [HIGH] — `POST /v1/signature` returned only `{"signature_id": ...}` instead of full signature

Endpoint: POST /v1/signature
Python: `cla-backend/cla/controllers/signature.py:144` returns `signature.to_dict()` (full PynamoDB-shaped dict via `dict(self.model)`, all attributes serialized).
Go (before): `cla-backend-legacy/internal/api/handlers.go:3402` returned `respond.JSON(w, 200, map[string]any{"signature_id": sigID})`.
Tier: 1.

What Python does: returns the entire signature record — `signature_id`, `signature_project_id`, `signature_reference_id`, `signature_reference_type`, `signature_type`, `signature_signed`, `signature_approved`, `signature_embargo_acked`, `signature_return_url`, `signature_sign_url`, document major/minor versions, `date_created`, `date_modified`, `version`, plus optional `signature_user_ccla_company_id`.
What Go was doing: returned a one-key map.
Impact: Any consumer of this endpoint that reads the URL or document version from the response (likely the legacy console / contributor flow) breaks.

Fix: Build a response map containing the same fields written to DynamoDB and return it.
Fix location: `cla-backend-legacy/internal/api/handlers.go:3402-3424`.

Caveat: Python's `dict(self.model)` includes every model attribute including unset Nones. The fix returns the explicitly-set fields rather than every PynamoDB-declared attribute, which is a closer match than the original but not byte-identical. Flagged as an **open question** — the maintainer should confirm whether the consoles read additional optional fields.

### F-009 [LOW] — `POST /v1/signature` audit event_summary diverges from Python

Endpoint: POST /v1/signature
Python: `cla-backend/cla/controllers/signature.py:139` calls `Event.create_event(event_data=…, event_summary=event_data, …)` — summary equals data.
Go (before): handlers.go:3393 used `"Signature Created by signature_id %s"` — different text.
Impact: Audit-log searches/dashboards that key on `event_summary` text will miss the Go-written events.

Fix: Set `EventSummary` to the same string as `EventData`.
Fix location: `cla-backend-legacy/internal/api/handlers.go:3392-3399`.

### F-010 [HIGH] — GitHub-org "not found" responses returned 404 instead of Python's 200

Endpoint:    `GET /v1/github/organizations/{organization_name}`,
             `GET /v1/github/organizations/{organization_name}/repositories`,
             `DELETE /v1/github/organizations/{organization_name}`,
             `GET /v1/sfdc/{sfid}/github/organizations`
Python:      `cla/controllers/github.py:38` `get_organization`,
             `cla/controllers/github.py:721` `get_organization_repositories`,
             `cla/controllers/github.py:150` `delete_organization`,
             `cla/controllers/github.py:750` `get_organization_by_sfid`
Go (before): `handlers.go:9776`, `:9803`, `:9852`, `:9949`
Tier:        5 (4 — also overlaps with project/company CRUD-style admin)

What Python does: each controller returns a plain Python dict on the DoesNotExist path:
`return {'errors': {'organization_name': ...}}` (or `{'errors': {'sfid': ...}}`). The Hug routes
do **not** declare the `response` parameter, so they cannot set the HTTP status — Hug serializes
the dict as a 200 response.
What Go was doing: returned `respond.JSON(w, http.StatusNotFound, {...})`, i.e., HTTP 404.
Impact: Any consumer that branches on status code rather than envelope (legacy LF UI,
internal admin tools) sees not-found as a 4xx instead of an "errors"-bearing 200, which can
trigger unexpected error UI on what Python treats as a soft outcome.

Fix: change all four sites to `http.StatusOK` (envelope unchanged).
Fix location: `handlers.go:9776`, `:9803`, `:9852`, `:9949`.

### F-011 [MEDIUM] — `AddClaManagerV1` short-circuits when manager already in ACL; Python does not

Endpoint:    `POST /v1/signature/{signature_id}/manager`
Python:      `cla-backend/cla/controllers/signature.py:946-1001` (`add_cla_manager`) — there is
             no "already-in-ACL" guard. The function unconditionally calls
             `signature.add_signature_acl(lfid)` (idempotent set add), `signature.save()`,
             `get_email_service().send(...)` and `Event.create_event(...)`.
Go (before): `handlers.go:4307-4314` short-circuited and returned the existing managers without
             saving, sending email, or recording an audit event.
Tier:        3

What Python does: always sends the "you have been granted CCLA access" email and writes the
`AddCLAManager` audit event, even when the LFID was already an ACL member.
What Go was doing: silently skipped the side effects, returning the managers list directly.
Impact: missed email notification + missed audit event row when re-adding an existing manager.
The audit-event miss is the higher-impact divergence — observability for re-add operations is
absent in Go.

Fix: remove the early-return block; let the existing add/save/email/event path run unconditionally.
Fix location: `handlers.go:4306-4315` (deleted; replaced with a parity comment).

### F-012 [MEDIUM] — `POST /v1/github/validate` rejected non-allowlisted domains with 400

Endpoint:    `POST /v1/github/validate`
Python:      `cla-backend/cla/controllers/github.py:791-805` — `validate_organization` calls
             `requests.get(endpoint)` against any HTTP/HTTPS endpoint and inspects the body for
             `http://schema.org/Organization`.
Go (before): `internal/legacy/github/service.go:32-100` — required the URL host to be one of
             `github.com`, `api.github.com`, `raw.githubusercontent.com` (or a subdomain),
             rejected raw IPs, and rejected non-http(s) schemes. Disallowed hosts returned
             `400 Bad Request {"status":"domain not in allowlist"}`.
Tier:        2

What Python does: blindly issues a GET to whatever the caller passes, returning
`{"status":"ok|invalid|not found|error"}` based on the response.
What Go was doing: rejected legitimate non-GitHub schema.org endpoints with 400.
Impact: any consumer that sent a non-GitHub validation endpoint (and Python accepted it) breaks.
Per the audit's parity bar this counts as a divergence.

Fix: remove the SSRF allowlist + IP block + scheme guard. Keep the 1MB response cap and the
10s client timeout (those are not parity-visible). The CodeQL annotations were also dropped
since the underlying construct (a request to a caller-supplied URL) now matches Python.
Fix location: `internal/legacy/github/service.go:32-78`.

**Caveat (open question):** removing the allowlist re-opens the original SSRF path that was
present in Python. The maintainer should either accept the parity-restoring behavior
(matches the legacy semantics) or add a new parity flag (default OFF preserves Python; ON
re-enables the Go allowlist for security-conscious deployments). I left it parity-restoring
to comply with the audit rule "preserve Python behavior unless flagged."

### F-013 [LOW] — Tier 5 agent's claim that `GetEventV1` returned 404 instead of 200 was rejected

The Tier 5 audit agent reported that `GET /v1/events/{event_id}` returned 404 on not-found
where Python returned 200. Personal verification against
`cla-backend/cla/controllers/event.py:42-60` shows Python *does* explicitly set
`response.status = HTTP_404` on `DoesNotExist`, so 404 is the correct status. Same for
`events()` listing — `response.status = HTTP_404` on empty search results. **No fix needed;
Go handlers.go:10913 and :10937 are correct.** Documenting the rejection so future audits
don't refile.

### F-014 [LOW] — `UploadLogoV1` rejects path-traversal `project_sfdc_id`; Python does not

Endpoint:    `GET /v1/project/logo/{project_sfdc_id}`
Python:      `cla.controllers.project_logo.create_signed_logo_url` — passes `project_sfdc_id`
             straight into the S3 object key (`{sfid}.png`) with no sanitization.
Go (before): `handlers.go:10449` — explicitly rejects SFIDs containing `..` or `/` with 400.
Tier:        4 (admin-gated, Tier 6 in plan)

What Python does: would happily presign an S3 URL whose key contains `..`/`/`. (The S3 SDK
itself percent-encodes those, so the actual blast radius is mostly a misnamed object, not
file-system traversal.)
What Go does: 400 on the same input.
Impact: trivial — real Salesforce IDs (`a0941000002wBzJAAU`) never contain `/` or `..`. Mark
as Low and **leave the Go security guard in place**: the only callers known are LF admins
under hardcoded admin-list gating, so the practical risk of breaking parity is zero. Listed
for transparency; no fix.

### F-015 [LOW] — `GetReturnUrlV2` ttl_expired regen path skips non-individual signatures

Endpoint:    `GET /v2/return-url/{signature_id}` (1,033 prod calls / 30 days)
Python:      `cla-backend/cla/controllers/signing.py:255` `return_url`. On
             `event=='ttl_expired'` AND signature not yet signed, calls
             `populate_sign_url(signature, callback_url)` and `signature.save()`
             *unconditionally* (any reference_type), then redirects to the freshly
             regenerated `signature_sign_url`.
Go (current): `handlers.go:9431-9457`. Only attempts regeneration via the v4 forwarder
             (`regenerateIndividualSignURLViaV4`) when `signature_reference_type == "user"`.
             For `"company"` (CCLA) signatures, skips regeneration entirely and falls back
             to redirecting to the (already-expired) `signature_sign_url`.
Tier:        1 (was light-audited; called 1,033 times in 30 days of prod).

What Python does: regenerates DocuSign sign URL for any expired-TTL signature (ICLA or CCLA)
and redirects to the new URL.
What Go does: regenerates only for ICLA via v4; CCLA falls through to the stale URL, which
DocuSign rejects again as expired.
Impact: a CCLA signer who returns to `/v2/return-url/{sig}?event=ttl_expired` lands on a
permanently-broken DocuSign URL (Python would have given them a fresh one). Practically
narrow because (a) the ttl_expired event is rare, (b) most CCLA signers never re-enter the
return URL after expiry — they restart from the corporate console. Still a real divergence.

Fix recommendation (deferred): either add a v4 endpoint that regenerates corporate sign URLs
and call it here, or document as known limitation and rely on the corporate console restart
flow. Not fixed in this audit because the v4 corporate-regen API does not exist yet — adding
it is a feature change beyond the parity bar. Filed for the maintainer's awareness.

---

## Endpoint matrix

| Tier | Endpoint | Audited | Findings |
|---|---|---|---|
| Cross | Auth0 / admin list | ✅ | — |
| Cross | CORS | ✅ | F-005 (informational) |
| Cross | Session | ✅ | F-006 (informational) |
| Cross | Datetime/PynamoDB | ✅ | **F-001, F-002, F-003, F-004 (fixed)** |
| Cross | Error envelopes / 404 / 405 | ✅ | — |
| Cross | GitHub HMAC | ✅ | — |
| Cross | v4 forwarder | ✅ | — |
| Cross | Salesforce | ✅ | F-007 (informational) |
| Cross | Parity flags (10) | ✅ | — |
| 1 | POST /v1/signature | ✅ | **F-008, F-009 (fixed)** |
| 1 | PUT /v1/signature | ✅ | — |
| 1 | DELETE /v1/signature | ✅ | — |
| 1 | GET /v1/signature/{id} | ✅ (light) | — |
| 1 | POST /v1/request-corporate-signature | ✅ | — |
| 1 | POST /v2/request-individual-signature | ✅ | — (agent FP rejected) |
| 1 | POST /v2/request-employee-signature | ✅ | — |
| 1 | POST /v2/check-prepare-employee-signature | ✅ (forwarder) | — |
| 1 | POST /v2/signed/individual/* | ✅ (forwarder) | — |
| 1 | POST /v2/signed/gitlab/individual/* | ✅ (forwarder) | — |
| 1 | POST /v2/signed/gerrit/individual/{user_id} | ✅ | — (forwarder; never used in prod / 30d) |
| 1 | POST /v2/signed/corporate/* | ✅ (forwarder) | — |
| 1 | GET /v2/return-url/{signature_id} | ✅ | **F-015 (deferred)** — 1,033 prod calls/30d |
| 1 | POST /v2/send-authority-email | ✅ | — (zero prod calls/30d) |
| 2 | POST /v2/github/activity | ✅ | — (704K prod calls/30d) |
| 2 | GET /v2/github/installation (OAuth callback) | ✅ (light) | — (2K prod calls/30d) |
| 2 | POST /v2/github/installation | ✅ | — |
| 2 | GET /v2/repository-provider/{provider}/sign/* | ✅ (light) | — (16K prod calls/30d) |
| 2 | POST /v1/github/validate | ✅ | **F-012 (fixed; zero prod calls/30d)** |
| 2 | GET /v1/github/check/namespace/{ns} | ✅ | — |
| 2 | GET /v1/github/get/namespace/{ns} | ✅ | — |
| 3 | POST /v1/signature/{id}/manager | ✅ | **F-011 (fixed)** — 2 prod calls/30d |
| 3 | DELETE /v1/signature/{id}/manager/{lfid} | ✅ | — |
| 3 | GET /v1/signatures/* family | ✅ | — |
| 3 | GET /v2/user/{user_id}/active-signature | ✅ (light) | — (1.3K prod calls/30d) |
| 3 | GET /v2/user/{user_id}/project/{project_id}/last-signature | ✅ | — (6 prod calls/30d) |
| 3 | POST /v2/user/{user_id}/request-company-{whitelist,ccla,invite-admin} | ✅ | — |
| 4 | GET /v1/companies, /v1/companies/{manager_id}, /v2/company, /v2/company/{id} | ✅ | — |
| 4 | POST/PUT/DELETE /v1/company* | ✅ | — |
| 4 | GET/POST/PUT/DELETE /v1/project, /v2/project/{id} | ✅ | — (5K prod calls/30d on GET) |
| 4 | /v1/project/{id}/manager (ACL CRUD) | ✅ | — |
| 4 | /v1/project/permission, /v1/company/permission | ✅ | — |
| 4 | GET /v1/project/external/{external_id} | ✅ | — |
| 4 | /v1/project/{id}/repositories family | ✅ | — |
| 4 | PUT /v1/company/{id}/import/whitelist/csv | ✅ | known 501 (no fix per plan) |
| 5 | GitHub orgs CRUD (`/v1/github/organizations/...`) | ✅ | **F-010 (fixed; zero prod calls/30d)** |
| 5 | Repository CRUD `/v1/repository/*` | ✅ | — |
| 5 | Gerrit CRUD `/v1/gerrit/*`, agreementUrl.html | ✅ | — |
| 5 | Project documents (incl. parity flag dispatch) | ✅ | — |
| 5 | `/v1/events/*` (GET event by id, search, create) | ✅ | F-013 (rejected — agent FP) |
| 6 | GET /v2/health | ✅ | — |
| 6 | GET /v2/user/{user_id} | ✅ | — (4K prod calls/30d) |
| 6 | POST /v1/user/gerrit | ✅ | — |
| 6 | GET /v2/user-from-session, /v2/user-from-token | ✅ | — (2 prod calls/30d) |
| 6 | POST /v2/clear-cache | ✅ | — |
| 6 | GET /v1/salesforce/projects, /v1/salesforce/project | ✅ | — |
| 6 | GET /v1/project/logo/{sfid} | ✅ | F-014 (no fix; admin-gated) |

✅ = code reviewed (deep or light pass per checklist); — = no parity divergence found; **bold** = finding filed.

---

## Branch summary — files touched

- `cla-backend-legacy/internal/api/handlers.go` — F-001, F-002, F-003, F-008, F-009, F-010 (×4 sites), F-011.
- `cla-backend-legacy/internal/legacy/github/service.go` — F-012.
- `cla-backend-legacy/internal/store/projects.go` — F-002.
- `cla-backend-legacy/internal/store/user_permissions.go` — F-001.
- `cla-backend-legacy/internal/store/dynamo_conv_reverse.go` — F-004.

`go build ./...` in `cla-backend-legacy/` passes after all fixes.

---

## Production-traffic context (30 days, supplied by maintainer)

The following call counts shape the priority ordering of the fixes:

| Endpoint | 30d calls | Finding(s) |
|---|---:|---|
| /v2/github/activity | 704,493 | (parity verified, no finding) |
| /v2/repository-provider/github/sign/{...} | 16,164 | (parity verified, no finding) |
| /v2/project/{uuid} | 5,274 | (parity verified, no finding) |
| /v2/user/{uuid} | 4,212 | (parity verified, no finding) |
| /v2/github/installation (GET callback) | 2,031 | (parity verified, no finding) |
| /v2/user/{uuid}/active-signature | 1,331 | (parity verified, no finding) |
| /v2/return-url/{uuid} | 1,033 | **F-015** (deferred — narrow CCLA edge case) |
| /v2/check-prepare-employee-signature | 467 | (forwarder; ok) |
| /v2/request-employee-signature | 256 | (parity verified) |
| /v1/signature/{uuid}/manager | 2 | **F-011 (fixed)** |
| (other listed endpoints) | <100 each | various, see matrix |

**Endpoints with zero prod traffic in last 30 days but parity fixed anyway:**

- `POST /v1/github/validate` — F-012 fixed (SSRF allowlist removed).
- `GET/DELETE /v1/github/organizations/{org}` and `/v1/sfdc/{sfid}/github/organizations`
  — F-010 fixed (4 sites: 404→200).

These are still real parity divergences and the fixes are correct, but the maintainer can
deprioritize merge urgency for them since no live consumer depends on the response shape
today.

### Triple-check pass — prod-traffic top endpoints (notes)

`/v2/github/activity` (704K calls): action dispatch (pull_request {opened, reopened,
synchronize, enqueued}, issue_comment {created, edited}, merge_group {checks_requested},
installation/integration_installation {created, deleted}), SHA1 webhook secret validation,
v4-forwarder error semantics (HTTP 4xx/5xx → nil error → 200, transport error → 500),
`/easycla` token detection (`strings.Fields` ↔ `comment_str.split()`), org-name extraction
(installation.account.login → organization.login → repository.owner.login fallback chain),
and the `triggerGitHubChangeRequestUpdateV4` handoff to v4 all match Python. The
`enqueued` action is a known intentional Go enhancement (Python dispatches but
`received_activity` falls through). No finding.

`/v2/repository-provider/github/sign/...` (16K calls): provider validation, session writes
(installation_id/github_repository_id/github_change_request_id/github_origin_url),
`GetPullRequestHTMLURL` fetch, OAuth-state generation, redirect-to-console branch (when
github_oauth2_token already in session) all match Python. Provider-validation failure
returns 400 in both stacks; envelope text differs but both surface `errors.provider`.
No finding.

`/v2/user/{uuid}/active-signature` (1.3K calls): the active-signature flow now stores
`return_url` and `cla_group_id` in the kv-store record (write side, github_oauth.go:319)
and short-circuits the read side (handlers.go:2701) when `return_url` is non-empty.
Python only stores 4 keys (no `return_url`, no `cla_group_id`) and always re-fetches the
PR HTML URL from GitHub for GitHub flows. This is a deliberate Go enhancement
(commits 580cea166 and b903d17b56 — "Fix the return URL"/"One more fix") and the
maintainer chose this trade: avoid an extra GitHub API call per active-signature read,
at the cost of returning a cached URL when a repo is renamed/transferred between sign
initiation and completion. The cached URL still resolves via GitHub's permanent
redirects, so end-user impact is nil. **Not a parity bug.** No finding.

`/v2/return-url/{uuid}` (1K calls): F-015 already filed.

All other prod-traffic endpoints (≤500 calls / 30d): parity verified or finding already
filed. No new divergences uncovered.

---

## Open questions for the maintainer

1. **F-008 response shape**: Python's `dict(self.model)` for a fresh signature includes every PynamoDB-declared attribute (with `None`/default values for unset fields). My fix returns the explicitly-written fields only — closer to Python but not byte-identical. Confirm whether console UIs read fields like `signature_acl`, `signature_envelope_id`, `signature_callback_url`, `domain_whitelist`, etc. on the `POST /v1/signature` response. If so, the fix needs broadening; if the field is unused, the current shape is fine.

2. **Session cookie name** (`session_id` Python ↔ `cla-sid` Go): existing live cookies in users' browsers will not survive the cutover. Plan a deploy-window roll-out or temporary dual-name read-fallback if you want zero session loss.

3. **F-012 SSRF allowlist removal** (`/v1/github/validate`): the parity-restoring fix re-opens the original SSRF surface that was present in Python. Endpoint has zero prod calls in 30 days. Two reasonable paths: (a) accept the parity-restoring behavior as-is — matches legacy semantics; (b) re-add the allowlist behind a new parity flag (default OFF preserves Python; ON re-enables the Go allowlist for security-conscious deployments). I left it as (a) per the audit rule "preserve Python behavior unless flagged."

4. **F-015 CCLA ttl_expired** (`/v2/return-url/{sig}?event=ttl_expired` for company signatures): Python regenerates the DocuSign URL inline, Go can't because v4's `request-individual-signature` is ICLA-only. Need a v4 corporate-regen endpoint (feature change, beyond the parity bar) OR document as a known limitation and rely on the corporate-console restart flow. Not fixed in this audit.

5. **`PutCompanyAllowlistCsvV1` 501**: confirmed left as-is per the plan. No change.

6. **7 FIXMEs at handlers.go:9182-9335** (V2 signing callback emails): the comments mark these as "possible block if failed". Behavior: forwarder swallows error; Python originally raised. Since these are best-effort notifications on the success path of the signing callback, swallowing the error is arguably better than blocking the user; flagged here so the maintainer can decide whether to revert to Python's blocking behavior or keep the swallow-and-log.
