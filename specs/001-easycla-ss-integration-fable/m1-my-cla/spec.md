# Feature Specification: M1 — Read-only "My CLAs" in the Self Serve Me Lens

**Parent program**: [../spec.md](../spec.md) (EasyCLA → LFX Self Serve Integration) | **Milestone doc**: [../01-milestone-read-only-me-lens-fable.md](../01-milestone-read-only-me-lens-fable.md)
**Created**: 2026-07-11 | **Status**: Planned (see [plan.md](plan.md))

This is the extracted, implementable slice for Milestone 1. Program-wide context, assumptions, and resolved decisions live in the parent spec; this file is what `/speckit-tasks` and `/speckit-implement` operate on.

## User Story (P1)

A contributor logs into LFX Self Serve and, under the Me lens, opens "My CLAs". They see every ICLA they have signed and every currently valid ECLA, each showing the project/CLA group, signing date, and status. For signed ICLAs they can download the signed PDF. Signing still routes to the existing Contributor Console (linked out).

**Acceptance Scenarios**:

1. **Given** a logged-in user who has signed at least one ICLA, **When** they open My CLAs, **Then** each signed ICLA is listed with project name, date signed, and a working signed-PDF download.
2. **Given** a logged-in user with a valid ECLA, **When** they open My CLAs, **Then** the ECLA is listed with company name, project, and acknowledgement date, and no PDF download is offered.
3. **Given** a logged-in user with no CLA history, **When** they open My CLAs, **Then** an empty state explains what CLAs are and links to documentation.
4. **Given** any user viewing My CLAs, **When** they look for signing actions, **Then** none exist in Self Serve; any "sign" pointers link out to the Contributor Console.

## Functional Requirements

- **FR-001**: Self Serve MUST display, for the logged-in user, all ICLAs they have signed (any status: valid, superseded, expired), with project/CLA group name, date signed, and validity status.
- **FR-002**: Self Serve MUST display all of the user's currently valid ECLAs with company name, project/CLA group, and acknowledgement date, and MUST NOT offer a PDF for ECLAs (none exists).
- **FR-003**: Users MUST be able to download the signed PDF for each of their signed ICLAs via a time-limited link.
- **FR-004**: The view MUST be read-only; any signing affordance MUST link out to the existing Contributor Console.
- **FR-005**: The system MUST resolve the Self Serve identity (LF SSO) to the user's EasyCLA user record(s) by LF username, verified emails, AND GitHub account(s) linked to the LF identity — covering pre-LF-login history — and aggregate agreements across all matched records.
- **FR-005a**: When no GitHub account is linked to the LF identity (or no agreements are found), the UI MUST offer a "Don't see your CLAs? Link your GitHub account" action into Self Serve's existing identity-linking flow, and re-resolve after linking.
- **FR-006**: Users MUST see only their own agreements; no access to other users' signature data through this surface.

## Success Criteria

- **SC-001**: 100% of a sampled user population's signed ICLAs and valid ECLAs visible in EasyCLA's data are also visible in Self Serve; ICLA PDF download success rate ≥ 99%.
- Unmatched-identity telemetry live with an agreed threshold (launch gate for M2).

## Scope boundaries

In: Me-lens module `/me/clas`, SS server routes, identity resolution + telemetry, PDF presigned-URL hand-off, empty state, feature flag.
Out: any signing/writes, CCLA data (M4), approval lists/roles, changes to the PR remediation link, EasyCLA backend changes beyond the two read endpoints `GET /v4/my-clas` and `GET /v4/my-clas/{signatureID}/pdf` (implemented in PR #5125) — see [contracts/upstream-easycla-api.md](contracts/upstream-easycla-api.md).

## Design artifacts

[plan.md](plan.md) · [research.md](research.md) · [data-model.md](data-model.md) · [contracts/](contracts/) · [quickstart.md](quickstart.md)
