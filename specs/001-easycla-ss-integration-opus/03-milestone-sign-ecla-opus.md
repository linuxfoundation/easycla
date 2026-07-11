<!-- SPDX-License-Identifier: CC-BY-4.0 -->
# Milestone 3 — Sign an ECLA (employee acknowledgement) in SS

**Status**: Draft · **Effort**: **L** · **Retires**: **Contributor Console** (after burn-in) · **Prereq**: M1, M2

## Goal

Move the **corporate contributor** path into SS: a contributor covered by their employer's CCLA can, from a PR "signed agreement missing" link, complete the flow in SS — select/confirm their company, and either (a) be acknowledged under an existing CCLA (**ECLA**) or (b) trigger the "notify a CLA Manager" / "request to be added to the approved list" path when they are not yet covered. After M3 reaches parity and burns in, the **Contributor Console can be retired entirely**.

This is the first milestone that touches **CLA roles** — `cla-manager`, `cla-manager-designee`, `cla-signatory` — because the corporate path surfaces "who manages this company's CLA" and "ask them to approve me." Per the [overview §4.1](00-overview-opus.md), SS handles roles through the **EasyCLA adapter** (delegating to `/v4`), not a new OpenFGA sync.

## Why ECLA is different from ICLA (grounds the scope)
- An **ECLA has no DocuSign envelope and no PDF.** It is an approval record created in DynamoDB when a covered employee acknowledges, or automatically when `auto_create_ecla` is on and the approval list matches. So M3 is **not** "another DocuSign flow" — it is a **coverage-check + acknowledgement + notify-manager** flow.
- The hard part is **eligibility and role interaction**: is the user covered (email/domain/GitHub-org/GitLab-group on the company's approval list)? If not, who is the CLA Manager to notify, and how does the designee/self-nomination path work?

## User Scenarios & Testing

### User Story 1 — Acknowledge as a covered employee (Priority: P1)

A contributor whose company has a CCLA, and who is already covered by the approval list, confirms their company in SS and is acknowledged (ECLA), turning the PR check green — no DocuSign.

**Acceptance Scenarios**:
1. **Given** a contributor covered by their company's approval list, **When** they confirm their company in SS, **Then** an ECLA acknowledgement is recorded and the PR check passes, with no DocuSign step.
2. **Given** the company has `auto_create_ecla` enabled and the user matches, **When** the PR is evaluated, **Then** coverage is reflected without manual acknowledgement.

### User Story 2 — Not yet covered: notify a CLA Manager (Priority: P1)

A contributor selects their company but is not on the approved list. SS lets them request to be added / notify the company's CLA Manager(s).

**Acceptance Scenarios**:
1. **Given** an uncovered contributor who selects a company with known CLA Managers, **When** they request access, **Then** the manager(s) are notified (existing backend notify path) and the user sees a pending state.
2. **Given** a company with **no** CLA Manager yet, **When** the contributor proceeds, **Then** SS surfaces the designee / company-admin invitation path rather than dead-ending.

### User Story 3 — Company selection & creation (Priority: P2)

Company search (with the existing org search + external-data lookup) and, where a company is not yet in the system, the create-company path — at parity with the Contributor Console.

**Acceptance Scenarios**:
1. **Given** a contributor typing a company name, **When** results return, **Then** they can select the correct organization (including disambiguation by website/domain).
2. **Given** a company not in the system, **When** the contributor proceeds, **Then** the existing create-company path is available.

### User Story 4 — CLA Manager designee / self-nomination (Priority: P3)

The corporate path's role affordances — self-designate as CLA Manager designee, or nominate — at parity, authorized via the EasyCLA adapter.

**Acceptance Scenarios**:
1. **Given** an eligible user, **When** they self-designate, **Then** the designee role is created through the existing EasyCLA API and reflected back.

### Edge Cases
- Contributor's email domain matches multiple companies → disambiguation required.
- Company is **sanctioned** → block acknowledgement with the correct message.
- Approval list changes between coverage-check and acknowledgement → re-check at acknowledge time.
- Gerrit/GitLab corporate contributors → parity for non-GitHub providers (R7).

## Requirements

### Functional
- **FR-3.1**: SS MUST let a covered employee record an ECLA acknowledgement via the existing backend path; no DocuSign, no PDF.
- **FR-3.2**: SS MUST evaluate coverage using the company's approval list (email/domain/GitHub-org/GitLab-group) as the backend does today, and re-check at acknowledge time.
- **FR-3.3**: SS MUST provide the "notify CLA Manager / request to be added" path, including the empty-manager case (designee / company-admin invite).
- **FR-3.4**: SS MUST provide company search and create-company at parity.
- **FR-3.5**: **All CLA-role reads/writes (manager, designee, signatory) MUST be delegated to the EasyCLA `/v4` API (adapter).** SS MUST NOT create a parallel role store and MUST NOT infer CLA roles from the platform lens gate (**R2**).
- **FR-3.6**: SS MUST support the corporate path for GitHub, Gerrit, and GitLab entry points (R7).
- **FR-3.7**: After M3 reaches parity, the PR deep-link's corporate path MUST be repointable to SS (config/flag, reversible — R6), enabling Contributor Console retirement.

### Non-Functional
- **NFR-3.1**: Coverage evaluation SHOULD be fast enough to feel synchronous during company confirmation.
- **NFR-3.2**: Manager-notification actions MUST be idempotent (no notification storms on repeated clicks).

### Key Entities
- **Coverage decision**: inputs (user identities, company approval list) → covered / not-covered / needs-manager.
- **CLA role association** (read/written via adapter): manager, designee, signatory, scoped to company+project.
- **Notification request**: uncovered user → company CLA managers.

## Role-model note (the crux, applied)
M3 is where the [role-model gap](00-overview-opus.md#4-the-central-challenge-the-role-model-gap) first bites. The adapter approach keeps EasyCLA as the single source of truth for CLA roles — concretely, EasyCLA's `/v4` API fronts **ACS** (role definitions/invites, e.g. `SendUserInvite`) and the **Organization Service** (scope grants, `IsUserHaveRoleScope`); SS renders and initiates, EasyCLA authorizes and stores. There is **no ACS→OpenFGA bridge today** (R9), which is exactly why M3 must not attempt one — it delegates. The **two-layer authz** rule (R2) is a hard requirement: platform lens access must never be read as CLA-manager authority.

## Retirement gate — Contributor Console
After M3 is at parity for **all** entry providers (GitHub/Gerrit/GitLab) and both paths (individual from M2, corporate here) and has burned in behind the reversible deep-link flag, the **Contributor Console UI can be decommissioned.** This is a separate go/no-go decision, not automatic on merge.

## Success Criteria
- **SC-3.1**: A covered employee completes acknowledgement in SS with the PR check passing and no DocuSign.
- **SC-3.2**: An uncovered contributor can notify a CLA Manager (and the no-manager case is handled).
- **SC-3.3**: CLA-role state is identical whether viewed from SS or the legacy console (single source of truth; no drift).
- **SC-3.4**: No path grants CLA-management authority from platform lens access alone (R2 verified by test).
- **SC-3.5**: With the corporate path live in SS, the Contributor Console can be turned off without loss of function.

## Assumptions
- The existing employee-signature/acknowledgement, coverage-check, notify-managers, company-search, create-company, and designee APIs are reusable from the SS BFF.
- The adapter strategy from the overview is ratified by architecture review; if instead early-OpenFGA is chosen, M3 grows to include the sync work described (and rejected) in the overview.
