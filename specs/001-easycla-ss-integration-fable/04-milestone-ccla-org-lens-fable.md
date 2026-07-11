# Milestone 4 — CCLA Management in the Organization Lens; retire the Corporate CLA Console

**Status**: Draft | **Depends on**: M1 (plumbing); pairs with M3 (designee hand-off) | **Retires**: Corporate CLA Console **and its GraphQL BFF** | **Effort**: XL
**Spec**: [spec.md](spec.md) | **Overview**: [00-overview-fable.md](00-overview-fable.md)

## Goal

Everything a CLA manager or signatory does in the Corporate CLA Console moves to Self Serve's Organization lens; the console (Angular 13 app + Node/Apollo BFF) is retired.

## Current state (facts)

- **The console is two systems**: an Angular 13/NgRx frontend (~161 components) and a **dedicated GraphQL BFF** (Node/Express + Apollo, ~648 files, 20+ CLA queries / 8 mutations) that bridges to `/cla-service/v4` REST and aggregates (companies, Salesforce signing entities, metrics, Elasticsearch/Snowflake for analytics). Migration must absorb or replace the BFF's logic, not just the screens.
- **Feature inventory** (verified): CCLA signing initiation (`POST /v4/request-corporate-signature`, incl. send-by-email to a signatory), signing-entity management, approval-list CRUD (email, domain, GitHub org, GitHub username, GitLab; `PUT …/approval-list`), auto-create-ECLA toggle (`PUT …/ecla-auto-create`), employee acknowledgements (paginated/searchable), ICLA listings per CLA group, CLA manager add/remove/designee (+ requests), signed-CLA views at foundation and project level, active-CLA list, CCLA PDF viewing, activity logs + CSV export, CLA metrics, CLA-enabled foundation/project browsing.
- **Permissions today**: Auth0 login → permission strings like `signature_approval_list:update:project|organization:{projectId}|{companyId}` resolved from **ACS** (roles `cla-manager`, `cla-signatory`, `cla-manager-designee`, hardcoded in ACS, Salesforce-backed, org- and project|org-scoped).
- **SS Organization lens today**: dark-launched; access model is `b2b_org#writer/auditor` OpenFGA relations via member-service; per-org People/Settings modules exist. **No CLA concepts.**

## The role-mapping decision (core of this milestone)

Two authorization worlds meet here:

| | EasyCLA/ACS | SS Org lens |
|---|---|---|
| Model | role + scope tuples (Salesforce-backed) | OpenFGA relations (`b2b_org#writer`…) |
| Granularity | per company **and per project/CLA group** | per org (today) |
| Enforced by | EasyCLA v4 backend on every write | each V2 service via Heimdall/OpenFGA |

Key mismatch: an org-lens admin (`b2b_org#writer`) is **not** a CLA manager, and a CLA manager for one CLA group is not one for another. CLA authority is finer-grained than the org lens's current model.

**Options:**

- **A. Bridge (recommended)**: SS asks EasyCLA "what CLA authority does this user have for this company?" (existing v4 role/manager endpoints or the user's ACS-derived permissions) and gates UI accordingly; every write still lands on v4, which enforces via ACS. One source of truth; no sync; matches M3's pattern. Cost: the org lens embeds an EasyCLA-specific authorization vocabulary (contained in the SS `cla` server module).
- **B. Model CLA in OpenFGA now** (`cla_group#manager@user:x`, synced from ACS via fga-sync): platform-consistent, but creates a **second, non-enforcing copy** of CLA authority (v4 still checks ACS), with sync lag exactly where the designee flow is already async — a recipe for "SS says I can, EasyCLA says I can't". Only worth doing when enforcement moves (M6).
- **C. Replace ACS roles with org-lens roles** ("org admin = CLA manager"): simplifies UX but **changes the legal/authorization semantics** of who can alter approval lists and sign CCLAs — a product/legal decision, not an engineering one; also breaks project-scoped CLA managers. Not recommended within a parity milestone.

**Recommendation: A for M4, B deferred into M6, C rejected.** This makes "role differences" a contained adapter, not a migration blocker — consistent with the program strategy.

## Scope

### In

1. **Org-lens CLA module** covering the full feature inventory above, organized per company (and signing entity) × project/CLA group.
2. **BFF absorption**: re-home the BFF's aggregation into SS server routes calling v4 (+ v3 org search, metrics endpoints). Decide per query: call-through vs. small new v4 read endpoints where the BFF did heavy client-side joins.
3. **CCLA signing**: signatory flow reusing the M2 pattern (v4 returns DocuSign `sign_url`; send-by-email variant supported). No DocuSign integration in SS.
4. **Role bridging** per Option A, enforced server-side in v4; SS gating is UX only.
5. **Entry-point continuity**: emails and M3's designee hand-off point into the org-lens CLA module.
6. **Decommission package**: console + BFF teardown, redirect stub, docs.

### Out

- Changing role semantics or moving role storage (M6).
- Project-side administration (M5).
- Analytics beyond what the console ships today (Snowflake/ES-backed insights: port only what's actually used — audit usage first; candidates for deliberate drop with PM sign-off).

## Risks

| Risk | Notes |
|------|-------|
| Scope illusion: "migrate a console" hides a BFF | Sized XL for this reason; inventory-driven parity checklist is the contract with PM |
| Org lens is dark-launched | M4 depends on org-lens GA timeline — external dependency to confirm at review |
| Company mapping mismatches (EasyCLA company vs Salesforce org vs signing entities) | The BFF handles these today; capture its edge-case handling before rewriting |
| Feature drift during long build | Freeze Corporate Console changes at M4 start; dual-maintenance window explicit |
| Auto-create-ECLA side effects | Approval-list writes can create signatures; regression-test with production-like data |
| Silent parity gaps (CSV columns, pagination keys, search semantics) | Golden-file comparisons against console outputs |

## Exit criteria

- SC-004: full regression checklist green in org lens; console + BFF decommissioned; support-ticket rate flat over the following quarter.
- Role-bridge behavior documented (who sees what, and why) for support teams.
