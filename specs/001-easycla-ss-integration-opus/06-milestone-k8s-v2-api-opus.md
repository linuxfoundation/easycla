<!-- SPDX-License-Identifier: CC-BY-4.0 -->
# Milestone 6 — Move the EasyCLA API to Kubernetes as an LFX V2 service

**Status**: Draft · **Effort**: **XL (largest)** · **Retires**: EasyCLA Lambda deployment (eventually) · **Prereq**: M1–M5 recommended (so the API's only consumers are SS + PR webhooks, not multiple legacy UIs)

## Goal

Re-platform the EasyCLA API from **AWS Lambda behind the API Gateway** onto **Kubernetes as an LFX "V2" service**, and — as a **separate, independently-justified decision** — evaluate replacing **DynamoDB with Postgres** (or another store). Per scope approval, these two are **decoupled**: lift to K8s first behind a repository interface; change the datastore only if independently justified.

This is the largest and most strategic milestone and the one to be **most critical about** — it is a backend rewrite, not a lift-and-shift, because "V2 service" is a specific stack.

## What "V2 service" actually means (grounded in `lfx-v2-*` repos)
Not simply "runs in a container." The LFX V2 pattern (from `lfx-v2-member-service`, `lfx-v2-compliance-service`, `lfx-v2-helm`, `lfx-v2-fga-sync`) is:
- **Go + Goa v3 DSL** — API defined in a design DSL; HTTP servers + OpenAPI generated into `gen/`.
- **Transport**: HTTP REST (:8080) **and NATS JetStream** (RPC subjects `lfx.<svc>.<op>`, pub/sub events, KV caches).
- **Authorization**: **OpenFGA** relationship tuples, enforced via the **Heimdall** gateway, kept in sync by **`lfx-v2-fga-sync`** (services publish `lfx.fga-sync.*` on mutation). JWT via Heimdall for authn.
- **Deployment**: per-service **Helm chart** + the aggregate `lfx-platform` chart; **ArgoCD** GitOps; Chainguard/ko images.
- **Observability**: OpenTelemetry (logs/traces/metrics).
- **Data direction is not uniform**: the newest service (`member-service`) **dropped Postgres** in favor of Salesforce + OpenSearch (via a Query/Indexer service) + NATS-KV. So "V2 ⇒ Postgres" is **false** as a platform rule (**R5**).

**Implication**: re-implementing EasyCLA as a V2 service means adopting Goa, NATS, OpenFGA/Heimdall, Helm/ArgoCD, and OTel — a substantial rewrite of the API and, critically, of the **authorization model** (from ACS/Org-Service scopes + DynamoDB ACL → OpenFGA tuples). This is where the [role-model convergence](00-overview-opus.md#4-the-central-challenge-the-role-model-gap) finally happens.

### The authorization migration — concrete, and its own hardest sub-task
The ACS/fga-sync research makes this scopeable rather than hand-wavy:

- **Source of truth for the model already exists as data.** `acs-cli/services/11-cla-service.yaml` (~1271 lines) declares every CLA role → policy → resource mapping (e.g. `cla-manager → FullAccessSignatures + AddDeleteCLAManager + UpdateSignatureApprovalLists`), and `acs/db/init.sql` seeds the roles and their object-type scopes (`project`, `organization`, `project|organization`). **This is the translation input** for the OpenFGA model — you are not reverse-engineering authz from Go code.
- **fga-sync is generic** — no fga-sync code changes are needed. M6 adds CLA object types (e.g. `cla_group`, `cla_signature`, `cla_contract_signatory`) to the model in `lfx-v2-helm/.../openfga/model.yaml`, referencing the existing `project` and `b2b_org` (company) types for the company+project scoping that ACS expresses as the `project|organization` composite. The service then publishes `update_access`/`member_put`/`member_remove` on the standard NATS subjects.
- **Staged, verifiable cutover is supported.** `exclude_relations` lets ACS and OpenFGA co-own state during transition; the `lfx.access_check.request` subject and the fga-sync tuple-change audit tool support the equivalence proof (FR-6.3).
- **Scope discipline (R10).** M6 migrates *CLA's* authz to OpenFGA. It does **not** retire ACS platform-wide — ACS remains the incumbent for every other V1 consumer.

**This authorization translation + equivalence proof is the single riskiest and least-parallelizable part of M6.** The RBAC→ReBAC mapping is semantic, not mechanical: ACS policies (allow/deny statements over resource-actions) must be re-expressed as OpenFGA relations, and edge cases (deny statements, `is_appointed` grants, level distinctions like member/non-member/staff) may not have clean tuple equivalents. Budget for a design spike here before committing M6 effort.

## The DynamoDB → Postgres question (be critical)
The brief asks "should we?" — the honest answer is **decouple and default to *not now***:
- **Why decouple**: coupling a runtime re-platform (K8s) with a datastore migration multiplies blast radius — you'd be debugging Goa/NATS/OpenFGA *and* data-migration correctness simultaneously, against a service that gates every LF contribution.
- **DynamoDB's real pain** cited (performance/other issues) is worth validating: much of it may be **access-pattern/GSI design** rather than DynamoDB itself. A rewrite behind a clean repository interface lets you fix access patterns first and measure whether the store is still the bottleneck.
- **If a change is justified**, Postgres is one option — but the platform's own V2 direction (Salesforce + OpenSearch + NATS-KV) suggests the decision isn't automatically "Postgres." Evaluate against actual query/reporting needs (e.g. the Corporate activity-log/metrics queries that are awkward in DynamoDB).

**Recommendation**: **M6a** = API → K8s (Goa/NATS/OpenFGA), datastore unchanged behind a repository interface. **M6b** (optional, later, independently approved) = datastore evaluation & migration.

## User Scenarios & Testing
*(This milestone is engineering-facing; "users" are API consumers — SS, PR webhooks, remaining integrations.)*

### User Story 1 — Behavioral parity behind a new runtime (Priority: P1)
Every existing consumer (SS BFF, GitHub/Gerrit/GitLab PR callbacks, DocuSign callbacks) works unchanged against the K8s service.

**Acceptance Scenarios**:
1. **Given** a PR gated by EasyCLA, **When** a contributor signs (via SS), **Then** the K8s service records it and flips the check exactly as the Lambda did.
2. **Given** a DocuSign completion callback, **When** it hits the K8s service, **Then** the PDF is stored and the PR updated identically.

### User Story 2 — Authorization on OpenFGA (Priority: P1)
CLA roles (`cla-manager`/`cla-manager-designee`/`cla-signatory`/`company-admin`/`project-manager`/`cla-program-manager`) are re-expressed as OpenFGA object types + relations (translated from `acs-cli/services/11-cla-service.yaml`) and enforced via Heimdall, replacing ACS policy checks, Org-Service scope checks, and the DynamoDB ACL — with **identical effective permissions**, proven against a pre-migration snapshot.

**Acceptance Scenarios**:
1. **Given** an existing CLA Manager, **When** authorization is evaluated post-migration, **Then** they retain exactly the access they had (no over/under-grant), verified against a pre-migration permission snapshot.
2. **Given** an ACS policy that uses a deny statement or a level/appointed distinction with no direct tuple analog, **When** it is translated, **Then** the mapping decision is documented and covered by an equivalence test (not silently dropped).

### User Story 3 — Repository-interface isolation of the datastore (Priority: P2)
Data access sits behind a repository interface so M6b can swap stores without touching business logic.

### Edge Cases
- PR gating during cutover → zero-downtime / staged traffic shift; no unsigned contributions slip through.
- Permission drift during ACS→OpenFGA migration → snapshot-and-verify; block cutover on mismatch.
- DynamoDB features with no clean Postgres analog (or vice versa) if M6b proceeds → identified before migration.

## Requirements

### Functional
- **FR-6.1**: The K8s service MUST preserve behavioral parity for all existing API consumers and callbacks.
- **FR-6.2**: The service MUST adopt the V2 stack: Goa v3, HTTP + NATS, Helm/ArgoCD, OTel.
- **FR-6.3**: Authorization MUST move to OpenFGA/Heimdall with an **effective-permission-equivalence** guarantee vs the current ACS/Org-Service/ACL model, verified against a pre-migration snapshot.
- **FR-6.4**: Data access MUST be behind a repository interface; **the datastore change (M6b) MUST be a separate, independently-approved decision** (R5) — M6a keeps the current store.
- **FR-6.5**: Cutover MUST be staged and reversible with no PR-gating gap.

### Non-Functional
- **NFR-6.1**: Latency/throughput MUST meet or beat the Lambda deployment for the PR-check hot path.
- **NFR-6.2**: If M6b proceeds, a data-migration plan MUST include verification (row/record counts, spot-checks, dual-read window).

### Key Entities
- **OpenFGA authorization model** for CLA: new object types (e.g. `cla_group`, `cla_signature`, `cla_contract_signatory`) with relations (manager/signatory/designee/…), referencing existing platform types `project` and `b2b_org` (company) for the company+project scoping. Translated from the `acs-cli` CLA declaration.
- **fga-sync publish contract**: the standard `update_access`/`member_put`/`member_remove` NATS messages EasyCLA emits on role change (no fga-sync code change; add types to the model only).
- **Repository interface**: the datastore-agnostic boundary enabling M6b.

## Sizing guidance (the brief asked "how much work?")
Relative, not a commitment:
- **M6a (K8s, keep DynamoDB)**: **XL.** Full API rewrite in Goa, NATS integration, OpenFGA authz model + fga-sync, Helm/ArgoCD, staged cutover with PR-gating continuity. The **OpenFGA authorization migration with equivalence proof** is the single riskiest sub-task.
- **M6b (datastore migration)**: **additional L–XL** on top, dominated by data migration + verification + any query-pattern redesign — and only worth doing if M6a's measurements show the store is genuinely the constraint.
- **Doing both at once**: **not additive but multiplicative in risk** — explicitly *not* recommended.

## Success Criteria
- **SC-6.1**: All consumers and callbacks pass parity tests against the K8s service; PR gating never gaps during cutover.
- **SC-6.2**: Post-migration permissions exactly match a pre-migration snapshot (zero drift).
- **SC-6.3**: The datastore sits behind a repository interface, so M6b is possible without business-logic changes.
- **SC-6.4**: Hot-path performance ≥ the Lambda baseline.

## Assumptions
- M1–M5 have reduced API consumers to SS + PR webhooks (+ any residual integrations), shrinking parity surface and cutover risk. Doing M6 earlier is possible but forces parity against the legacy consoles too.
- A representative pre-migration permission snapshot can be produced from ACS/Org-Service/ACL state for the equivalence proof.
