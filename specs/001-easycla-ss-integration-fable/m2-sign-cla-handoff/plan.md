# Implementation Plan: Milestone 2 — My CLAs actions: proactive sign entry, invalidation, status

**Branch**: `docs/easycla-ss-m2-speckit` | **Date**: 2026-08-04 | **Spec**: [spec.md](spec.md)
**Input**: M2 feature spec [spec.md](spec.md) (extracted from the program spec's User Story 2, revised 2026-08-04 — [../spec.md](../spec.md)), [../02-milestone-sign-icla-fable.md](../02-milestone-sign-icla-fable.md), and the [M2 UI mockup Final/v16](https://github.com/linuxfoundation/easyclav2-migration-planning/blob/main/Mockups/M2/EasyCLA_MyCLAs_Full_Prototype_Final.html). M1 is a completed dependency; M3–M6 are roadmap context only.

## Summary

Extend M1's **My CLAs** page (Me lens) per the mockup: (1) a "Sign CLA" modal search (project / CLA group / repo source / pasted repo link) that resolves the user's EasyCLA `userID` server-side (existing `GET /v4/user-from-token`) and — after an account-authorization step for the platform they'll contribute with — redirects to the Contributor Console's existing decision-screen URL (`{console}/#/cla/project/{claGroupID}/user/{userID}`); (2) per-row **CLA invalidation** with confirmation modals (ICLA via the existing `invalidateICLA` endpoint with SS-side ownership enforcement; ECLA needs a new backend endpoint), blocked server-side during impersonation via SS's existing impersonation-readonly middleware; (3) a **status** column (Valid / Needs attention / Invalidated) with a "Request approval →" deep link into the Console for ECLAs that no longer match Approved List criteria.

The ICLA/ECLA choice, its legal guidance, and all signing logic stay in the Console. SS makes no signing-initiation calls and never touches DocuSign; nothing is cut over or retired.

**Constraints**: simple and straightforward (reuse the Console's deep-link entry, existing endpoints, and existing SS middleware); independently deliverable in ~2 weeks — see Complexity Tracking for what threatens that.

## Technical Context

**Language/Version**: TypeScript (Angular 20.3 frontend + Node 22 / Express 4 SSR server) in `lfx-self-serve`; Go 1.25 in `cla-backend-go` for the ECLA-invalidation endpoint (and possibly status evaluation / no-PR ICLA shape, per clarify outcomes).
**Primary Dependencies**: `apps/lfx-one` (PrimeNG 20, standalone components, signals), Express server services (`MicroserviceProxyService` / `ApiClientService` pattern), existing impersonation-readonly middleware, LaunchDarkly feature flags, EasyCLA v2/v4 REST APIs via lfx-gateway. Extends M1's `my-clas` module and `cla` server module.
**Storage**: none in SS (stateless). EasyCLA DynamoDB + S3 untouched except via existing/new EasyCLA endpoints.
**Testing**: lfx-self-serve conventions — Jest/Karma unit tests, server route tests (incl. impersonation-block and ownership-enforcement paths), Cypress/E2E per repo norms; `cla-backend-go` unit tests for any new endpoint.
**Target Platform**: LFX One web app (SSR, Kubernetes via ArgoCD).
**Project Type**: web application (feature surface in existing monorepo) + small swagger-first backend slice.
**Performance Goals**: My CLAs interactive < 2s p95 including status; sign-entry search results < 1s p95; hand-off redirect adds no perceptible latency.
**Constraints**: the only *contributor-facing* writes from SS are invalidation calls (FR-007/FR-008) — no signing-initiation calls (FR-005); the one implicit write is the lookup-or-create side effect of `GET /v4/user-from-token` (FR-003) when resolving the EasyCLA user record, plus any identity-linking side effects of the account-authorization step (FR-004); server-side identity derivation and ownership enforcement (never trust client-supplied user IDs); invalidation blocked during impersonation (FR-009); PR-check remediation link untouched (FR-006); feature-flagged dark launch; 2-week delivery budget.
**Scale/Scope**: all LFX contributors; extends 1 existing Me-lens page, ~3–4 server routes (CLA-Group search, `userID` resolution/hand-off, ICLA invalidate, ECLA invalidate), 1 new upstream endpoint (ECLA invalidation) + possibly a status/listing extension per clarify outcomes.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`.specify/memory/constitution.md` is the unratified template — no project-specific gates exist. Default gates applied:

- **Simplicity**: no new services, storage, or state; extends M1's page and module; hand-off is the Console's existing deep link; invalidation reuses the existing ICLA endpoint and the existing impersonation middleware. The ECLA endpoint is the one genuinely new backend piece. PASS.
- **Security**: `userID` derived server-side; `invalidateICLA` has no upstream ownership check, so the SS server enforces self-only invalidation and blocks impersonated sessions (existing middleware); no arbitrary-user lookup exposed. PASS with these requirements carried into contracts.
- **No speculative work**: native SS signing, SSM cutover, per-platform splits, org/repo selection, sign-type UI, and Approved List management are all explicitly excluded. PASS.

**Post-Phase-1 re-check**: pending — re-run after `/speckit.clarify` resolves the five open questions in [spec.md](spec.md).

## Project Structure

### Documentation (this feature)

```text
specs/001-easycla-ss-integration-fable/          # program level (review docs, in PR #5132)
├── m1-my-cla/                                   # M1 (already on dev)
└── m2-sign-cla-handoff/                         # this milestone's feature directory
    ├── spec.md              # extracted M2 spec (US2 / FR-001…FR-011, revised 2026-08-04)
    ├── plan.md              # This file
    ├── research.md          # Phase 0 output — TBD by /speckit.plan (resolves the 5 open questions)
    ├── data-model.md        # Phase 1 output — TBD
    ├── quickstart.md        # Phase 1 output — TBD
    ├── contracts/           # TBD — hand-off URL, CLA-Group search, invalidation (incl. new ECLA endpoint)
    └── tasks.md             # Phase 2 output (/speckit.tasks — not created here)
```

### Source Code

Primary repo: `linuxfoundation/lfx-self-serve`

```text
apps/lfx-one/src/
├── app/modules/my-clas/                         # EXTEND M1's page: "Sign CLA" modal, Invalidate actions + modals, status column
├── app/app.routes.ts                            # EDIT only if a sub-route is needed (flag-guarded)
└── server/
    ├── routes/clas.route.ts                     # EXTEND: CLA-Group search; userID resolution + hand-off; invalidate routes (impersonation-blocked)
    ├── controllers/clas.controller.ts           # EXTEND: hand-off orchestration; ownership-enforced invalidation
    └── services/cla.service.ts                  # EXTEND: CLA-Group search client; invalidateICLA/ECLA clients; status mapping
```

Secondary repo: `linuxfoundation/easycla` `cla-backend-go` — new self-service ECLA-invalidation endpoint, swagger-first (`swagger/cla.v2.yaml` → `make swagger` → handler/service), no schema changes expected. Conditional (per clarify): status-evaluation extension to `GET /v4/my-clas`; no-PR ICLA request shape in `v2/sign` (+ matching `easycla-contributor-console` tweak).

**Structure Decision**: everything lands in M1's existing `my-clas` module and `cla` server seam — the mockup is explicitly an extension of the M1 page, not a new surface. The Console's decision screen owns everything after CLA-Group selection.

## Complexity Tracking

No constitution violations. Schedule risks against the 2-week budget, in order:

1. **ECLA-invalidation endpoint** (spec open question 2) — the one guaranteed new backend piece; semantics settled (per-signature flag flip, no Approved List mutation, notifications, backend ownership check) — execution risk only.
2. **Proactive-ICLA gap** (open question 4) — Console + backend delta; Gerrit precedent bounds it; the only other backend deliverable competing with 1.
3. **Account-authorization mechanics** (open question 1) — retired: Gerrit needs no step (same LF SSO), GitLab is conditional on SS shipping GitLab linking (config flip, M2 doesn't block), GitHub reuses M1's linking + picker; first-timer enrichment is a call to the existing v1 `updateUser` API.
4. **Status evaluation** (open question 3) — retired: `GET /v4/my-clas` already computes the coverage evaluation per ECLA row; M2 exposes it as a status field instead of collapsing it into `Valid`.

If the budget forces a cut, the mockup's pieces degrade independently: sign entry, ICLA invalidation, and the status column ride existing endpoints/evaluations; ECLA invalidation (with FR-008a templates) is the deferrable slice — decide at `/speckit.plan`.
