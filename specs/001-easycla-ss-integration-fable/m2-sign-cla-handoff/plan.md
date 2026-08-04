# Implementation Plan: Milestone 2 — Proactive CLA signing entry in Self Serve (hands off to Contributor Console)

**Branch**: `docs/easycla-ss-m2-speckit` | **Date**: 2026-08-04 | **Spec**: [spec.md](spec.md)
**Input**: M2 feature spec [spec.md](spec.md) (extracted from the program spec's User Story 2, revised 2026-08-04 — [../spec.md](../spec.md)) and [../02-milestone-sign-icla-fable.md](../02-milestone-sign-icla-fable.md). M1 is a completed dependency; M3–M6 are roadmap context only.

## Summary

Add a new, PR-independent "Sign a CLA" entry point to LFX Self Serve's Me lens. A logged-in contributor picks a CLA Group (and, where relevant, a GitHub org/repo), chooses ICLA or ECLA (gated by the CLA Group's `project_icla_enabled` / `project_ccla_enabled` flags), and is handed off to the existing Contributor Console — pre-scoped to that selection — to complete the actual signing. **Self Serve does not run the signing ceremony**: it makes no signing-initiation calls and never talks to DocuSign; the Console does that, exactly as today. The PR-check remediation link is unchanged. This milestone is additive — it retires nothing and cuts over nothing.

The core technical work is (1) enumerating CLA Groups + org/repo scope for the picker, and (2) constructing a hand-off into the Console that lands the user on the right screen. Both depend on unresolved design questions — the CLA-Group discovery endpoint and the hand-off contract — that `/speckit.plan` / `/speckit.clarify` must settle before `research.md` / `data-model.md` / `contracts/` can be written. This plan therefore stops at structure and constraints; it does not invent the contract.

## Technical Context

**Language/Version**: TypeScript (Angular 20.3 frontend + Node 22 / Express 4 SSR server) in `lfx-self-serve`. No `cla-backend-go` change is assumed unless CLA-Group discovery (FR-007) proves to need a new endpoint (open question).
**Primary Dependencies**: `apps/lfx-one` (PrimeNG 20, standalone components, signals), Express server services (`MicroserviceProxyService` / `ApiClientService` pattern), LaunchDarkly feature flags, EasyCLA v2/v3/v4 REST APIs via lfx-gateway. Reuses M1's `cla` server module seam where possible.
**Storage**: none in SS (stateless; the picker reads CLA-Group metadata and constructs a hand-off — no writes, no caching of agreement data beyond request scope). EasyCLA DynamoDB + S3 untouched.
**Testing**: lfx-self-serve conventions — Jest/Karma unit tests for services/components, server route tests, Cypress/E2E per repo norms (verify exact harness during implementation).
**Target Platform**: LFX One web app (SSR, Kubernetes via ArgoCD).
**Project Type**: web application (feature surface in existing monorepo).
**Performance Goals**: picker interactive with CLA-Group list < 2s p95; hand-off redirect construction adds no perceptible latency.
**Constraints**: no EasyCLA writes and no signing-initiation calls from SS (FR-005); server-side identity derivation only (never trust client-supplied user IDs); PR-check remediation link untouched (FR-006); feature-flagged dark launch.
**Scale/Scope**: all LFX contributors; per-user CLA-Group option sets are small. 1 new Me-lens surface, ~1–2 server routes (CLA-Group listing + hand-off construction), 0–1 upstream endpoints depending on the discovery open question.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`.specify/memory/constitution.md` is the unratified template — no project-specific gates exist. Default gates applied:

- **Simplicity**: no new services, no new storage, no state; a picker surface + hand-off in the existing `cla` server module. The hand-off is a redirect with scoped context, not a new integration. PASS.
- **Security**: CLA-Group options derived server-side from the session identity; SS makes no signing-initiation calls and exposes no arbitrary-user lookup. The Console remains the enforcement point for the signing flow itself. PASS.
- **No speculative work**: native SS signing, SSM cutover, and per-platform splits are explicitly excluded per the 2026-08-04 revision. The picker scope is capped to what a working hand-off needs; richer org-selection UX is deferred to M3. PASS.

**Post-Phase-1 re-check**: pending — cannot fully clear until the hand-off contract and discovery endpoint are resolved (see Open questions in [spec.md](spec.md)). Re-run this gate after `/speckit.clarify`.

## Project Structure

### Documentation (this feature)

```text
specs/001-easycla-ss-integration-fable/          # program level (review docs, in PR #5132)
├── m1-my-cla/                                   # M1 (already on dev)
└── m2-sign-cla-handoff/                         # this milestone's feature directory
    ├── spec.md              # extracted M2 spec (US2 / FR-001…FR-008, revised 2026-08-04)
    ├── plan.md              # This file
    ├── research.md          # Phase 0 output — TBD by /speckit.plan (resolves discovery endpoint)
    ├── data-model.md        # Phase 1 output — TBD (CLA-Group option model, hand-off params)
    ├── quickstart.md        # Phase 1 output — TBD
    ├── contracts/
    │   └── console-handoff.md      # TBD — the hand-off contract is an open question
    └── tasks.md             # Phase 2 output (/speckit.tasks — not created here)
```

### Source Code

Primary repo: `linuxfoundation/lfx-self-serve`

```text
apps/lfx-one/src/
├── app/modules/                                # NEW "Sign a CLA" Me-lens surface (picker)
│   └── sign-cla/                               # CLA-Group + org/repo + sign-type selection, hand-off
├── app/layouts/main-layout/main-layout.component.ts   # EDIT: add Me-lens entry behind flag
├── app/app.routes.ts                           # EDIT: register the picker route (lens: 'me', flag-guarded)
└── server/
    ├── routes/clas.route.ts                    # EDIT/EXTEND (M1's cla route): list CLA Groups; build Console hand-off URL
    ├── controllers/clas.controller.ts          # EDIT/EXTEND: session→available CLA Groups→hand-off orchestration
    └── services/cla.service.ts                 # EDIT/EXTEND: CLA-Group enumeration client (endpoint TBD)
```

Contingency repo (only if CLA-Group discovery needs new API — FR-007 open question): `linuxfoundation/easycla` `cla-backend-go` — one read endpoint, swagger-first (`swagger/cla.v2.yaml` → `make swagger` → handler), no schema changes.

**Structure Decision**: extend M1's `cla` server module rather than add a new one — the picker reuses the same session→identity→EasyCLA-read seam M1 established, adding CLA-Group enumeration and hand-off construction. The Angular surface is a new Me-lens module in the same shape as M1's `my-clas`.

## Complexity Tracking

No constitution violations to track. The one open risk is estimate-affecting, not complexity-adding: if CLA-Group discovery (FR-007) requires a new `cla-backend-go` endpoint, the milestone grows a swagger-first backend slice. Resolve via `/speckit.clarify` before committing to the Aug 21 target.
