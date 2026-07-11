<!-- SPDX-License-Identifier: CC-BY-4.0 -->
# Specification Quality Checklist: EasyCLA → LFX Self Serve Integration

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-07-11
**Feature**: [00-overview-opus.md](../00-overview-opus.md) + milestone docs 01–06

> **Note on this spec set**: This is an architecture-review + PM-scope document set, deliberately technology-aware (it names real services and data shapes). It intentionally exceeds the "no implementation details / non-technical stakeholders" defaults of the standard spec template, because its audience is the architecture team. The relevant content-quality bar here is *accuracy and clear scoping*, not technology-agnosticism.

## Content Quality

- [x] Focused on user value and business needs (each milestone leads with user stories & retirement value)
- [x] All mandatory sections completed (scenarios, requirements, success criteria, assumptions per milestone)
- [~] No implementation details — **intentionally not met**; see note above (arch-review audience)
- [~] Written for non-technical stakeholders — **PM-facing overview + goals are; milestone internals are technical by design**

## Requirement Completeness

- [x] No [NEEDS CLARIFICATION] markers remain (4 open questions resolved with the user before writing)
- [x] Requirements are testable and unambiguous (FRs phrased as MUST/SHOULD with acceptance scenarios)
- [x] Success criteria are measurable (parity, no-drift, no-gap, equivalence-proof, config-reversible)
- [~] Success criteria are technology-agnostic — **intentionally mixed** (arch-review audience)
- [x] All acceptance scenarios are defined (Given/When/Then per user story)
- [x] Edge cases are identified (per milestone)
- [x] Scope is clearly bounded (explicit in/out-of-scope + retirement gates per milestone)
- [x] Dependencies and assumptions identified (per-milestone Assumptions + cross-cutting risk table R1–R8)

## Feature Readiness

- [x] All functional requirements have clear acceptance criteria
- [x] User scenarios cover primary flows (prioritized P1–P3 per milestone)
- [x] Feature meets measurable outcomes defined in Success Criteria
- [~] No implementation details leak into specification — **by design, see note**

## Key Risks Tracked (must be resolved at the noted milestone)

- [ ] **R1** — Locate Corporate Console GraphQL backend; gap-analyze vs `/v4` REST (before M4 build)
- [ ] **R2** — Two-layer authz: CLA authority never inferred from platform lens (M3, M4, M5)
- [ ] **R4** — SS-Auth0 → EasyCLA user identity resolution (M1, reused after)
- [ ] **R6** — Deep-link / return-URL contract repointing (M2, M3)
- [ ] **R9** — No ACS→OpenFGA bridge exists; RBAC→ReBAC translation is net-new, owned by M6 (design spike before committing M6 effort)
- [ ] **R10** — M6 migrates CLA authz only; ACS stays incumbent for other consumers (do not scope platform-wide ACS retirement)
- [ ] Architecture team to **ratify or override** the "adapter now, converge at M6" role strategy (before M3)
- [ ] Architecture team to **confirm decouple** of K8s migration from DynamoDB→Postgres (M6)

## Authorization system facts established (code-grounded)

- **ACS** = incumbent RBAC (Go, PostgreSQL, ECS): roles → policies → statements → resource-actions; scopes via `project` / `organization` / `project|organization`. CLA roles seeded in `acs/db/init.sql`; full CLA permission surface declared in `acs-cli/services/11-cla-service.yaml`.
- **Organization Service** stores/checks the (user, role, scope) grants EasyCLA relies on at runtime.
- **OpenFGA** = strategic ReBAC (V2), synced by generic **`lfx-v2-fga-sync`** (four standard NATS subjects; add object types to the model, no service-code change). **No CLA types in the model yet.**
- **ACS and OpenFGA have zero integration.** Confirmed by inspection of both codebases.

## Notes

- The single largest sizing uncertainty is **M4** (Corporate Console GraphQL gap analysis, R1) and **M6a** (OpenFGA authorization-equivalence migration).
- Effort sizes are T-shirt (S/M/L/XL) relative indicators, not commitments.
- Authored with Claude **Opus** 4.8 (tagged for an Opus-vs-Fable comparison).
