<!-- SPDX-License-Identifier: CC-BY-4.0 -->
# Milestone 1 — Read-only agreements in the Me lens

**Status**: Draft · **Effort**: **S–M** · **Retires**: nothing (additive) · **Prereq for**: M2, M3

## Goal

Let a signed-in SS user see **their own** CLA agreements in the **Me lens** — every signed **ICLA** and every valid **ECLA** — read-only. Where a signed PDF exists (ICLA; and, if the user is also a corporate signer, CCLA), let them download it. Signing new agreements is **out of scope**: "Sign a new agreement" links continue to forward the user to the existing Contributor Console.

This milestone is intentionally the smallest useful slice. It requires **no change to the EasyCLA backend** — the read and PDF-download APIs already exist — so its real work is (a) the SS BFF wiring and (b) resolving the SS user's identity to their EasyCLA records.

## User Scenarios & Testing

### User Story 1 — See my agreements (Priority: P1)

A contributor signs in to SS, opens the Me lens, and sees a list of their CLA agreements: which project/CLA group, agreement type (ICLA / ECLA), the covering company for ECLAs, signed date, and status.

**Why this priority**: Highest value-to-cost ratio in the whole program. Delivers a visible, useful feature with no backend change and establishes the SS↔EasyCLA seam every later milestone reuses.

**Independent Test**: Sign in as a user known to have ≥1 ICLA and ≥1 ECLA; confirm both appear with correct type, company (ECLA), project, and date.

**Acceptance Scenarios**:
1. **Given** a user with a signed ICLA, **When** they open the Me lens CLA section, **Then** the ICLA is listed with project, signed date, and status = valid.
2. **Given** a user covered by a company CCLA (an ECLA record), **When** they view the list, **Then** the ECLA is shown with the covering company name and no PDF-download affordance.
3. **Given** a user with no agreements, **When** they view the section, **Then** an empty state explains they have no CLAs and links to the Contributor Console to sign one.

### User Story 2 — Download my signed ICLA PDF (Priority: P2)

From an ICLA row, the user downloads the signed PDF.

**Why this priority**: Valuable but strictly additive to Story 1; depends on the pre-signed-URL path.

**Independent Test**: For an ICLA row, click download and confirm the correct signed PDF opens.

**Acceptance Scenarios**:
1. **Given** an ICLA with a stored PDF, **When** the user clicks download, **Then** the signed PDF is delivered (via a short-lived pre-signed link).
2. **Given** an ECLA, **When** the user views the row, **Then** no download affordance is shown (ECLAs have no PDF).
3. **Given** a pre-signed link that has expired, **When** the user retries, **Then** a fresh link is generated rather than an error.

### User Story 3 — Forward to sign a new agreement (Priority: P3)

A "Sign a new agreement" affordance forwards the user to the existing Contributor Console (unchanged behavior), preserving M1's read-only boundary.

**Acceptance Scenarios**:
1. **Given** the read-only view, **When** the user chooses to sign a new agreement, **Then** they are forwarded to the Contributor Console.

### Edge Cases
- User whose SS/Auth0 identity maps to **multiple** EasyCLA user records (linked GitHub + GitLab + LF identities) — the list must union all, without duplicates.
- User with an **invalidated / superseded** ICLA (newer version exists) — status must distinguish valid vs superseded rather than hiding it.
- Agreement referencing a project/CLA group the user can no longer access — still shown (it's the user's own record), with graceful handling of missing project metadata.
- ECLA whose covering **CCLA has since become invalid** — "valid ECLA" per the brief means the ECLA record is approved/signed; surface the covering-company status if the backend exposes it, otherwise show the ECLA as-is and note the limitation.

## Requirements

### Functional
- **FR-1.1**: SS MUST display, in the Me lens, the signed-in user's own ICLA and ECLA agreements. CCLAs (corporate, company-referenced) are **not** listed here (they belong to the Org lens, M4) unless the user is personally the signer — deferred to M4.
- **FR-1.2**: Each row MUST show agreement type, project / CLA-group name, covering company (ECLA only), signed date, and validity status.
- **FR-1.3**: SS MUST allow download of a signed **ICLA** PDF where one exists, via a short-lived link. ECLA rows MUST NOT offer download.
- **FR-1.4**: The view MUST be strictly read-only; the only mutating affordance is an outbound link to the Contributor Console.
- **FR-1.5**: SS MUST resolve the authenticated user's identity to their EasyCLA user record(s) and union all matches (**R4**).
- **FR-1.6**: The BFF MUST authorize each request as "the requesting user reading their own agreements" — a user MUST NOT be able to read another user's agreements by changing an identifier.
- **FR-1.7**: On upstream (EasyCLA API) failure, SS MUST show a non-destructive error state, not a blank or broken page.

### Non-Functional
- **NFR-1.1**: The agreements list SHOULD render within a normal SS page-load budget for a user with a typical number of agreements (tens).
- **NFR-1.2**: PDF links MUST expire quickly enough to avoid becoming durable shareable URLs.

### Key Entities
- **Agreement (read model)**: type (ICLA/ECLA), project/CLA-group, covering company (ECLA), signed date, status, `hasPdf` flag. Derived from the EasyCLA signature record.
- **Identity mapping**: association between the SS/Auth0 principal and one-or-more EasyCLA user records.

## Integration notes (for architecture review)
- **Reuses existing EasyCLA APIs** — a user-signatures read (returns ICLA+ECLA) and ICLA/CCLA PDF download (pre-signed URL). No new EasyCLA endpoint expected; confirm response shape during design.
- **SS side (BFF pattern)**: new `cla` Angular module + Angular service → `/api/cla/*` Express route → controller → server service calling EasyCLA `/v4` with bearer-token pass-through; shared types in `packages/shared`. Add a Me-lens nav entry.
- **Identity resolution (R4)** is the one genuinely new piece and the main design risk here; get it right because M2–M5 reuse it.

## Success Criteria
- **SC-1.1**: A user with existing agreements sees a complete, correct list on first load (all ICLA+ECLA, no duplicates across linked identities).
- **SC-1.2**: Users can download any signed ICLA PDF they own.
- **SC-1.3**: No user can retrieve another user's agreements or PDFs (verified by an authorization test attempting cross-user access).
- **SC-1.4**: Zero changes required to the EasyCLA backend to ship M1.

## Assumptions
- The existing user-signatures and PDF-download APIs behave as the code indicates (ICLA+ECLA returned; ICLA/CCLA have pre-signed PDF download; ECLA has none).
- SS already has the Auth0 session and bearer token needed to call EasyCLA on the user's behalf, or an equivalent service credential is available to the BFF.
- "Valid ECLA" = an approved, signed ECLA record; covering-CCLA validity is surfaced only if cheaply available.
