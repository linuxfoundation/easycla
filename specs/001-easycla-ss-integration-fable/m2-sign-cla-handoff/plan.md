# Implementation Plan: Milestone 2 — My CLAs actions: proactive sign entry, CLA-manager-routed removal, status

**Branch**: `docs/easycla-ss-m2-speckit` | **Date**: 2026-08-04 (revised 2026-08-14) | **Spec**: [spec.md](spec.md)
**Input**: [spec.md](spec.md), [../spec.md](../spec.md), [../02-milestone-sign-icla-fable.md](../02-milestone-sign-icla-fable.md), and the [M2 UI mockup v17 Final](https://github.com/linuxfoundation/easyclav2-migration-planning/blob/main/Mockups/M2/EasyCLA_MyCLAs_Full_Prototype_Final.html). M1 is a completed dependency; M3–M6 are roadmap context only.

**2026-08-14 legal/stakeholder review**: self-service invalidation (ICLA and ECLA) is removed from scope entirely. This plan reflects that revision — see [lfx-self-serve#1253](https://github.com/linuxfoundation/lfx-self-serve/issues/1253) and its linked tasks for the ticket-level scope this now matches.

## Summary

Extend M1's **My CLAs** page (the Profile CLAs tab) per the mockup:

1. A **"Sign CLA" modal** backed by a new four-source search endpoint (project name / CLA-group name / org name with provenance / pasted repo URL — FR-001, FR-001a), which resolves the user's EasyCLA `userID` server-side and, after a read-only identity pre-flight (FR-004 — SS writes no identity), redirects to the Console's existing decision screen (`{console}/#/cla/project/{claGroupID}/user/{userID}`).
2. The **signed-under identity** shown inline per row (FR-007) — platform (GitHub/GitLab/Gerrit) + account, informational only.
3. **CLA-manager-routed removal/approval** for ECLAs (FR-008–FR-009a) — "Request Removal" and "Request approval" open a shared Contact-CLA-Manager modal that notifies the resolved CLA manager(s) (email or in-app, an implementation choice); Self Serve makes **no invalidation writes** of any kind. A CLA-manager viewer additionally sees "Manage in CCLA Console". ICLAs are informational-only — no removal action.
4. A **status column** (Valid / Needs attention / Revoked) driven by three independent response fields rather than M1's collapsed boolean (FR-010a) — **Revoked** is system-set (sanctions/OFAC), read-only, dated, and carries no user actions of any kind (FR-010, FR-011, FR-011a).

The ICLA/ECLA choice, its legal guidance, and all signing logic stay in the Console. SS makes no signing-initiation calls, no invalidation writes, and never touches DocuSign.

**Constraints**: reuse the Console's deep link, existing endpoints, and existing SS middleware; deliverable in ~3 weeks — see Complexity Tracking.

## Technical Context

**Language/Version**: TypeScript (Angular 20.3 + Node 22 / Express 4 SSR) in `lfx-self-serve`; Go 1.25 in `cla-backend-go`.
**Primary Dependencies**: `apps/lfx-one` (PrimeNG 20, standalone components, signals), Express server services (`MicroserviceProxyService` / `ApiClientService`), existing impersonation-readonly middleware, LaunchDarkly flags, EasyCLA v2/v4 REST APIs via lfx-gateway. Extends M1's Profile CLAs tab and `cla` server module.
**Storage**: none in SS (stateless). EasyCLA DynamoDB + S3 untouched except via existing/new endpoints.
**Testing**: Jest/Karma unit tests, server route tests (incl. impersonation-block and ownership-enforcement paths), Cypress/E2E per repo norms; `cla-backend-go` unit tests for new endpoints.
**Target Platform**: LFX One web app (SSR, Kubernetes via ArgoCD).
**Project Type**: web application (existing monorepo) + swagger-first backend slice.
**Performance Goals**: My CLAs interactive < 2s p95 including status; **sign-entry search < 300 ms p95 server time** (per-keystroke, with ~200 ms client debounce and a result cap — FR-001a); hand-off redirect adds no perceptible latency. The search budget MUST be met without a cache or new datastore — see the spec's Performance assumption for the measure-first sequence and the unverified cardinality it rests on.
**Constraints**: SS makes **no invalidation writes at all** — the only contributor-facing write is the Request-Removal/Request-approval notification send (FR-008), which mutates no signature or Approved-List state; no signing-initiation calls (FR-005); the one implicit write is `GET /v4/user-from-token`'s lookup-or-create side effect (FR-003); server-side identity derivation throughout; PR-check link untouched (FR-006); feature-flagged dark launch; 3-week budget.
**Scale/Scope**: all LFX contributors; extends 1 existing page, ~4 server routes (search, `userID` resolution/hand-off + pre-flight, manager-notification send), **2 new upstream endpoints** (four-source search; CLA-manager resolution + notification) and **1 revision** (`GET /v4/my-clas` three-field status + signed-under identity). No SS-owned identity-binding operation, no invalidation endpoint, and no new datastore or cache.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

No Spec Kit scaffolding exists in either repo (there is no `.specify/` directory in `easycla` or `lfx-self-serve`). M1's artifacts were authored in Spec-Kit *shape* by hand; M2 follows that convention. Committing `.specify/` is a separate change and not a prerequisite. Default gates:

- **Simplicity**: no new services, storage, or state; extends M1's existing tab and `cla` server seam; hand-off is the Console's existing deep link; the retired self-service invalidation design (revised `invalidateICLA`, new ECLA-invalidation endpoint) is **not built at all** — M2 now makes zero invalidation writes. The one genuinely new backend piece is the CLA-manager resolution + notification endpoint (FR-008). **PASS — simpler than the pre-2026-08-14 design.**
- **Security**: `userID` derived server-side. The manager-resolution/notification endpoint MUST verify the caller owns the ECLA signature it's requesting removal/approval for (reusing `authorizeIdentity`, `v2/my_clas/service.go:383`) so it can't be used to probe or spam notifications for another contributor's agreement — but it has no invalidation blast radius, since it writes no signature or Approved-List state. **PASS — smaller security surface than the retired design.**
- **No speculative work**: native SS signing, SSM cutover, per-platform splits, org/repo selection, sign-type UI, Approved List management, and **self-service invalidation** (removed 2026-08-14) are all excluded. **PASS.**

**Post-Phase-1 re-check**: all five original spec design questions are settled; what remains before Phase 1 is a **measurement** (CLA-group cardinality, FR-001a), not a clarification. Two scope decisions keep the Simplicity gate passing where it would otherwise have degraded — **no SS-owned identity-binding operation** (risk 3) and **no bespoke search cache or new datastore** (risk 5). The 2026-08-14 revision removes an entire risk category (self-service invalidation) rather than reopening the gate.

## Project Structure

### Documentation (this feature)

```text
specs/001-easycla-ss-integration-fable/          # program level (review docs, in PR #5132)
├── m1-my-cla/                                   # M1 (already on dev)
└── m2-sign-cla-handoff/                         # this milestone
    ├── spec.md              # extracted M2 spec (FR-001…FR-011a)
    ├── plan.md              # This file
    ├── research.md          # Phase 0 output
    ├── data-model.md        # Phase 1 — TBD
    ├── quickstart.md        # Phase 1 — TBD
    ├── contracts/           # Phase 1 — hand-off URL, search, manager-notification
    └── tasks.md             # Phase 2 (/speckit.tasks — not created here)
```

### Source Code

Primary repo: `linuxfoundation/lfx-self-serve`. Paths verified against `main` on 2026-08-11. **M1 shipped as a Profile tab, not the `app/modules/my-clas/` module its own plan predicted** — route `/profile/clas`, dark-launched behind the `my-clas-enabled` flag via `canMatch` (`app/modules/profile/profile.routes.ts`). M2 extends that tab.

```text
apps/lfx-one/src/
├── app/modules/profile/clas/                    # EXTEND M1's tab (profile-clas.component.ts|html):
│                                                #   Sign CLA modal, signed-under identity, Contact-CLA-Manager modal
│                                                #   (removal/approval modes), Manage-in-CCLA-Console link, status column
├── app/modules/profile/profile.routes.ts        # EDIT only if a sub-route is needed (already flag-guarded)
├── app/shared/services/my-clas.service.ts       # EXTEND: search, hand-off, manager-notification send
└── server/
    ├── routes/clas.route.ts                     # EXTEND: search; userID resolution + hand-off; manager-notification send
    ├── controllers/clas.controller.ts           # EXTEND: hand-off orchestration; manager-notification (ownership-checked)
    ├── services/cla.service.ts                  # EXTEND: search client; manager-notification client; status mapping
    └── types/cla.types.ts                       # EXTEND: upstream types for search/status/manager-notification

packages/shared/src/
├── interfaces/cla.interface.ts                  # EXTEND: MyClaAgreement gains the FR-010/FR-007 fields
├── utils/cla-view.utils.ts                      # EXTEND if row-level view logic is needed
└── constants/feature-flags.constants.ts         # ADD the M2 flag alongside MY_CLAS_ENABLED_FLAG
```

Two M1 details M2 must not regress, both in `server/services/cla.service.ts`: `getMyClas` filters to `valid === true`, dropping every invalid row, and `toMyClaAgreement` maps `status` from the collapsed boolean — FR-010 requires changing both. The `identityQuery` gateway padding stopgap (lfx-gateway#114) applies to any new multi-valued identity query.

Secondary repo: `linuxfoundation/easycla` `cla-backend-go` — new CLA-manager resolution + notification endpoint (FR-008/FR-008a), swagger-first (`swagger/cla.v2.yaml` → `make swagger` → handler/service). No invalidation endpoint is built. Schema impact is **additive only** per FR-010b (revocation timestamp + reason/actor, written by the sanctions-screening path and corporate-console manager actions, not by M2) — no migrations, but the attributes are read by v1/v2 paths, so consumers must tolerate empty values on pre-M2 records. Also: the `GET /v4/my-clas` status + signed-under-identity extension, and the no-PR ICLA request shape in `v2/sign` (+ a matching `easycla-contributor-console` tweak).

**Structure Decision**: everything lands in M1's Profile CLAs tab and the `cla` server seam — the mockup is an extension of the M1 page, not a new surface. The Console owns everything after CLA-Group selection.

## Complexity Tracking

No constitution violations. Schedule risks against the 3-week budget, in order:

1. **Status response shape** — the largest single item, unchanged in size by the 2026-08-14 revision, just re-termed. The upstream evaluation exists but the response shape does not: `row.Valid = sig.SignatureApproved && covered` (`v2/my_clas/service.go:218`) makes Needs attention unrepresentable, and telling **Revoked** apart from ordinary non-approval needs a third input. FR-010a requires three independent fields, the third being the revocation-provenance marker (FR-010b) — written by the sanctions-screening path and the corporate console, not by M2, but M2 must read and expose it. SS-side delta confirmed: M1's BFF filters non-valid rows and derives status from the boolean.
2. **CLA-manager resolution + notification endpoint** (FR-008/FR-008a) — the one genuinely new backend piece, and **smaller than the retired ECLA-invalidation endpoint it replaces**: resolve the CLA manager(s) for a company/CLA-group and send a notification (email is an acceptable, simpler delivery mechanism than an in-app manager list — FR-008 leaves this an implementation choice). No Approved-List mutation, no signature write, no durable-exclusion marker to design (unlike the retired design) — the marker in risk 1 is written elsewhere, not by this endpoint. Ownership check still required: the caller must own the ECLA they're requesting removal/approval for.
3. **Four-source search endpoint** — no existing endpoint fits (both listings are SFID-scoped; `GetCLAGroupByName` is exact-match-only), and the mockup searches four sources with a per-keystroke expectation. Includes a latent bug to fix: `GetCLAGroups` logs its search parameters then builds a projection with no filter (`project/repository/repository.go:529-538`), so search on this table has never been measured. **Deliberately no cache or new datastore** (FR-001a): serve from DynamoDB, fix the filter, measure p95; escalate to OpenSearch only if measurement demands it. **Blocking measurement**: the CLA-group row count, unobtainable in the authoring session (no `lfproduct-*` AWS profile locally; `lfx-*` SSO tokens expired) — an explicit unverified assumption, not a finding.
4. **Proactive-ICLA gap** — spiked and **smaller than feared** ([research.md](research.md) Spike 1). Two independent dependencies, one per repo: the Console's `findActiveSignature()` hard-stop (`individual-dashboard.component.ts:49-66`) and the backend's PR-metadata requirement (`v2/sign/service.go:2890,2903`). Fix is a widened branch condition in each plus an explicit proactive request signal — no DocuSign/schema/webhook change. Required for the GitHub sign-entry path. Caveat: `acl = github:{user.GithubID}` (`:1433`) must still be set, so the GitHub-ID binding must land before signing.
5. **Signed-under identity display** (FR-007) — small, additive: the signer identity and its repo-source type are already tied to the signature record; expose them in the `GET /v4/my-clas` response and render inline. No new endpoint.
6. **Account authorization** — **retired by a scope decision, not by new code.** M2 builds no identity-binding operation: SS runs a read-only pre-flight against `GET /v4/my-clas/identities` and delegates binding to the Console's existing GitHub OAuth (FR-003/FR-004). Residual work: two conditionals plus a picker when several GitHub identities are linked. Identity types verified compatible (numeric IDs on both sides).

**Retired entirely by the 2026-08-14 revision, not merely descoped**: the ICLA-invalidation revision (targeting `invalidateICLA` by signature ID, making it actor-aware) and the new self-service ECLA-invalidation endpoint with its durable self-exclusion marker. Neither is built. `invalidateICLA` remains untouched, a CLA-manager/admin-facing endpoint outside M2. This removes an entire risk category rather than shrinking one — smaller scope, not the same scope done differently.

**Not a risk**: the Console decision screen's deep-linkability, re-verified on 2026-08-11 — the project fetch lives in the embedded `<app-project-title>` child (`project-title.component.ts:46-58`), so FR-002's hand-off needs no Console-side change.

### If the budget forces a cut

- **Not cuttable**: the three-field status response (risk 1) — FR-010/FR-011 both depend on it. Likewise the search endpoint (risk 3): without it the modal has nothing to search.
- **Cheapest real saving — narrow the search sources.** Ship project + CLA-group + org name and defer **repo-URL** matching: by far the largest cardinality and the source most likely to force an indexing layer. The modal degrades gracefully — users search by name instead of pasting a link.
- **Next cheapest — defer the Manage-in-CCLA-Console link** (FR-009a): it's additive UI for a manager viewer, and Request Removal/approval (FR-008/FR-011) work without it.
- **Not worth cutting**: the GitHub sign entry. Its no-PR delta (risk 4) is measured and small and risk 6 is retired rather than deferred, so narrowing to Gerrit-only saves almost nothing.
- **Do not "save time" by adding a cache** to hit the search budget: it trades a latency problem for a correctness problem on merge-gating data, and adds infrastructure this milestone does not otherwise need.
