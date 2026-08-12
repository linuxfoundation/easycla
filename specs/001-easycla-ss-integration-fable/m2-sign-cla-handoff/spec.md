# Feature Specification: M2 — My CLAs actions: proactive sign entry (hands off to Console), invalidation, status

**Parent program**: [../spec.md](../spec.md) (EasyCLA → LFX Self Serve Integration) | **Milestone doc**: [../02-milestone-sign-icla-fable.md](../02-milestone-sign-icla-fable.md)
**UI mockup (source of truth)**: [EasyCLA_MyCLAs_Full_Prototype_Final.html](https://github.com/linuxfoundation/easyclav2-migration-planning/blob/main/Mockups/M2/EasyCLA_MyCLAs_Full_Prototype_Final.html) (v16)
**Created**: 2026-08-04 | **Status**: Planned (see [plan.md](plan.md))

This is the extracted, implementable slice for Milestone 2. Program-wide context lives in the parent spec; this file is what `/speckit.plan`, `/speckit.tasks`, and `/speckit.implement` operate on.

M2 extends M1's **My CLAs** page (the Profile CLAs tab at `/profile/clas`) with three additions: a proactive "Sign a CLA" entry that hands off to the existing Contributor Console, per-CLA **invalidation**, and a richer **status** column. Self Serve never runs the DocuSign ceremony; nothing is cut over or retired; the PR-check remediation link is unchanged.

> **Authority notes**
> - This slice is the authoritative M2 contract. The program-level brief (`../02-milestone-sign-icla-fable.md`, in [#5132](https://github.com/linuxfoundation/easycla/pull/5132)) frames M2 as signing with ICLA/ECLA choice *in* Self Serve and omits invalidation/status; this spec supersedes it (PM revision 2026-08-07). That brief must be updated, or its M2 material marked superseded, before Spec Kit consumes both.
> - The upward links `../spec.md` and `../02-milestone-sign-icla-fable.md` resolve once #5132 merges. The `m1-my-cla/` sibling is already on `dev`.

## Constraints

- **Simple and straightforward** — no new services, no new state, no bespoke contracts where an existing one works.
- **Independently deliverable in ~3 weeks** — schedule risks are in [plan.md](plan.md).

## User Story (P2)

A contributor opens **My CLAs** in the Self Serve Me lens. There they can:

1. **Sign a CLA** — open a modal, search for a CLA Group, and continue to the Console's decision screen to complete signing. GitHub signers are asked to authorize the GitHub account they'll contribute with; Gerrit needs no account step.
2. **Invalidate a CLA** — each signed row offers Invalidate in its "⋮" menu, with a typed-`INVALIDATE` confirmation modal. For an ICLA this marks the agreement invalid; for an ECLA it confirms the contributor is no longer covered by that company's CCLA.
3. **See status** — each row shows Valid, Needs attention (with a note), or Invalidated, plus a "Request approval →" link where applicable.

**Acceptance Scenarios**:

1. **Given** a contributor on My CLAs, **When** they search the "Sign CLA" modal and select a CLA Group, **Then** they land on the Console decision screen for that group (`/#/cla/project/{claGroupID}/user/{userID}`), with the ICLA/ECLA choice presented by the Console.
2. **Given** a GitHub-backed CLA Group, **When** they continue, **Then** SS's read-only pre-flight checks for a bound GitHub identity; if absent the Console's GitHub OAuth binds it, and if several are linked the contributor picks one. SS writes no identity. Gerrit-backed groups skip the check.
3. **Given** an SS session whose GitHub ID is bound to a *different* EasyCLA user record, **When** they proceed, **Then** M2 logs the mismatch and continues (reconciliation deferred to M3).
4. **Given** a signed ICLA, **When** they invalidate and confirm, **Then** that specific signature is invalidated, the row shows Invalidated, and the event log records a self-invalidation reason.
5. **Given** a valid ECLA, **When** they invalidate, **Then** the modal explains coverage under that company's CCLA ends, and confirming ends it.
6. **Given** a support user **impersonating** a contributor, **When** they view My CLAs, **Then** the SS server rejects any invalidation request regardless of what the UI shows.
7. **Given** an ECLA that no longer matches the company's Approved List criteria, **Then** the row shows Needs attention with a note and a "Request approval →" link into the Console.
8. **Given** the existing "Signed Agreement Missing" PR-check link, **When** clicked, **Then** it behaves exactly as before M2.

## Functional Requirements

### Sign a CLA (entry + hand-off)

- **FR-001**: My CLAs MUST offer a "Sign CLA" action opening a modal with one search box over **four sources**: project name, CLA-group name, org name with repo-source provenance (GitHub/GitLab/Gerrit, displayed), and repo URL (a pasted link resolved to its CLA Group). Results MUST come from a **new unscoped search endpoint** — no existing endpoint covers these four sources. Rows with several linked orgs MUST collapse them into an expandable "N linked orgs" affordance. There is no org/repo selection step: the signing unit is the CLA Group.
- **FR-001a**: The search endpoint MUST meet a per-keystroke budget of **< 300 ms p95 server time** (with ~200 ms client debounce and a server-side result cap) **without introducing a cache or new datastore in M2**. Implementation order: (a) serve from DynamoDB with the search term applied server-side; (b) fix the dead search filter (see Verified facts); (c) measure p95 at real cardinality before adding any layer. If (a)–(c) prove insufficient, the escalation is a **platform search index (OpenSearch)**, not a hand-rolled cache — rationale in [research.md](research.md). Exact-name lookups MAY reuse the `project-name-lower-search-index` GSI, but a GSI MUST NOT be forced to serve substring matching across four fields.
- **FR-002**: On selection, SS MUST hand off to the Console's existing decision-screen URL — `{console}/#/cla/project/{claGroupID}/user/{userID}` — without `?redirect=`. The ICLA/ECLA choice, legal guidance, and `project_icla_enabled`/`project_ccla_enabled` gating stay in the Console.
- **FR-003**: SS MUST resolve the EasyCLA `userID` server-side from the session identity; never from client input. SS MUST NOT write a GitHub identity onto an EasyCLA user record — `user.GithubID` becomes the signature ACL that gates merges, so binding stays owned by the Console's existing GitHub OAuth. For Gerrit, `GET /v4/user-from-token` alone is correct.
- **FR-004**: Before a GitHub hand-off, SS MUST run a **read-only identity pre-flight** against the shipped `GET /v4/my-clas/identities`:
  - **Gate per repo-source type**: a Gerrit-backed group MUST NOT be gated; a GitHub-backed one requires a bound GitHub ID. GitLab follows the same shape by config.
  - **A missing identity MUST NOT block**: hand off to the Console's GitHub-OAuth entry, which binds it as it does today.
  - **Compare as numeric GitHub IDs**, not usernames (both sides already agree on this — see Verified facts).
  - **When several GitHub identities are linked**, the flow MUST ask which account the contributor will contribute with — no silent auto-pick. Single-identity users see no picker.
  - **Mismatches are log-only in M2**: a GitHub ID bound to a different EasyCLA record MUST NOT trigger reconciliation (M3 item).
- **FR-005**: SS MUST NOT call any signing-initiation endpoint (`request-individual-signature`, `request-employee-signature`, etc.). The Console makes those calls.
- **FR-006**: SS MUST NOT change the GitHub PR status-check remediation link. No SSM cutover in M2.

### Invalidation

Primary use cases (PM, 2026-08-07): an *ICLA* signed when an ECLA was appropriate, and an *ECLA* after changing employers. Both end with the user signing a new agreement via the Sign CLA entry, so the post-invalidation state SHOULD point there. Invalidation is strictly **per-row** — no bulk action. ICLA and ECLA are independent agreements: no ordering is enforced and no eligibility warnings are added.

- **FR-007**: Each signed ICLA row MUST offer Invalidate with a typed-`INVALIDATE` confirmation modal (copy per mockup; irreversible). The record is kept for audit. The existing `PUT /v4/cla-group/{claGroupID}/user/{userID}/icla` MUST be revised, not reused unchanged: it MUST gain an ownership check reusing M1's `authorizeIdentity` boundary; it MUST target the specific `signatureID` from the row rather than resolving by `(claGroupID, userID)` and taking `sigs[0]`; and its side effects MUST become actor- and reason-aware (self-service email per FR-008a, event data reflecting contributor self-invalidation). SS MUST also enforce self-only invalidation as defense in depth.
- **FR-008**: Each valid ECLA row MUST offer Invalidate framed per the mockup — "This confirms you're no longer covered under {company}'s Corporate CLA (CCLA) for {project}, and marks your ECLA as invalid". The company's Approved List MUST NOT be mutated; its CLA managers MUST be notified so they can update it. This requires a **new** `cla-backend-go` endpoint. It MUST apply a **persistent self-exclusion** honored by Approved-List re-validation and PR gating — a bare `signature_approved` flip is not durable, because `auto_create_ecla` re-processing re-approves still-listed employees (see Verified facts).
- **FR-008a**: The user MUST receive an email when they invalidate their own ICLA or ECLA. The existing ICLA template is manager-framed, so a self-service-framed template is needed for both flows; the ECLA flow additionally notifies the company's CLA managers.
- **FR-008b**: Both flows MUST record structured invalidation metadata on the signature: an **invalidation timestamp** and an **invalidation reason/actor** (e.g. `self` / `cla_manager` / `approval_list_criteria`), alongside a contributor-framed `signature.note`. Rationale: `date_modified` is not a usable invalidation date — any write bumps it and `InvalidateProjectRecord` does not set it, so the date cannot be reconstructed today. The reason/actor value MUST be the same field FR-008's self-exclusion relies on — one concept, one field, not two overlapping ones. Signatures invalidated before M2 carry no such metadata, so consumers MUST tolerate empty values.
- **FR-009**: Invalidation MUST be blocked server-side for impersonated sessions using SS's existing middleware (`apps/lfx-one/src/server/middleware/impersonation-readonly.middleware.ts`) — not merely hidden in the UI.

### Status

- **FR-010**: Each row MUST show one of three statuses with the mockup's exact labels and notes: **Valid** (approved ∧ covered); **Needs attention** (approved ∧ ¬covered) with *"No longer matches {company}'s approval criteria."*; **Invalidated** (¬approved), and when self-invalidated *"You confirmed you no longer work at {company}."* Source: extend `GET /v4/my-clas`, which already computes the coverage evaluation per ECLA row. SS-side this also requires relaxing M1's valid-only row filter and its boolean-derived status mapping (see Verified facts).
- **FR-010a**: The `GET /v4/my-clas` response MUST carry **three independent fields** rather than one boolean: (a) approved-ness, (b) coverage, (c) invalidation provenance (FR-008b's reason/actor). Today approved-ness and coverage are collapsed into `row.Valid`, which makes **Needs attention** unrepresentable; without (c), self- and manager-invalidated rows cannot be told apart. `Valid` MAY be retained as a derived convenience but MUST NOT be the only signal. This is the largest single M2 change to the read path and a prerequisite for FR-010 and FR-011.
- **FR-011**: "Needs attention" ECLA rows MUST link "Request approval →" into the Console's existing request-authorization flow as a deep link only. Per the mockup this action is **conditional on that state** and MUST be removed when the row becomes Invalidated. No SS-side approval-request logic.
- **FR-011a**: ICLA rows MUST offer **Download PDF** as an active action; ECLA rows MUST render it **disabled** with the tooltip "Covered by Corporate CLA (CCLA)". ECLAs have no signed document of their own.

## Performance assumption (search) — UNVERIFIED

FR-001a's "no cache in M2" position rests on an assumption that has **not been measured**:

- **ASSUMPTION**: CLA-group cardinality is in the **hundreds to low thousands** — CLA groups are staff-created legal constructs, not user-generated content — at which size a projected `Scan` with a server-side filter is comfortably inside the budget.
- **NOT MEASURED**: no row count was obtained. The attempt failed for environmental reasons only (no `lfproduct-*` profile in the local `~/.aws/config`; `lfx-*` SSO tokens expired). **This is a gap in evidence, not evidence of small size.**
- **To verify** (blocking for FR-001a's approach only, not the rest of M2):
  `aws dynamodb describe-table --table-name cla-dev-projects --profile lfproduct-dev --region us-east-1 --query 'Table.ItemCount'`
- **If it fails** — or if per-keystroke **repo-URL** search is needed at full fidelity, where cardinality is genuinely much larger — narrow repo-URL matching to exact/suffix match on the `RepositoryNameIndex` GSI first, and escalate to OpenSearch rather than a cache. Reasoning in [research.md](research.md).

## Success Criteria

- **SC-002**: ≥ 95% of contributors who start the proactive sign entry land on the Console decision screen for the right CLA Group without support intervention.
- 100% of invalidation attempts during impersonation are rejected server-side; invalidations are attributable in the EasyCLA event log.

## Scope boundaries

**In**: My CLAs extensions per mockup v16 — Sign CLA modal search + hand-off (incl. pre-hand-off account authorization), ICLA/ECLA invalidation with typed confirmation, impersonation write-block, status column + notes, "Request approval →" deep link, feature flag.

**Out**: DocuSign/webhook/PDF changes; signing UI or ICLA/ECLA choice in SS (Console owns both); org/repo selection step; PR-redirect cutover (`CLAContributorv2Base` SSM flip); Approved List management (M4); corporate org-selection UX polish (M3); sign entry for GitLab-backed CLA groups (~2 projects — conditional on SS shipping GitLab account linking within M2's window; enabled by config if it lands, follow-up otherwise, and M2 does not block on it).

## Verified Console/backend facts

Read from `easycla-contributor-console`, `cla-backend-go`, and `lfx-self-serve` (2026-08-04 through 2026-08-12).

**Hand-off target**

- The Console decision screen (`cla/project/:projectId/user/:userId`) is **deep-linkable today**. The project fetch lives in the embedded `<app-project-title>` child, not the container: `ProjectTitleComponent.getProject()` emits into `ClaDashboardComponent.setProject()` (`project-title.component.ts:46-58`, wired at `cla-dashboard.component.html:3`); the container's storage read (`cla-dashboard.component.ts:63`) is a fallback the child overwrites. Two dependencies the hand-off must respect: `getProject()` sources `projectId` from **storage** (written from the route param in `ngOnInit` before the child's `ngAfterViewInit`, so ordering is correct but writable browser storage is required — the component already warns that private windows break features); and `getUser()` errors with "There is an invalid user ID in the URL" when `USER_ID` is absent (`:84-101`), so SS MUST supply a real resolved `userID` — reinforcing FR-003.
- `GET /v4/user-from-token` does lookup-or-create by LF username → LF email (`cla-backend-go/cmd/server.go`, `v2/current_user`). Records it creates carry **no GitHub identity**.
- The Console's **ECLA path has no PR-context dependency**. The **ICLA GitHub path does**: the Console requires an active-signature record (created only by the PR flow) and the backend hard-requires `repository_id`/`pull_request_id` for the DocuSign callback. The Gerrit-type request has no such dependency, which bounds the delta — but the Gerrit precedent proves only that the ceremony completes without a PR callback; it does not cover the GitHub-specific `acl = github:{user.GithubID}` binding (`v2/sign/service.go:1433`), which the proactive path MUST still set. See [research.md](research.md) Spike 1.

**Identity**

- The PR check resolves commit authors by **GitHub ID → GitHub username → email** (`cla-backend-go/github/github_repository.go`) — hence FR-004's account step.
- The two sides compare as **numeric GitHub IDs**. EasyCLA declares `user_github_id` as a Go `string` (`users/models.go:18`) but stores and queries it as a number: `GetUserByUserName` strips the `github:` prefix, `strconv.Atoi`s it, and queries `github-id-index` with an integer (`users/repository.go:719-728`; its comment notes "Username for GitHub comes in as github:123456"). SS produces the same shape via `normalizeGithubId()`, which reduces Auth0's `github|13434323` form to the bare numeric ID (`server/services/cla.service.ts:193-196`).
- **The ownership boundary already exists and MUST be reused.** `authorizeIdentity` (`v2/my_clas/service.go:383`) validates every requested identity key against the caller's own EasyCLA records *and* their platform user-service identities, reporting unverifiable keys in `skippedIdentities`. M1's SS controller records the division: the upstream endpoint, not the SS controller, is the authorization boundary. FR-007/FR-008's ownership checks are a reuse of this helper.
- **The multi-account picker needs no new read endpoint.** `GET /v4/my-clas/identities` (`getMyIdentities`, `v2/my_clas/service.go:301`; handler `v2/my_clas/handlers.go:97`; swagger `cla.v2.yaml:2837`) already returns the deduplicated identity set.

**Invalidation**

- **The ICLA endpoint exists but needs revision**: `PUT /v4/cla-group/{claGroupID}/user/{userID}/icla` (`invalidateICLA`) performs **no ownership check** (`v2/signatures/handlers.go`); resolves by `(claGroupID, userID)` and returns `sigs[0]` with only a warning when several match (`signatures/repository.go` `GetIndividualSignature`); and its side effects are admin-framed — manager-worded email (`InvalidateICLASignatureTemplate`) and event data recording a project-deletion reason (`SignatureProjectInvalidatedEventData`).
- **No self-service ECLA-invalidation endpoint exists** — the logic exists only inside the Approved List flow.
- **`InvalidateProjectRecord` writes only** `signature_approved = false` and `note` (`signatures/repository.go:2089-2130`) — no timestamp, actor, or reason, and it does not set `date_modified`. It is already signature-ID-keyed, so FR-008b's attributes are additive to the same atomic write.
- **The `auto_create_ecla` re-approval trigger is exactly a bare flag flip.** The re-validation loop calls `ValidateProjectRecord` when `!SignatureApproved || !SignatureSigned` (`signatures/service.go:898`), and `ProcessEmployeeSignature` sets `hasSigned = true` whenever `UserIsApproved` succeeds, independent of the signature's own flag (`:1578-1583`). FR-008's durability caveat is the live code path.

**Status and search**

- `GET /v4/my-clas` already computes `eclaCoveredByCurrentApprovalList` per ECLA row, then collapses it: `row.Valid = sig.SignatureApproved && covered` (`v2/my_clas/service.go:218`) — the reason FR-010a needs three fields.
- **FR-010 requires relaxing an M1 filter, not just adding a column.** SS's `getMyClas` drops every non-valid row (`filter((cla) => cla.valid === true)`) and `toMyClaAgreement` derives `status` from the collapsed boolean, both in `server/services/cla.service.ts`.
- **No existing endpoint can serve the FR-001 search.** Both CLA-group listings are scope-bound by foundation/project SFID (`listClaGroupsUnderFoundation`, `listFoundationClaGroups`); `GetCLAGroupByName` (`project/repository/repository.go:408-440`) is exact-match-only via the `project-name-lower-search-index` GSI.
- **`GetCLAGroups`'s search parameters are dead code.** It accepts `SearchField`/`SearchTerm`/`FullMatch` and logs them (`project/repository/repository.go:529-533`), then builds `expression.NewBuilder().WithProjection(buildProjection()).Build()` (`:538`) with no `WithFilter`, so `expr.Filter()` (`:547`) is nil. It is an unfiltered full-table `Scan` with decorative search parameters — a latent bug, and the reason search performance on this table has never been measured. Fixing it is part of FR-001a.
- The `RepositoryNameIndex` GSI (`GitHubGetRepositoryByName` → `cla_group_id`) remains the right mechanism for the pasted-URL case.
- **No Spec Kit scaffolding exists in either repo** — there is no `.specify/` directory in `easycla` or `lfx-self-serve`. These artifacts follow Spec Kit *shape* by hand, as M1's did. Initializing `.specify/` is a separate change and does not block M2.

## Decisions

Resolved design questions, with rationale in [research.md](research.md).

1. **Account authorization** — SS gains no identity-binding capability. It runs the FR-004 read-only pre-flight and delegates binding to the Console's existing GitHub OAuth. Rejected alternative: an SS-owned, ownership-checked binding operation covering bind-to-unbound-record and create-record-bound-to-GitHub-ID. Reason: `user.GithubID` is the merge-gating signature ACL, and a second writer risks two systems disagreeing about whose GitHub ID it is. Per-platform shape: *Gerrit* needs no account step (same LF SSO as SS); *GitHub* resolves by GitHub ID via M1's existing link flow; *GitLab* is a config flip, never a redesign. No live platform session is needed at sign time — the hand-off works off the server-resolved `userID`. **M3**: reconciling a GitHub ID bound to a different EasyCLA record.
2. **ECLA invalidation mechanism** — the self-exclusion marker MUST be honored at three **call sites**, not inside `UserIsApproved`, which receives only `(user, cclaSignature)` (`signatures/service.go:1607`) and cannot see the employee signature. The sites: the `auto_create_ecla` re-validation guard (`:898`), `ProcessEmployeeSignature`'s approval branch (`:1578`), and `eclaCoveredByCurrentApprovalList` (`v2/my_clas/service.go:214`, which also feeds FR-010). The endpoint itself remains an execution deliverable.
3. **Status evaluation** — extend `GET /v4/my-clas` (FR-010a) rather than adding a new endpoint; the Approved-List evaluation already runs there. Carried-over caveat: GitLab group membership can't be evaluated without per-group OAuth and defers to the flag — moot while GitLab is deferred.
4. **Proactive ICLA active-signature gap** — widen two branch conditions with an *explicit* proactive request signal (not inferred from absent metadata, so PR-flow bugs can't silently degrade), sourcing `return_url` from the stored redirect. The two dependencies are the Console's `findActiveSignature()` hard-stop (`individual-dashboard.component.ts:49-66`) and the backend's PR-metadata requirement (`v2/sign/service.go:2890,2903`). No DocuSign, schema, or webhook change. The GitHub-ID binding must land *before* signing.
5. **CLA-Group search** — one new unscoped, multi-source endpoint per FR-001/FR-001a, with the dead filter fixed and p95 measured before any caching or indexing layer.

## Design artifacts

[plan.md](plan.md) · [research.md](research.md) (Phase 0)

Phase 1 (`data-model.md`, `contracts/`, `quickstart.md`) is not blocked on clarification. One **measurement** is outstanding: CLA-group cardinality (see Performance assumption). Contracts to write: the four-source search endpoint (FR-001/FR-001a), the extended `GET /v4/my-clas` three-field response (FR-010a), the new ECLA-invalidation endpoint and revised ICLA one (FR-007/FR-008/FR-008b), and the hand-off URL + pre-flight read (FR-002/FR-003/FR-004).
