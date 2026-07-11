<!-- SPDX-License-Identifier: CC-BY-4.0 -->
# Milestone 4: CCLA Management in Organization Lens

**Status**: Draft
**Depends on**: Milestone 0 (role adapter), Milestone 1 (identity resolution)
**Retires**: Corporate CLA Dashboard (`lfx-corp-cla-console`) — candidate, after parity and burn-in

## User story

As a CLA Manager or CLA Signatory for my company, I want to manage my company's CCLA — sign it, maintain the approval list, manage other CLA managers — from the Organization lens in Self Serve, so I don't need a separate Corporate CLA Dashboard app.

## Scope, per confirmed Corporate Console functionality

**In scope (full parity target):**
- CCLA signing (DocuSign flow, same backend-mediated pattern as Milestone 2 — reuse existing `/v4` sign endpoints, no new DocuSign integration needed).
- Approval-list management: add/remove by email, domain, GitHub org, GitLab group.
- CLA Manager add/remove, and CLA Manager Designee handling.
- Activity/audit log view for the company's CLA activity.
- Contributor acknowledgement visibility (which employees have ECLA'd under this CCLA).
- Download of the signed CCLA PDF (confirmed CCLAs do have signed PDFs).

**Out of scope:**
- Anything project-side (CLA group creation, PDF template management, GitHub App installation) — that is Milestone 5's PCC-derived scope, not the Organization lens.
- Changes to how CLA roles are granted at the ACS level — this milestone consumes role state via the Milestone 0 adapter, it doesn't change the underlying role system.

## The GraphQL backend risk (R1) — must resolve before this milestone is built

The Corporate Console's data layer is Apollo GraphQL against `https://lf-backend-cla.platform.linuxfoundation.org/graphql`. This GraphQL server's source code was **not found in any locally cloned repository** during research. Before this milestone can be scoped with confidence:

1. **Locate the GraphQL server's source** (it may live in a repo not yet cloned locally, or may be a thin layer over the same Go `/v4` REST API).
2. If it turns out to expose functionality or data shapes not available via the Go `/v4` REST API, that functionality needs either an additive REST endpoint or a design decision to reimplement it differently.
3. **Design target for this milestone is the existing Go `/v4` REST API**, not a new GraphQL consumer in Self Serve — treat the current GraphQL layer as a legacy integration to bypass, not a pattern to replicate, unless investigation reveals REST genuinely cannot support some required capability.

This is flagged as an explicit action item, not a detail to discover mid-build — it changes the shape of this milestone's backend integration work.

## Functional requirements

1. The system MUST let a CLA Manager or CLA Signatory sign a CCLA for their company via the existing DocuSign-backed flow, matching the pattern established in Milestone 2 (no new DocuSign integration).
2. The system MUST let an authorized CLA Manager add/remove approval-list entries by email, domain, GitHub organization, and GitLab group.
3. The system MUST let an authorized CLA Manager add/remove other CLA Managers, and manage CLA Manager Designee status, consistent with current Corporate Console capability.
4. The system MUST show an activity/audit log of CLA-related actions for the company, matching current Corporate Console visibility.
5. The system MUST show which employees have an active ECLA acknowledgment under the company's CCLA.
6. The system MUST allow downloading the signed CCLA PDF.
7. Every mutating action (approval-list edit, manager add/remove, signing) MUST be authorized via the Milestone 0 EasyCLA-role adapter, never inferred from the user's general Organization-lens access in Self Serve (R2, hard requirement — this is the milestone with the largest surface of mutating, role-gated actions, so it is the highest-risk milestone for this failure mode).
8. The system MUST resolve which company/companies the logged-in user is a CLA Manager/Signatory for, reusing the Milestone 1 identity-resolution pattern.

## Risks specific to this milestone

| Risk | Mitigation |
|---|---|
| R1: GraphQL backend source not located; unknown functionality gap risk. | Resolve as a pre-milestone spike before detailed design/estimation is finalized. |
| This is the highest-role-surface milestone — most CLA Manager/Signatory-only actions live here, so the two-layer-authz risk (R2) has the most opportunities to go wrong. | Prioritize a shared, mandatory BFF-level authorization check (per Milestone 0's recommendation) over per-route ad hoc checks, specifically for this milestone's build. |
| Sanctioned-organization handling and foundation-level CLA nuances exist in the current Corporate Console (per its module structure) and are easy to under-scope if only the "happy path" CCLA flow is planned. | Explicitly inventory Corporate Console's full module list during design (approval lists, manager designee, activity logs, sanctioned-org handling, foundation-level CLA) rather than assuming a minimal CCLA-signing flow is sufficient for parity. |
| Retiring the Corporate Console removes the only UI for company CLA administration used across every EasyCLA-enabled project's corporate contributors — a high-blast-radius cutover. | Same burn-in/parity-verification discipline as Milestone 3; do not treat "code merged" as "ready to retire." |

## Effort

**XL** — the largest UI-parity milestone (Corporate Console has the deepest feature set of any of the three consoles), compounded by the unresolved GraphQL-backend dependency and the heaviest role-authorization surface of any milestone before M6.
