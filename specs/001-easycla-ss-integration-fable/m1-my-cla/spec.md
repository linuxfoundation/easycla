# Feature Specification: M1 — Read-only "My CLAs" in the Self Serve Me Lens

**Parent program**: [../spec.md](../spec.md) (EasyCLA → LFX Self Serve Integration) | **Milestone doc**: [../01-milestone-read-only-me-lens-fable.md](../01-milestone-read-only-me-lens-fable.md)
**Created**: 2026-07-11 | **Status**: Implemented (dark-launched behind the `my-clas-enabled` flag) — delivery epic [linuxfoundation/lfx-self-serve#1157](https://github.com/linuxfoundation/lfx-self-serve/issues/1157) (closed); backend PRs [linuxfoundation/easycla#5125](https://github.com/linuxfoundation/easycla/pull/5125), [linuxfoundation/easycla#5128](https://github.com/linuxfoundation/easycla/pull/5128); API reference [docs/MY_CLAS_API.md](../../../docs/MY_CLAS_API.md)

This is the extracted, implementable slice for Milestone 1, updated 2026-09-01 to reflect the implementation. Program-wide context, assumptions, and resolved decisions live in the parent spec.

## User Story (P1)

A contributor logs into LFX Self Serve and opens the **"CLAs" tab in the Profile hub** (`/profile/clas`). They see every ICLA and ECLA they have signed, each showing the project/CLA group, signing date, and status. For signed ICLAs they can download the signed PDF. Signing still routes to the existing Contributor Console (linked out).

> As built, the page landed as a Profile-hub tab (constant `MY_CLAS_PATH`, route `/profile/clas`) rather than the originally planned standalone Me-lens module at `/me/clas`, and it lists **all** agreements with their computed status — including no-longer-valid ECLAs, which are exactly the rows that carry M2's Request-approval action.

**Acceptance Scenarios**:

1. **Given** a logged-in user who has signed at least one ICLA, **When** they open My CLAs, **Then** each signed ICLA is listed with project name, date signed, and a working signed-PDF download.
2. **Given** a logged-in user with a valid ECLA, **When** they open My CLAs, **Then** the ECLA is listed with company name, project, and acknowledgement date, and no PDF download is offered.
3. **Given** a logged-in user with no CLA history, **When** they open My CLAs, **Then** an empty state explains what CLAs are and links to documentation.
4. **Given** any user viewing My CLAs, **When** they look for signing actions, **Then** none exist in Self Serve; any "sign" pointers link out to the Contributor Console.

## Functional Requirements

- **FR-001**: Self Serve MUST display, for the logged-in user, all ICLAs they have signed (any status: valid, superseded, expired), with project/CLA group name, date signed, and validity status.
- **FR-002**: Self Serve MUST display all of the user's ECLAs with company name, project/CLA group, and acknowledgement date, and MUST NOT offer a PDF for ECLAs (none exists). *(As built, this widened from "currently valid ECLAs only": every ECLA is shown with its computed status, because invalid rows are the ones needing M2's manager-routed Request-approval action.)*
- **FR-003**: Users MUST be able to download the signed PDF for each of their signed ICLAs via a time-limited link.
- **FR-004**: The view MUST be read-only; any signing affordance MUST link out to the existing Contributor Console.
- **FR-005**: The system MUST resolve the Self Serve identity (LF SSO) to the user's EasyCLA user record(s) by LF username, verified emails, AND GitHub account(s) linked to the LF identity — covering pre-LF-login history — and aggregate agreements across all matched records. *(As built, resolution lives inside EasyCLA, not SS: SS forwards session-derived identity keys to `GET /v4/my-clas`, and EasyCLA verifies each key against the caller's LF account using its own user records, the platform user-service, and the Auth0 Management API — the third source added in [linuxfoundation/easycla#5172](https://github.com/linuxfoundation/easycla/pull/5172) — before searching it; unverifiable keys are reported back in `skippedIdentities`.)*
- **FR-005a**: When no GitHub account is linked to the LF identity (or no agreements are found), the UI MUST offer a "Don't see your CLAs? Link your GitHub account" action into Self Serve's existing identity-linking flow, and re-resolve after linking.
- **FR-006**: Users MUST see only their own agreements; no access to other users' signature data through this surface. *(Enforced server-side in EasyCLA — ownership is checked per identity key, and the PDF endpoint returns 404, never 403, for anything not owned.)*

## Success Criteria

- **SC-001**: 100% of a sampled user population's signed ICLAs and valid ECLAs visible in EasyCLA's data are also visible in Self Serve; ICLA PDF download success rate ≥ 99%.
- Unmatched-identity telemetry live with an agreed threshold (launch gate for M2).

## Scope boundaries

In: Profile-hub "CLAs" tab at `/profile/clas`, SS server routes, identity-key forwarding + telemetry, PDF presigned-URL hand-off, empty state, feature flag.
Out: any signing/writes, CCLA data (M3, Organization lens), approval lists/roles, changes to the PR remediation link, EasyCLA backend changes beyond the three read endpoints `GET /v4/my-clas`, `GET /v4/my-clas/{signatureID}/pdf`, and `GET /v4/my-clas/identities` (implemented in [linuxfoundation/easycla#5125](https://github.com/linuxfoundation/easycla/pull/5125)) — see [contracts/upstream-easycla-api.md](contracts/upstream-easycla-api.md).

## Design artifacts

[plan.md](plan.md) · [data-model.md](data-model.md) · [contracts/](contracts/)
