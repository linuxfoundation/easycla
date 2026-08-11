# Implementation Plan: Milestone 2 — My CLAs actions: proactive sign entry, invalidation, status

**Branch**: `docs/easycla-ss-m2-speckit` | **Date**: 2026-08-04 | **Spec**: [spec.md](spec.md)
**Input**: M2 feature spec [spec.md](spec.md) (extracted from the program spec's User Story 2, revised 2026-08-04 — [../spec.md](../spec.md)), [../02-milestone-sign-icla-fable.md](../02-milestone-sign-icla-fable.md), and the [M2 UI mockup Final/v16](https://github.com/linuxfoundation/easyclav2-migration-planning/blob/main/Mockups/M2/EasyCLA_MyCLAs_Full_Prototype_Final.html). M1 is a completed dependency; M3–M6 are roadmap context only.

## Summary

Extend M1's **My CLAs** page (the Profile CLAs tab — see Source Code) per the mockup: (1) a "Sign CLA" modal search (project / CLA group / repo source / pasted repo link) that resolves the user's EasyCLA `userID` server-side (platform-aware per FR-003: linked GitHub ID first, `user-from-token` fallback + enrichment) and — after an account-authorization step for the platform they'll contribute with — redirects to the Contributor Console's existing decision-screen URL (`{console}/#/cla/project/{claGroupID}/user/{userID}`); (2) per-row **CLA invalidation** with confirmation modals (ICLA via a revised `invalidateICLA` flow — signature-ID-targeted, actor-aware side effects, SS-side ownership enforcement; ECLA needs a new backend endpoint), blocked server-side during impersonation via SS's existing impersonation-readonly middleware; (3) a **status** column (Valid / Needs attention / Invalidated) with a "Request approval →" deep link into the Console for ECLAs that no longer match Approved List criteria.

The ICLA/ECLA choice, its legal guidance, and all signing logic stay in the Console. SS makes no signing-initiation calls and never touches DocuSign; nothing is cut over or retired.

**Constraints**: simple and straightforward (reuse the Console's deep-link entry, existing endpoints, and existing SS middleware); independently deliverable in ~3 weeks — see Complexity Tracking for what threatens that.

## Technical Context

**Language/Version**: TypeScript (Angular 20.3 frontend + Node 22 / Express 4 SSR server) in `lfx-self-serve`; Go 1.25 in `cla-backend-go` for the ECLA-invalidation endpoint (and possibly status evaluation / no-PR ICLA shape, per clarify outcomes).
**Primary Dependencies**: `apps/lfx-one` (PrimeNG 20, standalone components, signals), Express server services (`MicroserviceProxyService` / `ApiClientService` pattern), existing impersonation-readonly middleware, LaunchDarkly feature flags, EasyCLA v2/v4 REST APIs via lfx-gateway. Extends M1's `my-clas` module and `cla` server module.
**Storage**: none in SS (stateless). EasyCLA DynamoDB + S3 untouched except via existing/new EasyCLA endpoints.
**Testing**: lfx-self-serve conventions — Jest/Karma unit tests, server route tests (incl. impersonation-block and ownership-enforcement paths), Cypress/E2E per repo norms; `cla-backend-go` unit tests for any new endpoint.
**Target Platform**: LFX One web app (SSR, Kubernetes via ArgoCD).
**Project Type**: web application (feature surface in existing monorepo) + small swagger-first backend slice.
**Performance Goals**: My CLAs interactive < 2s p95 including status; sign-entry search results < 1s p95; hand-off redirect adds no perceptible latency.
**Constraints**: the only *contributor-facing* writes from SS are invalidation calls (FR-007/FR-008) — no signing-initiation calls (FR-005); the one implicit write is the lookup-or-create side effect of `GET /v4/user-from-token` (FR-003) when resolving the EasyCLA user record, plus any identity-linking side effects of the account-authorization step (FR-004); server-side identity derivation and ownership enforcement (never trust client-supplied user IDs); invalidation blocked during impersonation (FR-009); PR-check remediation link untouched (FR-006); feature-flagged dark launch; 3-week delivery budget.
**Scale/Scope**: all LFX contributors; extends 1 existing Me-lens page, ~3–4 server routes (CLA-Group search, `userID` resolution/hand-off, ICLA invalidate, ECLA invalidate), 1 new upstream endpoint (ECLA invalidation) + possibly a status/listing extension per clarify outcomes.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

No Spec Kit scaffolding exists in either repo (verified 2026-08-11: there is no `.specify/` directory in `easycla` or `lfx-self-serve` — earlier references in this plan and M1's to "the unratified `.specify/memory/constitution.md` template" were inaccurate). M1's artifacts were authored in Spec-Kit *shape* by hand; M2 follows that convention. Committing `.specify/` scaffolding is a separate change and is not a prerequisite for M2. Default gates applied:

- **Simplicity**: no new services, storage, or state; extends M1's existing Profile CLAs tab and `cla` server seam; hand-off is the Console's existing deep link; invalidation reuses the existing impersonation middleware and revises (not replaces) the existing ICLA flow. The ECLA endpoint is the one genuinely new backend piece; the ICLA revision and no-PR request shape are bounded deltas to existing flows. PASS.
- **Security**: `userID` derived server-side; the v4 API is independently reachable by any authenticated caller, so backend signature-ownership enforcement is **mandatory** for both the revised ICLA endpoint and the new ECLA endpoint (the revised call still takes a target signature/user identifier — bypassing the SS route must not permit invalidating another contributor's agreement). **This is a reuse, not a new mechanism** (verified): M1 already built the ownership boundary in `v2/my_clas/service.go` — `authorizeIdentity` (`:383`) verifies every requested identity key against the caller's own EasyCLA records *and* their platform user-service identities, dropping unverifiable keys into `skippedIdentities`. M1's SS controller records the resulting contract: "EasyCLA re-verifies each key belongs to the caller and owns the signature, so the upstream endpoint — not this controller — is the ownership authorization boundary." Both M2 invalidation endpoints MUST resolve ownership through that same helper rather than introducing a parallel check. The SS server retains self-only enforcement and impersonation blocking (existing middleware) as defense in depth; no arbitrary-user lookup exposed. PASS with these requirements carried into contracts.
- **No speculative work**: native SS signing, SSM cutover, per-platform splits, org/repo selection, sign-type UI, and Approved List management are all explicitly excluded. PASS.

**Post-Phase-1 re-check**: pending — re-run after `/speckit.clarify` confirms the remaining open items in [spec.md](spec.md) (questions 1 and 3 resolved; 5 narrowed; 2 and 4 are execution deliverables).

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

1. **ECLA-invalidation endpoint** (spec open question 2) — the one guaranteed new backend piece; semantics settled (**durable self-exclusion** the Approved-List re-validation and PR gating honor — not a bare `signature_approved` flip, which `auto_create_ecla` would silently reactivate per FR-008; no Approved List mutation; notifications; backend ownership check) — the self-exclusion mechanism is part of this deliverable.
2. **Proactive-ICLA gap** (open question 4) — Console + backend delta; Gerrit precedent bounds it. **Required for the GitHub sign-entry path** — without it the picker hands GitHub ICLA signers into a flow they cannot complete.
2a. **ICLA-invalidation revision** (spec FR-007, verified) — the existing endpoint must become signature-ID-targeted and actor-/reason-aware (self-service email, correct event data); small but real backend delta alongside 1–2.
3. **Account-authorization mechanics** (open question 1) — mostly retired: Gerrit needs no step (same LF SSO), GitLab is conditional on SS shipping GitLab linking (config flip, M2 doesn't block), GitHub reuses M1's linking + picker. The multi-account **picker needs no new read endpoint** (verified 2026-08-11): `GET /v4/my-clas/identities` (`getMyIdentities`, `v2/my_clas/service.go:301`, handler `:97`) already returns the deduplicated `"<type>:<value>"` identity set — "exactly the identity set the My CLAs API authorizes a non-admin caller to search" — which is the picker's candidate list. **Not fully retired:** first-timer identity enrichment cannot use v1 `updateUser` — its `githubUsername` branch returns 400 for a previously-unseen username and, when a username exists, updates that record without verifying the linked identity belongs to the authenticated caller (`users/handlers.go:84-107`). A backend-owned, ownership-checked identity-binding operation is required. Scope is **larger than a pure enrichment primitive** (per PR #5144 review): `user-from-token` returns whatever record matches LF username/email — possibly one already bound to a *different* linked GitHub ID (`cmd/server.go:1004-1042`) — so blind enrichment would overwrite that binding. The operation must therefore cover both bind-to-unbound-record and **create-record-bound-to-GitHub-ID**, since `user-from-token` cannot produce the second record itself.
4. **Status evaluation** (open question 3) — retired: `GET /v4/my-clas` already computes the coverage evaluation per ECLA row; M2 exposes it as a status field instead of collapsing it into `Valid`.

If the budget forces a cut: the status column rides the existing evaluation; the **GitHub sign entry is not cuttable to "existing endpoints"** — it requires the no-PR ICLA delta (risk 2), so the only sign-entry fallback is narrowing to Gerrit-only; ICLA invalidation requires the revision in risk 2a; ECLA invalidation (with FR-008a templates) remains the cleanest deferrable slice — decide at `/speckit.plan`.
