# Feature Specification: M1 — Read-only "My CLAs" in the Self Serve Me Lens

**Parent program**: [../spec.md](../spec.md) (EasyCLA → LFX Self Serve Integration) | **Milestone doc**: [../01-milestone-read-only-me-lens-fable.md](../01-milestone-read-only-me-lens-fable.md)
**Created**: 2026-07-11 | **Status**: Implemented (dark-launched behind the `my-clas-enabled` flag) — delivery epic [linuxfoundation/lfx-self-serve#1157](https://github.com/linuxfoundation/lfx-self-serve/issues/1157) (closed); backend PRs [linuxfoundation/easycla#5125](https://github.com/linuxfoundation/easycla/pull/5125), [linuxfoundation/easycla#5128](https://github.com/linuxfoundation/easycla/pull/5128); API reference [docs/MY_CLAS_API.md](../../../docs/MY_CLAS_API.md)

This is the extracted, implementable slice for Milestone 1, updated 2026-09-01 to reflect the implementation. Program-wide context, assumptions, and resolved decisions live in the parent spec.

## User Story (P1)

A contributor logs into LFX Self Serve and opens the **"CLAs" tab in the Profile hub** (`/profile/clas`). They see every ICLA and ECLA they have signed, each showing the project/CLA group and signing date. For signed ICLAs they can download the signed PDF. Signing still routes to the existing Contributor Console (linked out).

> As built, the page landed as a Profile-hub tab (constant `MY_CLAS_PATH`, route `/profile/clas`) rather than the originally planned standalone Me-lens module at `/me/clas`, and it lists **all** agreements, including no-longer-valid ECLAs.
>
> **Status display is M2, not M1.** M1's shipped table was Project / Type / Signed / Document ([`profile-clas.component.html`](https://github.com/linuxfoundation/lfx-self-serve/blob/main/apps/lfx-one/src/app/modules/profile/clas/profile-clas.component.html) as of lfx-self-serve#1226) — the backend supplied the boolean `valid`, but no status column was rendered. The visible five-value status column arrived with the M2 overlay (lfx-self-serve#1440) and is gated behind `my-clas-m2-enabled`; with the flag off the fourth column is still Document.
>
> Within that M2 overlay, the manager actions are scoped differently from one another: Request approval requires `status=needs_attention` with `statusReason=not_on_approval_list`, Request Removal is offered on any non-revoked ECLA, and Contact CLA Manager on `valid`/`needs_attention` — so only `revoked` rows carry no action (see [docs/MY_CLAS_API.md](../../../docs/MY_CLAS_API.md)).

**Acceptance Scenarios**:

1. **Given** a logged-in user who has signed at least one ICLA, **When** they open My CLAs, **Then** each signed ICLA is listed with project name, date signed, and a working signed-PDF download.
2. **Given** a logged-in user with an ECLA, **When** they open My CLAs, **Then** the ECLA is listed with company name, project, and acknowledgement date — regardless of whether it is still valid — and no PDF download is offered. *(The visible status column is part of the M2 overlay, behind `my-clas-m2-enabled`; M1 rendered no status.)*
3. **Given** a logged-in user with no CLA history, **When** they open My CLAs, **Then** an empty state explains what CLAs are and links to documentation.
4. **Given** any user viewing My CLAs, **When** they look for signing actions, **Then** none exist in Self Serve; any "sign" pointers link out to the Contributor Console.

## Functional Requirements

- **FR-001**: Self Serve MUST display, for the logged-in user, all ICLAs they have signed regardless of status, with project/CLA group name, date signed, and validity status. *(As built, M1 consumed the boolean `valid` but rendered no status column — its table was Project / Type / Signed / Document. The five-value computed status — `valid`/`needs_attention`/`invalidated`/`revoked`/`unknown` — and its column arrived in M2 behind `my-clas-m2-enabled`. Neither exposes `superseded` or `expired`: the original wording named statuses the endpoint has never produced. `superseded` is reserved in the SS interface for forward compatibility.)*
- **FR-002**: Self Serve MUST display all of the user's ECLAs with company name, project/CLA group, and acknowledgement date, and MUST NOT offer a PDF for ECLAs (none exists). *(As built, this widened from "currently valid ECLAs only": every ECLA is listed regardless of validity, because the not-yet-covered rows are the ones needing M2's manager-routed Request-approval action — specifically `needs_attention` / `not_on_approval_list`, not every non-valid status. The status itself became visible only with the M2 overlay.)*
- **FR-003**: Users MUST be able to download the signed PDF for each of their signed ICLAs via a time-limited link.
- **FR-004**: The view MUST be read-only; any signing affordance MUST link out to the existing Contributor Console.
- **FR-005**: The system MUST resolve the Self Serve identity (LF SSO) to the user's EasyCLA user record(s) by LF username, verified emails, AND GitHub account(s) linked to the LF identity — covering pre-LF-login history — and aggregate agreements across all matched records. *(As built, resolution lives inside EasyCLA, not SS: SS forwards session-derived identity keys to `GET /v4/my-clas`. For untrusted callers EasyCLA verifies each key against the caller's LF account using its own user records, the platform user-service, and the Auth0 Management API — the third source added in [linuxfoundation/easycla#5172](https://github.com/linuxfoundation/easycla/pull/5172) — before searching it, reporting unverifiable keys in `skippedIdentities`; admins and callers with an allow-listed `azp` supply keys directly without per-key verification, but that path is switched off for SS today.)*
- **FR-005a**: When no GitHub account is linked to the LF identity (or no agreements are found), the UI MUST offer a "Don't see your CLAs? Link your GitHub account" action into Self Serve's existing identity-linking flow, and re-resolve after linking.
- **FR-006**: Users MUST see only their own agreements; no access to other users' signature data through this surface. *(Enforced server-side in EasyCLA: every returned signature must belong to the resolved identity set, and the PDF endpoint returns 404, never 403, for anything not owned. Per-**key** ownership is additionally verified for untrusted callers, which is how SS is treated as deployed; on the trusted path that check is skipped and FR-006 relies on SS deriving keys only from the authenticated session.)*

## Success Criteria

- **SC-001**: 100% of a sampled user population's signed ICLAs and ECLAs visible in EasyCLA's data are also visible in Self Serve (as built, all ECLAs — not only valid ones — per FR-002); ICLA PDF download success rate ≥ 99%.
- Unmatched-identity telemetry live with an agreed threshold (launch gate for M2).

## Scope boundaries

In: Profile-hub "CLAs" tab at `/profile/clas`, SS server routes, identity-key forwarding + telemetry, PDF presigned-URL hand-off, empty state, feature flag.
Out: any signing/writes, CCLA data (M3, Organization lens), approval lists/roles, changes to the PR remediation link, EasyCLA backend changes beyond the three read endpoints `GET /v4/my-clas` and `GET /v4/my-clas/{signatureID}/pdf` ([linuxfoundation/easycla#5125](https://github.com/linuxfoundation/easycla/pull/5125)) plus `GET /v4/my-clas/identities` ([linuxfoundation/easycla#5128](https://github.com/linuxfoundation/easycla/pull/5128)) — see [contracts/upstream-easycla-api.md](contracts/upstream-easycla-api.md).

## Design artifacts

[plan.md](plan.md) · [data-model.md](data-model.md) · [contracts/](contracts/)
