# Implementation Plan: Milestone 1 — Read-only "My CLAs" in Self Serve Me Lens

**Branch**: `001-easycla-ss-integration` | **Date**: 2026-07-11 | **Spec**: [spec.md](spec.md)
**Input**: M1 feature spec [spec.md](spec.md) (extracted from the program spec [../spec.md](../spec.md), User Story 1 / FR-001…FR-006) and [../01-milestone-read-only-me-lens-fable.md](../01-milestone-read-only-me-lens-fable.md). M2–M6 are roadmap context only.

## Summary

Add a read-only "My CLAs" module to LFX Self Serve's Me lens showing the logged-in user's signed ICLAs (with signed-PDF download) and valid ECLAs, backed by the existing EasyCLA v3/v4 APIs via the crowdfunding-style server-side integration pattern. The core technical problem is resolving the LF SSO identity to EasyCLA user record(s), including pre-LF-login history. Feature-flagged, no writes, no EasyCLA business-logic changes; at most one small read endpoint added to `cla-backend-go` if identity lookup by email proves necessary (research indicates existing endpoints likely suffice).

## Technical Context

**Language/Version**: TypeScript (Angular 20.3 frontend + Node 22 / Express 4 SSR server) in `lfx-self-serve`; Go 1.25 (`cla-backend-go`) only if the contingency endpoint is needed
**Primary Dependencies**: `apps/lfx-one` (PrimeNG 20, standalone components, signals), Express server services (`MicroserviceProxyService`/`ApiClientService` pattern), LaunchDarkly feature flags, EasyCLA v3/v4 REST APIs via lfx-gateway (`/cla-service/v3|v4`)
**Storage**: none in SS (stateless proxy; no caching of agreement data beyond request scope). EasyCLA DynamoDB + S3 remain untouched upstream.
**Testing**: lfx-self-serve conventions — Jest/Karma unit tests for services/components, server route tests; contract fixtures against EasyCLA dev; Cypress/E2E per repo norms (verify exact harness during implementation)
**Target Platform**: LFX One web app (SSR, Kubernetes via ArgoCD)
**Project Type**: web application (feature module in existing monorepo)
**Performance Goals**: page interactive with agreement list < 2s p95 against dev/prod EasyCLA; PDF link issuance < 1s p95 (presigned URL fetch on click)
**Constraints**: read-only (no EasyCLA writes); server-side identity derivation only (never trust client-supplied user IDs); presigned URLs are 15-minute TTL — fetch on demand; feature-flagged dark launch
**Scale/Scope**: all LFX users with CLA history (~hundreds of thousands of signature records upstream; per-user result sets are small — typically < 50 agreements); 1 new lens module, ~2 server routes, 0–1 upstream endpoints

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`.specify/memory/constitution.md` is the unratified template — no project-specific gates exist. Default gates applied instead:

- **Simplicity**: no new services, no new storage, no state; single SS server module + lens module. PASS
- **Security**: user-scoped data derived from session server-side; upstream v4 endpoint lacks per-user authz (verified in `v2/signatures/handlers.go` — `GetUserSignatures` performs no ownership check), so the SS server MUST be the enforcement point and MUST NOT expose arbitrary-user lookup. PASS with this requirement carried into contracts.
- **No speculative work**: M2–M6 needs (signing, roles) explicitly excluded; the `cla` server module is the only deliberately reusable seam. PASS

**Post-Phase-1 re-check**: design adds no projects, no new patterns beyond the existing crowdfunding integration precedent. PASS

## Project Structure

### Documentation (this feature)

```text
specs/001-easycla-ss-integration-fable/          # program level (review docs)
└── m1-my-clas/                                  # this milestone's feature directory
    ├── spec.md              # extracted M1 spec (US1 / FR-001…006)
    ├── plan.md              # This file
    ├── research.md          # Phase 0 output
    ├── data-model.md        # Phase 1 output
    ├── quickstart.md        # Phase 1 output
    ├── contracts/
    │   ├── ss-me-clas-api.md        # SS server API consumed by the Angular module
    │   └── upstream-easycla-api.md  # EasyCLA endpoints consumed, incl. auth notes
    └── tasks.md             # Phase 2 output (/speckit-tasks — not created here)
```

### Source Code

Primary repo: `/Users/michal/src/github/linuxfoundation/lfx-self-serve`

```text
apps/lfx-one/src/
├── app/modules/my-clas/                     # NEW Angular lens module (Me lens)
│   ├── my-clas.routes.ts                    # exported MY_CLAS_ROUTES, lazy-loaded at /me/clas
│   ├── my-clas.component.ts|html            # list view: ICLA + ECLA sections, empty state
│   └── components/
│       └── agreement-card/…                 # per-agreement row (type, project, company, date, status, PDF action)
├── app/layouts/main-layout/main-layout.component.ts   # EDIT: add Me-lens sidebar item behind flag
├── app/app.routes.ts                        # EDIT: register 'me/clas' route (lens: 'me', flag-guarded)
└── server/
    ├── routes/clas.route.ts                 # NEW: GET /api/me/clas ; GET /api/me/clas/:signatureId/pdf-url
    ├── controllers/clas.controller.ts       # NEW: session→identity→aggregate orchestration
    ├── services/cla.service.ts              # NEW: EasyCLA v3/v4 client (identity lookup, signatures, signed-doc URL)
    └── types/cla.types.ts                   # NEW: upstream + view-model types

packages/shared/src/interfaces/              # shared UI interfaces if needed (MyClaAgreement)
```

Contingency repo (only if email-based lookup gap confirmed): `/Users/michal/src/github/linuxfoundation/easycla/cla-backend-go` — one read endpoint in `users/` or `v2/`, swagger-first (`swagger/cla.v2.yaml` → `make swagger` → handler), no schema changes.

**Structure Decision**: single feature module in the existing `lfx-one` app plus mirrored server routes — identical in shape to the crowdfunding module (`app/modules/crowdfunding` + `server/routes/crowdfunding.route.ts`), which is the repo's established pattern for integrating a non-V2 backend.

## Complexity Tracking

No constitution violations; table not required.
