# Research — Milestone 1: Read-only "My CLAs" (Phase 0)

All Technical Context unknowns resolved. Facts verified in code on 2026-07-11 unless noted.

## R1. Which EasyCLA endpoints supply "my ICLAs and ECLAs"?

**Decision**: `GET /v4/signatures/user/{userID}` (swagger `cla.v2.yaml` path `/signatures/user/{userID}`; handler `v2/signatures/handlers.go` `SignaturesGetUserSignaturesHandler`, paginated via `pageSize`/`nextKey`, also accepts an optional `userName` query param passed to the v1 service). Returns the user's signature records — ICLAs and ECLAs are distinguished by `signature_user_ccla_company_id` (present ⇒ ECLA) per the signatures data model (`signatures/models.go`).

**Rationale**: purpose-built per-user endpoint; no fan-out per project; v2 response models already include project/company references.

**Alternatives considered**: per-project queries (`/signatures/project/...`) — O(projects) calls, rejected; new bespoke endpoint — unnecessary.

## R2. Identity resolution: LF SSO → EasyCLA user record(s)

**Decision**: resolve server-side in SS, in order:
1. `GET /v3/users/username/{userName}` (handler `users/handlers.go:198` `GetUserByUserName`) with the session's LF username → primary EasyCLA `userID`.
2. If not found or to catch pre-LF-login history: `GET /v3/users/search` with `searchField=user_emails` / `lf_email` against the user's verified emails (from session claims; optionally enriched via `lfx-v2-auth-service` NATS lookup, which SS already uses). Union all matched user records; query R1 per `userID`; merge + dedupe agreements.
3. Instrument: emit telemetry counting (a) no-match users, (b) multi-record users — the M2 launch gate metric from the milestone doc.

**Rationale**: both lookup endpoints already exist — the contingency `cla-backend-go` endpoint is **not needed** unless dev testing shows `users/search` can't match by email reliably (e.g., auth constraints or index gaps; recent "downcase emails" work reduces casing misses). Keep the contingency in scope but expect to drop it.

**Alternatives considered**: new `GET /v4/users/by-identity` (only as contingency); client-side resolution (rejected — leaks other users' data paths, violates server-side enforcement); using `GET /v4/user-from-token` (attractive — derives the EasyCLA user from the caller's token — but depends on SS's token being an accepted audience; evaluate during T1 spike, use if it works since it's the least-privilege option).

## R3. AuthN/AuthZ toward EasyCLA: which token does SS send?

**Decision**: two-step verification spike, then commit:
1. Preferred: forward the user's bearer token through lfx-gateway to `/cla-service/v4/*` (gateway route `cla-service.yaml` applies `secured` middleware; EasyCLA v4 validates Auth0 JWTs). If EasyCLA accepts SS's audience, use it — least privilege and enables `user-from-token`.
2. Fallback (crowdfunding precedent): dedicated audience via token exchange (`exchangeRefreshTokenForAudience` util) or SS M2M credentials, with the SS server binding the subject (session user) to every upstream query itself.

**Critical constraint (verified)**: v4 `GetUserSignatures` performs **no ownership check** — any authenticated principal can query any `userID`. Therefore the SS server is the authorization boundary: routes derive `userID` strictly from the session, never from request input. Also flag this upstream as a hardening observation (out of M1 scope).

## R4. Signed-PDF download

**Decision**: on user click, SS calls `GET /v4/signatures/{signatureID}/signed-document` (or the ICLA-specific `/signatures/project/{claGroupID}/icla/{signatureID}/pdf`) after re-checking the signature belongs to one of the session's resolved EasyCLA userIDs; return the presigned S3 URL (15-min TTL, `utils/s3.go` `PresignedURLValidity`) as a redirect/JSON for the browser. Never prefetch on page load; never proxy the PDF bytes through SS.

**Rationale**: TTL makes prefetching stale; redirect keeps SS stateless and avoids buffering documents. ECLAs: no document endpoint offered at all (no PDF exists — confirmed in data model).

## R5. SS integration pattern & feature flag

**Decision**: follow the crowdfunding module shape: Angular lens module (`app/modules/my-clas`, lazy route `me/clas`, `data: { lens: 'me' }`), Express server route + controller + service (`server/routes|controllers|services`), upstream base URL from env (`CLA_SERVICE_BASE_URL` pointing at lfx-gateway `/cla-service`). Gate with a LaunchDarkly flag mirroring `CROWDFUNDING_ENABLED_FLAG` usage in `main-layout.component.ts` (sidebar item + route guard). Direct `ApiClientService`-style HTTP client rather than `MicroserviceProxyService` if the auth model ends up non-standard (R3 fallback), matching how `cfFetch()` bypasses the proxy for crowdfunding.

**Alternatives considered**: `MicroserviceProxyService` with a new `LFX_V2_CLA_SERVICE` env — preferred if R3 option 1 works, since it reuses token forwarding; choose in the R3 spike, both are established patterns.

## R6. "Valid ECLA" / ICLA status semantics

**Decision**: mirror enforcement semantics from the signatures data model: a listed agreement requires `signature_signed=true`; status shown as **Valid** when `signature_approved=true` and not invalidated/revoked; ICLAs additionally labeled **Superseded** when their `signature_document_major_version` is older than the CLA group's current major version (field available on signature records; comparison against CLA group document only if present in the v2 response — verify field availability in T1 spike; if absent, show signed-version number and omit the superseded badge in v1 of the page). ECLAs shown only when currently valid (per FR-002); ICLAs shown regardless of status with a status label (per FR-001).

**Rationale**: must match what PR gating honors, not approximate it; degrade gracefully where the response model lacks a field rather than fan out extra calls.

## R7. Testing approach

**Decision**: unit tests for identity-merge logic (multi-record, no-match, dedupe) and controller authz (session-derived ID only); recorded-fixture contract tests for the two upstream response shapes; one E2E happy path against dev (user with ICLA + ECLA fixture data) behind the flag. Golden acceptance check = SC-001 sampling script comparing SS output to direct EasyCLA queries for a set of test users.
