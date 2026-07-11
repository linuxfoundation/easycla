# Research — Milestone 1: Read-only "My CLAs" (Phase 0)

All Technical Context unknowns resolved. Facts verified in code on 2026-07-11 unless noted.

## R1. Which EasyCLA endpoints supply "my ICLAs and ECLAs"?

**Decision**: `GET /v4/signatures/user/{userID}` (swagger `cla.v2.yaml` path `/signatures/user/{userID}`; handler `v2/signatures/handlers.go` `SignaturesGetUserSignaturesHandler`, paginated via `pageSize`/`nextKey`, also accepts an optional `userName` query param passed to the v1 service). Returns the user's signature records — ICLAs and ECLAs are distinguished by `signature_user_ccla_company_id` (present ⇒ ECLA) per the signatures data model (`signatures/models.go`).

**Rationale**: purpose-built per-user endpoint; no fan-out per project; v2 response models already include project/company references.

**Alternatives considered**: per-project queries (`/signatures/project/...`) — O(projects) calls, rejected; new bespoke endpoint — unnecessary.

## R2. Identity resolution: LF SSO → EasyCLA user record(s)

**Decision**: resolve server-side in SS using **three keys, unioned**:
1. **LF username** — `GET /v3/users/username/{userName}` (handler `users/handlers.go:198`; GSI `lf-username-index`).
2. **Verified emails** — all verified emails on the LF identity (session claims, enriched via `lfx-v2-auth-service` NATS lookup which SS already uses); matched against `lf_email` and `user_emails`.
3. **Linked GitHub account(s)** — SS/Auth0 supports linking GitHub to the LF identity (existing social-connection flow, `social-verification.service.ts`, NATS `user_identity.link`); the linked identity provides the GitHub numeric ID + username. GitHub-derived EasyCLA records (typically missing `lf_username`) are keyed on exactly this — it is the **highest-precision key** for pre-LF-login history. Prefer the immutable numeric `github_id` over username (renames/recycling). **Verified in SS**: nothing is persisted in SS itself — Auth0 stores the linked identity, whose `user_id` is GitHub's numeric ID (`Auth0Identity.user_id`, `packages/shared/src/interfaces/profile.interface.ts:520`; username only in `profileData.nickname`), and `Auth0Service.getUserIdentities()` already fetches identities server-side via NATS auth-service. Spike check: confirm the auth-service returns the bare numeric ID (not a `github|<id>`-prefixed form) — a mismatched key silently returns zero matches, so cover with a fixture test.

Union all matched user records; query R1 per `userID`; merge + dedupe agreements. Telemetry: count (a) no-match users, (b) multi-record users, (c) resolutions that only succeeded via GitHub link — the M2 launch-gate metrics.

**UX hook**: when the result set is empty or the LF identity has no linked GitHub account, the page shows a "Don't see your CLAs? Link your GitHub account" call-to-action into SS's existing identity-linking flow, then re-resolves.

**GitHub ID storage (verified — no new storage needed)**: `user_github_id` is already stored on the users table as a DynamoDB **Number** with GSI `github-id-index` (`users/repository.go:103-105`, `:1046`). It is populated on exactly the records M1 needs: the console's GitHub OAuth get-or-create writes it N-typed (`cla-backend-legacy/internal/api/github_oauth.go:282,306`), and the employee-signature precheck backfills missing id↔username pairs via GitHub API lookups (`cla-backend-legacy/internal/api/handlers.go:8854-8884`). Caveats: very old records may be username-only (treat username matches as hints, numeric ID as authority); any new endpoint must reuse the existing repository methods — the GSI hash key is N-typed and an S-typed write/query misses the index (documented in the backfill code).

**Upstream API gap (verified — changes the contingency assessment)**: the GSI-backed lookups all exist server-side (`lf-username-index`, `lf-email-index`, `github-id-index` in `users/repository.go`), but **`GetUserByGitHubID`/`GetUserByGitHubUsername` are not exposed over HTTP**, and the only generic search that is (`GET /v3/users/search`) performs a **DynamoDB table scan** with a filter expression (`users/repository.go:1279`) — unsuitable for per-request resolution. Therefore the previously-contingent EasyCLA endpoint is now **expected to be required**: `GET /v4/users/by-identity?lfUsername=…&email=…&githubId=…` (one small read endpoint wrapping the existing GSI queries; swagger-first; no schema changes). See `contracts/upstream-easycla-api.md`.

**Alternatives considered**: `/v3/users/search` for email/GitHub matching (rejected — table scan); client-side resolution (rejected — leaks other users' data paths, violates server-side enforcement); `GET /v4/user-from-token` (useful for the LF-username record if the token audience works, but by design it only matches/creates by `lf_username`/`lf_email` — misses GitHub-derived history, and per `cmd/server.go:930` never backfills `lf_username`, so it cannot replace the union).

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
