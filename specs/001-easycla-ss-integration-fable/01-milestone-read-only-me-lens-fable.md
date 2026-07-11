# Milestone 1 — Read-only "My CLAs" in the Me Lens

**Status**: Draft | **Depends on**: nothing | **Retires**: nothing | **Effort**: S
**Spec**: [spec.md](spec.md) | **Overview**: [00-overview-fable.md](00-overview-fable.md)

## Goal

A logged-in Self Serve user sees, under the Me lens, all ICLAs they have signed and all currently valid ECLAs, read-only, with signed-PDF download for ICLAs. Signing continues to happen in the Contributor Console (linked out).

## User value

Contributors currently have **no self-service view** of their CLA history anywhere — the Contributor Console is transactional (only shows state for one project during signing). This milestone creates the first such view, and de-risks everything after it by building the SS↔EasyCLA plumbing (auth, identity mapping, API module) on a read-only surface.

## Current state (facts)

- ICLAs/ECLAs live in the `cla-{stage}-signatures` DynamoDB table. ECLA = ICLA-shaped record with `signature_user_ccla_company_id` set and **no PDF**; ICLA has a PDF in S3 `cla-signature-files-{stage}` (`contract-group/{projectID}/cla/{userID}/{signatureID}.pdf`), served via 15-minute presigned URLs.
- The v4 API already has a per-user endpoint: `GET /v2 (v4 path) /users/{userID}/signatures` returning the user's ICLAs and ECLAs, plus per-signature PDF endpoints (`/signatures/{signatureID}/signed-document`, ICLA PDF routes).
- EasyCLA user records key on GitHub ID / emails / optional `lf_username`; one person may hold several records; some records have no LF username at all.
- Self Serve Me lens: route-scoped modules (`me` lens), Express server routes proxying backends; crowdfunding is the existing pattern for integrating a non-V2 backend.

## Scope

### In

1. **Me lens module "My CLAs"** (e.g. `/me/clas`): list of ICLAs (all statuses, labeled valid/superseded) and valid ECLAs; per-row: agreement type, project/CLA group name, company (ECLA), date signed, status; ICLA rows offer signed-PDF download.
2. **SS server-side `cla` module**: Express routes calling `/cla-service/v4` (and v3 where needed) through lfx-gateway; user-scoped only.
3. **Identity resolution**: map the logged-in LF identity (username + verified emails, enriched via lfx-v2-auth-service where useful) to EasyCLA user record(s); aggregate signatures across all matches; telemetry for logged-in-but-unmatched users.
4. **PDF download**: SS obtains the presigned URL from EasyCLA and hands it to the browser; SS never stores documents.
5. **Empty state** with explanation + link to docs; "need to sign?" pointer to the Contributor Console.

### Out

- Any signing, approval-list, or role functionality.
- CCLA visibility (that is company data → M4, Organization lens).
- Backend or schema changes beyond (possibly) one new read endpoint (see below).
- Changing the PR remediation link.

## Functional requirements

FR-001…FR-006 in [spec.md](spec.md).

## Design notes & decisions

- **API sufficiency**: `GET /users/{userID}/signatures` requires the EasyCLA `userID`. SS must first resolve LF identity → EasyCLA user(s). If no efficient lookup-by-LF-username/email endpoint exists in v4, add one small read endpoint to EasyCLA (`GET /v4/users/by-identity?...` or reuse existing user search) rather than scanning client-side. This is the only backend change anticipated.
- **Authorization**: user-scoped data; the SS server must enforce "only the logged-in user's records" (derive `userID` server-side from the session, never trust a client-passed ID). Token model: user's bearer through the gateway if the EasyCLA v4 auth accepts it, else SS M2M with server-side subject binding — decide in design; crowdfunding token-exchange is the fallback pattern.
- **Validity semantics**: define "valid ECLA" precisely from the signature flags (`signature_approved`, `signature_signed`, not revoked/invalidated) and current-employer semantics; superseded ICLA document versions shown with status, not hidden — matches what CLA enforcement would actually honor.
- **Multiple EasyCLA users per person**: merge results by (type, project, company), dedupe; show all — an incomplete list here erodes trust in every later milestone.

## Risks

| Risk | Notes |
|------|-------|
| Identity mapping misses records (email casing, unlinked GitHub identity) | Recent backend work downcases emails; still expect gaps. Ship telemetry; treat unmatched-user rate as a launch gate for M2. |
| Presigned URL TTL (15 min) vs. UI caching | Fetch URL on click, not on page load. |
| "Valid ECLA" definition disagreements | Align with the exact gating logic in the PR check, not an approximation; validate against production samples. |
| v4 Lambda latency (DynamoDB scans on some signature queries) | Read-only page tolerates it; paginate; don't fan out per-project calls. |

## Exit criteria

- SC-001 met (sampled-parity vs. backend data; PDF download ≥ 99%).
- Unmatched-identity telemetry live with an agreed threshold.
- SS `cla` server module reviewed as the base for M2.

## Explicitly cheap to roll back

Feature-flag the module; no data written, no console behavior changed.
