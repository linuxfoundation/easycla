# Tasks: Milestone 1 — Read-only "My CLAs" in Self Serve Me Lens

**Input**: Design documents from `specs/001-easycla-ss-integration-fable/m1-my-cla/`
**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/), [quickstart.md](quickstart.md), spike runbook [docs/easycla-ss-migration/spike-runbook.md](../../../docs/easycla-ss-migration/spike-runbook.md)

**Tests**: included — research R7 explicitly defines the testing approach (unit tests for identity merge + controller authz, recorded-fixture contract tests, one E2E, SC-001 sampling script).

**Repos**:

- **SS** = `/Users/michal/src/github/linuxfoundation/lfx-self-serve` (primary; paths below relative to `apps/lfx-one/src/` unless noted)
- **CLA** = `/Users/michal/src/github/linuxfoundation/easycla/cla-backend-go` (one read endpoint only)

Before touching either repo, verify current file paths/conventions — the plan's structure was derived from the crowdfunding module and repo layout may have drifted.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an incomplete task)
- **[US1]**: the single P1 user story ("see my ICLAs/ECLAs, download ICLA PDFs")

---

## Phase 1: Setup — spikes & decisions (dev environment)

**Purpose**: convert the plan's remaining unknowns into facts before writing code. Outcomes are recorded back into `research.md` (R2/R3/R6) — they change which implementation branch later tasks take.

- [ ] T001 Run **spike 1** (SS-minted user token → secured v4 read) exactly per `docs/easycla-ss-migration/spike-runbook.md` steps 1–2 (user A); record token-claims checkpoint result and HTTP status in `specs/001-easycla-ss-integration-fable/m1-my-cla/research.md` R3. If the audience exchange fails, file the Auth0 client-grant config item (auth0-terraform) — that is the fix, not a code change.
- [ ] T002 Run **spike 2** (role-less user → `GET /cla-service/v4/signatures/user/{userID}`) per the runbook (user B). Decision point: **200** ⇒ user-token model confirmed; **403** ⇒ choose between the small ACS policy addition and the M2M fallback (SS server binds session userID itself) and record the decision + rationale in `research.md` R3. Blocks T014.
- [ ] T003 [P] Spike: confirm the GitHub-ID format returned by `Auth0Service.getUserIdentities()` in SS dev — bare numeric ID vs `github|<id>`-prefixed (`Auth0Identity.user_id`, SS `packages/shared/src/interfaces/profile.interface.ts`). Record in `research.md` R2; the parsing rule feeds T016/T018 fixtures (a mismatched key silently returns zero matches).
- [ ] T004 [P] Spike: curl dev `GET /cla-service/v4/signatures/user/{userID}` and `GET /cla-service/v4/signatures/{signatureID}/signed-document` for a known test user (quickstart "Useful direct calls"); capture raw JSON responses as fixtures into SS `apps/lfx-one/src/server/services/__fixtures__/cla/` (or repo-convention fixture location). Verify which fields the v2 model actually exposes: document major/minor version (R6 superseded detection), `signedOn`, company name/ID, `signed`/`approved`. Record field availability in `research.md` R6; if document version is absent, the superseded badge is dropped per R6.
- [ ] T005 [P] Audit the T004 fixture payloads for v1 user-service/org-service IDs per architecture-proposal P9 (company references on ECLA rows are the likely spot); note in `research.md` which display fields depend on v1 IDs and the planned lookup (`lfx.lookup_v1_user_sfid.by_username`/`.by_email` NATS RPCs) or confirm none are needed for M1's read-only display.
- [ ] T006 Confirm with the EasyCLA team that `GET /v4/users/by-identity` will be accepted (contract `contracts/upstream-easycla-api.md` §1) and agree on the LaunchDarkly flag name per SS repo convention (plan says `my-clas-enabled`, name TBD); create the dev flag. Record final endpoint shape + flag name in the contracts/plan.

**Checkpoint**: token model decided (T002), fixtures captured (T004), upstream endpoint agreed (T006).

---

## Phase 2: Foundational — upstream endpoint + SS plumbing (blocking)

**Purpose**: the identity-lookup endpoint and the SS server scaffolding every US1 task builds on.

### Upstream endpoint in cla-backend-go (sequential — same module, swagger-first)

- [ ] T007 Add `GET /v4/users/by-identity` to CLA `swagger/cla.v2.yaml` (query params: `lfUsername`, repeated `email`, repeated `githubId`; response: array of existing v2 user models, deduped) and run `make swagger` to regenerate `gen/` (never hand-edit `gen/`). License header on any new file.
- [ ] T008 Implement the service method in CLA `v2/users/` (or extend the existing users service per module pattern): union `GetUserByLFUserName` + per-email `GetUserByEmail`/`user_emails` match + per-githubId `GetUserByGitHubID`, reusing the existing GSI-backed repository methods in `users/repository.go` (`lf-username-index`, `lf-email-index`, `github-id-index` — githubId must stay **N-typed** or the GSI misses); dedupe by `user_id`. Unit tests for union/dedupe/empty-input in the same package.
- [ ] T009 Wire the swagger-generated operation in the module's `handlers.go` `Configure(...)` (request/response translation only, per three-layer convention); register in `cmd/server.go` alongside the other v2 modules. Verify the path is on the **secured** router (gateway `cla-service.yaml` prefix rules) — it must NOT land on the public router; note in `contracts/upstream-easycla-api.md` that SS remains the authorization boundary.
- [ ] T010 Run CLA `make fmt && make build-mac && make test && make lint`; open the PR against `dev` with DCO signoff; after merge, verify the endpoint on dev via curl (quickstart-style call) and record the verified request/response in `contracts/upstream-easycla-api.md`.
- [ ] T010a **Add `projectName` to the signature response** (research R6 decision, 2026-07-28): add a `projectName` string to `swagger/common/signature.yaml`, `make swagger`, and populate it in the `/v4/signatures/user/{userID}` converter from the already-loaded CLA group (`claGroup.ProjectName`, cf. `v2/signatures/service.go:399,403,417`). Bundle with the T007–T010 PR. Closes the SS `projectName`-shows-ID gap; SS then maps `EasyClaSignature.projectName` straight through in `toMyClaAgreement` (drop the `projectID` fallback).

### SS server plumbing (parallel with T007–T010)

- [ ] T011 [P] ~~Add SS env config `CLA_SERVICE_BASE_URL`~~ **DROPPED (T016 impl): no new env needed** — base URL derived as `${API_GW_AUDIENCE}/cla-service` (research R5 impl note). Remaining T011 scope is only the T006 flag name in LD flag constants if the repo centralizes them.
- [x] T012 [P] Create SS types in `server/types/cla.types.ts` — upstream response shapes + `ResolvedClaIdentity { lfUsername, emails[], githubIds[], easyclaUserIds[], githubLinked }` — and the shared UI interfaces `MyClaAgreement`/`MyClasResponse`/`PdfUrlResponse` in `packages/shared/src/interfaces/cla.interface.ts` (+ barrel). **Done** (`feat/easycla-my-clas-server`). Note: upstream shape verified against swagger, not T004 fixtures (T004 still pending for field-availability confirmation).

**Checkpoint**: by-identity endpoint live on dev; SS types + config in place — US1 implementation can start (username-only resolution can proceed against `/v3/users/username/{userName}` even if T010 is still in review).

---

## Phase 3: User Story 1 — My CLAs list + ICLA PDF download (Priority: P1) 🎯 MVP

**Goal**: logged-in user sees all their signed ICLAs (any status, labeled) and currently valid ECLAs under Me lens `/me/clas`, downloads ICLA PDFs via short-lived links, sees the GitHub-link CTA when unlinked, and gets a clear empty state. Read-only; signing links out to Contributor Console.

**Independent Test**: quickstart.md verification table — dev user with ICLA + ECLA sees both correctly classified; PDF opens; CLA-less user sees empty state; cross-user PDF request → 404.

### Tests first (write before implementation; must fail initially)

- [ ] T013 [P] [US1] Unit tests for pure logic in `server/services/__tests__/cla.service.spec.ts` (repo test convention): ICLA/ECLA classification per data-model.md rule (`type=cla & referenceType=user`, `cclaCompanyID` set ⇒ ECLA); status derivation per R6 (signed+approved ⇒ valid; signed+!approved ⇒ "no longer valid"; !signed excluded/labeled; superseded only if document version available per T004); multi-record merge + dedupe by `signatureID`; ECLA-invalid filtering; sort by `signedOn` desc. Use T004 fixtures.
- [ ] T014 [P] [US1] Unit tests for identity resolution in the same suite: union of username/email/githubId matches; GitHub-ID parsing per T003 outcome (fixture test for the prefixed-vs-bare form); no-match ⇒ empty `easyclaUserIds` + `unmatched=true`; no linked GitHub identity ⇒ `githubLinked=false`. Mock the upstream client and (per T002 outcome) the token source.
- [ ] T015 [P] [US1] Route/controller tests in `server/routes/__tests__/clas.route.spec.ts`: 401 without session; userID derived **only** from session (a request supplying user IDs is ignored); `GET /api/me/clas/:signatureId/pdf-url` returns **404** (never 403) for unknown, not-owned, and ECLA signature IDs; 502 `{ code: "UPSTREAM_ERROR" }` on upstream failure; flag off ⇒ 404 on both routes.

### Server implementation

- [x] T016 [US1] Implement `server/services/cla.service.ts`: uses `gatewayFetch` + `req.apiGatewayToken` (not a bespoke client — research R3/R5 impl note); `resolveIdentity` does **username-only** resolution via `/v3/users/username/{userName}` for now (GitHub-link presence computed for the T023 CTA; three-key union via `GET /v4/users/by-identity` is a marked TODO awaiting T010), `getUserSignatures` (paginate `pageSize`/`lastKeyScanned`, merge), `getSignedDocumentUrl`. Exported pure helpers (`isIcla`/`isEcla`/`deriveStatus`/`toMyClaAgreement`/`buildAgreements`/`normalizeGithubId`) — ICLA/ECLA via `claType` (R6 finding), fallback heuristic retained. **Done.** ⚠ still pending for full T016: three-key union + verified-email/GitHub-ID matching once T010's endpoint is on dev.
- [x] T017 [US1] Implement `server/controllers/clas.controller.ts`: session → `getMyClas` → `MyClasResponse` with `identity.{matchedUserIds, unmatched, githubLinked}`; PDF handler re-verifies the requested `signatureID` is an owned ICLA in the session's agreement set before calling upstream (authz boundary per R3), returns **404 (never 403)** for unknown/not-owned/ECLA. **Done.**
- [x] T018 [US1] Implement `server/routes/clas.route.ts` (`GET /api/me/clas`, `GET /api/me/clas/:signatureId/pdf-url`), mounted at `/api/me` in `server.ts`. **No server-side flag gate** — gating is Angular-side (T020), per repo convention (R5 impl note). **Done.** Passes tsc + eslint.
- [ ] T019 [US1] Add identity-resolution telemetry in the controller/service per R2: counters for (a) no-match users, (b) multi-record users, (c) GitHub-link-only resolutions — using SS's existing logging/metrics convention. These are the M2 launch-gate metrics (SC-002); note the emitted metric names in `research.md` R2.

### Angular module

- [x] T020 [P] [US1] **Placement corrected**: My CLAs is a **Profile tab**, not a Me-lens sidebar item — created child route `profile/clas` in `modules/profile/profile.routes.ts` gated by `myClasEnabledGuard` (CanMatch, flag `my-clas-enabled`), and the profile-layout tab-row conditionally appends the "My CLAs" tab when the flag is on. Flag constant `MY_CLAS_ENABLED_FLAG`. **Done.** (No `modules/my-clas`/`me/clas` route; no main-layout sidebar change.)
- [x] T021 [P] [US1] Create `modules/profile/clas/agreement-card/` — per-agreement row: kind badge, status label (ICLA), signed date, **Download PDF** only when `pdfAvailable` (opens a blank tab synchronously on click then sets the resolved short-lived URL — popup-safe, never prefetched), ECLA rows show "Covered by Corporate CLA (CCLA)". **Done.**
- [x] T022 [US1] Create `modules/profile/clas/profile-clas.component.ts|html`: fetch `/api/me/clas`; ICLA and ECLA sections; empty state (AS3); "Sign a new CLA" link-out to the Contributor Console (env `contributorConsole`, AS4); "history may be incomplete" hint when `identity.unmatched`; error state with retry (502). **Done.** Note: `MyClasService` sorts server-side (`signedOn` desc in `buildAgreements`).
- [~] T023 [US1] GitHub-link CTA (FR-005a): **CTA implemented in T022** (`showGithubCta` when `githubLinked=false` or `unmatched`, links into `/profile/identities`). Remaining: component tests for CTA visibility rules (folded into T024).
- [ ] T024 [P] [US1] Component unit tests for `my-clas.component` and `agreement-card` per repo test convention: renders ICLA with download + ECLA without (AS1/AS2), empty state (AS3), no signing affordances (AS4), unmatched hint, error state.

### Integration

- [ ] T025 [US1] E2E happy path per repo norms (verify harness first — plan flags it): dev user with ICLA + ECLA fixture data, flag on → list renders both correctly; PDF click yields a working S3 URL; CLA-less user → empty state. Covers quickstart rows AS1–AS4.
- [ ] T026 [US1] Multi-record aggregation verification on dev (quickstart FR-005 row): test user with a pre-LF-login GitHub-only EasyCLA record + a linked record shows the union; resolution-via-GitHub-only telemetry increments. Create the fixture user via the dev Contributor Console flow if none exists.

**Checkpoint**: US1 fully functional behind the flag on dev — independently testable via quickstart.md.

---

## Phase 4: Polish & cross-cutting

- [ ] T027 [P] SC-001 sampling script (location per convention — SS repo `scripts/` or easycla `utils/`; shell or TS): for N dev users, compare SS `GET /api/me/clas` output against direct EasyCLA queries (`by-identity` + `signatures/user/{userID}`); report mismatches. Run and record the result in `specs/001-easycla-ss-integration-fable/m1-my-cla/research.md` (SC-001: 100% parity, PDF success ≥ 99%).
- [ ] T028 [P] File the upstream hardening observation (out of M1 scope, per R3/contracts): v4 `GetUserSignatures` lacks a per-user ownership check — GitHub issue or team ticket referencing `v2/signatures/handlers.go`, cross-linking the feasibility memo §3.
- [ ] T029 Run the full quickstart.md validation table end-to-end on dev; fix gaps; update quickstart.md with the final flag name, env vars, and by-identity endpoint usage.
- [ ] T030 Docs sync: update `specs/001-easycla-ss-integration-fable/m1-my-cla/` (research R2/R3/R6 spike outcomes if not already recorded, contracts with final shapes) and mark M1 status in `specs/001-easycla-ss-integration-fable/01-milestone-read-only-me-lens-fable.md`. Commits DCO-signed throughout.

---

## Dependencies & execution order

- **Phase 1 (T001–T006)**: T001 → T002 (same runbook, sequential); T003/T004/T005 parallel; T006 anytime (needs EasyCLA-team response — start early).
- **Phase 2**: T007 → T008 → T009 → T010 (swagger-first chain, CLA repo); T011/T012 parallel with all of it (SS repo). T002 gates T016's token wiring; T004 gates T013's fixtures.
- **Phase 3 (US1)**: T013/T014/T015 first (tests, parallel) → T016 → T017 → T018 → T019 (server chain); T020/T021 parallel with the server chain once T012 exists → T022 → T023 → T024; T025/T026 last (need dev deployment of T010's endpoint for full resolution; T025 can run with username-only resolution earlier).
- **Phase 4**: after US1 checkpoint; T027/T028 parallel.

Cross-repo note: only T007–T010 (+T028) touch `cla-backend-go`; everything else is `lfx-self-serve`. The SS work does not hard-block on T010 — username-only resolution via `/v3/users/username/{userName}` is the interim path, with full three-key resolution switched on when the endpoint lands.

## Parallel example (after Phase 2)

```text
# Tests together:
T013 cla.service classification/status tests
T014 identity-resolution tests
T015 route authz tests
# Then server chain (T016→T019) while UI scaffolding proceeds:
T020 routes + sidebar    T021 agreement-card
```

## Implementation strategy

MVP = all of Phase 3 (single story). Suggested slicing for early demo value: land T016–T018 + T020–T022 with username-only resolution (works for post-LF-login users) behind the flag, then add GitHub/email union + CTA (T023, full T016) when the by-identity endpoint is on dev. **Stop and validate** with quickstart.md before Phase 4; SC-001 sampling (T027) is the exit evidence for PM.
