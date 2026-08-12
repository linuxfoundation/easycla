# Implementation Plan: Milestone 2 — My CLAs actions: proactive sign entry, invalidation, status

**Branch**: `docs/easycla-ss-m2-speckit` | **Date**: 2026-08-04 | **Spec**: [spec.md](spec.md)
**Input**: M2 feature spec [spec.md](spec.md) (extracted from the program spec's User Story 2, revised 2026-08-04 — [../spec.md](../spec.md)), [../02-milestone-sign-icla-fable.md](../02-milestone-sign-icla-fable.md), and the [M2 UI mockup Final/v16](https://github.com/linuxfoundation/easyclav2-migration-planning/blob/main/Mockups/M2/EasyCLA_MyCLAs_Full_Prototype_Final.html). M1 is a completed dependency; M3–M6 are roadmap context only.

## Summary

Extend M1's **My CLAs** page (the Profile CLAs tab — see Source Code) per the mockup: (1) a "Sign CLA" modal backed by a **new four-source search endpoint** (project name / CLA-group name / org name with repo-source provenance / pasted repo URL — FR-001, FR-001a) that resolves the user's EasyCLA `userID` server-side and, after a **read-only identity pre-flight** (FR-004 — SS writes no identity; the Console's existing GitHub OAuth binds when needed), redirects to the Contributor Console's existing decision-screen URL (`{console}/#/cla/project/{claGroupID}/user/{userID}`); (2) per-row **CLA invalidation** with confirmation modals (ICLA via a revised `invalidateICLA` flow — signature-ID-targeted, actor-aware side effects, ownership enforced upstream via `authorizeIdentity`; ECLA needs a new backend endpoint), blocked server-side during impersonation via SS's existing impersonation-readonly middleware; (3) a **status** column (Valid / Needs attention / Invalidated) driven by **three independent response fields** rather than M1's collapsed boolean (FR-010a), with a "Request approval →" deep link into the Console for ECLAs that no longer match Approved List criteria, and Download PDF active on ICLA rows / disabled-with-tooltip on ECLA rows (FR-011a).

The ICLA/ECLA choice, its legal guidance, and all signing logic stay in the Console. SS makes no signing-initiation calls and never touches DocuSign; nothing is cut over or retired.

**Constraints**: simple and straightforward (reuse the Console's deep-link entry, existing endpoints, and existing SS middleware); independently deliverable in ~3 weeks — see Complexity Tracking for what threatens that.

## Technical Context

**Language/Version**: TypeScript (Angular 20.3 frontend + Node 22 / Express 4 SSR server) in `lfx-self-serve`; Go 1.25 in `cla-backend-go` for the ECLA-invalidation endpoint (and possibly status evaluation / no-PR ICLA shape, per clarify outcomes).
**Primary Dependencies**: `apps/lfx-one` (PrimeNG 20, standalone components, signals), Express server services (`MicroserviceProxyService` / `ApiClientService` pattern), existing impersonation-readonly middleware, LaunchDarkly feature flags, EasyCLA v2/v4 REST APIs via lfx-gateway. Extends M1's `my-clas` module and `cla` server module.
**Storage**: none in SS (stateless). EasyCLA DynamoDB + S3 untouched except via existing/new EasyCLA endpoints.
**Testing**: lfx-self-serve conventions — Jest/Karma unit tests, server route tests (incl. impersonation-block and ownership-enforcement paths), Cypress/E2E per repo norms; `cla-backend-go` unit tests for any new endpoint.
**Target Platform**: LFX One web app (SSR, Kubernetes via ArgoCD).
**Project Type**: web application (feature surface in existing monorepo) + small swagger-first backend slice.
**Performance Goals**: My CLAs interactive < 2s p95 including status; **sign-entry search < 300 ms p95 server time** (per-keystroke expectation, with ~200 ms client debounce and a server-side result cap — FR-001a; tightened from the earlier < 1s figure, which predated the per-keystroke requirement); hand-off redirect adds no perceptible latency. The search budget MUST be met without a bespoke cache or new datastore in M2 — see spec Performance assumptions for the measure-first sequence and the unverified cardinality assumption it rests on.
**Constraints**: the only *contributor-facing* writes from SS are invalidation calls (FR-007/FR-008) — no signing-initiation calls (FR-005); the one implicit write is the lookup-or-create side effect of `GET /v4/user-from-token` (FR-003) when resolving the EasyCLA user record, plus any identity-linking side effects of the account-authorization step (FR-004); server-side identity derivation and ownership enforcement (never trust client-supplied user IDs); invalidation blocked during impersonation (FR-009); PR-check remediation link untouched (FR-006); feature-flagged dark launch; 3-week delivery budget.
**Scale/Scope**: all LFX contributors; extends 1 existing page (M1's Profile CLAs tab), ~4–5 server routes (search, `userID` resolution/hand-off + identity pre-flight, ICLA invalidate, ECLA invalidate), and **2 new upstream endpoints** (four-source CLA-Group search; ECLA invalidation) plus **2 revisions to existing ones** (`GET /v4/my-clas` three-input status per FR-010a; signature-ID-targeted, actor-aware `invalidateICLA`). No new SS-owned identity-binding operation (FR-003/FR-004) and no new datastore or cache (FR-001a).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

No Spec Kit scaffolding exists in either repo (verified 2026-08-11: there is no `.specify/` directory in `easycla` or `lfx-self-serve` — earlier references in this plan and M1's to "the unratified `.specify/memory/constitution.md` template" were inaccurate). M1's artifacts were authored in Spec-Kit *shape* by hand; M2 follows that convention. Committing `.specify/` scaffolding is a separate change and is not a prerequisite for M2. Default gates applied:

- **Simplicity**: no new services, storage, or state; extends M1's existing Profile CLAs tab and `cla` server seam; hand-off is the Console's existing deep link; invalidation reuses the existing impersonation middleware and revises (not replaces) the existing ICLA flow. The ECLA endpoint is the one genuinely new backend piece; the ICLA revision and no-PR request shape are bounded deltas to existing flows. PASS.
- **Security**: `userID` derived server-side; the v4 API is independently reachable by any authenticated caller, so backend signature-ownership enforcement is **mandatory** for both the revised ICLA endpoint and the new ECLA endpoint (the revised call still takes a target signature/user identifier — bypassing the SS route must not permit invalidating another contributor's agreement). **This is a reuse, not a new mechanism** (verified): M1 already built the ownership boundary in `v2/my_clas/service.go` — `authorizeIdentity` (`:383`) verifies every requested identity key against the caller's own EasyCLA records *and* their platform user-service identities, dropping unverifiable keys into `skippedIdentities`. M1's SS controller records the resulting contract: "EasyCLA re-verifies each key belongs to the caller and owns the signature, so the upstream endpoint — not this controller — is the ownership authorization boundary." Both M2 invalidation endpoints MUST resolve ownership through that same helper rather than introducing a parallel check. The SS server retains self-only enforcement and impersonation blocking (existing middleware) as defense in depth; no arbitrary-user lookup exposed. PASS with these requirements carried into contracts.
- **No speculative work**: native SS signing, SSM cutover, per-platform splits, org/repo selection, sign-type UI, and Approved List management are all explicitly excluded. PASS.

**Post-Phase-1 re-check**: **all five spec open questions are resolved** (1 and 5 on 2026-08-12; 2 and 4 via [research.md](research.md); 3 on 2026-08-08). What remains before Phase 1 is a **measurement, not a clarification** — the CLA-group cardinality behind FR-001a. Two scope decisions taken on 2026-08-12 keep the Simplicity gate passing where it would otherwise have degraded: **no SS-owned identity-binding operation** (read-only pre-flight instead — FR-003/FR-004, risk 3) and **no bespoke search cache or new datastore** (measure-first, OpenSearch as the escalation if needed — FR-001a, risk 5). Both are recorded as PASS-preserving choices; reversing either reopens this gate.

## Project Structure

### Documentation (this feature)

```text
specs/001-easycla-ss-integration-fable/          # program level (review docs, in PR #5132)
├── m1-my-cla/                                   # M1 (already on dev)
└── m2-sign-cla-handoff/                         # this milestone's feature directory
    ├── spec.md              # extracted M2 spec (US2 / FR-001…FR-011, revised 2026-08-04)
    ├── plan.md              # This file
    ├── research.md          # Phase 0 output — TBD by /speckit.plan (confirms remaining open items)
    ├── data-model.md        # Phase 1 output — TBD
    ├── quickstart.md        # Phase 1 output — TBD
    ├── contracts/           # TBD — hand-off URL, CLA-Group search, invalidation (incl. new ECLA endpoint)
    └── tasks.md             # Phase 2 output (/speckit.tasks — not created here)
```

### Source Code

Primary repo: `linuxfoundation/lfx-self-serve`

Paths verified against `lfx-self-serve@main` on 2026-08-11. **M1 did not ship at the path its own plan predicted**: there is no `app/modules/my-clas/` module. My CLAs shipped as a **Profile tab** at route `/profile/clas`, dark-launched behind the `my-clas-enabled` flag via `canMatch` (`app/modules/profile/profile.routes.ts`). M2 extends that tab.

```text
apps/lfx-one/src/
├── app/modules/profile/clas/                    # EXTEND M1's tab (profile-clas.component.ts|html):
│                                                #   "Sign CLA" modal, Invalidate actions + modals, status column
├── app/modules/profile/profile.routes.ts        # EDIT only if a sub-route is needed (already flag-guarded via myClasEnabledGuard)
├── app/shared/services/my-clas.service.ts       # EXTEND: search, hand-off, invalidation client calls
└── server/
    ├── routes/clas.route.ts                     # EXTEND: CLA-Group search; userID resolution + hand-off; invalidate routes (impersonation-blocked)
    ├── controllers/clas.controller.ts           # EXTEND: hand-off orchestration; ownership-enforced invalidation
    ├── services/cla.service.ts                  # EXTEND: CLA-Group search client; invalidateICLA/ECLA clients; status mapping
    └── types/cla.types.ts                       # EXTEND: upstream types for search/invalidation/status

packages/shared/src/
├── interfaces/cla.interface.ts                  # EXTEND: MyClaAgreement gains the FR-010 status field
├── utils/cla-view.utils.ts                      # EXTEND if row-level view logic is needed
└── constants/feature-flags.constants.ts         # ADD the M2 flag alongside MY_CLAS_ENABLED_FLAG
```

Two M1 implementation details M2 must not regress (both in `server/services/cla.service.ts`): `getMyClas` currently **filters to `valid === true`**, dropping every invalid row — FR-010's Invalidated/Needs-attention statuses require relaxing that filter, not just adding a column; and `toMyClaAgreement` maps `status` from the collapsed boolean (`cla.valid ? 'valid' : 'inactive'`), which is the SS-side half of the FR-010 change. The `identityQuery` gateway padding stopgap (lfx-gateway#114) applies to any new multi-valued identity query M2 adds.

Secondary repo: `linuxfoundation/easycla` `cla-backend-go` — new self-service ECLA-invalidation endpoint, swagger-first (`swagger/cla.v2.yaml` → `make swagger` → handler/service). Schema impact is limited to **additive** attributes on the signatures table per FR-008b (invalidation timestamp + reason/actor, the latter doubling as FR-008's durable self-exclusion marker) — no migrations, but the attributes are read by v1/v2 code paths, so consumers must tolerate empty values on pre-M2 records. Conditional (per clarify): status-evaluation extension to `GET /v4/my-clas`; no-PR ICLA request shape in `v2/sign` (+ matching `easycla-contributor-console` tweak).

**Structure Decision**: everything lands in M1's existing Profile CLAs tab (`app/modules/profile/clas/`) and the `cla` server seam — the mockup is explicitly an extension of the M1 page, not a new surface. The Console's decision screen owns everything after CLA-Group selection.

## Complexity Tracking

No constitution violations. Schedule risks against the 3-week budget, in order:

1. **ECLA-invalidation endpoint** (spec open question 2) — the one guaranteed new backend piece; semantics settled (**durable self-exclusion** the Approved-List re-validation and PR gating honor — not a bare `signature_approved` flip, which `auto_create_ecla` would silently reactivate per FR-008; no Approved List mutation; notifications; backend ownership check) — the self-exclusion mechanism is part of this deliverable. **Mechanism spiked** ([research.md](research.md) Spike 2): `UserIsApproved` takes only `(user, cclaSignature)` (`signatures/service.go:1607`) and cannot see the employee signature, so the marker MUST be honored at the three **call sites** instead — the re-validation guard (`:898`), `ProcessEmployeeSignature`'s approval branch (`:1578`), and `eclaCoveredByCurrentApprovalList` (`v2/my_clas/service.go:214`, which also feeds FR-010). Threading the signature into `UserIsApproved` was considered and rejected (interface-wide change; conflates two concepts). Still the largest single M2 backend item: endpoint + 3 one-condition guards + 2 notification templates + additive attributes.
2. **Proactive-ICLA gap** (open question 4) — **spiked and located; smaller than feared** (see [research.md](research.md) Spike 1). Two independent dependencies, one per repo: the Console's `findActiveSignature()` hard-stop (`individual-dashboard.component.ts:49-66`) and the backend's PR-metadata requirement (`v2/sign/service.go:2890,2903`). Fix is a widened branch condition in each plus an explicit proactive signal in the request shape — no DocuSign/schema/webhook change. **Required for the GitHub sign-entry path**, but the Gerrit-only fallback now saves little; recommend keeping GitHub in M2. Caveat the Gerrit precedent does *not* cover: `acl = github:{user.GithubID}` (`:1433`) must still be set, so the GitHub-ID binding must land before signing.
2a. **ICLA-invalidation revision** (spec FR-007, verified) — the existing endpoint must become signature-ID-targeted and actor-/reason-aware (self-service email, correct event data); small but real backend delta alongside 1–2.
3. **Account-authorization mechanics** (open question 1) — **RETIRED (2026-08-12) by a scope decision, not by new code.** The remaining item was a backend-owned, ownership-checked identity-binding operation covering bind-to-unbound-record *and* create-record-bound-to-GitHub-ID. **M2 builds neither**: SS runs a read-only pre-flight against `GET /v4/my-clas/identities` (`getMyIdentities`, `v2/my_clas/service.go:301`), gates per repo-source type, and delegates binding to the Console's existing GitHub OAuth when an identity is missing (spec FR-003/FR-004). Rationale: `user.GithubID` is the merge-gating signature ACL (`v2/sign/service.go:1433`), and a second writer into it risks two systems disagreeing about whose GitHub ID it is — the hazard commit `d0a4f81e0` on this branch already had to guard. The `updateUser`/`user-from-token` overwrite analysis previously tracked here is therefore **moot for M2** (retained in spec open question 1 as the record of why the write path was rejected). Residual work: two conditionals plus a picker when several GitHub identities are linked. Identity types verified compatible — numeric GitHub IDs on both sides (`users/repository.go:719-728`; `cla.service.ts:193-196`). Deferred to M3: reconciling a GitHub ID bound to a different EasyCLA record (log-only in M2).
4. **Status evaluation** (open question 3) — **larger than previously tracked, and now the joint-largest backend item with risk 1.** The upstream evaluation exists, but the response shape does not: `row.Valid = sig.SignatureApproved && covered` (`v2/my_clas/service.go:218`) collapses approved-ness and coverage into one boolean, making the mockup's **Needs attention** (approved ∧ ¬covered) unrepresentable; and distinguishing self- from manager-invalidated rows needs a third input, FR-008b's reason/actor. Spec FR-010a therefore requires **three independent fields** on `GET /v4/my-clas`, not one. This couples risk 4 to risk 1 — the invalidation-provenance field is the same marker the ECLA self-exclusion relies on, so the two should be designed together, not sequenced apart. **SS-side delta also confirmed** (2026-08-11): M1's BFF filters out every non-valid row and derives `status` from the boolean, both in `server/services/cla.service.ts` — see Source Code.
5. **Four-source search endpoint** (open question 5) — **new risk, previously mis-scoped as endpoint reuse.** No existing endpoint fits: both CLA-group listings are foundation/project-SFID-scoped (`listClaGroupsUnderFoundation`, `listFoundationClaGroups`), `GetCLAGroupByName` is exact-match-only via the `project-name-lower-search-index` GSI, and the mockup searches **four** sources (project, CLA group, org-with-provenance, repo URL) with a per-keystroke expectation. Includes a latent bug to fix: `GetCLAGroups` logs `SearchField`/`SearchTerm`/`FullMatch` (`project/repository/repository.go:529-533`) then builds a projection with **no filter** (`:538`) — an unfiltered full-table `Scan` whose search parameters do nothing, meaning search on this table has never been measured. **Deliberately no cache or new datastore in M2** (spec FR-001a + Performance assumptions): serve from DynamoDB, fix the filter, measure p95; escalate to OpenSearch — not a bespoke cache — only if measurement demands it, because stale results here mean signing against the wrong CLA Group, invalidation spans four write paths, and Lambda gives per-container caches N staleness windows. **Blocking measurement**: CLA-group row count, unobtainable in the authoring session (no `lfproduct-*` AWS profile locally; `lfx-*` SSO tokens expired) — recorded as an explicit unverified assumption, not a finding.
**Not a risk** — the Console decision screen's deep-linkability was re-verified on 2026-08-11 and needs no change: the project fetch lives in the embedded `<app-project-title>` child (`project-title.component.ts:46-58`), not the container. An intermediate draft of [research.md](research.md) tracked this as a fifth risk in error; retracted.

If the budget forces a cut (revised 2026-08-12, now that risks 3–5 are correctly scoped):

- **Not cuttable**: the three-input status response (risk 4) — FR-010/FR-011 both depend on it, and it shares the invalidation-provenance field with risk 1, so cutting it strands the ECLA work too. Likewise the search endpoint (risk 5): without it the Sign CLA modal has nothing to search.
- **Cheapest real saving — narrow the search sources.** Ship project + CLA-group + org name in M2 and defer **repo-URL** matching (the source with by far the largest cardinality and the one most likely to force an indexing layer). The modal degrades gracefully: users search by name instead of pasting a link.
- **Next cheapest — defer ECLA invalidation** (with its FR-008a templates), still the cleanest severable slice, though it now costs the FR-008b provenance field that risk 4 wants; if cut, ship the field anyway.
- **No longer worth cutting**: the GitHub sign entry. Its no-PR delta (risk 2) is measured and small, and risk 3's binding work is now retired rather than deferred, so **narrowing to Gerrit-only saves almost nothing** — not recommended.
- **Do not "save time" by adding a cache** to hit the search budget (FR-001a): it trades a latency problem for a correctness problem on merge-gating data, and adds infrastructure this milestone does not otherwise need.
