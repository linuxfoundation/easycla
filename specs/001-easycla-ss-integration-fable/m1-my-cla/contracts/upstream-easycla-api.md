# Contract: EasyCLA endpoints consumed by M1

All via lfx-gateway `/cla-service` prefix (`lfx-gateway/dynamic/services/cla-service.yaml`), `secured` middleware (user bearer token for the api-gw audience, per research R3). Full reference: [docs/MY_CLAS_API.md](../../../../docs/MY_CLAS_API.md).

**Implementation update (2026-07-28, supersedes the original design below):** instead of the originally-anticipated `GET /v4/users/by-identity` lookup plus SS-side aggregation over `GET /v4/signatures/user/{userID}` and `GET /v4/signatures/{signatureID}/signed-document`, M1 shipped two consolidated read endpoints in `cla-backend-go` (`v2/my_clas` module, swagger-first in `cla.v2.yaml`, no schema changes). Rationale: the per-user signatures endpoint excludes ECLAs, the signed-document endpoint requires project-scoped ACLs contributors do not hold, and "valid ECLA" evaluation against current approval lists is impossible from SS.

## 1. GET /cla-service/v4/my-clas

- Query parameters (all optional, all capped and deduplicated): `lfUsername`, repeated `email`, `secondaryEmail`, `githubId`, `githubUsername`, `gitlabId`, `gitlabUsername`, `gerritUsername`. `lfUsername` defaults to the authenticated principal's username.
- Resolves the identity keys to EasyCLA user records (union of **all** GSI matches per key, deduplicated by `user_id`), fetches every signed ICLA/ECLA, and computes per-row validity: ICLA = signed+approved; ECLA = signed+approved AND employer not sanctioned AND an approved+signed CCLA exists for the CLA group AND the user matches its current approval lists (v1 `UserIsApproved`, the PR-gating function; GitLab-group-only approvals defer to `signature_approved`).
- **Ownership enforcement (upstream, defense-in-depth):** for non-admin callers each key must belong to the authenticated user — verified against all EasyCLA records matching the token LFID, then against the LF account's user-service profile emails and connected identities (source-scoped, `DataSource=platform` when present, canonical spellings used for exact-match index lookups). Unverifiable keys are not searched and are reported in `skippedIdentities`. This exceeds the original "SS is the authorization boundary" caveat — SS session-derived parameters remain the expected usage.
- Out: `my-cla-list` (`lfUsername`, `userIds`, `skippedIdentities`, `resultCount`, `clas[]` with `signatureID`, `claType icla|ecla`, `claGroupID`, `claGroupName`, company fields, `signedOn`, `signed`, `approved`, `valid`, document versions, `pdfAvailable`). ICLAs are returned in all signed statuses (labeled via `valid`); consumers display ECLAs only when `valid` (FR-002).

## 2. GET /cla-service/v4/my-clas/{signatureID}/pdf

- Same identity parameters and ownership enforcement; the signature must resolve to an owned, signed ICLA and the S3 object must exist.
- Out: `{signatureID, url, expiresInSeconds: 900}` — presigned S3 URL (15-min TTL, bucket `cla-signature-files-{stage}`). Unknown/not-owned/unsigned/ECLA/missing-document all return **404** (never 403). Fetch on click only (TTL).

## 3. ACS registration

Both paths ride the gateway's secured catch-all router (no lfx-gateway change); ACS resources `my_clas`/`my_clas_pdf` (`anyRole: true`, `ViewMyClas` policy on the `user` role) are registered via `acs-cli` (`services/11-cla-service.yaml`) and must be synced per environment before rollout.

## Superseded original design (kept for history)

`GET /v4/users/by-identity` (identity lookup wrapping the GSI queries: `lf-username-index`, `lf-email-index`, `github-id-index`) + `GET /v4/signatures/user/{userID}` (per-user signatures — later found to exclude ECLAs via its `signature_user_ccla_company_id` not-exists filter) + `GET /v4/signatures/{signatureID}/signed-document` (presigned PDF — requires project-scoped authorization unavailable to plain contributors). The consolidated endpoints above replace all three for the M1 flow; `/v3/users/search` remains unsuitable (table scan).
