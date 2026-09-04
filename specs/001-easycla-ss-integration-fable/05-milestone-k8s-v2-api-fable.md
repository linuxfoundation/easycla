# Milestone 5 — EasyCLA API as a V2 Kubernetes Service; DynamoDB → Postgres Evaluation

**Status**: Draft — **decision milestone**, not committed scope | **Depends on**: M1–M4 (UI-first sequencing confirmed; hybrid-strangler alternative noted below) | **Retires**: Lambda/API-GW deployment (including the legacy `/v1`/`/v2` Go surface, deployed from the same `cla-backend` serverless stack) | **Effort**: XL without DB migration; XXL with
**Spec**: [spec.md](spec.md) | **Overview**: [00-overview-fable.md](00-overview-fable.md)

> **Not planned.** The program aims to complete **M1–M3**; M4 and M5 are not scheduled and may never be implemented. This document is a design option retained for reference, not committed scope.

## What "being a V2 service" means (verified against the platform)

From the ~15 existing services in `lfx-v2-argocd` and the fga-sync/gateway contracts:

- Kubernetes deployment via **Helm chart + ArgoCD app** per env; image at `ghcr.io/linuxfoundation/<service>`.
- **Heimdall** token validation at the edge; **OpenFGA** authorization (object types + relations registered in the FGA model, tuples maintained via `lfx-v2-fga-sync` NATS subjects `lfx.fga-sync.*`, checks via `lfx.access_check.*`).
- **NATS** for messaging/eventing; OpenTelemetry; routed via **lfx-gateway**.
- Today **no CLA object types exist in the FGA model** — a CLA authorization model (e.g., `cla_group#manager|signatory`, company-scoped relations) must be designed and registered; this is where the ACS→OpenFGA role migration actually happens (deferred here deliberately from M3–M4).

## What EasyCLA actually runs today (the true scope)

1. **Go API** — `/v3` (us-east-1) + `/v4` (us-east-2) Lambdas. Mitigating fact: `cmd/server_standalone.go` already runs the same API as a plain HTTP server → containerization of the main API is cheap.
2. **Legacy Go backend** (`cla-backend-legacy`) — the `/v1`/`/v2` surface is already ported to Go (Python fully removed) and deployed from the `cla-backend` stack on the original `api.*` domains. This shrinks M5 meaningfully versus the old Python-port assumption, but it remains a second API codebase and deployment: M5 folds its endpoints into the V2 service (or consciously containerizes it alongside) and retires the separate stack.
3. **Auxiliary Lambdas** — `cmd/` contains ~10 more binaries: **dynamo-events (DynamoDB Streams consumer driving audit/event fan-out), zipbuilder, metrics, user-subscribe, gitlab-repository-check, ldap_gerrit_check**, etc. Each needs a K8s home (Deployment/CronJob) or retirement. The Streams consumer is the architecturally sticky one — see DB section.
4. **Inbound integrations that must not drop events during cutover**: GitHub App webhooks, GitLab webhooks, Gerrit hooks, **DocuSign envelope callbacks**, SNS/SES flows.
5. **Config**: SSM Parameter Store + assumed AWS IAM → K8s secrets management (`lfx-secrets-management`) and per-env values.
6. **Gateway**: already routed via lfx-gateway (`cla-service.yaml` → Lambda) — cutover is retargeting existing routes from the Lambda backend to the K8s Service, which also enables **percentage/path-based gradual cutover**, a real de-risking advantage.

## Effort: with vs. without database replacement

### Track A — K8s move, keep DynamoDB (XL)

Containerize the standalone server; IRSA for DynamoDB/S3/SES access (note: tables in us-east-1, cluster region matters for latency); port auxiliary lambdas (Streams consumer can keep running as a Lambda indefinitely — streams don't care where the API lives); fold in the legacy `/v1`/`/v2` Go surface; Heimdall in front (replacing Auth0-middleware-in-app), OpenFGA model + fga-sync for CLA roles (the ACS divorce — the hardest design item, includes migrating existing ACS role grants to FGA tuples and deciding what Salesforce still needs to know); observability; dual-run + gradual gateway cutover.

Rough shape: **a few engineer-quarters**, dominated not by "moving compute" but by (a) the authorization migration and (b) consolidating the legacy `/v1`/`/v2` surface. API code paths largely survive as-is (both backends are already Go).

### Track B — Track A + DynamoDB → Postgres (XXL)

Everything above, plus:

- **Repository-layer rewrite across every module** (`signatures/`, `company/`, `project/`, `repositories/`, `approval_list/`, `events/`, v2 packages…): the pattern (handlers/service/repository) contains the blast radius to the repository layer, but signatures queries lean on Dynamo-specific GSIs, sparse fields (`signature_user_ccla_company_id`), and composite keys (`sig_type_signed_approved_id`) that need genuine remodeling, not transliteration.
- **Data migration of ~19 tables** with dual-write or CDC backfill, checksum verification, and a rollback story; signature records are legally meaningful — zero-loss is a hard requirement (FR-052). S3 PDFs are unaffected.
- **Replacing DynamoDB Streams**: the event/audit fan-out (dynamo-events) must be re-sourced (transactional outbox → NATS is the platform-consistent answer) — this is an architecture change, not a port.
- **Session store, key-value store tables** → Postgres or platform equivalents.

Rough shape: **roughly doubles Track A**, and concentrates risk in the least-forgiving data (signatures).

### Should you replace the DB? (critical answer)

- The stated motivation is "DynamoDB causes performance and other issues". **Interrogate that before committing**: the notorious pain points (scan-heavy signature queries, hot lookups) are query/index-design problems that Postgres would solve — but several could also be solved with targeted GSIs/caching at 5% of the cost. Quantify the top offenders first.
- **However**, if the API is being rewritten as a V2 service anyway (Track A includes touching every module's wiring), the *marginal* cost of Postgres is much lower than a standalone DB migration, Postgres matches platform norms (relational fits CLA's join-heavy access patterns: signatures × companies × projects × approval lists), and it removes the permanent us-east-1 tether.
- **Recommendation**: commit Track A; run Track B as a **phased strangler inside M5** — new service reads/writes Postgres per domain (start with low-risk tables: events, store; signatures last) behind the repository interface, rather than a big-bang migration. Do not attempt Postgres without the service rewrite, and do not block the K8s move on it.

## Scope decision for the review (Open Decision Q3)

Because M3–M4 build SS↔v4 adapters and an ACS role bridge that M5 then replaces, the board should choose explicitly:

1. **UI-first (current plan)**: full user value early; accept adapter rework in M5. Keep adapters thin (single SS server module) to bound the waste.
2. **Platform-first**: rewrite API + roles first, migrate UIs onto the new service. Cleanest end-state, but ~a year of no user-visible progress and big-bang risk.
3. **Hybrid strangler (recommended if M5 is truly committed)**: after M2, stand up a **CLA read/query V2 service** (the easy 70% of SS's needs; akin to the platform's query-service pattern) while writes stay on v4; M3/M4 consume it; M5 completes writes + roles + legacy Go surface consolidation. Spreads M5 across the program instead of stacking it at the end.

## Risks

| Risk | Notes |
|------|-------|
| Authorization cutover (ACS → OpenFGA) breaks corporate flows subtly | Dual-evaluate (log-only FGA checks alongside ACS) for a full release cycle before enforcement flips |
| Webhook/callback loss at cutover | Gateway-level gradual cutover + replay/reconciliation jobs for GitHub events and DocuSign envelopes |
| Legacy `/v1`/`/v2` semantics lost when folding into the V2 service | The legacy Go port shipped with parity tooling (`cla-backend-legacy/internal/parity`, py2go comparison tests in-repo) — extend those contract tests for the consolidation |
| Signature data-migration defects | Phased per-table strangler; checksum + row-count + spot legal review; signatures table last |
| Cross-region latency (K8s cluster vs us-east-1 Dynamo) during Track A | Measure; co-locate or accept until Track B |
| Scope creep: "while we're rewriting…" | The parity contract (FR-050/051) is the spec; enhancements are separate features |

## Exit criteria

- SC-006 (zero lost events; latency/error parity over 30 days) and full E2E suite parity.
- ACS CLA roles decommissioned only after one clean cycle of FGA-enforced operation.
- Main and legacy `/v1`/`/v2` API Lambdas torn down (auxiliary Lambdas like the Streams consumer may remain per Track A above); runbooks/on-call updated.
