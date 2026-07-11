# EasyCLA → LFX Self Serve: Program Overview

**Audience**: Architecture review board, PM (scope approval)
**Created**: 2026-07-11
**Status**: Draft
**Spec**: [spec.md](spec.md)

## 1. What we are doing and why

EasyCLA today spans four UIs (Contributor Console, Corporate CLA Console, PCC EasyCLA module, plus the landing page) and a backend running as AWS Lambdas behind API Gateway / lfx-gateway. The program migrates all EasyCLA user-facing functionality into **LFX Self Serve (LFX One)** under its Me / Organization / Project lenses, retiring three UIs, and — as a separately gated decision — re-platforms the EasyCLA API as an LFX V2 Kubernetes service, optionally replacing DynamoDB with Postgres.

Six milestones, each independently shippable and reversible:

| # | Milestone | Retires | Doc |
|---|-----------|---------|-----|
| M1 | Read-only "My CLAs" in Me lens | nothing | [01](01-milestone-read-only-me-lens-fable.md) |
| M2 | Sign ICLA in SS (PR link → SS) | nothing | [02](02-milestone-sign-icla-fable.md) |
| M3 | Sign ECLA in SS (corporate flow) | Contributor Console | [03](03-milestone-sign-ecla-fable.md) |
| M4 | CCLA management in Organization lens | Corporate CLA Console + its BFF | [04](04-milestone-ccla-org-lens-fable.md) |
| M5 | EasyCLA admin in Project lens | PCC EasyCLA module | [05](05-milestone-project-lens-pcc-fable.md) |
| M6 | API → Kubernetes V2 service (± Postgres) | Lambda/API GW deployment | [06](06-milestone-k8s-v2-api-fable.md) |

## 2. Current state (verified in code)

### 2.1 Backend

- **Go backend** (`cla-backend-go`): serves `/v3` (v1 code, us-east-1) and `/v4` (v2 code, us-east-2, LFX-Platform-integrated) as Lambdas (`serverless.yml`, runtime `provided.al2`). A standalone server entrypoint already exists (`cmd/server_standalone.go`) — relevant to M6.
- **Legacy Go backend** (`cla-backend-legacy`): the Python backend has been **removed**; the legacy `/v1`/`/v2` surface is a Go implementation deployed from the `cla-backend` stack on the original `api.*` domains (with parity tooling under `internal/parity`). The Contributor Console still calls these endpoints for core flows: `GET /v2/user/{id}/active-signature`, `POST /v2/check-prepare-employee-signature`, `POST /v2/request-employee-signature`, `GET /v2/project/{id}`, `GET /v2/user/{id}/project/{id}/last-signature`, `POST /v1/user/gerrit`. Contributor milestones keep calling this surface; M6 absorbs it. (Note: the repo's CLAUDE.md still describes `cla-backend`/`cla-backend-legacy` as Python — stale.)
- **Routing**: `/cla-service/v3|v4/*` already routes through **lfx-gateway** (Traefik fork) to the Lambda (`lfx-gateway/dynamic/services/cla-service.yaml`). EasyCLA is thus already behind the platform gateway — M6 is a compute/data migration, not a gateway onboarding.
- **Data**: ~19 DynamoDB tables (`cla-{stage}-signatures`, `-users`, `-companies`, `-projects`, `-projects-cla-groups`, `-repositories`, `-github-orgs`, `-gitlab-orgs`, `-gerrit-instances`, `-approvals`, `-events`, `-store`, `-session-store`, …). Signed PDFs in S3 `cla-signature-files-{stage}` with 15-minute presigned URLs.
- **Signature model** (single `signatures` table):
  - ICLA: `signature_type=cla`, `reference_type=user`, has PDF.
  - CCLA: `signature_type=ccla`, `reference_type=company`, has PDF, carries approval lists + `auto_create_ecla`.
  - **ECLA: `signature_type=cla`, `reference_type=user`, `signature_user_ccla_company_id=<company>` — no PDF, confirmed.** ECLAs can be auto-created by approval-list edits.
- **DocuSign** lives in Go `v2/sign`: creates envelopes, returns an embedded-signing `sign_url`, receives webhooks, downloads the signed PDF to S3, updates the record. Consoles never talk to DocuSign directly.
- **PR gating**: failed status check links to `{CLAContributorv2Base}/#/cla/project/{claGroupID}/user/{userID}?redirect=<PR URL>`; the base URL is an SSM parameter per environment — **console→SS cutover is a config flip with instant rollback**.
- **Sanctions screening (SSS)**: corporate flows are gated by a live compliance check (`check-prepare-employee-signature`, and re-screen at CCLA finalization); enabled/required per environment.

### 2.2 UIs being absorbed

- **Contributor Console** (Angular, hash-routed): individual-vs-corporate decision; ICLA via `POST /v4/request-individual-signature` → redirect to `sign_url`; corporate flow with company search (v3), pre-checks (v2), ECLA record (v2), request-authorization (`POST /v4/notify-cla-managers`), CLA-manager-designee bootstrap (`POST /v4/.../cla-manager-designee` with up-to-30-retry polling for async ACS role assignment), invite-company-admin. **LF SSO login is required for all flows** (the entry dashboard forces login for unauthenticated users — added recently), so SS's login requirement matches the status quo.
- **Corporate CLA Console**: Angular 13 + NgRx + **its own Node/Apollo GraphQL BFF** (~648 backend files) bridging to `/cla-service/v4`. Features: CCLA signing (`/v4/request-corporate-signature`), 5 approval-list types, acknowledgement/ICLA reporting, CLA manager CRUD + designee, auto-create-ECLA toggle, foundation/project signed views, activity logs + CSV, metrics. Permissions arrive as `resource:action:project|organization:{ids}` strings resolved from ACS.
- **PCC EasyCLA module**: **v1-frontend only** (nothing in PCC v2). CLA group wizard (ICLA/CCLA/ICLA-required flags, project-tree enrollment), template select/preview (server-side PDF generation), GitHub App install + repo enrollment (incl. enforce-all), Gerrit instances, GitLab groups, ICLA/CCLA reporting with signature invalidation, events + CSV. Gated by a ~24-entry permission matrix (project-scoped) plus project-status guard.

### 2.3 Target platform (Self Serve / LFX V2)

- **Self Serve**: Angular 20 + Express SSR monorepo (`apps/lfx-one`). Lenses are route-scoped (`me | org | project | foundation`); org lens is dark-launched. Server proxies V2 services over HTTP via the gateway (`MicroserviceProxyService`), NATS for auth-service. **Crowdfunding already demonstrates the pattern we need: an external (non-V2) backend integrated with its own Auth0 audience and token exchange, server-side routes in SS.**
- **AuthN**: Auth0 OIDC (code+PKCE) with Express sessions; `lfx-v2-auth-service` is a NATS-based user-metadata/identity abstraction (it is *not* a role manager and *not* the login flow itself).
- **AuthZ**: services enforce via Heimdall + **OpenFGA** relations (`project#writer`, `b2b_org#writer`, …) synced by `lfx-v2-fga-sync` over NATS. **No CLA object types exist in the FGA model today.**
- **"V2 service" means**: Helm chart + ArgoCD app per environment, `ghcr.io/linuxfoundation/*` image, Heimdall token validation, OpenFGA authorization, NATS for messaging/fga-sync, OpenTelemetry — see the ~15 existing services in `lfx-v2-argocd/apps/*/lfx-v2-applications.yaml`.

### 2.4 Roles today (the "role difference" made concrete)

- EasyCLA roles `cla-manager`, `cla-signatory`, `cla-manager-designee` are **hardcoded in ACS** (Salesforce-backed Postgres), scoped `organization` or `project|organization`. EasyCLA assigns them via organization-service/ACS APIs; assignment is asynchronous (hence the console's retry loops).
- LFX V2 authorization is **OpenFGA relations** synced from platform services; ACS and OpenFGA are separate systems with different models (role+scope tuples vs. relationship tuples) and different sources of truth.

## 3. Architectural strategy (recommended)

**Strangler pattern with EasyCLA v4 as the enforcement core until M6.**

1. **M1–M5: Self Serve is a new client of the existing EasyCLA APIs**, following the crowdfunding precedent: SS Express server gets a `cla` service module that calls `/cla-service/v3|v4` through lfx-gateway with the user's token (plus M2M where needed). No business logic is reimplemented; enforcement (approval lists, sanctions, roles) stays server-side in EasyCLA.
2. **DocuSign never moves in M2–M4.** The consoles already only fetch a `sign_url` and redirect; SS does the same. A dedicated "DocuSign bridge service" is unnecessary — building one would duplicate webhook handling, PDF storage, and envelope state that already live in `v2/sign` (critical analysis in doc 02).
3. **Roles: bridge, don't migrate, until M6.** SS org/project lenses ask EasyCLA (or ACS-derived claims) "what CLA authority does this user have here?" rather than modeling CLA in OpenFGA early. Reason: EasyCLA's backend enforces via ACS on every write — a parallel OpenFGA model would be cosmetic (UI-gating only) while creating a second source of truth to keep in sync. CLA object types enter OpenFGA when the API is rewritten (M6), i.e., when enforcement itself moves.
4. **Cutover per milestone is a config flip** (the SSM redirect base for contributor flows; lens feature flags for org/project), giving SC-007's rollback guarantee.

### Correction to a stated assumption

> "I believe the main challenge here is the role differences between EasyCLA and SS."

Partially. The role difference is the **defining challenge of M6** (and shapes M4/M5 UX), but for the UI migrations it is largely **bridgeable**, because enforcement lives in the EasyCLA backend, not in the consoles — SS can act as another client. The challenges that are as large or larger:

1. **Identity history, not roles**: both consoles now require LF login, so there is no anonymous-contributor gap going forward. What remains is mapping the LF identity to EasyCLA user records — including **historical signatures created before the login requirement**, which may carry no LF username — so M1's aggregation and unmatched-user telemetry still matter, at lower risk than roles-as-blocker framing suggests.
2. **Two Go API surfaces**: contributor-critical endpoints run on the legacy `/v1`/`/v2` Go surface (`cla-backend-legacy`) alongside `/v3`/`/v4` — no Python port is needed (already done), but M6 must still absorb or retire a second API codebase and deployment.
3. **Corporate Console's hidden BFF**: M4 is not only 160 components but also absorbing a GraphQL aggregation backend.
4. **M6's blast radius**: webhooks (GitHub/GitLab/Gerrit), DocuSign callbacks, DynamoDB-stream-driven lambdas (events, notifications, zip building), and 19 tables — far beyond "move the API".

## 4. Cross-cutting risks

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Identity mapping gaps (LF account ↔ EasyCLA user records, esp. pre-LF-login history) | Users see empty/partial "My CLAs"; support load | M1 ships the mapping + telemetry on unmatched users before any signing moves |
| ACS role assignment is async and Salesforce-coupled | Designee/manager flows in SS inherit today's retry-loop fragility | Keep retries server-side in SS; don't promise synchronous UX |
| Dual-console period (feature drift) | Fixes must land twice | Freeze console feature work per area once its SS milestone starts |
| Gerrit/GitLab slip behind GitHub | Console retirement blocked late | Per-platform sub-milestones (M2a–c, M3a–c) inside one release train, each with a parity checklist |
| Legacy `/v1`/`/v2` Go surface drifts from `/v4` during migration | Contributor flow outages | Single ownership of both surfaces; extend the existing parity/contract tests; absorb in M6 |
| M6 rework of M3–M5 adapters | Wasted effort | Q3 decided at this review; keep SS↔EasyCLA integration behind one server-side module |
| DocuSign webhook/timing edge cases resurface in new UI | "Signed but still red PR" complaints | Reuse backend flow untouched; add truthful pending states in SS |

## 5. What was missing from the original framing

Beyond the six milestones as described, the program must also account for:

- **Gerrit and GitLab flows** (contributor console and PCC both manage them) — explicitly scoped per milestone.
- **The legacy `/v1`/`/v2` Go surface** (`cla-backend-legacy`) — a second API codebase inside the blast radius even for "UI-only" milestones.
- **Corporate Console BFF retirement** (M4) and **easycla-landing-page** (untouched, but its links/docs reference consoles — needs a content pass at each retirement).
- **Sanctions screening (SSS)** enforcement surfacing in new corporate flows.
- **Email/notification templates** that embed console URLs (manager notifications, invites) — must be re-pointed at SS per milestone.
- **Auto-create-ECLA** behavior coupling approval-list edits to signature creation (M4 parity item).
- **Signature invalidation** semantics (PCC feature, M5).
- **Metrics/insights endpoints** consumed by the Corporate Console (M4).
- **API consumers beyond the consoles** (anyone calling v3/v4 directly; audit before M6 contract changes).
- **Decommission work as first-class scope**: DNS/CDN teardown, redirect stubs for bookmarked console URLs, support-doc updates in lfx-product-documentation.
- **Parity long-tail from the product documentation** (lfx-product-documentation/easycla/v2-current, reviewed 2026-07-11):
  - **Email-based CCLA signatory signing**: the CLA signatory signs via an emailed DocuSign link and **does not need an LF SSO account** — a distinct UX path that must survive M4 (don't force signatories into SS).
  - **Embargo/OFAC checkbox**: mandatory attestation gating the Sign button on ICLA/CCLA — required in M2/M4 signing UIs.
  - **ICLA status model**: docs show Active / Incomplete / Disabled / Invalidated — verified as **UI-derived labels** over the stored `signed`/`approved` booleans; approval-criteria deletion invalidates related acknowledgements (`invalidateSignatures`, verified) and PMs can invalidate ICLAs (verified endpoint) — M1 display and M5 admin parity.
  - **Multi-PR behavior**: signing updates only one PR — verified mechanism: a single `active_signature:{userID}` KV record holds one return context; other PRs re-check via the `/easycla` comment command (verified handler) — preserve and state in SS UX copy (M2).
  - **Manual signing fallback**: templates carry a project contact email enabling offline/email CLA signing outside DocuSign.
  - **ECLA version tracking**: acknowledgements record which CCLA version they were made under (M1 display candidate, M4 table parity).
  - **Rules**: one CLA group per project (hierarchy validation exists in `cla_groups` service; trace fully in M5); "cannot delete the last CLA Manager" is **documented but no code guard was found** in the v4 backend or Corporate Console — verify/decide enforcement during M4; a user's CLA role attaches to a single company at a time (documented known issue — constrains M4 role bridging).
  - **Gerrit constraints**: instances are LF-hosted and added via support ticket (not self-service), CLA enablement is all-or-nothing per instance, and contributors must sign out/in of Gerrit after signing — M2c/M3c/M5 scope is narrower than GitHub/GitLab.
  - **Ops details**: EasyCLA GitHub App needs Merge Queue read permission (else checks hang in "Expected"); auto branch protection covers only the default branch; PMs get automated emails on repo rename/archive/delete.

## 6. Sequencing rationale & effort signal

M1 (S) → M2 (M overall; sub-milestones M2a GitHub / M2b GitLab / M2c Gerrit) → M3 (L; M3a–M3c likewise) → M4 (XL) → M5 (L) → M6 (XL, or XXL with Postgres). M1–M3 build the contributor path in strictly increasing complexity; M4–M5 are parallelizable after M3's role-bridging pattern exists; M6 is gated on a separate go/no-go with the with/without-database analysis in doc 06.

Former open decisions Q1–Q3 are resolved in [spec.md](spec.md): contributor login is already mandatory in the console (no anonymous path needed), all three git platforms are in scope via per-platform sub-milestones, and sequencing is UI-first with M6 separately gated (the hybrid-strangler variant — an early CLA read/query V2 service — is the noted alternative if M6 is committed early).
