<!-- SPDX-License-Identifier: CC-BY-4.0 -->
# Milestone 5: EasyCLA Project Configuration in Project Lens

**Status**: Draft
**Depends on**: Milestone 0 (role adapter)
**Retires**: EasyCLA functionality within PCC (`lfx-pcc`) — candidate, after parity and burn-in

## User story

As a project maintainer or CLA Program Manager, I want to configure EasyCLA for my project — create CLA groups, install the GitHub App on repos, manage PDF templates, connect GitLab/Gerrit — from the Project lens in Self Serve, so I don't need to use PCC's separate `Tools > CLA` surface.

## Scope, per confirmed PCC EasyCLA functionality

Confirmed in `lfx-pcc/apps/v1-frontend/apps/v1-frontend/src/app/modules/tools/cla/`, this is a substantial configuration surface:

**In scope (full parity target):**
- CLA group create/edit.
- GitHub App installation and per-repository enrollment (enable/disable, auto-enable, branch protection settings).
- GitLab project/group connection and settings.
- Gerrit repository/group connection.
- PDF template management: template create/edit, form-field and anchor-string mapping for ICLA/CCLA documents.
- Approval-criteria configuration.
- CLA Manager assignment at the project-config level (distinct from the company-side manager assignment in Milestone 4).
- Signature and event views (including CSV export, per current PCC capability).
- Mass signature invalidation (a PCC capability worth calling out explicitly since it's a higher-consequence action).

**Out of scope:**
- Company-side CCLA management (Milestone 4's scope, not this one — though both touch "CLA Manager" as a role, they operate at different scopes: project-level vs. company-level).
- Any change to the underlying GitHub App's webhook/gating mechanism itself (R3) — only the configuration UI moves, not the gating logic.

## Functional requirements

1. The system MUST let an authorized project maintainer or CLA Program Manager create and edit CLA groups.
2. The system MUST support GitHub App installation and per-repository enrollment, matching PCC's current settings (enable/disable, auto-enable, branch protection).
3. The system MUST support GitLab and Gerrit repository/group connection, matching current PCC capability (parity with non-GitHub providers, R7).
4. The system MUST support PDF template management including form-field/anchor-string mapping, matching PCC's template editor capability.
5. The system MUST support approval-criteria configuration at the project-config level.
6. The system MUST support project-level CLA Manager assignment, distinct from and not to be confused with the company-level CCLA Manager role managed in Milestone 4.
7. The system MUST support signature/event viewing and CSV export.
8. The system MUST support mass signature invalidation, with appropriate confirmation/safeguards given its destructive, high-consequence nature.
9. Every mutating action MUST be authorized via the Milestone 0 EasyCLA-role adapter, never inferred from Self Serve's generic Project-lens access (R2, hard requirement) — project-level roles (Project Manager, CLA Program Manager) are a distinct role set from the company-level roles in Milestone 4, and both are distinct from generic Self Serve project membership.
10. Removing this functionality from PCC MUST NOT disrupt the GitHub App's actual webhook/gating behavior (R3) — this milestone moves configuration UI only.

## Risks specific to this milestone

| Risk | Mitigation |
|---|---|
| PCC's CLA module is broad (GitHub App, GitLab, Gerrit, templates, approval criteria, manager assignment, signature views, mass invalidation) — easy to under-scope if treated as "just move the GitHub App settings." | Use the full module inventory above as the parity checklist; do not narrow scope without an explicit, documented decision. |
| "CLA Manager" exists as a role at both the project-config level (this milestone) and the company/CCLA level (Milestone 4) — conflating the two in the Self Serve UI or in the role-adapter design would misrepresent who can do what. | Keep the two role scopes explicitly distinct in both the UI copy and the Milestone 0 adapter's API surface; do not assume "CLA Manager" means the same thing in both contexts. |
| Mass signature invalidation is destructive and project-wide; a UI/permission bug here has outsized consequences compared to most other milestone actions. | Treat this action with extra scrutiny (confirmation dialogs, audit logging, restricted role-check) beyond the baseline requirement. |
| GitHub App installation flows typically involve GitHub's own OAuth/App-installation redirect UX, which may behave differently when initiated from a different origin (Self Serve vs. PCC) — needs explicit verification, not an assumption that "it's just an iframe/redirect so it'll just work." | Verify the GitHub App installation redirect flow specifically from Self Serve's domain/origin during design, not just carry over PCC's implementation assumptions. |

## Effort

**L** — broad feature surface but each piece (GitHub App config, template editor, approval criteria) is a fairly self-contained port of existing PCC functionality against the same underlying `/v4` API; less architecturally novel than Milestone 4's GraphQL-backend unknown, but larger in raw surface area than Milestones 1–3.
