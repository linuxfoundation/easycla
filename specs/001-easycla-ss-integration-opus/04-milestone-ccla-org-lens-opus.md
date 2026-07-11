<!-- SPDX-License-Identifier: CC-BY-4.0 -->
# Milestone 4 — CCLA management in the Org lens

**Status**: Draft · **Effort**: **XL** · **Retires**: **Corporate CLA Dashboard** (after burn-in) · **Prereq**: M1–M3

## Goal

Migrate the **Corporate CLA Dashboard** (`lfx-corp-cla-console`) into SS's **Org lens**, at feature parity, then retire it. This is the largest UI migration: it is the heaviest **role surface** (CLA managers, signatories, designees), it owns the **CCLA signing** flow, **approval-list management** across four coverage types, activity logs, contributor acknowledgements, sanctioned-org handling, and foundation-level CLA — and it currently runs against a **separate GraphQL backend** that we could not locate in the `easycla` repo (**R1**).

## Known risk carried into this milestone (R1 — must resolve first)
The Corporate Console uses an **Apollo GraphQL** backend (`lf-backend-cla.<env>.platform.linuxfoundation.org/graphql`) plus a BFF — **not** the Go `/v4` REST surface used by the Contributor Console and PCC. Its source was **not found** in the `easycla` repo during research.
- **Decision (from scope approval): design SS integration against the Go `/v4` REST API** as the strategic target; treat the GraphQL service as legacy to bypass.
- **Blocking action item before build**: locate/inventory the GraphQL service and confirm that every Corporate-Console operation below has a `/v4` REST equivalent. **Any operation that exists only in GraphQL is a scope addition** (either add a `/v4` endpoint or, as a fallback, have the SS BFF call GraphQL transitionally). This gap analysis is the first task of M4 and may resize it.

## Feature inventory to reach parity (from the Corporate Console)
1. **CCLA signing** — authority name/email, returns DocuSign `signUrl` (backend-mediated, same pattern as ICLA), return `?cclaSigned=true`; sanctioned-org handling.
2. **Approval-list management** — add/remove across **four coverage types**: individual email, email domain, GitHub org/team, GitLab group; plus the `auto_create_ecla` toggle.
3. **CLA Manager management** — add / remove managers; assign designee; approve self-nominations.
4. **Contributor acknowledgements (ECLA) view** — paginated, searchable list of covered employees.
5. **Activity logs** — signature/manager/approval-list events, paginated, searchable; CSV export where present.
6. **Company & foundation views** — company CLA dashboard; foundation-level CLA (a single CCLA covering all projects in a foundation) and per-project CLA management.
7. **Metrics/insights** — signed/pending counts, per-company-per-project metrics.

## User Scenarios & Testing

### User Story 1 — Manage the approved list (Priority: P1)

A CLA Manager, in the Org lens for their company, adds and removes contributors/domains/GitHub-orgs/GitLab-groups and toggles auto-create-ECLA.

**Acceptance Scenarios**:
1. **Given** a CLA Manager, **When** they add an email domain, **Then** it is saved and future contributors on that domain are covered (and auto-ECLA'd if enabled).
2. **Given** a CLA Manager, **When** they remove a GitHub org, **Then** coverage from that org stops for future evaluations.
3. **Given** a **non**-CLA-Manager who nonetheless has platform Org-lens access, **When** they open the approval list, **Then** editing is denied (R2 — CLA authority comes from EasyCLA, not the lens).

### User Story 2 — Sign a CCLA (Priority: P1)

An authorized signatory signs the corporate CLA via the backend-mediated DocuSign flow and returns to SS with the CCLA reflected as signed.

**Acceptance Scenarios**:
1. **Given** a signatory, **When** they initiate CCLA signing, **Then** SS requests the corporate signature from the backend and redirects to the DocuSign `signUrl`.
2. **Given** a sanctioned organization, **When** signing is attempted, **Then** it is blocked with the correct message.

### User Story 3 — Manage CLA Managers & designees (Priority: P1)

Add/remove managers, assign a designee, approve a self-nomination — all authorized and stored via the EasyCLA adapter.

**Acceptance Scenarios**:
1. **Given** an authorized manager, **When** they add another manager, **Then** the change is written through EasyCLA and visible identically in any remaining legacy surface.

### User Story 4 — Acknowledgements, activity, metrics (Priority: P2)
Paginated acknowledgements list, activity log with search + CSV export, and insight counts at parity.

### User Story 5 — Foundation-level CLA (Priority: P3)
Sign/view a CCLA at the foundation level covering all its projects, plus per-project management, at parity.

### Edge Cases
- A Corporate-Console operation with **no `/v4` equivalent** (R1) → surfaced as an explicit gap, not silently dropped.
- Large approval lists / activity logs → pagination and search must scale.
- Concurrent edits by two managers → last-write behavior must match today's semantics (no silent clobber of the whole list).
- Company with multiple signing entities → correct entity selection preserved.

## Requirements

### Functional
- **FR-4.1**: SS MUST reach **feature parity** with the Corporate Console for items 1–7 above, in the Org lens.
- **FR-4.2**: SS MUST integrate against the Go `/v4` REST API; a **gap analysis vs the GraphQL backend MUST precede build** and any GraphQL-only operation MUST be resolved (new `/v4` endpoint or transitional GraphQL call) before that feature ships (R1).
- **FR-4.3**: **All CLA authorization (manager/signatory/designee, approval-list edits, CCLA signing) MUST be enforced by the EasyCLA layer, never inferred from platform Org-lens access** (R2). This is the milestone where R2 is most dangerous.
- **FR-4.4**: CCLA signing MUST use the existing backend-mediated DocuSign flow (`signUrl` redirect); SS MUST NOT integrate DocuSign directly.
- **FR-4.5**: Approval-list edits MUST preserve today's coverage semantics, including the `auto_create_ecla` interaction that generates ECLA records.
- **FR-4.6**: Activity log and acknowledgements MUST support pagination and search; CSV export MUST be preserved where it exists today.

### Non-Functional
- **NFR-4.1**: Manager-facing lists MUST remain responsive for large companies (many contributors/domains/events).
- **NFR-4.2**: The migration MUST allow running SS Org-lens CLA and the legacy Corporate Console **in parallel** during burn-in, reading/writing the same source of truth (no divergence).

### Key Entities
- **Approval list**: four coverage collections + auto-create-ECLA flag, scoped company+project (+ foundation).
- **CLA role set**: managers, signatories, designees (via adapter).
- **Activity event**; **Acknowledgement (ECLA) record** (from M3's model); **CCLA signature** (has PDF).

## Retirement gate — Corporate CLA Dashboard
After parity across items 1–7 (including R1 gaps closed) and a parallel-run burn-in with identical state in SS and the legacy console, the **Corporate CLA Dashboard can be decommissioned** — a separate go/no-go decision.

## Success Criteria
- **SC-4.1**: A CLA Manager can perform every Corporate-Console task from the SS Org lens.
- **SC-4.2**: No approval-list, manager, or signing action is authorized by platform lens access alone (R2 verified).
- **SC-4.3**: State written in SS and in the legacy console is identical during parallel run (no drift).
- **SC-4.4**: Every Corporate-Console operation has a confirmed `/v4` (or explicitly-approved transitional) path; the R1 gap list is empty at go-live.
- **SC-4.5**: With parity reached, the Corporate CLA Dashboard can be turned off without loss of function.

## Assumptions
- The Go `/v4` API covers most Corporate-Console operations; the gap analysis (FR-4.2) will quantify the remainder. **This is the biggest sizing uncertainty in the program** and is why M4 is XL.
- The adapter role strategy holds; if early-OpenFGA is chosen instead, M4 additionally owns the CLA-role OpenFGA sync.
