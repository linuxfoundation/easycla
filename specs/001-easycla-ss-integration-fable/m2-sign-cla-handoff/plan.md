# Implementation Plan: Milestone 2 — Proactive CLA signing entry in Self Serve (hands off to Contributor Console)

**Branch**: `docs/easycla-ss-m2-speckit` | **Date**: 2026-08-04 | **Spec**: [spec.md](spec.md)
**Input**: M2 feature spec [spec.md](spec.md) (extracted from the program spec's User Story 2, revised 2026-08-04 — [../spec.md](../spec.md)) and [../02-milestone-sign-icla-fable.md](../02-milestone-sign-icla-fable.md). M1 is a completed dependency; M3–M6 are roadmap context only.

## Summary

Add a "Sign a CLA" page to LFX Self Serve's Me lens: a searchable list of CLA Groups. On selection, SS resolves the user's EasyCLA `userID` server-side (existing `GET /v4/user-from-token`, lookup-or-create) and redirects to the Contributor Console's existing decision-screen URL — `{console}/#/cla/project/{claGroupID}/user/{userID}`, the same shape the PR-check link uses. The ICLA/ECLA choice, its legal guidance, and all signing logic stay in the Console. SS makes no signing-initiation calls and never touches DocuSign; nothing is cut over or retired.

**Constraints**: simple and straightforward (no new services, state, or contracts — reuse the Console's existing deep-link entry); independently deliverable in ~2 weeks.

The SS-side build is deliberately thin: one page, one or two server routes, one redirect. The milestone's real design work is the two `/speckit.clarify` items in [spec.md](spec.md): **GitHub identity binding** (a proactive ICLA must land on a user record the PR check can match — recommended: require M1's GitHub-account linking for the ICLA path) and the **proactive-ICLA active-signature gap** (Console + backend assume PR-derived context on the GitHub ICLA path; the Gerrit path proves a no-PR shape already works). Both are scoped and evidenced in the spec's "Verified Console/backend facts" section; neither is resolved here.

## Technical Context

**Language/Version**: TypeScript (Angular 20.3 frontend + Node 22 / Express 4 SSR server) in `lfx-self-serve`. `cla-backend-go` / `easycla-contributor-console` changes only if the clarify items land inside M2 (planning decision).
**Primary Dependencies**: `apps/lfx-one` (PrimeNG 20, standalone components, signals), Express server services (`MicroserviceProxyService` / `ApiClientService` pattern), LaunchDarkly feature flags, EasyCLA v2/v4 REST APIs via lfx-gateway. Reuses M1's `cla` server module seam.
**Storage**: none in SS (stateless: list CLA Groups, resolve `userID`, redirect). EasyCLA DynamoDB + S3 untouched.
**Testing**: lfx-self-serve conventions — Jest/Karma unit tests for services/components, server route tests, Cypress/E2E per repo norms (verify exact harness during implementation).
**Target Platform**: LFX One web app (SSR, Kubernetes via ArgoCD).
**Project Type**: web application (feature surface in existing monorepo).
**Performance Goals**: picker interactive with CLA-Group list < 2s p95; hand-off redirect adds no perceptible latency.
**Constraints**: no EasyCLA writes and no signing-initiation calls from SS (FR-005); server-side identity derivation only (never trust client-supplied user IDs); PR-check remediation link untouched (FR-006); feature-flagged dark launch; 2-week delivery budget.
**Scale/Scope**: all LFX contributors; 1 new Me-lens page, ~1–2 server routes (CLA-Group listing + `userID` resolution/hand-off), 0 new upstream endpoints unless the CLA-Group listing question (FR-007) requires one.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`.specify/memory/constitution.md` is the unratified template — no project-specific gates exist. Default gates applied:

- **Simplicity**: no new services, storage, state, or contracts — the hand-off is the Console's existing deep-link URL, and the ICLA/ECLA choice is not duplicated in SS. PASS.
- **Security**: `userID` derived server-side from the session via `user-from-token`; SS makes no signing calls and exposes no arbitrary-user lookup; the Console/backend remain the enforcement point for signing. PASS.
- **No speculative work**: native SS signing, SSM cutover, per-platform splits, org/repo selection, and sign-type UI are all explicitly excluded. PASS.

**Post-Phase-1 re-check**: pending — re-run after `/speckit.clarify` resolves the GitHub-identity-binding and active-signature questions (see Open questions in [spec.md](spec.md)).

## Project Structure

### Documentation (this feature)

```text
specs/001-easycla-ss-integration-fable/          # program level (review docs, in PR #5132)
├── m1-my-cla/                                   # M1 (already on dev)
└── m2-sign-cla-handoff/                         # this milestone's feature directory
    ├── spec.md              # extracted M2 spec (US2 / FR-001…FR-008, revised 2026-08-04)
    ├── plan.md              # This file
    ├── research.md          # Phase 0 output — TBD by /speckit.plan (resolves the 3 open questions)
    ├── data-model.md        # Phase 1 output — TBD
    ├── quickstart.md        # Phase 1 output — TBD
    ├── contracts/           # TBD — expected small: hand-off URL + CLA-Group listing
    └── tasks.md             # Phase 2 output (/speckit.tasks — not created here)
```

### Source Code

Primary repo: `linuxfoundation/lfx-self-serve`

```text
apps/lfx-one/src/
├── app/modules/sign-cla/                        # NEW Me-lens page: searchable CLA Group list → hand-off
├── app/layouts/main-layout/main-layout.component.ts   # EDIT: add Me-lens entry behind flag
├── app/app.routes.ts                            # EDIT: register the route (lens: 'me', flag-guarded)
└── server/
    ├── routes/clas.route.ts                     # EXTEND (M1's cla route): list CLA Groups; resolve userID + build hand-off URL
    ├── controllers/clas.controller.ts           # EXTEND: session → user-from-token → hand-off orchestration
    └── services/cla.service.ts                  # EXTEND: CLA-Group listing client (endpoint TBD, FR-007)
```

Conditional repos (only if the clarify items land inside M2): `easycla-contributor-console` (handle missing active-signature on the individual dashboard) and `cla-backend-go` (no-PR ICLA request shape; Gerrit-type precedent in `v2/sign`).

**Structure Decision**: extend M1's `cla` server module — the picker reuses the session→identity→EasyCLA-read seam M1 established. The Angular surface is one page in the same shape as M1's `my-clas` module. No sign-type UI, no org/repo picker: the Console's decision screen owns everything after CLA-Group selection.

## Complexity Tracking

No constitution violations. Schedule risks, in order:

1. **GitHub identity binding** (spec open question 1) — if resolution requires backend enrichment work, it competes with the 2-week budget; the fallback (require M1's GitHub link as a precondition, hand off the GitHub-anchored record) is the simple path.
2. **Proactive-ICLA gap** (spec open question 2) — Console + backend delta; decide in/out of M2 at `/speckit.clarify`. ECLA works proactively with zero changes, so a worst-case fallback exists but weakens the milestone.
3. **CLA-Group listing** (spec open question 3) — new read endpoint would add a swagger-first `cla-backend-go` slice.
