<!-- SPDX-License-Identifier: CC-BY-4.0 -->
# EasyCLA → LFX Self Serve Integration — Overview

**Feature Branch**: `001-easycla-ss-integration`
**Created**: 2026-07-11
**Status**: Draft — for Architecture Review & PM scope approval
**Authored with**: Claude **Opus** 4.8 (tagged for an Opus-vs-Fable session comparison)

> **Audience.** This document set is written for the LFX architecture team and product management. It is deliberately technology-aware (it names the real services and data shapes involved) because its purpose is an architecture review, not a customer-facing spec. Each milestone has its own file with user stories, scope, requirements, risks, and an effort indication.

---

## 1. Purpose

Consolidate the two standalone EasyCLA end-user web apps — the **CLA Contributor Console** (`easycla-contributor-console`) and the **Corporate CLA Dashboard** (`lfx-corp-cla-console`) — and the EasyCLA project-configuration surface currently in **PCC** into **LFX Self Serve (SS)** (`lfx-self-serve`, app `apps/lfx-one`), while maintaining feature parity. A later, optional milestone re-platforms the EasyCLA API itself onto Kubernetes as an LFX "V2" service.

The work is broken into six milestones that are independently valuable and independently shippable, ordered so that each earlier milestone reduces risk for the next.

---

## 2. What EasyCLA is (current-state, code-grounded)

EasyCLA is the Linux Foundation's Contributor License Agreement service. It gates GitHub PRs and Gerrit/GitLab reviews on CLA authorization and lets contributors sign agreements. Three agreement types exist, and the distinction drives most of this plan:

| Type | Meaning | Signature record (DynamoDB `cla-{stage}-signatures`) | Signed PDF? |
|------|---------|------------------------------------------------------|-------------|
| **ICLA** | Individual CLA — a person signs for themselves | `signature_type=cla`, `reference_type=user`, `user_ccla_company_id` empty | **Yes** (S3 `contract-group/{claGroup}/icla/{user}/{sig}.pdf`) |
| **CCLA** | Corporate CLA — a company signs, covering its employees | `signature_type=ccla`, `reference_type=company` | **Yes** (S3 `.../ccla/{company}/{sig}.pdf`) |
| **ECLA** | Employee acknowledgement under a company's CCLA | `signature_type=cla`, `reference_type=user`, **`user_ccla_company_id` set** | **No** — it is an approval record only; `signed=true` is written programmatically, no DocuSign |

**Confirmed against code** (`cla-backend-go/signatures/dbmodels.go`, `utils/constants.go`, `signatures/repository.go`): ECLAs have no signed PDF; ICLAs and CCLAs do. This confirms the assumption in the original request.

### 2.1 Backends

- **`cla-backend-go`** (Go, primary): serves `/v3` (v1 product, us-east-1) and `/v4` (v2 product, us-east-2). **Owns the DocuSign integration** (`v2/sign/`), signing flows, PR-gating callbacks, PDF storage, and the role writes to ACS / Organization Service. Deployed as **AWS Lambdas behind the LFX API Gateway (Traefik)**.
- **`cla-backend`** (Python, legacy): Gerrit interaction and some `/v1`/`/v2` endpoints. **No active DocuSign integration.** Being superseded by Go.
- **A separate GraphQL backend** (`lf-backend-cla.<env>.platform.linuxfoundation.org/graphql`) backs the Corporate Console. **Its source was not located in the `easycla` repo** during research — see Risk R1.

### 2.2 The consoles today (integration seams)

- **Contributor Console** (Angular 13): entered from a GitHub/Gerrit/GitLab PR "signed agreement missing" link → individual-vs-corporate choice → ICLA/ECLA flows. Calls Go `/v4` (and legacy Python `/v2`) REST.
- **Corporate Console** (Angular 13 + NgRx + **Apollo GraphQL**): CCLA signing, approval-list management (email/domain/GitHub org/GitLab group), CLA-manager add/remove + designee, activity logs, contributor acknowledgements, sanctioned-org handling, foundation-level CLA. Calls the **GraphQL** backend + a BFF.
- **PCC EasyCLA** (`lfx-pcc`, `Tools > CLA`): CLA-group create/edit, GitHub App install + repo enrollment, GitLab/Gerrit connect, **PDF template management**, signatures view, approval criteria, events/CSV. Proxies to Go `/cla-service/v4/*` via PCC's own BFF.

### 2.3 Critical property for signing milestones

**DocuSign is 100% backend-mediated.** No frontend (neither console) integrates DocuSign directly. The Go backend creates the envelope and returns a `signUrl`; the frontend merely redirects to it, and DocuSign redirects back to a `returnUrl`. Callbacks (`/v4/signed/...`) write the signed PDF to S3 and post the PR status. **Implication: SS does not need a DocuSign integration or a new "DocuSign micro-service" — it calls the existing Go signing API and follows the `signUrl`.** (This directly answers, and closes, the "should we build a small DocuSign service?" question in the original brief: **no**.)

---

## 3. Target architecture in SS

SS is **Angular 20 (signals, zoneless) + an Express SSR BFF**, monorepo `apps/lfx-one` + `packages/shared`, authenticated with **Auth0** (`express-openid-connect`). It organizes features into four **lenses**: **Me**, **Foundation**, **Project**, **Org**.

The established data-flow pattern is a **Backend-for-Frontend (BFF)**:

```
Angular component
  → Angular service (HttpClient) → GET/POST /api/cla/...
    → Express route → controller → server service (ApiClientService, bearer-token pass-through)
      → upstream EasyCLA API (Go /v4 REST)
```

**Every EasyCLA milestone up to M6 integrates through this seam: SS's BFF calls the existing EasyCLA Go `/v4` REST API.** No EasyCLA backend rewrite is required to migrate the UIs. Lens mapping:

- **Me lens** → M1 (read-only agreements), M2 (sign ICLA), M3 (sign ECLA)
- **Org lens** → M4 (CCLA management, from Corporate Console)
- **Project lens** → M5 (EasyCLA project config, from PCC)

---

## 4. The central challenge: the role-model gap

The original brief correctly identified role differences as the crux. Grounded in code, here is the actual gap:

**EasyCLA authorization (today) = ACS (RBAC).** EasyCLA uses its own role namespace — `project-manager`, `cla-manager`, `cla-manager-designee`, `cla-signatory`, `company-admin` (plus `cla-program-manager`) — living in **ACS (Access Control Service)**, a **PostgreSQL-backed, Go RBAC service** (roles → policies → statements → resource-actions; deployed on ECS/API-Gateway). Crucially, **the entire CLA authorization model is already defined declaratively as data**:
- CLA roles are **seeded in ACS** (`acs/db/init.sql`), scoped to object types `project`, `organization`, and the composite `project|organization`.
- The full CLA permission surface — ~100 API resources, 45+ policies, and role→policy mappings (e.g. `cla-manager → FullAccessSignatures + AddDeleteCLAManager + UpdateSignatureApprovalLists`) — is declared in **`acs-cli/services/11-cla-service.yaml`** (~1271 lines) and applied via the `acs-cli sync` command.
- At runtime, EasyCLA calls **ACS** for role lookup/invites and the **Organization Service** to store/check scope grants (`orgClient.IsUserHaveRoleScope(...)`); the API Gateway runs a generic ACS authorizer middleware; some state also lives in the DynamoDB signature record's `SignatureACL`.

**LFX V2 / SS authorization (target) = OpenFGA (ReBAC).** **OpenFGA** relationship tuples (`user → relation → object`) enforced via the **Heimdall** gateway, kept in sync by **`lfx-v2-fga-sync`** (now a *generic* sync service — see below). **`lfx-v2-auth-service` is authentication (Auth0/NATS/JWT + user lookup), *not* authorization** — correcting the brief's assumption that SS role management lives there.

**ACS and OpenFGA are two entirely separate authorization systems with no integration, sync, or bridge between them** (verified: zero cross-references in either codebase). ACS is the **incumbent RBAC** that every V1 service (including EasyCLA) depends on; OpenFGA is the **strategic ReBAC** for V2. This is not "old vs deprecated" — ACS is actively maintained with no announced sunset — but they are **mechanically different models** (RBAC roles/policies + composite scopes in ACS/Org-Service, vs. relationship tuples in a single graph engine). Moving EasyCLA authz onto OpenFGA is **net-new bridging work that does not exist today**.

**Encouraging finding for the eventual migration:** `lfx-v2-fga-sync` is now **generic** — any object type added to the OpenFGA model (in `lfx-v2-helm/.../openfga/model.yaml`) can be synced via four standard NATS subjects (`update_access`/`delete_access`/`member_put`/`member_remove`) with **no fga-sync code change**. It supports `exclude_relations` (staged dual-source cutover), `mutually_exclusive_with` (role transitions), an `lfx.access_check.request` subject, and an audit tool for tuple-change verification. The current model has `project`, `b2b_org` (company), etc. but **no CLA types** — EasyCLA would add `cla_group`/`cla_signature`/… referencing the existing `project`/`b2b_org` types. The `acs-cli` CLA YAML is effectively a **ready-made source inventory** for that OpenFGA model. This makes the M6 convergence concretely scopeable (see M6).

### 4.1 Recommendation — Adapter now, converge at M6 (with an explicit two-layer authz model)

> **This is a recommendation for the architecture team to ratify, not a settled decision.** The tradeoffs are laid out so it can be overridden.

**Do NOT build an EasyCLA↔OpenFGA sync before the M6 rewrite.** The ACS research **strengthens** this: there is **no existing ACS→OpenFGA bridge** anywhere in the platform, so "map to OpenFGA early" would mean EasyCLA *pioneers* a bespoke sync between the two systems while still writing ACS/Org-Service (which it must, because PR-gating and — until they are retired — both consoles depend on ACS). That OpenFGA mirror would be a perpetually-lagging second source of truth. Authorization drift either blocks legitimate CLA managers (support load) or over-grants (compliance incident). That is a large, permanent tax paid across three milestones to make the eventual rewrite marginally cheaper — a bad trade.

Instead, across M3/M4/M5 SS uses a **two-layer authorization model**:

1. **Coarse lens gate — platform OpenFGA/Heimdall.** "Can this user open the Org lens for company X / the Project lens for project Y at all?" Reuse SS's existing mechanism unchanged.
2. **Fine-grained CLA authorization — EasyCLA adapter.** "Is this user a `cla-manager` for this company+project? May they edit the approval list / add a manager?" **Delegate to the EasyCLA `/v4` API**, which already owns and enforces this. SS renders the UI; EasyCLA remains the single source of truth for CLA role state — **no dual-write**.

**Then converge at M6.** The Goa rewrite is the correct and only place to move CLA authorization onto OpenFGA — a single cutover rather than a multi-year sync. And it is now **concretely scopeable**: the `acs-cli/services/11-cla-service.yaml` role→policy→resource declaration is a ready-made inventory to translate into an OpenFGA model, and fga-sync's generic contract + `exclude_relations` gives a staged, verifiable cutover mechanism (detailed in M6).

**⚠️ Risk this creates (must be designed for): the two layers can disagree.** A user may have platform Org-lens access yet *not* be a CLA manager. **CLA-specific mutating actions must be authorized by the EasyCLA layer, never inferred from the lens gate**, or M4 will silently grant CLA management to anyone with org access. This is called out as Risk R2 and is a hard requirement in M4.

*(Alternatives considered and rejected for now: "map to OpenFGA early at M3" — rejected due to dual-write drift; "leave as an open spike" — rejected because role handling is the core of M3–M5 and cannot be left unscoped for review.)*

---

## 5. Cross-cutting risks & open items

| ID | Risk / open item | Milestone(s) | Disposition |
|----|------------------|--------------|-------------|
| **R1** | Corporate Console's **GraphQL backend** (`lf-backend-cla`) source not located in `easycla` repo. | M4 | **Action item before M4 build**: locate/inventory it. **Design target = the Go `/v4` REST API** (per decision); treat GraphQL as legacy to bypass. |
| **R2** | **Two-layer authz disagreement** (platform lens access ≠ CLA role). | M3, M4, M5 | Hard requirement: CLA mutations authorized by EasyCLA layer only. |
| **R3** | **PR-gating webhook machinery** (GitHub/Gerrit/GitLab status checks) is what generates the "signed agreement missing" entry links. It stays in the Go backend across M1–M5 and is only touched in M6. | all | **Retiring the consoles does NOT retire gating.** Made explicit so scope is not misread. |
| **R4** | **User-identity mapping.** EasyCLA keys signatures by an internal user UUID and separately stores GitHub/GitLab/LF-username/email. SS identifies users via Auth0. M1 must resolve "SS Auth0 user → EasyCLA user id(s)" reliably (incl. users with multiple linked identities). | M1 (and reused after) | Design the identity-resolution step in M1; reused by all later milestones. |
| **R5** | **DynamoDB → Postgres** is a *separate* decision from K8s migration; the newest V2 service (`lfx-v2-member-service`) actually **dropped Postgres** for Salesforce + OpenSearch + NATS-KV. | M6 | **Decouple** (see M6). Lift to K8s first behind a repository interface; justify any DB change independently. |
| **R6** | **Deep-link / return-URL contract.** PR checks currently deep-link to the Contributor Console with `projectId`/`userId`/`redirect`. Redirecting to SS instead (M2/M3) changes a contract the Go backend and PR-comment templates emit. | M2, M3 | Backend PR-comment link target becomes configurable; staged cutover. |
| **R7** | **Gerrit / GitLab flows** have console-specific handling (e.g. Gerrit logout/login after signing). Parity must not silently drop non-GitHub providers. | M2, M3, M5 | Explicitly in-scope for parity; called out per milestone. |
| **R8** | **"V2 service" is a heavy, specific stack** (Go + Goa DSL + NATS JetStream + OpenFGA/Heimdall + Helm/ArgoCD + OTel), not merely "runs in K8s." | M6 | Scoped in M6; effort reflects a rewrite, not a lift-and-shift. |
| **R9** | **No ACS→OpenFGA bridge exists.** EasyCLA authz lives in ACS (RBAC, PostgreSQL) + Org-Service; the V2 target is OpenFGA (ReBAC). The two are entirely separate with no sync — moving CLA authz to OpenFGA is net-new work. Mitigating: `acs-cli/services/11-cla-service.yaml` is a ready-made role→policy inventory to translate, and fga-sync is generic (add CLA object types to the model, publish standard messages). | M6 | Owned by M6 (convergence point). Do **not** attempt before M6 (see §4.1). |
| **R10** | **ACS is not "deprecated," it is incumbent.** Every V1 service depends on ACS; there is no announced sunset. Framing CLA authz migration as "retire ACS" overstates scope — M6 moves *CLA's* authz to OpenFGA; ACS persists for other consumers. | M6 | Scope M6 to CLA object types only; do not take on platform-wide ACS retirement. |

---

## 6. Milestone sequencing

```
M1  Read-only agreements in Me lens        (no EasyCLA backend change; BFF wiring + identity mapping)
      │  establishes: SS↔EasyCLA BFF seam, user-identity resolution (R4)
      ▼
M2  Sign ICLA in SS (Me lens)              (reuse existing /v4 signing + signUrl redirect; change PR link target)
      │  establishes: SS as a signing entry point; return-URL contract (R6)
      ▼
M3  Sign ECLA in SS (Me lens)              (introduces CLA roles: manager/designee/signatory via adapter)
      │  → Contributor Console can be retired after M3
      ▼
M4  CCLA management in Org lens            (Corporate Console parity; heaviest role surface; GraphQL risk R1)
      │  → Corporate Console can be retired after M4
      ▼
M5  EasyCLA project config in Project lens (PCC parity: GH app, repos, templates)
      │  → EasyCLA removed from PCC after M5
      ▼
M6  EasyCLA API → Kubernetes V2 service    (optional/strategic; DB decision decoupled)
```

**Retirement gates:** Contributor Console after **M3**; Corporate Console after **M4**; EasyCLA-in-PCC after **M5**. Each retirement is a distinct decision requiring the corresponding milestone to have reached parity and burned in.

---

## 7. Did the brief miss anything?

Additions surfaced by the research, now folded into the milestones:

1. **User-identity resolution (R4)** — a prerequisite the brief didn't name; scoped into M1.
2. **PR-gating stays put (R3)** — clarifies that UI migration ≠ gating migration.
3. **Return-URL / deep-link contract (R6)** — the mechanism that routes contributors from a PR to a UI; must be re-pointed for M2/M3.
4. **The Corporate Console's separate GraphQL backend (R1)** — a hidden dependency for M4.
5. **DocuSign already centralized in Go** — the proposed "small DocuSign service" is unnecessary.
6. **Non-GitHub providers (Gerrit/GitLab)** — parity must cover them (R7).
7. **CCLA also has a PDF** (not only ICLA) — relevant to M1's download scope.

---

## 8. Milestone documents

- [01 — Read-only agreements in Me lens](01-milestone-read-only-me-lens-opus.md)
- [02 — Sign ICLA in SS](02-milestone-sign-icla-opus.md)
- [03 — Sign ECLA in SS](03-milestone-sign-ecla-opus.md)
- [04 — CCLA management in Org lens](04-milestone-ccla-org-lens-opus.md)
- [05 — EasyCLA project config in Project lens](05-milestone-project-lens-pcc-opus.md)
- [06 — EasyCLA API → Kubernetes V2 service](06-milestone-k8s-v2-api-opus.md)

> **Effort indications** in the milestone docs are **T-shirt sizes (S/M/L/XL)** for relative planning only — not commitments. They assume the adapter strategy in §4.1.
