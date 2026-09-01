# Milestone 4 — EasyCLA Project Administration in the Project Lens; remove from PCC

**Status**: **DECISION-GATED** (2026-07-15 review): whether project-level EasyCLA administration moves to Self Serve or stays in PCC is an open product decision (Kieran/Manish/Heather) — this document describes the move-to-SS option; do not start until the placement decision lands | **Depends on**: M3 patterns (role bridge, reporting components) | **Retires**: PCC EasyCLA module (v1-frontend) | **Effort**: L
**Spec**: [spec.md](spec.md) | **Overview**: [00-overview-fable.md](00-overview-fable.md)

## Goal

Project administrators configure EasyCLA from Self Serve's Project lens: CLA groups, templates, GitHub/GitLab/Gerrit enrollment, signature reporting, events. The EasyCLA module is then removed from PCC.

## Current state (facts)

- EasyCLA admin lives **only in PCC v1-frontend** (`modules/tools/cla/…`); PCC v2 has none — so there is no "PCC v2 first" question; SS is the natural target.
- **Feature inventory** (verified, called via PCC BFF `/cla-services/*` → EasyCLA v4):
  - **CLA groups**: multi-step creation wizard (name/description validation, ICLA/CCLA flags, conditional "CCLA requires ICLA", project-hierarchy enrollment across foundation/child levels), edit, delete, enroll/unenroll projects.
  - **Templates**: pick from template library, live ICLA/CCLA PDF preview (server-generated; MetaFields: project name, entity name, contact email), regenerate, download; custom templates are locked from editing.
  - **GitHub**: EasyCLA GitHub App install flow (env-specific app URLs), org connect/disconnect, per-repo enforcement toggles, enforce-on-all, branch-protection status surfacing, archived-repo handling.
  - **Gerrit**: instance list per CLA group, add/remove, per-instance repo browsing.
  - **GitLab**: group connect by URL (auto-enable flag), repo enable/update, disconnect.
  - **Reporting**: ICLA signatures (search/paginate/CSV, **signature invalidation**), CCLA/approval-criteria views, approved corporate contributors, activity/events log + CSV.
- **Permissions today**: ~24-entry matrix of `resource:action` checks project-scoped through PCC's ACS-derived permissions, plus guards (project must be Active/Formation; EasyCLA maintenance flag).
- **SS Project lens today**: exists with per-project modules (meetings, committees, settings…), gated by `project#writer`-style OpenFGA relations plus per-controller checks.

## Role mapping

Simpler than M3: PCC's CLA permissions are all **project-scoped admin** authority — they map naturally onto the Project lens's existing "project writer/admin" concept. Two credible options:

- **A. Map to project lens authority (recommended)**: SS gates the CLA admin module on the same project-admin relation used by the lens's other admin features; EasyCLA v4 continues its own server-side checks. v4 accepts SS-authenticated calls with api-gw-audience access tokens (verified in the role-mapping feasibility analysis — authorization keys on the username, not the client/audience); confirm the project-scoped ACS policies admit these operations via spikes 1–2 in [docs/easycla-ss-migration/role-mapping-feasibility.md](../../docs/easycla-ss-migration/role-mapping-feasibility.md), or fall back to the M2M + subject pattern.
- **B. Reproduce PCC's fine-grained 24-permission matrix**: fidelity, but the matrix mostly collapses to "project CLA admin: yes/no" in practice — reproduce only if an actual persona split (e.g., view-only staff) is confirmed by PM. Audit real role assignments before choosing.

## Scope

### In

1. Project-lens **CLA administration module** covering the inventory above (wizard, templates, three git platforms, reporting incl. invalidation, events/CSV).
2. **GitHub App install round-trip**: leaving SS to GitHub and returning with install context (mirrors PCC's modal flow; env-specific app slugs).
3. Server routes in SS calling EasyCLA v4 directly (drop PCC's BFF hop).
4. Maintenance-mode + project-status gating parity.
5. **Removal package**: delete PCC v1 EasyCLA module + routes + guards, docs updates.

### Out

- Template *authoring* changes (library and PDF generation stay backend-side, untouched).
- Any change to gating/enforcement behavior (GitHub checks, branch protection logic).
- PCC v2 work of any kind.

## Parity details from the product documentation

- **One CLA group per project**: a project (or its parent) can belong to only one CLA group — the wizard's hierarchy validation encodes this; port the rule, not just the UI.
- **Gerrit is narrower than it looks**: instances are LF-hosted and onboarded via support ticket (not self-service), and enablement is all-or-nothing per instance — the Project-lens Gerrit UI is list/link/unlink only.
- **Ops behaviors to preserve**: automated PM emails on repo rename/archive/delete; auto branch protection covers only the default branch (documented limitation); the GitHub App needs Merge Queue read permission or checks hang in "Expected".
- **Signature invalidation** is a documented PM capability (e.g., contributor left the company) — confirmation UX + audit event required (already in scope; docs confirm the use case).

## Risks

| Risk | Notes |
|------|-------|
| GitHub App install flow has PCC-specific redirect/return assumptions | Verify the App's configured callback targets; may need an EasyCLA config change to accept SS as the return surface |
| CLA group creation wizard encodes project-hierarchy rules (foundation/child validation) | Port the validation semantics, not the code; test against real hierarchies incl. standalone projects |
| Signature invalidation is a destructive admin action | Parity + confirmation UX + audit event verified |
| PCC and SS project identity (SFID vs slug) mismatches | SS project lens keys on project context; ensure clean SFID mapping for v4 calls |
| Long tail of small features (archived repos toggle, CSV formats, Gerrit instance quirks) | Inventory-driven checklist; golden-file CSV comparisons as in M3 |

## Exit criteria

- SC-005: a project can be fully CLA-onboarded (group → template → GitHub org → repos → gating verified on a test PR) purely in SS; PCC EasyCLA module removed behind a feature flag first, then deleted.
- No regression in enrollment-to-enforcement latency.
