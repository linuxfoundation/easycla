# Feature Specification: M2 — My CLAs actions: proactive sign entry (hands off to Console), CLA invalidation, status

**Parent program**: [../spec.md](../spec.md) (EasyCLA → LFX Self Serve Integration) | **Milestone doc**: [../02-milestone-sign-icla-fable.md](../02-milestone-sign-icla-fable.md)
**UI mockup (source of truth)**: [EasyCLA_MyCLAs_v8_Full_Prototype_v8.html](https://github.com/linuxfoundation/easyclav2-migration-planning/blob/main/Mockups/M2/EasyCLA_MyCLAs_v8_Full_Prototype_v8.html)
**Created**: 2026-08-04 | **Status**: Planned (see [plan.md](plan.md))

This is the extracted, implementable slice for Milestone 2. Program-wide context, assumptions, and resolved decisions live in the parent spec; this file is what `/speckit.plan`, `/speckit.tasks`, and `/speckit.implement` operate on.

> **Scope (revised 2026-08-04, per Heather/PM; UI per mockup v8)**: M2 extends M1's **My CLAs** page with three additions — a proactive "Sign a CLA" entry that hands off to the existing Contributor Console, per-CLA **invalidation**, and a richer **status** column. Self Serve never runs the DocuSign ceremony; nothing is cut over or retired; the PR-check remediation link is unchanged.
>
> **Link note**: the upward links `../spec.md` and `../02-milestone-sign-icla-fable.md` point at program-level docs still under review in PR #5132 (targeting `dev`); they resolve once #5132 merges. The `m1-my-cla/` sibling folder is already on `dev`.

## Constraints

- **Simple and straightforward** — no new services, no new state, no bespoke contracts where an existing one works. Prefer reusing what the Console and EasyCLA backend already do.
- **Independently deliverable in ~2 weeks** — M2 must ship on its own. Schedule risks are called out below; `/speckit.plan` decides sequencing within the milestone.

## User Story (P2)

A contributor opens **My CLAs** (M1's page) in the Self Serve Me lens. There they can:

1. **Sign a CLA** — click "+ Sign a CLA", search by project name, CLA group name, or linked repo source (GitHub or Gerrit; GitLab deferred — see Scope boundaries), pick a CLA Group, and continue to the existing Contributor Console decision screen to complete the signing (ICLA/ECLA choice and the DocuSign ceremony stay in the Console). Before the Console opens, GitHub signers are asked to authorize the GitHub account they'll contribute with; Gerrit needs no account step (same LF SSO as Self Serve).
2. **Invalidate a CLA** — each signed CLA row offers an Invalidate action with a confirmation modal. For an ICLA: "This will mark your ICLA for {project} as invalid… This action cannot be undone." For an ECLA: "Confirm you no longer work at {company}?" — confirming ends their coverage under that company's CCLA.
3. **See status** — each row shows a status: Valid, or "Needs attention" with a note (e.g. an ECLA that is signed but no longer matches the company's Approved List criteria), with a "Request approval →" link into the Console's existing request-authorization flow where applicable.

**Acceptance Scenarios**:

1. **Given** a logged-in contributor on My CLAs, **When** they open "+ Sign a CLA", search, and select a CLA Group, **Then** they land on the Contributor Console's decision screen for that CLA Group (`/#/cla/project/{claGroupID}/user/{userID}`), with the ICLA/ECLA choice and its legal guidance presented by the Console — not re-implemented in SS.
2. **Given** the contributor selected a GitHub-backed CLA Group, **When** they continue to sign, **Then** they are first asked to authorize (link) the GitHub account they want to use — one-time, skipped if already linked, with an account picker when more than one GitHub account is linked — so the resulting signature is bound to the identity their contributions come from. Gerrit-backed CLA Groups skip this step (same LF SSO as Self Serve). (See Open questions for mechanics.)
3. **Given** a contributor with a signed ICLA, **When** they click Invalidate and confirm, **Then** the ICLA is invalidated via the existing EasyCLA endpoint, the row's status changes to Invalidated, and the action is recorded in the EasyCLA event log.
4. **Given** a contributor with a valid ECLA, **When** they click Invalidate, **Then** the modal asks them to confirm they no longer work at that company, and confirming ends their ECLA coverage.
5. **Given** a support user **impersonating** a contributor in Self Serve, **When** they view My CLAs, **Then** invalidation is not possible — the SS server rejects any invalidation request for an impersonated session regardless of what the UI shows.
6. **Given** a contributor whose ECLA no longer matches the company's Approved List criteria, **When** they view My CLAs, **Then** the row shows "Needs attention" with an explanatory note and a "Request approval →" link into the Console's request-authorization flow.
7. **Given** the existing "Signed Agreement Missing" PR-check link, **When** any contributor clicks it, **Then** it behaves exactly as before M2 — unchanged; the proactive entry is a parallel path to the same Console.

## Functional Requirements

**Sign a CLA (entry + hand-off)**

- **FR-001**: My CLAs MUST offer a "+ Sign a CLA" action expanding an inline search: one search box matching project names, CLA group names, and linked repo/org sources (GitHub, Gerrit) as search metadata. GitLab-backed CLA groups are excluded until SS ships GitLab account linking (see Scope boundaries / open question 1). No org/repo selection step — the signing unit is the CLA Group. *([NEEDS CLARIFICATION]: listing endpoint.)*
- **FR-002**: On selection, Self Serve MUST hand off to the Contributor Console's existing decision-screen URL — `{console}/#/cla/project/{claGroupID}/user/{userID}` — without the optional `?redirect=` param. The ICLA/ECLA choice, legal guidance, and `project_icla_enabled`/`project_ccla_enabled` gating stay in the Console; SS MUST NOT re-implement them.
- **FR-003**: Self Serve MUST resolve the EasyCLA `userID` server-side from the session identity via the existing `GET /v4/user-from-token` endpoint (lookup-or-create). No client-supplied user IDs.
- **FR-004**: Before the hand-off, GitHub signers MUST be taken through an account-authorization step (per the mockup's note) so a proactively signed CLA lands on a user record resolvable from their commits — one-time linking via M1's flow, account picker when multiple GitHub accounts are linked. Gerrit needs no account step (same LF SSO). The step MUST be platform-parametrized so GitLab can be added by config once SS ships GitLab linking. *(Per-platform matrix in open question 1.)*
- **FR-005**: Self Serve MUST NOT call any signing-initiation endpoint (`request-individual-signature`, `request-employee-signature`, etc.). The Console makes those calls, exactly as today.
- **FR-006**: Self Serve MUST NOT change the GitHub PR status-check remediation link. No SSM cutover or per-environment switch in M2.

**Invalidation**

- **FR-007**: Each signed ICLA row MUST offer Invalidate with a confirmation modal (copy per mockup; irreversible). Confirming calls the existing `PUT /v4/cla-group/{claGroupID}/user/{userID}/icla`. Because that endpoint performs **no ownership check** (verified), the SS server MUST enforce that users can only invalidate their own ICLAs — same enforcement-point pattern as M1.
- **FR-008**: Each valid ECLA row MUST offer Invalidate framed as "Confirm you no longer work at {company}?". *([NEEDS CLARIFICATION]: no self-service ECLA-invalidation endpoint exists — new `cla-backend-go` work; see Open questions.)*
- **FR-009**: Invalidation MUST be impossible during Self Serve **impersonation**: the invalidation route(s) MUST be blocked server-side for impersonated sessions using SS's existing impersonation-readonly middleware (`apps/lfx-one/src/server/middleware/impersonation-readonly.middleware.ts`) — not merely hidden in the UI.

**Status**

- **FR-010**: Each row MUST show a status: Valid; "Needs attention" with an explanatory note (e.g. ECLA signed but no longer matching the company's Approved List criteria); Invalidated after invalidation. *([NEEDS CLARIFICATION]: where the Approved List evaluation comes from.)*
- **FR-011**: "Needs attention" ECLA rows MUST link "Request approval →" into the Contributor Console's existing request-authorization flow (deep link; no new SS flow).

## Success Criteria

- **SC-002** (M2): ≥ 95% of contributors who start the proactive sign entry land on the Contributor Console decision screen for the right CLA Group without support intervention.
- Invalidation: 100% of invalidation attempts during impersonation are rejected server-side; invalidations are attributable in the EasyCLA event log.

## Scope boundaries

**In**: My CLAs page extensions per mockup v8 — "+ Sign a CLA" search + hand-off (incl. pre-hand-off account authorization), ICLA/ECLA invalidation with confirmation modals, impersonation write-block, status column + notes, "Request approval →" deep link, feature flag.

**Out**: any DocuSign/webhook/PDF changes; signing UI or ICLA/ECLA choice in SS (Console owns both); org/repo selection step; PR-redirect cutover (`CLAContributorv2Base` SSM flip); Approved List management (M4); corporate org-selection UX polish (Heather flagged this for M3); sign entry for GitLab-backed CLA groups (~2 projects — conditional on SS shipping GitLab account linking within M2's window; enabled by config if it lands, follow-up otherwise; M2 does not block on it — see open question 1).

## Verified Console/backend facts (2026-08-04)

Read from `easycla-contributor-console`, `cla-backend-go`/`cla-backend-legacy`, and `lfx-self-serve`:

- The Console decision screen (`cla/project/:projectId/user/:userId` → `ClaDashboardComponent`) fetches the project itself, gates ICLA/ECLA by the project flags, and treats `?redirect=` as optional. It is deep-linkable today.
- `GET /v4/user-from-token` exists and does lookup-or-create by LF username → LF email (`cla-backend-go/cmd/server.go`, `v2/current_user`). Records it creates carry **no GitHub identity**.
- The PR check resolves commit authors by **GitHub ID → GitHub username → email** (`cla-backend-go/github/github_repository.go`) — hence FR-004's account-authorization step.
- The Console's **ECLA path has no PR-context dependency** — works proactively as-is. The **ICLA GitHub path does not**: the Console requires an active-signature record (created only by the PR flow) and the backend's `return_url_type=github` request hard-requires its `repository_id`/`pull_request_id` for the DocuSign callback (`v2/sign/service.go`). The Gerrit-type request has no such dependency — precedent that no-PR ICLA signing already works.
- **ICLA invalidation endpoint exists**: `PUT /v4/cla-group/{claGroupID}/user/{userID}/icla` (`invalidateICLA`), logs an `InvalidatedSignature` event — but the handler performs **no ownership check** (`v2/signatures/handlers.go`); SS is the enforcement point.
- **No self-service ECLA-invalidation endpoint exists** (invalidation logic exists internally in the Approved List flow only).
- SS already blocks writes during impersonation via `impersonation-readonly.middleware.ts` (returns `AuthorizationError`); FR-009 reuses it.

## Open questions (for `/speckit.clarify`)

1. **Account-authorization mechanics (direction adopted)** — the mockup resolves the *what*: users authorize their GitHub/GitLab/Gerrit account before the Console opens. To clarify: the *how* per platform (Auth0 identity linking as in M1 vs. an OAuth step in the flow), and how the authorized identity gets bound to the EasyCLA user record used in the hand-off (existing GitHub-anchored record vs. enriching the LF-created one). Sub-decisions already reasoned through (2026-08-07):
   - **Per-platform matrix**: *Gerrit* — no account step ever: Gerrit authenticates via the same LF SSO as Self Serve, so the LF identity from `user-from-token` already is the signing identity. *GitHub* — one-time Auth0 account link via M1's existing flow; resolved by GitHub ID. *GitLab* — **conditional** (decided 2026-08-07): the sign entry requires SS's GitLab account linking, which SS may ship before M2 completes. If it lands in time, GitLab enables in M2 by config (same pattern as GitHub; EasyCLA user records already carry `user_gitlab_id`/`user_gitlab_username`); if not, GitLab CLA groups (~2 projects) stay excluded from the "+ Sign a CLA" entry and enable as a follow-up. Either way the account step MUST be platform-parametrized — GitLab is a config flip, never a redesign. M2 does not block on it.
   - **Multiple linked GitHub accounts**: EasyCLA user records carry a *single* GitHub identity, so two linked GH accounts typically mean two EasyCLA records — an ICLA signed on one does **not** cover commits from the other by GitHub-ID match (only the fragile email fallback). When >1 GitHub account is linked, the sign flow MUST ask "which account will you contribute with?" and resolve the EasyCLA record by that GitHub ID; no silent auto-pick. Single-account users never see the picker.
   - **No live platform session required at sign time**: a GitHub login is needed exactly once, to complete the Auth0 account link. The hand-off and signing work off the server-resolved `userID` — the Console decision screen is deep-linkable and the DocuSign ceremony needs nothing from GitHub. This is a constraint on open question 4: the no-PR ICLA request shape must accept the routed user without demanding a fresh GitHub OAuth, or it reintroduces the session dependency the proactive path removes.
2. **ECLA invalidation endpoint** — new `cla-backend-go` API (swagger-first) for an employee ending their own CCLA coverage; define semantics (signature invalidation + Approved List effects + events). Main backend schedule risk.
3. **Status evaluation** — source for "no longer matches Approved List criteria": extend `GET /v4/my-clas` vs. a separate check; must not require SS to re-implement Approved List logic. Second schedule risk.
4. **Proactive ICLA active-signature gap** — Console + backend assume PR-derived context on the GitHub ICLA path (facts above). Recommended: a no-PR request shape mirroring the existing Gerrit behavior (small Console + backend delta). Constraint from open question 1: the new shape must not require a live GitHub OAuth session — identity is already bound server-side by SS.
5. **CLA-Group listing** — which endpoint enumerates CLA Groups (+ project names and org/repo sources as search metadata); reuse from M1's lens if possible, else new read endpoint.

## Design artifacts

[plan.md](plan.md) · `research.md`, `data-model.md`, `contracts/`, `quickstart.md` — to be generated by the Spec Kit planning flow once the open questions above are resolved.
