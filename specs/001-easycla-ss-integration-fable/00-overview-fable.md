# EasyCLA → LFX Self Serve: Program Overview

**Audience**: Architecture review board, PM (scope approval)
**Created**: 2026-07-11
**Status**: Revised 2026-09-01 — M1 and M2 implemented; former M2+M3 merged into one M2, later milestones renumbered (M4→M3, M5→M4, M6→M5)
**Spec**: [spec.md](spec.md)

## 1. What we are doing and why

EasyCLA today spans four UIs (Contributor Console, Corporate CLA Console, PCC EasyCLA module, plus the landing page) and a backend running as AWS Lambdas behind API Gateway / lfx-gateway. The program migrates EasyCLA user-facing functionality into **LFX Self Serve (LFX One)** under its Me / Organization / Project lenses — the one deliberate exception is emailed CCLA signatory signing, which stays an external, LF SSO-independent DocuSign path through M3 (see §5) — and — as a separately gated decision — re-platforms the EasyCLA API as an LFX V2 Kubernetes service, optionally replacing DynamoDB with Postgres. The Corporate CLA Console (with its BFF) and the PCC EasyCLA module are retired by their milestones; **Contributor Console and `easycla-landing-page` retirement is no longer scheduled** — M2 shipped as a hand-off *to* the Console (the retire-Console framing was dropped as too disruptive, epic [lfx-self-serve#1229](https://github.com/linuxfoundation/lfx-self-serve/issues/1229)), so their retirement is deferred to a future product decision.

Five milestones, each independently shippable and reversible — **M1–M3 are the planned program; M4 and M5 are not planned yet**:

| # | Milestone | Status | Retires | Doc |
|---|-----------|--------|---------|-----|
| M1 | Read-only "My CLAs" in Me lens | **Implemented** (dark-launched) | nothing | [01](01-milestone-read-only-me-lens-fable.md) |
| M2 | Sign-CLA entry + My CLAs actions in SS, hands off to Contributor Console (merges former M2+M3; ICLA and ECLA both via the Console decision screen) | **Implemented** (dark-launched) | nothing | [02](02-milestone-sign-cla-fable.md) |
| M3 | CCLA management in Organization lens | **In progress** | Corporate CLA Console + its BFF | [03](03-milestone-ccla-org-lens-fable.md) |
| M4 | EasyCLA admin in Project lens | **Not planned** (decision-gated) | PCC EasyCLA module | [04](04-milestone-project-lens-pcc-fable.md) |
| M5 | API → Kubernetes V2 service (± Postgres) | **Not planned** (decision-gated) | Lambda/API GW deployment | [05](05-milestone-k8s-v2-api-fable.md) |

**Program scope**: the program aims to complete **M1–M3**. **M4 and M5 are not planned yet** — they remain decision-gated design options and may never be implemented; the milestone docs for them are retained for reference, not as committed scope. The target architecture through M3 is summarized in [ARCHITECTURE.md](../../ARCHITECTURE.md).

## 2. Current state (verified in code)

### 2.1 Backend

- **Go backend** (`cla-backend-go`): serves `/v3` (v1 code, us-east-1) and `/v4` (v2 code, us-east-2, LFX-Platform-integrated) as Lambdas (`serverless.yml`, runtime `provided.al2`). A standalone server entrypoint already exists (`cmd/server_standalone.go`) — relevant to M5.
- **Legacy Go backend** (`cla-backend-legacy`): the Python backend has been **removed**; the legacy `/v1`/`/v2` surface is a Go implementation deployed from the `cla-backend` stack on the original `api.*` domains (with parity tooling under `internal/parity`). The Contributor Console still calls these endpoints for core flows: `GET /v2/user/{id}/active-signature`, `POST /v2/check-prepare-employee-signature`, `POST /v2/request-employee-signature`, `GET /v2/project/{id}`, `GET /v2/user/{id}/project/{id}/last-signature`, `POST /v1/user/gerrit`. The Console (reached via M2's hand-off and the PR-check link) keeps calling this surface; M5 absorbs it.
- **Routing**: `/cla-service/v3|v4/*` already routes through **lfx-gateway** (Traefik fork) to the Lambda (`lfx-gateway/dynamic/services/cla-service.yaml`). EasyCLA is thus already behind the platform gateway — M5 is a compute/data migration, not a gateway onboarding.
- **Data**: ~19 DynamoDB tables (`cla-{stage}-signatures`, `-users`, `-companies`, `-projects`, `-projects-cla-groups`, `-repositories`, `-github-orgs`, `-gitlab-orgs`, `-gerrit-instances`, `-approvals`, `-events`, `-store`, `-session-store`, …). Signed PDFs in S3 `cla-signature-files-{stage}` with 15-minute presigned URLs.
- **Signature model** (single `signatures` table):
  - ICLA: `signature_type=cla`, `reference_type=user`, has PDF.
  - CCLA: `signature_type=ccla`, `reference_type=company`, has PDF, carries Approved Lists + `auto_create_ecla`.
  - **ECLA: `signature_type=cla`, `reference_type=user`, `signature_user_ccla_company_id=<company>` — no PDF, confirmed.** ECLAs can be auto-created by Approved List edits.
- **DocuSign** lives in Go `v2/sign`: creates envelopes, returns an embedded-signing `sign_url`, receives webhooks, downloads the signed PDF to S3, updates the record. Consoles never talk to DocuSign directly.
- **PR gating**: failed status check links to `{CLAContributorv2Base}/#/cla/project/{claGroupID}/user/{userID}?redirect=<PR URL>`; the base URL is an SSM parameter per environment — **a console→SS cutover would be a config flip with instant rollback**, though no milestone through M3 changes it (M2 deliberately left it unchanged).
- **Sanctions screening (SSS)**: corporate flows are gated by a live compliance check (`check-prepare-employee-signature`, and re-screen at CCLA finalization); enabled/required per environment.

### 2.2 UIs being absorbed

- **Contributor Console** (Angular, hash-routed): individual-vs-corporate decision; ICLA via `POST /v4/request-individual-signature` → redirect to `sign_url`; corporate flow with company search (v3), pre-checks (v2), ECLA record (v2), request-authorization (`POST /v4/notify-cla-managers`), CLA-manager-designee bootstrap (`POST /v4/.../cla-manager-designee` with up-to-30-retry polling for async ACS role assignment), invite-company-admin. **LF SSO login is required for all flows** (the entry dashboard forces login for unauthenticated users — added recently), so SS's login requirement matches the status quo.
- **Corporate CLA Console**: Angular 13 + NgRx + **its own Node/Apollo GraphQL BFF** (~648 backend files) bridging to `/cla-service/v4`. Features: CCLA signing (`/v4/request-corporate-signature`), 5 Approved List types, acknowledgement/ICLA reporting, CLA manager CRUD + designee, auto-create-ECLA toggle, foundation/project signed views, activity logs + CSV, metrics. Permissions arrive as `resource:action:project|organization:{ids}` strings resolved from ACS.
- **PCC EasyCLA module**: **v1-frontend only** (nothing in PCC v2). CLA group wizard (ICLA/CCLA/ICLA-required flags, project-tree enrollment), template select/preview (server-side PDF generation), GitHub App install + repo enrollment (incl. enforce-all), Gerrit instances, GitLab groups, ICLA/CCLA reporting with signature invalidation, events + CSV. Gated by a ~24-entry permission matrix (project-scoped) plus project-status guard.

### 2.3 Target platform (Self Serve / LFX V2)

- **Self Serve**: Angular 20 + Express SSR monorepo (`apps/lfx-one`). Lenses are route-scoped (`me | org | project | foundation`); org lens is dark-launched. Server proxies V2 services over HTTP via the gateway (`MicroserviceProxyService`), NATS for auth-service. **Crowdfunding already demonstrates the pattern we need: an external (non-V2) backend integrated with its own Auth0 audience and token exchange, server-side routes in SS.**
- **AuthN**: Auth0 OIDC (code+PKCE) with Express sessions; `lfx-v2-auth-service` is a NATS-based user-metadata/identity abstraction (it is *not* a role manager and *not* the login flow itself).
- **AuthZ**: services enforce via Heimdall + **OpenFGA** relations (`project#writer`, `b2b_org#writer`, …) synced by `lfx-v2-fga-sync` over NATS. **No CLA object types exist in the FGA model today.**
- **"V2 service" means**: Helm chart + ArgoCD app per environment, `ghcr.io/linuxfoundation/*` image, Heimdall token validation, OpenFGA authorization, NATS for messaging/fga-sync, OpenTelemetry — see the ~15 existing services in `lfx-v2-argocd/apps/*/lfx-v2-applications.yaml`.

### 2.4 Roles today (the "role difference" made concrete)

- EasyCLA roles `cla-manager`, `cla-signatory`, `cla-manager-designee` are **hardcoded in ACS** (an LFX v1 component with its own Postgres database), scoped `organization` or `project|organization`. EasyCLA assigns them via organization-service/ACS APIs; assignment is asynchronous (hence the console's retry loops).
- LFX V2 authorization is **OpenFGA relations** synced from platform services; ACS and OpenFGA are separate systems with different models (role+scope tuples vs. relationship tuples) and different sources of truth.

## 3. Architectural strategy (recommended)

**Strangler pattern with EasyCLA v4 as the enforcement core until M5.**

1. **M1–M4: Self Serve is a new client of the existing EasyCLA APIs**, following the crowdfunding precedent: SS Express server gets a `cla` service module that calls `/cla-service/v3|v4` through lfx-gateway with the user's token (plus M2M where needed). No business logic is reimplemented; enforcement (Approved Lists, sanctions, roles) stays server-side in EasyCLA. *(Implemented for M1/M2: the SS `cla` server module calls the `/v4/my-clas`, `/v4/cla-group/search`, and `/v4/self-serve/prepare-sign` surfaces — see docs 01/02.)*
2. **DocuSign never moves.** The consoles already only fetch a `sign_url` and redirect. In M2 (as implemented) SS never reaches this step — it hands the user to the Contributor Console before signing starts; the Console fetches `sign_url` exactly as it does today, and native signing in SS is not scheduled. In M3 the org lens reuses the same pattern for CCLA signing (v4 returns the `sign_url`). A dedicated "DocuSign bridge service" is unnecessary — building one would duplicate webhook handling, PDF storage, and envelope state that already live in `v2/sign`.
3. **Roles: bridge, don't migrate, until M5.** SS org/project lenses gate UI via the user's self permission check against ACS (`user-service/v1/me/permissions/checks`, per architecture review 2026-07-20) rather than modeling CLA in OpenFGA early. Reason: EasyCLA's backend enforces via ACS on every write — a parallel OpenFGA model would be cosmetic (UI-gating only) while creating a second source of truth to keep in sync. CLA object types enter OpenFGA when the API is rewritten (M5), i.e., when enforcement itself moves.
4. **Cutover per milestone is a config flip** (lens feature flags for org/project), giving SC-007's rollback guarantee. *(Revised 2026-09-01: this is no longer a single shared switch. The SSM contributor-redirect base was the mechanism for a Contributor Console cutover that is no longer scheduled — M2 leaves that URL untouched. M3 and M4 retire different surfaces (Corporate Console, PCC project views) and each needs its own routing/feature-flag rollback plan rather than inheriting this one.)*

### Correction to a stated assumption

> "I believe the main challenge here is the role differences between EasyCLA and SS."

Partially. The role difference is the **defining challenge of M5** (and shapes M3/M4 UX), but for the UI migrations it is largely **bridgeable**, because enforcement lives in the EasyCLA backend, not in the consoles — SS can act as another client. The challenges that are as large or larger:

1. **Identity history, not roles**: both consoles now require LF login, so there is no anonymous-contributor gap going forward. What remains is mapping the LF identity to EasyCLA user records — including **historical signatures created before the login requirement**, which may carry no LF username — so M1's aggregation and unmatched-user telemetry still matter, at lower risk than roles-as-blocker framing suggests. *(M1 as implemented resolves identity server-side in EasyCLA from three sources: the caller's EasyCLA records, the platform user-service, and the Auth0 Management API.)*
2. **Two Go API surfaces**: contributor-critical endpoints run on the legacy `/v1`/`/v2` Go surface (`cla-backend-legacy`) alongside `/v3`/`/v4` — no Python port is needed (already done), but M5 must still absorb or retire a second API codebase and deployment.
3. **Corporate Console's hidden BFF**: M3 is not only 160 components but also absorbing a GraphQL aggregation backend.
4. **M5's blast radius**: webhooks (GitHub/GitLab/Gerrit), DocuSign callbacks, DynamoDB-stream-driven lambdas (events, notifications, zip building), and 19 tables — far beyond "move the API".

## 4. Cross-cutting risks

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Identity mapping gaps (LF account ↔ EasyCLA user records, esp. pre-LF-login history) | Users see empty/partial "My CLAs"; support load | Shipped in M1: EasyCLA-side identity resolution (EasyCLA records + user-service + Auth0) with `skippedIdentities`/unmatched telemetry |
| ACS role assignment is async | Designee/manager flows in SS inherit today's retry-loop fragility | Keep retries server-side in SS; don't promise synchronous UX |
| Dual-console period (feature drift) | Fixes must land twice | Freeze console feature work per area once its SS milestone starts; the Contributor Console is now retained indefinitely as M2's hand-off target, so its signing flows stay maintained |
| GitLab contributors can't start signing from SS | Sign-CLA entry gap for GitLab-only CLA groups | Shipped as a documented M2 gap: GitLab-only groups are blocked with an explanatory dialog until SS can verify a GitLab identity; the PR/MR remediation path is unaffected |
| Legacy `/v1`/`/v2` Go surface drifts from `/v4` during migration | Contributor flow outages | Single ownership of both surfaces; extend the existing parity/contract tests; absorb in M5 |
| M5 rework of M3–M4 adapters | Wasted effort | Q3 decided at this review; keep SS↔EasyCLA integration behind one server-side module |
| DocuSign webhook/timing edge cases resurface in new UI | "Signed but still red PR" complaints | Reuse backend flow untouched; add truthful pending states in SS |
| CLA permissions invisible to the OpenFGA/v2-Swagger-based automatic docs generation (architecture review, 2026-07-20) | Permission docs gap during M3/M4 | Document role-bridge behavior manually (M3 exit criterion); resolved when CLA enters OpenFGA at M5 |

## 5. What was missing from the original framing

Beyond the five milestones as described, the program must also account for:

- **Gerrit and GitLab flows** (contributor console and PCC both manage them) — explicitly scoped per milestone.
- **The legacy `/v1`/`/v2` Go surface** (`cla-backend-legacy`) — a second API codebase inside the blast radius even for "UI-only" milestones.
- **Corporate Console BFF retirement** (M3) and **easycla-landing-page** (originally attached to the old M3 decommission package; retirement now deferred along with the Contributor Console's — see doc 02).
- **Sanctions screening (SSS)** enforcement surfacing in new corporate flows.
- **Email/notification templates** that embed console URLs (manager notifications, invites) — must be re-pointed at SS per milestone.
- **Auto-create-ECLA** behavior coupling Approved List edits to signature creation (M3 parity item).
- **Signature invalidation** semantics (PCC feature, M4).
- **Metrics/insights endpoints** consumed by the Corporate Console (M3).
- **API consumers beyond the consoles** (anyone calling v3/v4 directly; audit before M5 contract changes).
- **v1 user-service/org-service IDs inside v4 payloads** (user-service deprecation is anticipated in the LFX v2 transition; org-service has no announced deprecation): audit where they appear and resolve for rendering via the `lfx.lookup_v1_user_sfid.by_username` / `.by_email` NATS RPCs (lfx-v1-sync-helper) (users) and the v1 org service over the api-gw secondary token (orgs) — architecture-review action 2026-07-20.
- **Decommission work as first-class scope**: DNS/CDN teardown, redirect stubs for bookmarked console URLs, support-doc updates in lfx-product-documentation.
- **Parity long-tail from the product documentation** (lfx-product-documentation/easycla/v2-current, reviewed 2026-07-11):
  - **Email-based CCLA signatory signing**: the CLA signatory signs via an emailed DocuSign link and **does not need an LF SSO account** — a distinct UX path that must survive M3 (don't force signatories into SS).
  - **Embargo/OFAC checkbox**: mandatory attestation gating the Sign button on ICLA/CCLA — required in M3's CCLA signing UI (M2 as implemented runs no signing UI; the Console keeps its checkbox).
  - **ICLA status model**: docs show Active / Incomplete / Disabled / Invalidated — verified as **UI-derived labels** over the stored `signed`/`approved` booleans; approval-criteria deletion invalidates related acknowledgements (`invalidateSignatures`, verified) and PMs can invalidate ICLAs (verified endpoint) — surfaced in M1/M2's computed `status` (`invalidated` pill + `invalidatedAt`); M4 admin parity.
  - **Multi-PR behavior**: signing updates only one PR — verified mechanism: a single `active_signature:{userID}` KV record holds one return context; other PRs re-check via the `/easycla` comment command (verified handler) — unchanged by M2 (the ceremony stays in the Console); becomes SS UX copy only if signing ever moves natively.
  - **Manual signing fallback**: templates carry a project contact email enabling offline/email CLA signing outside DocuSign.
  - **ECLA version tracking**: acknowledgements record which CCLA version they were made under (M1 display candidate, M3 table parity).
  - **Rules**: one CLA group per project (hierarchy validation exists in `cla_groups` service; trace fully in M4); "cannot delete the last CLA Manager" is **documented but no code guard was found** in the v4 backend or Corporate Console — verify/decide enforcement during M3; a user's CLA role attaches to a single company at a time (documented known issue — constrains M3 role bridging).
  - **Gerrit constraints**: instances are LF-hosted and added via support ticket (not self-service), CLA enablement is all-or-nothing per instance, and contributors must sign out/in of Gerrit after signing — M2's Gerrit hand-off (LF-username confirmation card → Console) and M4's admin surface are correspondingly narrower than GitHub.
  - **Ops details**: EasyCLA GitHub App needs Merge Queue read permission (else checks hang in "Expected"); auto branch protection covers only the default branch; PMs get automated emails on repo rename/archive/delete.

## 6. Sequencing rationale & effort signal

M1 (S, **shipped**) → M2 (M, **shipped**; single proactive-picker/hand-off flow, no per-platform split — revised 2026-08-04) → M3 (XL) → M4 (L, decision-gated) → M5 (XL, or XXL with Postgres; decision-gated). M1–M2 built the contributor path; M4 is parallelizable after M3's role-bridging pattern exists; M5 is gated on a separate go/no-go with the with/without-database analysis in doc 05.

Former open decisions Q1–Q3 are resolved in [spec.md](spec.md): contributor login is already mandatory in the console (no anonymous path needed); all three git platforms were in scope via per-platform sub-milestones — as implemented, M2 ships GitHub and Gerrit sign entry, with GitLab-only groups blocked pending a GitLab identity-verification story; and sequencing is UI-first with M5 separately gated (the hybrid-strangler variant — an early CLA read/query V2 service — is the noted alternative if M5 is committed early).
