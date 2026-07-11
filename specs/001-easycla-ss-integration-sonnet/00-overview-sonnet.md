<!-- SPDX-License-Identifier: CC-BY-4.0 -->
# EasyCLA → LFX Self Serve Integration — Overview

**Feature Branch**: `001-easycla-ss-integration`
**Created**: 2026-07-11
**Status**: Draft — for Architecture Review & PM scope approval
**Authored with**: Claude Sonnet 5

> **Audience.** Written for the LFX architecture team and product management. It is deliberately technology-aware (names real services and data shapes) because its purpose is an architecture review, not a customer-facing product spec. Each milestone has its own file with scope, requirements, risks, and a relative effort indication.

---

## 1. Purpose

Consolidate two standalone EasyCLA end-user web apps — the **CLA Contributor Console** (`easycla-contributor-console`) and the **Corporate CLA Dashboard** (`lfx-corp-cla-console`) — plus the EasyCLA project-configuration surface currently embedded in **PCC** (`lfx-pcc`) — into **LFX Self Serve** (`lfx-self-serve`, app `apps/lfx-one`), while maintaining feature parity. A later, separately-decided milestone re-platforms the EasyCLA API itself onto Kubernetes as an LFX "V2" service, optionally alongside a DynamoDB→Postgres migration.

The work is broken into seven milestones (a role-unification milestone plus six functional milestones), sequenced so each earlier milestone reduces risk for the next and can retire a legacy surface once it lands.

---

## 2. What EasyCLA is today (code-grounded)

EasyCLA gates GitHub PRs and Gerrit/GitLab reviews on CLA authorization and lets contributors and companies sign agreements. Three agreement types exist, and the distinction drives most of this plan.

| Type | Meaning | Key DB fields (`cla-backend-go/signatures/models.go:87-91`) | Signed PDF? |
|------|---------|---|---|
| **ICLA** | Individual signs for themselves | `signature_type=cla`, `reference_type=user`, no company id | **Yes** — stored in S3 at `contract-group/{claGroupID}/icla/{userID}/{signatureID}.pdf` (`signatures/handlers.go:59-62`) |
| **CCLA** | A company signs, covering its employees | `signature_type=ccla`, `reference_type=company` | **Yes** — S3 `contract-group/{claGroupID}/ccla/{companyID}/{signatureID}.pdf` (`signatures/handlers.go:105`) |
| **ECLA** | Employee acknowledgment under a company's existing CCLA | `signature_type=cla`, `reference_type=user`, **company id set**, `auto_create_ecla` flag | **No** — pure DB record, no DocuSign envelope, no PDF |

This confirms the user's original assumption: ECLAs are agreement-only records, not signed documents. CCLA also has a PDF — relevant scope detail for Milestone 1's "download signed PDF" requirement (it's not ICLA-only).

### 2.1 Backends

- **`cla-backend-go`** (Go, primary): serves `/v3` (v1 product, us-east-1) and `/v4` (v2 product, us-east-2, LFX-Platform-integrated). Owns the DocuSign integration (`v2/sign/`), PR/Gerrit/GitLab gating callbacks, PDF storage, and role writes via the ACS client. Deployed as AWS Lambdas behind the LFX API Gateway (Traefik).
- **`cla-backend`** (Python, legacy): Gerrit interaction and some `/v1`/`/v2` endpoints, no DocuSign logic of its own. Being superseded by Go.
- **DynamoDB**: sole datastore, ~14 tables (signatures, companies, projects, repositories, github/gitlab orgs, gerrit instances, manager requests, approvals, template, store, users, metrics).

### 2.2 The consoles today (integration seams)

- **Contributor Console** (`easycla-contributor-console`, Angular): entry point is a "signed agreement missing" link from a GitHub/Gerrit/GitLab PR check → individual-vs-corporate choice → ICLA sign (DocuSign redirect) or ECLA/approval-list flow. Calls Go `/v4` REST.
- **Corporate Console** (`lfx-corp-cla-console`, Angular + Apollo GraphQL): CCLA signing, approval-list management (email/domain/GitHub-org/GitLab-group), CLA-manager add/remove + designee, activity logs, contributor acknowledgements. Its data layer is **Apollo GraphQL** against `https://lf-backend-cla.platform.linuxfoundation.org/graphql` (`frontend/src/environments/environment.prod.ts:26`) — **this GraphQL server's source was not found in any locally cloned repo.** Treat it as an external dependency to be inventoried before Milestone 4 (see Risk R1).
- **PCC EasyCLA** (`lfx-pcc/apps/v1-frontend`, `modules/tools/cla/`): CLA-group create/edit, GitHub App install + repo enrollment, GitLab/Gerrit connect, PDF template management (with form-field/anchor mapping), approval-criteria config, CLA-manager assignment, signature/event views. Proxies to the Go backend via PCC's own BFF (`v1-backend/src/modules/cla-services/`).

### 2.3 DocuSign is entirely backend-mediated

Confirmed at `cla-backend-go/v2/sign/docusign.go:481-529` (`GetSignURL`): the Go backend calls the DocuSign API directly, creates the envelope, and returns an embedded-signing `signUrl`. The frontend's only job is to redirect the browser to that URL; DocuSign redirects back to a `returnUrl` after signing. **No frontend, in either console, integrates DocuSign directly, and there is no separate DocuSign microservice.**

**This directly answers and closes the user's "should we build a small DocuSign service?" question: no.** Self Serve does not need new DocuSign integration work — its backend-for-frontend simply calls the existing `/v4` signing endpoints and redirects to the returned `signUrl`, exactly as the Contributor Console does today. Building a dedicated signing microservice now would duplicate logic that Milestone 6 (API rewrite) will restructure anyway.

---

## 3. Target architecture in Self Serve

Self Serve (`lfx-self-serve/apps/lfx-one`) is Angular + an Express server-side BFF layer (`src/server/services/`), organized into lenses (`src/app/modules/`: crowdfunding, mentorship, committees, meetings, etc. — no CLA-related module exists yet). The established data-flow pattern:

```
Angular component → Angular service (HttpClient) → BFF route (Express)
  → BFF server service → upstream EasyCLA Go /v4 REST API
```

**Every milestone through Milestone 5 integrates through this seam** — Self Serve's BFF calls the existing EasyCLA `/v4` API. No EasyCLA backend rewrite is required to migrate the UIs; the rewrite is deliberately deferred to Milestone 6.

Lens mapping, per the user's framing:
- **Me lens** → Milestone 1 (read-only agreements), Milestone 2 (sign ICLA), Milestone 3 (sign ECLA)
- **Organization lens** → Milestone 4 (CCLA management)
- **Project lens** → Milestone 5 (EasyCLA project config, replacing the PCC surface)

---

## 4. The central challenge: the role-model gap

The user's instinct — that role differences are the crux — is correct, and the research sharpens exactly where the gap is.

**EasyCLA authorization today = ACS, a role/policy RBAC system.** EasyCLA-specific roles (`cla-manager`, `cla-manager-designee`, `cla-signatory`, `project-manager`, `cla-program-manager`, and others) are declared as data, not code, in `acs-cli/services/11-cla-service.yaml` (1271 lines: ~40+ resources, role→policy→resource-action mappings — e.g. `cla-manager` maps to full signature access, manager add/remove, approval-list updates, auto-create-ECLA). At runtime the Go backend calls the ACS client to check/grant these roles; the API Gateway also runs an ACS-authorizer middleware. This is a mature, working system — not an EasyCLA-specific hack.

**Self Serve / LFX V2 authorization = OpenFGA, a relationship-based (ReBAC) system**, synced from NATS messages by `lfx-v2-fga-sync`, a genuinely generic service: it exposes four standard operations (`update_access`, `delete_access`, `member_put`, `member_remove`) that work for **any** object type defined in the platform's OpenFGA model, with no code change required to add a new type (`lfx-v2-fga-sync/handler_generic.go`). The current deployed model (per the platform's ReBAC design references) defines generic governance types (`user`, `team`, `project`, `committee`, `meeting`) — **no CLA-specific types exist in it today.**

**These are two separate, unbridged systems.** No sync, no shared source of truth, no code cross-referencing the other. This is not "EasyCLA using a deprecated pattern" — ACS is the incumbent, actively-used authorization system for essentially all V1 LFX services, with no announced retirement. Moving CLA authorization onto OpenFGA is net-new integration work, not a migration of something already half-done.

**Encouraging finding:** `acs-cli/services/11-cla-service.yaml` is effectively a ready-made inventory of every CLA role, resource, and permission — exactly what would need translating into OpenFGA relation types. And `lfx-v2-fga-sync`'s generic contract means adding CLA object types doesn't require new sync-service code, only a model change and message-publishing from whichever service owns CLA state. This makes eventual convergence concretely scopeable rather than an open-ended unknown — see the dedicated Milestone 0 doc.

### 4.1 Recommendation: adapt now, converge later, as its own milestone

Rather than fold role-bridging piecemeal into Milestones 3, 4, and 5 (each of which needs *some* CLA-role awareness), this plan carves it out as **Milestone 0**, sequenced before Milestone 3 (the first milestone whose write-actions require role checks). Milestones 1 and 2 need no new role work — read-only display and self-service ICLA signing require no manager/signatory concept.

Milestone 0's recommended shape: build a **thin adapter layer**, not a bidirectional sync. Self Serve's BFF continues to use the platform's coarse OpenFGA/lens-level gate to answer "can this user open the Organization lens for company X at all," but delegates all fine-grained CLA-role questions ("is this user a cla-manager for this project+company," "may they edit the approval list") to the existing EasyCLA `/v4` API, which remains the single source of truth. No dual-write, no drift risk from an early, partial OpenFGA mirror. Full detail, risks, and the eventual Milestone-6 convergence path are in the Milestone 0 document.

**Risk to flag now:** the two authorization layers can disagree — a user may have generic Organization-lens access in Self Serve without being a CLA manager for that org's CLA. Every CLA-mutating action in Milestones 3–5 must check the EasyCLA layer explicitly; it must never be inferred from lens access. This is a hard requirement, not a suggestion, and is repeated in each affected milestone doc.

---

## 5. Cross-cutting risks and open items

| ID | Risk / open item | Milestone(s) | Disposition |
|----|---|---|---|
| R1 | Corporate Console's GraphQL backend (`lf-backend-cla.platform.linuxfoundation.org/graphql`) source was not found in any locally cloned repo. | M4 | Action item before M4 build: locate and inventory it, or confirm it is being decommissioned in favor of direct `/v4` REST calls. |
| R2 | Two-layer authorization disagreement (platform lens access ≠ CLA role). | M0, M3, M4, M5 | Hard requirement: CLA-mutating actions are authorized by the EasyCLA layer only, never inferred from lens access. |
| R3 | PR/Gerrit/GitLab gating webhook machinery (what generates "signed agreement missing" links) stays in the Go backend through M1–M5; only touched in M6. | all | Retiring the consoles does not retire gating — call this out explicitly so scope isn't misread. |
| R4 | User-identity mapping: EasyCLA keys signatures by an internal user id and separately stores GitHub/GitLab username and email; Self Serve identifies users via its own auth. Milestone 1 must resolve "Self Serve user → EasyCLA user record(s)" reliably, including users with multiple linked identities. | M1 (reused by all later) | Design the identity-resolution step in M1. |
| R5 | DynamoDB→Postgres is a separate decision from the Kubernetes move. | M6 | Decouple explicitly — see M6 doc. |
| R6 | Deep-link / return-URL contract: PR checks currently link to the Contributor Console with project/user identifiers. Redirecting to Self Serve instead (M2/M3) changes a contract the Go backend and PR-comment templates emit. | M2, M3 | Backend PR-comment link target must become configurable; staged cutover required. |
| R7 | Gerrit/GitLab flows have console-specific handling distinct from GitHub. Parity must not silently drop non-GitHub providers. | M2, M3, M5 | Explicitly in scope for parity in each relevant milestone. |
| R8 | "V2 service" is a specific, heavier stack (Go backend + Kubernetes/Helm/ArgoCD deployment + OpenFGA-based authorization + shared platform conventions), not merely "runs in a container." | M6 | Scoped in M6; effort estimate reflects a rewrite, not a lift-and-shift. |
| R9 | No ACS↔OpenFGA bridge exists anywhere in the platform today. | M0, M6 | M0 uses an adapter, not a bridge; M6 is the convergence point. |
| R10 | CCLA also produces a signed PDF (not ICLA-only) — relevant to Milestone 1's download-PDF requirement. | M1 | Scope M1's PDF download to cover both ICLA and any CCLA signatures the user can see (e.g., as a company representative), not ICLA exclusively. |

---

## 6. Milestone sequencing

```
M0  Role/permission adapter for EasyCLA-in-Self-Serve   (no end-user feature; unblocks M3+)
      │  establishes: adapter pattern, identity-resolution groundwork
      ▼
M1  Read-only ICLA/ECLA agreements in Me lens           (no EasyCLA backend change; BFF wiring + identity mapping, R4)
      │  establishes: Self Serve↔EasyCLA BFF seam
      ▼
M2  Sign ICLA in Self Serve (Me lens)                   (reuse existing /v4 signing + signUrl redirect; change PR link target, R6)
      │  establishes: Self Serve as a signing entry point
      ▼
M3  Sign ECLA in Self Serve (Me lens)                   (first milestone needing CLA-role checks; uses M0 adapter)
      │  → Contributor Console retirement candidate after M3
      ▼
M4  CCLA management in Organization lens                (Corporate Console parity; heaviest role surface; GraphQL risk R1)
      │  → Corporate Console retirement candidate after M4
      ▼
M5  EasyCLA project config in Project lens               (PCC parity: GitHub App, repos, templates)
      │  → EasyCLA removed from PCC after M5
      ▼
M6  EasyCLA API → Kubernetes V2 service                  (separately decided; DB migration decoupled)
```

**Retirement gates:** Contributor Console after M3; Corporate Console after M4; EasyCLA-in-PCC after M5. Each retirement is its own decision, contingent on the corresponding milestone reaching parity and burning in with real usage — not an automatic consequence of code being merged.

---

## 7. Did the original brief miss anything?

Yes — five things, now folded into the milestones above:

1. **User-identity resolution (R4)** wasn't named but is a prerequisite; scoped into M1.
2. **PR-gating stays in the Go backend (R3)** — UI migration is not gating migration; worth stating explicitly so the architecture team doesn't assume M3 retires the webhook machinery.
3. **The return-URL/deep-link contract (R6)** — the mechanism that actually routes a contributor from a failing PR check to a signing UI must be re-pointed at Self Serve for M2/M3; this is a coordination point with the GitHub-App-side code, not just a frontend change.
4. **The Corporate Console's separate GraphQL backend (R1)** is a hidden dependency for M4 that needs inventory before that milestone can be scoped precisely.
5. **CCLA also has a signed PDF** — the brief assumed only ICLA does; confirmed CCLA does too, which affects M1's download scope (R10).

The user's own framing of "role differences are the main challenge" is correct and, if anything, understates it slightly: it's not just that the models differ, but that there is zero existing bridge between them anywhere in the platform, so every downstream milestone (M3–M5) depends on a design decision (M0) that doesn't yet exist in any form.

---

## 8. Milestone documents

- [00 — Role/permission adapter (cross-cutting)](00-milestone-role-adapter-sonnet.md)
- [01 — Read-only agreements in Me lens](01-milestone-read-only-me-lens-sonnet.md)
- [02 — Sign ICLA in Self Serve](02-milestone-sign-icla-sonnet.md)
- [03 — Sign ECLA in Self Serve](03-milestone-sign-ecla-sonnet.md)
- [04 — CCLA management in Organization lens](04-milestone-ccla-org-lens-sonnet.md)
- [05 — EasyCLA project config in Project lens](05-milestone-project-lens-pcc-sonnet.md)
- [06 — EasyCLA API → Kubernetes V2 service](06-milestone-k8s-v2-api-sonnet.md)

> **Effort indications** in the milestone docs are relative T-shirt sizes (S/M/L/XL) for planning discussion only, not commitments. They assume the adapter strategy in §4.1 / Milestone 0.
