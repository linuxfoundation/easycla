<!-- SPDX-License-Identifier: CC-BY-4.0 -->
# Milestone 6: EasyCLA API → Kubernetes V2 Service

**Status**: Draft — decision point for architecture team, not a committed scope
**Depends on**: none of Milestones 1–5 strictly block this technically, but doing it after them is lower-risk (UI consumers are already decoupled behind a stable `/v4` contract by then)
**Retires**: the Lambda/API-Gateway deployment of `cla-backend-go`, and (optionally, as a separate decision) DynamoDB as EasyCLA's datastore

## Framing

This milestone is qualitatively different from Milestones 0–5: those move UI surface area onto an unchanged backend contract. This milestone changes the backend itself. The user's instinct to treat it separately and be "a little hesitant" is well-founded — this is the highest-cost, highest-risk milestone in the plan, and unlike the others, **it is not clearly a prerequisite for anything the end user experiences.** Milestones 1–5 are fully achievable with EasyCLA's backend exactly as it is today.

This document presents the tradeoffs for the architecture team to decide, rather than asserting the move should happen. It decouples two decisions that are often conflated:

1. **Move the API to Kubernetes** (change compute/deployment model).
2. **Migrate DynamoDB to Postgres** (change datastore).

## What "being a V2 service" concretely means here

Based on existing LFX V2 services (e.g., `lfx-v2-auth-service`, `lfx-v2-project-service`, and others deployed via `lfx-v2-argocd`/`lfx-v2-opentofu`), a V2 service in this ecosystem means, concretely:

- A Go service (this part EasyCLA already satisfies — `cla-backend-go` is already Go).
- Deployed as a container via Helm, managed by ArgoCD, across dev/staging/prod, in its own namespace — not a Lambda behind API Gateway.
- Typically backed by its own PostgreSQL database provisioned via OpenTofu — though this is **not universal**: at least one recent V2 service (`lfx-v2-member-service`) was built without Postgres, using Salesforce, OpenSearch, and NATS-KV instead. Postgres is the common default, not a hard requirement of "being a V2 service."
- Participating in the platform's shared infrastructure conventions: NATS for eventing, and — per Milestone 0's findings — OpenFGA/`lfx-v2-fga-sync` for authorization, rather than ACS.

**Important scoping implication:** "moving to Kubernetes" as commonly meant in this platform is not a lift-and-shift of the existing Lambda code into a container. It implies adopting the platform's V2 conventions (eventing, authorization model, deployment tooling), which is closer to a rewrite than a re-host. Estimate effort accordingly — see below.

## Decision 1: API move to Kubernetes (decoupled from DB choice)

**What this requires, regardless of DB decision:**
- Re-implementing `cla-backend-go`'s `/v4` API surface (and, if `/v3` must be preserved for any remaining legacy consumer, that too) as a containerized Go service.
- Rebuilding the DocuSign integration, DocRaptor/PDF generation, and PR/Gerrit/GitLab gating webhook logic in the new service — none of this is UI-facing, so none of Milestones 1–5 change it, but all of it has to move.
- Adopting NATS-based eventing where the current architecture uses DynamoDB Streams (`v2/dynamo_events/`) for signature-event processing, if DynamoDB is retained — or restructuring event handling entirely if Postgres is adopted (see Decision 2).
- Adopting the OpenFGA-based authorization convergence scoped in Milestone 0 (this is in fact the natural point to retire the ACS-based CLA authorization entirely, per Milestone 0's convergence path — `acs-cli/services/11-cla-service.yaml`'s role/policy declarations become the input to a new OpenFGA model).
- Re-establishing the API Gateway routing (Traefik) to point at the new Kubernetes service instead of Lambda targets, with a staged cutover per endpoint or endpoint-group to limit blast radius.

**This is a genuine rewrite of a substantial, business-critical backend** — not a container-wrapping exercise. Effort should be estimated as such.

## Decision 2: DynamoDB → Postgres (separable)

**Arguments for doing it now, alongside the API move:**
- If a rewrite is happening anyway, the data-access layer is being touched regardless; deferring a DB migration to a second, later rewrite means touching the same code twice.
- The user's stated motivation — DynamoDB is "causing many performance and other issues" — is worth taking seriously if true, but this document cannot independently verify that claim from the codebase; it should be substantiated with concrete incident/performance data before being used to justify scope, not assumed.
- Postgres is the more common (though not universal) V2-service datastore choice on this platform, so aligning with that convention has operational-consistency benefits (shared tooling, team familiarity, one less bespoke pattern to support).

**Arguments against bundling it into this milestone:**
- It roughly doubles the risk surface of an already-large rewrite: a new service *and* a new data model *and* a data migration, all at once, for a system that handles legally-significant records (signed agreements) where data-integrity mistakes are unusually costly.
- `lfx-v2-member-service` demonstrates Postgres is not obligatory for a V2 service — precedent exists for choosing a different datastore (or, by extension, for deferring the datastore decision independently of the compute-platform decision).
- A DynamoDB migration to Postgres requires its own careful plan (schema design from ~14 denormalized tables, data backfill/dual-write strategy, verification), which deserves its own scoping exercise rather than being folded silently into "move to K8s."

**Recommendation for this document:** decouple the two decisions. Structure Milestone 6 as:
- **6a — API to Kubernetes, DynamoDB retained** (behind a repository-interface abstraction so a later DB swap doesn't require touching business logic again).
- **6b — DynamoDB → Postgres**, an optional, separately-scoped follow-on milestone, justified independently with concrete performance/operational evidence rather than assumed as "efficient since we're already rewriting."

This is presented as a recommendation for the architecture team to ratify or override, not a foregone conclusion — the "do both at once" argument above is real and some teams will reasonably prefer it. The recommendation here is offered because bundling increases the risk of an already-large, legally-sensitive rewrite without a demonstrated, quantified performance problem to justify the added risk.

## Relative effort

| Scope | Relative effort | Primary drivers |
|---|---|---|
| **6a: API → Kubernetes only** (DynamoDB retained) | **XL** | Full re-implementation of `/v4` (+ possibly `/v3`) surface, DocuSign/DocRaptor integration, all gating webhook logic, OpenFGA authorization convergence (per Milestone 0), Gateway routing cutover. Not a container-wrap; a genuine rewrite of a business-critical backend. |
| **6b: + DynamoDB → Postgres** (additive to 6a) | **+L to +XL on top of 6a** | New schema design from ~14 DynamoDB tables' worth of access patterns, data-migration/backfill strategy, dual-write or cutover window planning, and full regression testing given the legal significance of signature records. The addition is not "just change the driver" — DynamoDB's access-pattern-first modeling and Postgres's relational modeling are different enough that a straight schema port is unlikely to be appropriate. |

These are relative, directional sizes for architecture-review discussion, not committed estimates — a proper estimate requires a dedicated scoping/design spike for 6a before 6b can be sized with any confidence.

## Risks

| Risk | Mitigation |
|---|---|
| Underestimating this as "just deploy the existing Go code to a container" when it is closer to a full rewrite once V2 conventions (NATS, OpenFGA, Helm/ArgoCD) are adopted. | Treat the effort table above as a floor, not a ceiling; validate with a scoping spike before committing a timeline. |
| Bundling the DB migration increases risk for a system handling legally-significant records, without an independently justified performance case. | Require concrete DynamoDB performance/incident evidence as a precondition for approving 6b, separate from approving 6a. |
| This milestone is the actual point where ACS-based CLA authorization is retired in favor of OpenFGA (per Milestone 0) — if that convergence is under-scoped here, it becomes the same kind of "discovered late" risk Milestone 0 was created to avoid for the earlier milestones. | Treat Milestone 0's convergence-path section as this milestone's authorization-design starting point, not a from-scratch design exercise. |
| Doing this before Milestones 1–5 complete would mean building new UI against a backend that's simultaneously being rewritten — significant coordination risk. | Sequence after Milestones 1–5 land, as shown in the overview's sequencing diagram, unless there's a compelling reason to reorder. |

## Open question for the architecture team

Should this milestone be scoped and approved at all in the near term, given that none of the user-facing milestones (1–5) require it? The plan as a whole is fully deliverable without Milestone 6. Recommend treating this document as a basis for a future decision point, not as committed roadmap scope alongside Milestones 0–5.
