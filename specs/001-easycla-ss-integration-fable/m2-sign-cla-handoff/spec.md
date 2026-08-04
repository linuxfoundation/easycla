# Feature Specification: M2 — Proactive CLA signing entry in Self Serve (hands off to Contributor Console)

**Parent program**: [../spec.md](../spec.md) (EasyCLA → LFX Self Serve Integration) | **Milestone doc**: [../02-milestone-sign-icla-fable.md](../02-milestone-sign-icla-fable.md)
**Created**: 2026-08-04 | **Status**: Planned (see [plan.md](plan.md))

This is the extracted, implementable slice for Milestone 2. Program-wide context, assumptions, and resolved decisions live in the parent spec; this file is what `/speckit.plan`, `/speckit.tasks`, and `/speckit.implement` operate on.

> **Scope (revised 2026-08-04, per Heather/PM)**: M2 is purely additive — it retires nothing and cuts nothing over. Self Serve never runs the DocuSign ceremony; it hands off to the existing Contributor Console for signing. See the User Story below.

> **Link note**: the upward links `../spec.md` and `../02-milestone-sign-icla-fable.md` point at program-level docs still under review in PR #5132 (targeting `dev`); they resolve once #5132 merges. The `m1-my-cla/` sibling folder is already on `dev`.

## User Story (P2)

A contributor who wants to sign a CLA — without first hitting a failing PR check — logs into LFX Self Serve and, under the Me lens, opens "Sign a CLA". They pick which CLA Group they need to sign for (and, where relevant, narrow to a specific GitHub org/repo), choose ICLA or ECLA (gated by what that CLA Group allows), and are handed off to the existing Contributor Console, pre-scoped to that selection, to complete the actual signing there. Signing itself — the DocuSign ceremony — happens in the Console, exactly as it does today.

**Acceptance Scenarios**:

1. **Given** a logged-in contributor with no failing PR, **When** they open "Sign a CLA" in the Me lens and select a CLA Group that allows ICLAs and choose ICLA, **Then** they are handed off to the Contributor Console landed on that CLA Group's individual-signing flow, without the Console re-asking which CLA Group or sign type.
2. **Given** a logged-in contributor selecting a CLA Group that allows ECLAs and choosing ECLA, **When** they proceed, **Then** they are handed off to the Contributor Console's corporate/employee flow pre-scoped to that CLA Group.
3. **Given** a CLA Group that spans multiple GitHub orgs/repos, **When** the contributor selects it, **Then** the picker lets them narrow to the relevant org/repo before hand-off (exact interaction — full tree, flat list, or defer-to-Console — is an open design question).
4. **Given** a CLA Group that allows only one sign type (e.g. `project_icla_enabled` true, `project_ccla_enabled` false), **When** the contributor selects it, **Then** only the permitted sign type is offered.
5. **Given** the existing "Signed Agreement Missing" PR-check link, **When** any contributor clicks it, **Then** it behaves exactly as before M2 — it points at the Contributor Console, unchanged; the new picker is a parallel, additive path to the same destination.

## Functional Requirements

- **FR-001**: Self Serve MUST offer a new, PR-independent entry point in the Me lens where a logged-in user can start signing a CLA without any failing-PR context.
- **FR-002**: The picker MUST let the user select a CLA Group, and where a CLA Group spans multiple GitHub orgs/repos, MUST let the user narrow to a specific org/repo. *([NEEDS CLARIFICATION]: interaction depth.)*
- **FR-003**: The picker MUST offer the sign-type choice (ICLA or ECLA) gated by the selected CLA Group's `project_icla_enabled` / `project_ccla_enabled` flags — the same source the Contributor Console uses for its decision screen.
- **FR-004**: Self Serve MUST hand off the user to the existing Contributor Console, pre-scoped to the chosen CLA Group and sign type, to complete the actual signing — for both the ICLA and ECLA/CCLA paths.
- **FR-005**: Self Serve MUST NOT call any signing-initiation endpoint (`request-individual-signature`, `request-employee-signature`, `check-prepare-employee-signature`, etc.) itself in M2. The Console makes those calls, exactly as it does today.
- **FR-006**: Self Serve MUST NOT change the GitHub PR status-check remediation link. It keeps pointing at the Contributor Console; there is no SSM cutover or per-environment switch in M2.
- **FR-007**: Self Serve MUST enumerate the CLA Groups (and their org/repo scope) available to present in the picker. *([NEEDS CLARIFICATION]: discovery endpoint.)*
- **FR-008**: The hand-off MUST carry enough context (CLA Group ID, org/repo, sign type, user identity) for the Console to land the user on the correct screen without re-asking. *([NEEDS CLARIFICATION]: hand-off contract.)*

## Success Criteria

- **SC-002** (M2): ≥ 95% of contributors who start the new proactive picker in Self Serve are successfully handed off to the Contributor Console pre-scoped to the right CLA Group / sign type, without support intervention.

## Scope boundaries

**In**: new Me-lens "Sign a CLA" picker (CLA Group + optional org/repo + ICLA/ECLA), SS server route(s) to enumerate CLA Groups and construct the Console hand-off, feature flag.

**Out**: any DocuSign/webhook/PDF backend changes; native signing ceremony in SS; PR-redirect cutover (`CLAContributorv2Base` SSM flip); CCLA/approval-list management (M4). Also out: corporate org-selection UX polish for the CCLA path — Heather flagged this for M3, so do not over-invest here ahead of that.

## Open questions (for `/speckit.plan` / `/speckit.clarify`)

- **Org/repo scope in the picker** — for a CLA Group spanning many orgs/repos, is it a full tree picker, a flat list, or does SS defer org/repo selection to the Console after CLA-Group selection? Not resolved by PM input; needs a design pass.
- **Hand-off contract** — what context SS passes the Console (CLA Group ID, org/repo, sign type, user) and in what shape (URL query params analogous to today's `?redirect=<...>`, but with no PR URL to preserve). Needs its own contract; do not assume the PR-redirect querystring works unchanged.
- **CLA-Group discovery source** — confirm which existing endpoint (if any, e.g. reused from M1's read-only lens) can enumerate CLA Groups + org/repo scope for a user to browse, or whether new API support is needed in `cla-backend-go` / `cla-backend-legacy`. This is the main estimate risk for the milestone.

## Design artifacts

[plan.md](plan.md) · `research.md`, `data-model.md`, `contracts/`, `quickstart.md` — to be generated by the Spec Kit planning flow (not hand-written here, since the hand-off contract and discovery endpoint are still open).
