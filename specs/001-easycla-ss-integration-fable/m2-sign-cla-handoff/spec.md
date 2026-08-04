# Feature Specification: M2 — Proactive CLA signing entry in Self Serve (hands off to Contributor Console)

**Parent program**: [../spec.md](../spec.md) (EasyCLA → LFX Self Serve Integration) | **Milestone doc**: [../02-milestone-sign-icla-fable.md](../02-milestone-sign-icla-fable.md)
**Created**: 2026-08-04 | **Status**: Planned (see [plan.md](plan.md))

This is the extracted, implementable slice for Milestone 2. Program-wide context, assumptions, and resolved decisions live in the parent spec; this file is what `/speckit.plan`, `/speckit.tasks`, and `/speckit.implement` operate on.

> **Scope (revised 2026-08-04, per Heather/PM)**: M2 is purely additive — it retires nothing and cuts nothing over. Self Serve never runs the DocuSign ceremony; it hands off to the existing Contributor Console for signing. See the User Story below.

> **Link note**: the upward links `../spec.md` and `../02-milestone-sign-icla-fable.md` point at program-level docs still under review in PR #5132 (targeting `dev`); they resolve once #5132 merges. The `m1-my-cla/` sibling folder is already on `dev`.

## Constraints

- **Simple and straightforward** — no new services, no new state, no bespoke contracts where an existing one works. Prefer reusing what the Console and EasyCLA backend already do.
- **Independently deliverable in ~2 weeks** — M2 must ship on its own. Anything that threatens that budget gets cut or deferred, not absorbed.

## User Story (P2)

A contributor who wants to sign a CLA — without first hitting a failing PR check — logs into LFX Self Serve and, under the Me lens, opens "Sign a CLA". They pick which CLA Group they need to sign for and are handed off to the existing Contributor Console's decision screen for that CLA Group, where they choose Individual or Corporate contributor (as PR-referred contributors do today) and complete the signing there. The DocuSign ceremony, the ICLA/ECLA choice, and all signing logic stay in the Console, exactly as today.

**Acceptance Scenarios**:

1. **Given** a logged-in contributor with no failing PR, **When** they open "Sign a CLA" in the Me lens and select a CLA Group, **Then** they land on the Contributor Console's decision screen for that CLA Group (`/#/cla/project/{claGroupID}/user/{userID}`), with the ICLA/ECLA choice and its legal guidance presented by the Console — not re-implemented in SS.
2. **Given** the contributor proceeds as Corporate Contributor from that decision screen, **When** they complete the employee acknowledgement, **Then** the ECLA is recorded exactly as in today's PR-referred flow (this path has no PR-context dependency — verified).
3. **Given** the contributor proceeds as Individual Contributor, **When** they reach the signing step, **Then** the ICLA flow completes and the resulting signature is attached to a user record resolvable from their GitHub commits (see Open questions — this is the milestone's main design question).
4. **Given** the existing "Signed Agreement Missing" PR-check link, **When** any contributor clicks it, **Then** it behaves exactly as before M2 — same Console URL shape, unchanged; the proactive picker is a parallel path to the same destination.

## Functional Requirements

- **FR-001**: Self Serve MUST offer a new, PR-independent "Sign a CLA" entry point in the Me lens for logged-in users.
- **FR-002**: The picker MUST let the user find and select a CLA Group — a simple searchable list (project/CLA Group names). No org/repo selection step: the signing unit is the CLA Group, and the Console hand-off URL carries only `claGroupID` + `userID`. Org/repo names MAY be used as search metadata to help users find the right CLA Group, nothing more.
- **FR-003**: On selection, Self Serve MUST hand off to the Contributor Console's existing decision-screen URL — `{console}/#/cla/project/{claGroupID}/user/{userID}` — the same shape the PR-check link uses, without the optional `?redirect=` param (there is no PR to return to). The ICLA/ECLA choice, its legal guidance text, and the `project_icla_enabled`/`project_ccla_enabled` gating stay in the Console; SS MUST NOT re-implement them.
- **FR-004**: Self Serve MUST resolve the EasyCLA `userID` for the hand-off server-side from the session identity, via the existing `GET /v4/user-from-token` endpoint (which looks up by LF username/email and creates the user record if missing). No client-supplied user IDs.
- **FR-005**: Self Serve MUST NOT call any signing-initiation endpoint (`request-individual-signature`, `request-employee-signature`, `check-prepare-employee-signature`, etc.). The Console makes those calls, exactly as it does today.
- **FR-006**: Self Serve MUST NOT change the GitHub PR status-check remediation link. No SSM cutover or per-environment switch in M2.
- **FR-007**: Self Serve MUST enumerate the CLA Groups available to present in the picker. *([NEEDS CLARIFICATION]: listing endpoint.)*
- **FR-008**: A proactively signed ICLA MUST be attached to a user record resolvable from the contributor's GitHub commits, or the signature will not turn their PRs green. *([NEEDS CLARIFICATION]: GitHub identity binding — the milestone's main design question, see below.)*

## Success Criteria

- **SC-002** (M2): ≥ 95% of contributors who start the proactive picker in Self Serve land on the Contributor Console decision screen for the right CLA Group without support intervention.

## Scope boundaries

**In**: Me-lens "Sign a CLA" page (searchable CLA Group list), SS server route(s) for CLA-Group listing and `userID` resolution, hand-off redirect, feature flag.

**Out**: any DocuSign/webhook/PDF changes; signing UI or ICLA/ECLA choice in SS (Console owns both); org/repo selection step; PR-redirect cutover (`CLAContributorv2Base` SSM flip); CCLA/approval-list management (M4); corporate org-selection UX polish (Heather flagged this for M3).

## Verified Console/backend facts (2026-08-04)

Read from `easycla-contributor-console` and `cla-backend-go`/`cla-backend-legacy`:

- The Console decision screen (`cla/project/:projectId/user/:userId` → `ClaDashboardComponent`) fetches the project itself, gates ICLA/ECLA by the project flags, and treats `?redirect=` as optional (used only by "Exit EasyCLA"). It is deep-linkable today.
- `GET /v4/user-from-token` exists and does lookup-or-create by LF username → LF email (`cla-backend-go/cmd/server.go`, `v2/current_user`). Records it creates carry **no GitHub identity**.
- The PR check resolves commit authors to user records by **GitHub ID → GitHub username → email** (`cla-backend-go/github/github_repository.go`). A signature on an LF-only record does not reliably match.
- The Console's **ECLA path has no PR-context dependency** — works proactively as-is. The **ICLA GitHub path does not**: the Console requires an active-signature record (`/v2/user/{id}/active-signature`, created only by the PR flow) and errors with "go back to your pull request" without it; the backend's `return_url_type=github` request hard-requires that record's `repository_id`/`pull_request_id` to build the DocuSign callback (`v2/sign/service.go`). The Gerrit-type request has no such dependency — precedent that no-PR ICLA signing already works in the backend.

## Open questions (for `/speckit.clarify`)

1. **GitHub identity binding (primary)** — a proactive ICLA must end up on a user record the PR check can match (GitHub ID/username; email is unreliable — commit emails are often GitHub noreply). When coming from a PR this is guaranteed by construction (the user record is created *from* the commit author); the proactive flow is LF-first and must capture the GitHub identity explicitly. Recommended direction: require a GitHub-linked LF account for the ICLA path, reusing M1's existing "Link your GitHub account" flow (Auth0 identity linking — effectively a one-time GitHub sign-in), and ensure the hand-off references a record carrying that GitHub identity (the existing GitHub-anchored record where one exists, else enrich). Do **not** build a bespoke GitHub OAuth step into the signing path. Needs design + likely a Heather touchpoint.
2. **Proactive ICLA active-signature gap** — Console + backend both assume PR-derived context on the GitHub ICLA path (facts above). Recommended direction: a no-PR request shape mirroring the existing Gerrit behavior (small Console + backend delta), with the post-sign return going to SS or a Console success screen. Whether this lands inside M2's two weeks or immediately after is a planning decision.
3. **CLA-Group listing** — which endpoint enumerates CLA Groups (+ project names, and org/repo names as search metadata) for the picker; reuse from M1's lens if possible, else new read endpoint. Estimate risk.

## Design artifacts

[plan.md](plan.md) · `research.md`, `data-model.md`, `contracts/`, `quickstart.md` — to be generated by the Spec Kit planning flow once the open questions above are resolved.
