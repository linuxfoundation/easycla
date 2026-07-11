<!-- SPDX-License-Identifier: CC-BY-4.0 -->
# Milestone 2 — Sign an ICLA in SS

**Status**: Draft · **Effort**: **M** · **Retires**: nothing yet (Contributor Console still needed for corporate path) · **Prereq**: M1 (identity mapping, BFF seam) · **Prereq for**: M3

## Goal

When a contributor clicks "Signed agreement missing" on a GitHub PR (or the Gerrit/GitLab equivalent) and their situation is **individual**, they land in **SS** and complete ICLA signing there instead of in the Contributor Console. Signing itself still happens in **DocuSign**, mediated by the **existing EasyCLA Go backend** — SS obtains a `signUrl` from the backend and redirects to it, exactly as the Contributor Console does today.

**Key finding that shapes this milestone**: DocuSign integration is entirely backend-side. **SS needs no DocuSign SDK, credentials, or a new DocuSign micro-service.** It calls the existing individual-signature request API, receives a `signUrl`, redirects the user to DocuSign, and DocuSign returns the user to a `returnUrl`. This closes the "should we build a small DocuSign service?" question from the brief: not needed.

## Scope boundary
- **In scope**: the *individual* ICLA signing path, initiated from a PR check, completed in SS via the backend-mediated DocuSign flow, for GitHub **and** Gerrit/GitLab (parity — R7).
- **Out of scope**: the corporate path (ECLA/CCLA — M3/M4). The individual-vs-corporate **decision** may still be made in a thin retained Contributor Console surface (see §"Retained Contributor Console").

## User Scenarios & Testing

### User Story 1 — Sign an ICLA from a GitHub PR, in SS (Priority: P1)

A contributor opens a PR; the CLA check comments "signed agreement missing" with a link. Following it (having chosen "individual") lands them in SS, which starts the ICLA flow, sends them to DocuSign, and on completion returns them to the PR, now passing.

**Why this priority**: The core of the milestone; the highest-volume signing path.

**Independent Test**: From a test PR, follow the individual-signing link into SS, complete DocuSign, confirm the signature is recorded and the PR check flips to passing.

**Acceptance Scenarios**:
1. **Given** an unsigned contributor on a gated PR who chooses "individual", **When** they proceed in SS, **Then** SS requests an individual signature from the backend and redirects them to the returned DocuSign `signUrl`.
2. **Given** the user completes signing in DocuSign, **When** DocuSign returns to the `returnUrl`, **Then** the backend callback records the signature, stores the PDF, and the PR check becomes passing.
3. **Given** the user already has a valid current-version ICLA, **When** they follow the link, **Then** SS does not create a duplicate and communicates they are already covered.

### User Story 2 — Resume an in-progress signature (Priority: P2)

A user who started but didn't finish signing returns and is taken back into the in-progress DocuSign envelope rather than starting over.

**Acceptance Scenarios**:
1. **Given** an in-progress (unsigned) signature, **When** the user re-enters the flow, **Then** SS resumes it (fresh `signUrl` for the same envelope) rather than creating a new one.

### User Story 3 — Gerrit / GitLab parity (Priority: P2)

The same SS ICLA flow works for Gerrit and GitLab entry points, honoring provider-specific post-sign behavior (e.g. the Gerrit re-login step).

**Acceptance Scenarios**:
1. **Given** a Gerrit contributor, **When** they sign in SS, **Then** the correct Gerrit return/callback path is used and any required re-login guidance is shown.
2. **Given** a GitLab contributor, **When** they sign in SS, **Then** the GitLab merge-request check is updated on return.

### Edge Cases
- DocuSign `signUrl` expires before use → SS regenerates rather than dead-ends.
- User abandons at DocuSign and returns to the PR unsigned → check remains failing; re-entry works.
- `returnUrl` tampering / open-redirect attempts → returnUrl must be validated against an allowlist.
- Backend returns a sanctioned/blocked status → SS shows the appropriate message, no signing.

## Requirements

### Functional
- **FR-2.1**: SS MUST initiate ICLA signing by calling the existing backend individual-signature request API and MUST redirect the user to the returned DocuSign `signUrl`. SS MUST NOT integrate DocuSign directly.
- **FR-2.2**: SS MUST pass a valid `returnUrl` / `returnUrlType` so the existing backend callback (`/v4/signed/...`) fires and updates the PR/MR/Gerrit check. The `returnUrl` MUST be validated (no open redirect — R6).
- **FR-2.3**: SS MUST support the ICLA flow for GitHub, Gerrit, and GitLab entry points (R7).
- **FR-2.4**: The backend PR-comment/deep-link target MUST be repointable from the Contributor Console to SS for the individual path, via configuration and staged rollout (R6).
- **FR-2.5**: SS MUST detect an existing valid signature and avoid duplicate envelopes; it MUST resume an in-progress envelope where one exists.
- **FR-2.6**: All signature state changes and PDF storage remain owned by the backend callback; SS MUST NOT write signature state directly.

### Non-Functional
- **NFR-2.1**: The redirect to DocuSign SHOULD occur promptly after the user proceeds (no long spinner).
- **NFR-2.2**: The cutover MUST be reversible (feature-flag the deep-link target) so a regression can revert to the Contributor Console quickly.

### Key Entities
- **Signature request**: project/CLA-group, user, returnUrl, returnUrlType → yields `signUrl`, envelope id.
- **Deep-link contract**: the parameters a PR check emits to route a contributor to the signing UI.

## Retained Contributor Console (transitional)
Per the brief, a thin Contributor Console surface may remain to host the **individual-vs-corporate decision** until M3 moves the corporate path into SS. M2 does **not** retire the Contributor Console. Two viable arrangements — to decide at design time:
- **(a)** PR link → thin console decision page → individual path deep-links into SS; corporate path stays in console.
- **(b)** PR link → SS hosts the decision too; SS handles individual, and forwards corporate to the console until M3.
Option (b) reduces the retained console sooner but enlarges M2 slightly. Recommend deciding based on how contained the decision page is.

## Success Criteria
- **SC-2.1**: A contributor can complete an ICLA end-to-end starting from a PR and finishing in SS, with the PR check flipping to passing.
- **SC-2.2**: No duplicate signatures are created for users who re-enter or who are already covered.
- **SC-2.3**: Gerrit and GitLab individual signing work with correct check updates.
- **SC-2.4**: The deep-link target can be flipped between Console and SS without a deploy (config/flag), and reverted.
- **SC-2.5**: SS holds no DocuSign credentials (verified by inspection).

## Assumptions
- The existing individual-signature request API and its callbacks are reusable unchanged from a new caller (SS BFF); the only backend change is making the outbound deep-link/PR-comment target configurable.
- returnUrl handling in the backend already supports the provider types; SS supplies the correct type.
