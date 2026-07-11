<!-- SPDX-License-Identifier: CC-BY-4.0 -->
# Milestone 5 — EasyCLA project configuration in the Project lens

**Status**: Draft · **Effort**: **L–XL** · **Retires**: **EasyCLA functionality in PCC** (after burn-in) · **Prereq**: M1–M4 (esp. role adapter)

## Goal

Move the EasyCLA **project-administration** surface out of **PCC** (`lfx-pcc`, `Tools > CLA`) into SS's **Project lens**, at parity, then remove EasyCLA from PCC. This is project-owner/admin functionality: creating CLA groups, installing the EasyCLA **GitHub App** and enrolling repos, connecting GitLab/Gerrit, managing **CLA PDF templates**, and viewing signatures/approval-criteria/events.

Good news for sizing: PCC already integrates via a **BFF that proxies to the Go `/cla-service/v4/*` REST API** — the **same REST surface** SS uses. So (unlike M4's GraphQL wrinkle) the backend contract is well understood and already REST. The effort is UI re-implementation + role mapping, not backend discovery.

## Feature inventory to reach parity (from PCC `Tools > CLA`)
1. **CLA group management** — create/edit CLA groups; enroll/unenroll projects; ICLA/CCLA enable flags; setup-completion state.
2. **GitHub integration** — install the EasyCLA **GitHub App** to an org; link/unlink repositories; configure auto-enable & branch protection.
3. **GitLab integration** — connect groups/projects; enroll/unenroll repos; auto-enable config.
4. **Gerrit integration** — add/list/remove Gerrit instances for a CLA group.
5. **CLA PDF template management** — choose/edit ICLA & CCLA templates (HTML bodies + field mappings); preview generated PDF.
6. **Signatures view** — ICLA & CCLA signature lists; invalidate/approve a signature.
7. **Approval criteria / CLA managers** — domain/email/GitHub-org allowlist criteria; CLA-manager visibility.
8. **Events / audit** — event stream + CSV export.

## User Scenarios & Testing

### User Story 1 — Enable EasyCLA for a project (Priority: P1)

A project admin, in the Project lens, creates a CLA group, enables ICLA/CCLA, installs the GitHub App on the org, and enrolls repositories — after which PRs on those repos are gated.

**Acceptance Scenarios**:
1. **Given** a project admin, **When** they create a CLA group and enroll a repo, **Then** the repo becomes CLA-gated (PR checks appear).
2. **Given** an admin installing the GitHub App, **When** installation completes, **Then** the org is connected and its repos are available to enroll.

### User Story 2 — Manage CLA PDF templates (Priority: P1)

An admin selects/edits ICLA and CCLA templates and previews the generated PDF before applying.

**Acceptance Scenarios**:
1. **Given** an admin editing a template's fields, **When** they preview, **Then** a watermarked preview PDF reflects their changes.
2. **Given** an applied template, **When** a contributor later signs, **Then** the signed document uses that template.

### User Story 3 — GitLab & Gerrit connections (Priority: P2)
Connect/configure GitLab groups/projects and Gerrit instances at parity (R7).

### User Story 4 — Signatures, approval criteria, events (Priority: P2)
View ICLA/CCLA signatures, invalidate/approve, manage approval criteria, view/export events at parity.

### Edge Cases
- GitHub App install callback / permissions revoked mid-flow → recover gracefully.
- Removing a CLA group with active signatures → confirm semantics match today (no silent data loss).
- Template edit that would invalidate existing in-progress envelopes → warn.
- Repo enrolled but org App uninstalled → surface the inconsistent state.

## Requirements

### Functional
- **FR-5.1**: SS MUST reach parity with PCC's `Tools > CLA` for items 1–8, in the Project lens.
- **FR-5.2**: SS MUST integrate via the Go `/cla-service/v4/*` REST API (same surface PCC uses).
- **FR-5.3**: **Project-admin authority for CLA configuration MUST be enforced by the EasyCLA layer** (project-manager / CLA-manager roles), not inferred from platform Project-lens access alone (R2).
- **FR-5.4**: GitHub App installation & repo enrollment MUST work end-to-end from SS, including the App install callback.
- **FR-5.5**: Template management MUST preserve preview and field-mapping behavior; applied templates MUST flow into subsequent signing.
- **FR-5.6**: Signature invalidate/approve and event CSV export MUST be preserved.

### Non-Functional
- **NFR-5.1**: Repo/org lists (potentially large) MUST paginate/scale.
- **NFR-5.2**: SS Project-lens CLA and PCC's CLA surface MUST be able to run in parallel during burn-in against the same backend (no divergence).

### Key Entities
- **CLA group**: project enrollment, ICLA/CCLA flags, template refs, setup state.
- **SCM connection**: GitHub org/repo, GitLab group/project, Gerrit instance (+ auto-enable/branch-protection config).
- **CLA template**: ICLA/CCLA HTML body + field mappings + generated-PDF refs.

## Retirement gate — EasyCLA in PCC
After parity for items 1–8 and a parallel-run burn-in, **EasyCLA functionality can be removed from PCC** — a separate go/no-go decision. Coordinate with the PCC team on nav/route removal and any shared components.

## Success Criteria
- **SC-5.1**: A project admin can perform every PCC `Tools > CLA` task from the SS Project lens.
- **SC-5.2**: GitHub App install + repo enrollment gates PRs correctly, driven from SS.
- **SC-5.3**: Template edits preview correctly and flow into signed documents.
- **SC-5.4**: CLA configuration authority is enforced by EasyCLA roles, not the platform lens gate (R2 verified).
- **SC-5.5**: With parity reached, EasyCLA can be removed from PCC without loss of function.

## Assumptions
- The `/cla-service/v4/*` REST surface PCC uses is stable and reusable from SS's BFF with minimal/no backend change.
- GitHub App install UX (which involves a GitHub-hosted install step + callback) can be reproduced under SS's domain/routing.
