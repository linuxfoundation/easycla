<!-- SPDX-License-Identifier: CC-BY-4.0 -->
# Milestone 0: Role/Permission Adapter for EasyCLA-in-Self-Serve

**Status**: Draft — cross-cutting, no direct end-user feature
**Depends on**: nothing (can start immediately)
**Unblocks**: Milestone 3 (sign ECLA), Milestone 4 (CCLA management), Milestone 5 (project config)

## Why this is its own milestone

Milestones 3, 4, and 5 all require answering "does this Self Serve user hold a specific CLA role (CLA Manager, CLA Manager Designee, CLA Signatory, Project Manager, etc.) for this project/company?" EasyCLA answers that question today via **ACS**, a role/policy RBAC system entirely separate from **OpenFGA**, the relationship-based authorization system Self Serve and other V2 services use. There is no existing bridge between the two anywhere in the platform.

Folding this into each of Milestones 3/4/5 separately risks three different, inconsistent, ad hoc solutions. Solving it once, here, gives every later milestone a single pattern to follow.

## Current state (confirmed)

- **EasyCLA roles are declared as data** in `acs-cli/services/11-cla-service.yaml` (~1,271 lines): resources, policies, and role→policy→resource-action mappings for roles including `cla-manager`, `cla-manager-designee`, `cla-signatory`, `project-manager`, `cla-program-manager`, `lf-program-manager`, `community-program-manager`. Example: `cla-manager` maps to full signature access, add/delete other managers, approval-list updates, and auto-create-ECLA permission.
- At runtime, `cla-backend-go` calls the ACS client to check and grant these roles; the LFX API Gateway also runs a general ACS-authorizer middleware in front of EasyCLA endpoints.
- **Self Serve / LFX V2 authorization is OpenFGA** (ReBAC — relationship tuples, not role/policy). `lfx-v2-fga-sync` syncs NATS-published authorization events into OpenFGA via four generic operations (`update_access`, `delete_access`, `member_put`, `member_remove`) that work for any object type defined in the platform's OpenFGA model — adding a new object type requires no sync-service code change, only a model update and event publishing from the owning service.
- The current OpenFGA model contains generic governance object types (`user`, `team`, `project`, `committee`, `meeting`, etc.) — **no CLA-specific types exist in it.**
- No code in either ACS-related repos or `lfx-v2-fga-sync` references the other system. They are fully independent today.

## Approach: adapter, not bridge

**Do not build a bidirectional or one-way sync between ACS and OpenFGA as part of this milestone.** A partial sync built now — before EasyCLA's backend itself moves — would create a second, lagging source of truth for CLA roles. Drift between the two either blocks legitimate CLA managers from doing their job (support burden) or over-grants access (a compliance problem, given CLAs are a legal/compliance-sensitive domain). That risk is not worth taking on to make a not-yet-scheduled Milestone 6 marginally easier.

Instead, adopt a **two-layer authorization model** for every CLA-related screen in Self Serve:

1. **Coarse lens gate — unchanged, platform OpenFGA/Heimdall.** Answers "can this user open the Organization lens for company X, or the Project lens for project Y, at all?" This reuses Self Serve's existing mechanism without modification.
2. **Fine-grained CLA authorization — EasyCLA adapter.** Answers "is this specific user a CLA Manager / Signatory / Designee for this project+company?" and "may they perform this specific mutating action?" This is delegated to the EasyCLA `/v4` API via the Self Serve BFF. EasyCLA remains the single, authoritative source for CLA role state; there is no duplicate copy of role data anywhere in Self Serve.

Concretely: the BFF's CLA-related routes call an existing (or lightly extended) EasyCLA `/v4` endpoint to fetch "what CLA roles does user X hold for project/company Y," and every mutating BFF route re-checks that before performing a write. The lens-level OpenFGA gate is necessary but never sufficient for a CLA-mutating action.

## Hard requirement carried into every downstream milestone

The two authorization layers **can and will disagree** in practice — e.g., a user with general Organization-lens access in Self Serve is not necessarily a CLA Manager for that org's CLA. Every CLA-mutating action added in Milestones 3, 4, and 5 must:

- Call the EasyCLA role-check explicitly, server-side, in the BFF.
- Never infer CLA-role authorization from the fact that a user could reach the page (i.e., never treat "passed the lens gate" as "may edit approval list").

This requirement is restated in the Milestone 3/4/5 documents; it originates here.

## Scope of this milestone

**In scope:**
- Define and document the BFF-side adapter pattern (interface, error handling for role-check failures, caching strategy if any — role checks should not add a synchronous EasyCLA API round-trip to every page render if avoidable).
- Identify or extend the specific EasyCLA `/v4` endpoint(s) needed to answer "what CLA roles does user X hold, scoped to project Y / company Z" in a single call usable by the BFF.
- Resolve the user-identity mapping question jointly with Milestone 1 (R4): the adapter needs a reliable "Self Serve user → EasyCLA user id" resolution to make any role check.
- Write the design doc for how Milestone 6 will eventually converge CLA roles onto OpenFGA (see below) — as a design artifact only, not implementation.

**Out of scope:**
- Any change to ACS itself.
- Any change to the OpenFGA model to add CLA object types (deferred to Milestone 6).
- A sync service or scheduled job copying ACS role state into OpenFGA.

## Eventual convergence path (for Milestone 6, documented now while context is fresh)

`acs-cli/services/11-cla-service.yaml` is effectively a ready-made inventory of every CLA role, resource, and permission relationship needed to design an OpenFGA model for CLA. Combined with `lfx-v2-fga-sync`'s generic, model-agnostic sync contract, adding CLA object types (e.g., `cla_group`, `cla_signature`, referencing existing `project` and `b2b_org`-equivalent types) at Milestone 6 time does not require new sync-service code — only:
1. A one-time model addition (`model.yaml`) defining CLA relations.
2. The rewritten EasyCLA V2 service publishing standard `update_access`/`member_put`/etc. events instead of (or alongside, during a staged cutover) writing to ACS.
3. A single, verifiable cutover — not a multi-year parallel-sync arrangement — because it happens at the same time as the API rewrite, when EasyCLA's own persistence and service boundaries are already being redefined.

This confirms Milestone 6 is the correct, and only, place to retire ACS-based CLA authorization in favor of OpenFGA. Note: this does **not** mean retiring ACS platform-wide — ACS remains the authorization system for other V1 services with no announced sunset. Milestone 6 scopes only CLA's object types.

## Risks

| Risk | Mitigation |
|---|---|
| Two-layer model adds a second authorization check to every CLA screen/action, increasing latency and code paths to test. | Document the pattern once here; every downstream milestone reuses it rather than inventing its own. Consider a short-TTL cache for role lookups if latency proves material. |
| Teams building M3/M4/M5 skip the EasyCLA-layer check because the lens gate "already let the user in." | This document and each milestone doc state the requirement explicitly; recommend a code-review checklist item and/or a shared BFF middleware that makes the EasyCLA check mandatory rather than optional per-route. |
| EasyCLA `/v4` may not currently expose a single convenient "all roles for user X scoped to Y" endpoint — may need a small, additive API change. | Confirm during this milestone's design work; if a gap exists, scope the additive endpoint here rather than discovering it mid-Milestone-3. |

## Effort

**S** — this is primarily a design and a thin adapter/interface, not new product surface. Most cost is investigation + getting the pattern right, reused three times downstream.
