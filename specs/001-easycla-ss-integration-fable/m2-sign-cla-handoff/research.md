# Phase 0 Research: M2 — backend spikes

**Input**: [spec.md](spec.md) Decisions 1–5 | [plan.md](plan.md) Complexity Tracking
**Date**: 2026-08-11, addendum 2026-08-12; **Spike 2 reframed 2026-08-14** for the legal/stakeholder review that removed self-service invalidation.

Four spikes tracing the design questions that could not be answered from the requirements text. Every claim cites the file and line it was read from — `cla-backend-go`, `easycla-contributor-console`, and `lfx-self-serve` at `dev`/`main`; mockup originally read at v16 (`easyclav2-migration-planning@86f5a35`), scope superseded by v17 Final.

Outcome: all five original spec design questions settled. One **measurement** outstanding — CLA-group cardinality for FR-001a (Spike 3). Spike 2's code-level findings (durability of a flag flip, the three call sites) still hold; its recommendation of a self-service invalidation endpoint does not — see the spike for what survives.

---

## Spike 1 — No-PR ICLA signing on the GitHub path

### The blocker, precisely located

**Two independent PR-context dependencies**, one per repo. Both must be addressed; fixing either alone leaves the path broken.

**(a) Console — `findActiveSignature()` hard-stops before any request is made.**
`IndividualDashboardComponent.ngOnInit` branches on the `hasGerrit` storage flag (`individual-dashboard.component.ts:40-46`):

```
if (this.hasGerrit) { this.postIndivdualRequestSignature(); }   // Gerrit: straight through
else               { this.findActiveSignature(); }              // GitHub: gated
```

`findActiveSignature()` calls `getUserActiveSignature(userId)` and, on an empty response, sets `status = 'Failed'` and shows *"Whoops, It looks like you don't have any signatures in progress. Try going back to your pull request and restarting the signing process…"* (`:49-66`). The active-signature record is created by the PR flow, so a proactively-arriving contributor hits exactly this dead end. It also supplies `return_url` from `activeSignatureModel.return_url` (`:73`) — the active signature is both a gate *and* a data source.

**(b) Backend — `getIndividualSignatureCallbackURL` requires PR metadata.**
For `return_url_type=github`, `v2/sign/service.go:1469-1475` unconditionally calls it, and it errors when either key is missing (`:3093-3117`):

```
errors.New("missing pull_request_id in metadata")   // :3101
errors.New("missing repository_id in metadata")     // :3114
```

Those values produce the DocuSign callback via `getInstallationIDFromRepositoryID` → `github.GetReturnURL(...)` (`:2909-2915`).

### The Gerrit precedent, and what it actually proves

Gerrit works because it avoids **both** dependencies rather than satisfying them: `GerritDashboardComponent` sets `HAS_GERRIT = true` (`gerrit-dashboard.component.ts:39`), routing past the Console gate and sourcing `return_url` from the stored `redirect` value (`individual-dashboard.component.ts:74`); and its `return_url_type` matches neither callback branch (`v2/sign/service.go:1469-1481`), so `callBackURL` stays empty and `acl` is never set (`:1487-1491`).

The precedent bounds the work but proves a **weaker** claim than the spec originally implied. It shows "the DocuSign ceremony completes with no PR callback" — not "GitHub ICLA signing without a PR works." The GitHub-specific pieces (PR-comment callback, `acl = github:{githubID}`) are what M2 must decide about, not merely re-route.

### Recommendation

Introduce an explicit no-PR (proactive) mode rather than special-casing on absent metadata:

1. **Backend** (`v2/sign/service.go`): make the github branch tolerate missing PR metadata *by intent* — when the request carries no PR context, skip `getIndividualSignatureCallbackURL` and fall back to the caller-supplied `return_url`, mirroring the Gerrit path's shape. Prefer an explicit request signal (a proactive flag or distinct `return_url_type`) over inferring from an empty metadata map, so a genuine PR-flow bug can't silently degrade into a callback-less signature.
2. **Keep `acl = github:{user.GithubID}`** (`:1433`). This is orthogonal to PR context and is the binding the PR check later matches on. `user.GithubID` must already be on the record before signing, or the proactive signature is bound to an empty ACL — which is why FR-003/FR-004's binding is load-bearing.
3. **Console** (`individual-dashboard.component.ts`): generalize the `hasGerrit` branch into a "no active signature required" condition covering the proactive entry, sourcing `return_url` from the stored redirect. A widening of an existing branch, not a new code path.

**Cost**: one branch condition plus a request-shape addition in the backend; one branch condition plus a `return_url` source change in the Console. **No DocuSign, schema, or webhook changes.** Consequence for [plan.md](plan.md): "narrow to Gerrit-only" now saves genuinely little — keep the GitHub path in M2.

Nothing in the traced code requires a live GitHub OAuth session at sign time; neither `postIndivdualRequestSignature` nor the backend's proactive path needs a GitHub token, only `user.GithubID` already stored on the record.

### Deep-linkability of the decision screen — confirmed, no gap

The fetch is in the embedded child. `cla-dashboard.component.html:3` wires `<app-project-title (successEmitter)="setProject($event)" …>`, and `ProjectTitleComponent.getProject()` fetches the project by ID and emits it into the container (`project-title.component.ts:46-58`). The container's storage read (`cla-dashboard.component.ts:63`) is a fallback the child overwrites. **FR-002's hand-off works as specified.**

Two ordering/data dependencies for the contracts:

- `getProject()` sources `projectId` from **storage**, not the route. The container writes `PROJECT_ID` from the route param in `ngOnInit` (`:65`) and the child reads it in `ngAfterViewInit`, so ordering is correct — but the hand-off depends on writable browser storage. `ClaDashboardComponent` already surfaces a private-window warning (`:69-75`), so this is a pre-existing Console property, not something M2 introduces.
- `getUser()` shows "There is an invalid user ID in the URL" when `USER_ID` is absent (`project-title.component.ts:84-101`). It is written from the route's `:userId`, so FR-002's URL shape satisfies it — but a placeholder or unresolved ID fails here, **reinforcing FR-003**.

> Method note: an earlier version of this document claimed the opposite, from reading `cla-dashboard.component.ts` without its template. Component-level claims about this Console need the `.html` read alongside the `.ts`.

---

## Spike 2 — Durable revocation marker (originally scoped for ECLA self-exclusion, repurposed for system-set Revoked)

> **2026-08-14 update**: this spike was written for a self-service ECLA-invalidation endpoint that no longer exists — self-service invalidation was removed by the legal/stakeholder review. The **durability problem and its three-call-site fix are unchanged and still needed**: the marker they protect is now the system-set **Revoked** state (sanctions/OFAC), written by the sanctions-screening path and by CLA-manager actions in the corporate console, not by any Self-Serve write. The "Recommendation" below describing a new SS-facing invalidation endpoint is **superseded** — see the note after it.

### Why a flag flip is insufficient

`InvalidateProjectRecord` writes **only** `signature_approved = false` and `note` (`signatures/repository.go:2089-2130`) — no timestamp, reason, or actor, which independently confirms FR-010b's rationale.

That flip is then undone by two separate paths:

1. **`auto_create_ecla` re-validation.** The employee-signature loop calls `ValidateProjectRecord` under exactly the condition a flip creates (`signatures/service.go:898`): `if !employeeSignatureModel.SignatureApproved || !employeeSignatureModel.SignatureSigned`, with the note *"signed and approved employee acknowledgement since auto_create_ecla feature flag set to true"*. Any Approved-List update re-approves the self-invalidated signature. **The durability caveat is the live code path, not a precaution.**
2. **PR-time re-authorization.** `ProcessEmployeeSignature` sets `hasSigned = true` whenever `UserIsApproved` succeeds (`:1578-1583`), *independent of the signature's own approved flag* — so PR gating would pass even without (1).

### The decisive design constraint

`UserIsApproved(ctx, user, cclaSignature)` (`signatures/service.go:1607`) receives **only the user and the CCLA**. It has no reference to the employee signature record; it matches the user against the CCLA's GitHub/GitLab username, email, and domain approval lists (`:1623-1660+`).

This rules out the obvious implementation: a marker stored on the employee signature **cannot** be honored inside `UserIsApproved` without changing its signature.

**Option A — exclusion checked at the call sites (recommended).** Keep `UserIsApproved` as the pure Approved-List predicate it is, and have consumers consult the marker *before* treating approval as coverage:

- `signatures/service.go:898` — add the marker to the re-validation guard so a self-excluded record is skipped, not re-approved.
- `ProcessEmployeeSignature` `:1578` — require `!selfExcluded` alongside `userApproved` before `hasSigned = true`.
- `v2/my_clas/service.go:1350` — the my-clas coverage evaluation `evaluateApproval` is the third consumer; the marker feeds FR-010's status directly.

Three call sites, each a one-condition change, `UserIsApproved`'s contract untouched. The marker is FR-010b's reason/actor field — one concept, one field.

**Option B — thread the employee signature into `UserIsApproved`.** Centralizes the check but changes a widely-called interface method (`:70`, and mocked) and conflates "is on the Approved List" with "has been revoked." **Not recommended.**

### Recommendation — superseded 2026-08-14

The rest of this section originally recommended a **new swagger-first self-service ECLA-invalidation endpoint**, written by the contributor from the Me lens. **That endpoint is not built.** Self-service invalidation (ICLA and ECLA) was removed entirely by the 2026-08-14 legal/stakeholder review. What survives from this spike:

- **The three call sites above are still exactly where the durability fix belongs** — they're now guarding the system-set **Revoked** marker (FR-010b) instead of a self-service one.
- **The attribute shape survives**: a revocation timestamp + reason/actor (now values like `sanctions_screening` / `cla_manager`, not `self`), atomically alongside `InvalidateProjectRecord`'s existing write. `InvalidateProjectRecord` does **not** set `date_modified`, so the new timestamp is still what makes the date recoverable.
- **What does not survive**: there is no M2-owned write path. The marker is written by the sanctions-screening job and by CLA-manager actions in the corporate console — both outside this milestone's code. M2 only needs to **read** the marker (for FR-010's status field) and honor it at the three call sites so a revoked ECLA doesn't silently get re-approved by `auto_create_ecla`.
- **No ownership check, no notification template, no ICLA-endpoint revision** are needed here — those belonged to the self-service write path. The revised FR-008 notification (Request Removal/approval) is a **read-and-notify** flow, not a write, so it doesn't reuse `authorizeIdentity` for a mutation — only to confirm the caller owns the signature they're requesting removal for.

---

## Spike 3 — CLA-Group search (and why not a cache)

### No existing endpoint fits

- `listClaGroupsUnderFoundation` (`GET /foundation/{projectSFID}/cla-groups`) and `listFoundationClaGroups` (`GET /foundation-mapping`) are both **scope-bound** by foundation/project SFID. The Sign CLA modal searches with no scope.
- `GetCLAGroupByName` (`project/repository/repository.go:408-440`) is **exact-match only**, via the `project-name-lower-search-index` GSI on `project_name_lower`.
- **`GetCLAGroups` (`:525`) accepts `SearchField`/`SearchTerm`/`FullMatch` and ignores them.** It logs them (`:529-533`), then builds `expression.NewBuilder().WithProjection(buildProjection()).Build()` (`:538`) — projection only, no `WithFilter` — so `expr.Filter()` (`:547`) is nil. An unfiltered full-table `Scan` with decorative search parameters. **A latent bug**, and directly relevant below: search performance on this table has never been measured, so any claim that "DynamoDB can't do this" is inherited, not tested.

Scope is also wider than first assumed — the mockup searches **four** sources (`onSearchInput()`, mockup `:540-544`): project name, CLA-group name, org name with provenance (`orgDisplay()`), and repo URL. The `RepositoryNameIndex`/`GitHubGetRepositoryByName` resolver remains right for the pasted-URL case, now one of four inputs.

### On building a cache: measure first

The proposal was a simple cache over DynamoDB to make search instant. Recommendation: **measure first, and if measurement demands a layer, use OpenSearch rather than a bespoke cache.** In order of weight:

1. **Correctness outranks latency here.** A stale entry means a contributor signs against the **wrong CLA Group** — a legal-correctness failure, not a slow page.
2. **Invalidation spans four write paths, once per search source** — CLA-group create/rename, project enroll/unenroll (`unenrollProjects` exists today), org additions, repo additions.
3. **Lambda undercuts in-process caching.** Cold containers start empty; concurrency yields N copies with N independent staleness windows. A real cache therefore means shared infrastructure (ElastiCache/DAX/OpenSearch): new IaC, cost, failure mode, on-call surface — disproportionate in a milestone whose other backend deltas are single-condition changes.
4. **A cache doesn't provide what "instant search" needs** — no ranking, no fuzzy matching, no prefix-across-fields. It buys invalidation complexity without search capability.
5. **Cheaper levers first**: fix the dead filter, ~200 ms client debounce, server-side result cap, dropdown-only projection.

**Explicitly unverified — the CLA-group row count.** The assumption is hundreds to low thousands (staff-created legal constructs, not user-generated content), where a projected scan is comfortably fast. It **could not be measured**: no `lfproduct-*` profile exists in the local `~/.aws/config` (only `lfx-*`), and those SSO tokens were expired with no interactive browser flow available. Recorded in [spec.md](spec.md) as an assumption to verify, not a finding. If per-keystroke **repo-URL** search is required at full fidelity, cardinality is genuinely much larger; the fallback is exact/suffix GSI matching before any indexing layer.

---

## Spike 4 — Identity binding: build nothing

### GitHub identity types are compatible

The open caveat was whether SS's GitHub identity and EasyCLA's `user.GithubID` are the same type. They are, and the comparison key is the **numeric GitHub ID**:

- EasyCLA declares `user_github_id` as a Go `string` (`users/models.go:18`) but stores and queries it as a **number**: `GetUserByUserName` strips the `github:` prefix, `strconv.Atoi`s it, and queries the `github-id-index` GSI with an integer (`users/repository.go:719-728`). Its comment states the ACL format outright — *"Username for GitHub comes in as github:123456"*. This is the only consumer of the `github:` ACL prefix in the repo.
- SS produces the same shape via `normalizeGithubId()`, reducing Auth0's `github|13434323` form to the bare numeric ID and keeping `githubUsernames` separate (`server/services/cla.service.ts:193-196`).

**A new rule follows**: SS carries `githubIds` as an **array** — multiple linked identities are possible — while a signature ACL binds exactly one. Per mockup v17 Final, the sign flow shows an account-picker modal whenever one or more GitHub identities are linked (no auto-select, even for a single one), and blocks with an empty state linking to Identities when none are linked (FR-004).

### Why the write path was rejected rather than designed

The open question was the shape of a backend-owned identity-binding operation (create-vs-enrich branches, gateway auth). **Recommendation adopted: build neither branch.** SS runs a read-only pre-flight against the shipped `GET /v4/my-clas/identities`, gates per repo-source type (Gerrit needs no GitHub ID), and when the identity is missing hands off to the Console's existing GitHub OAuth.

1. **`user.GithubID` is merge-gating state.** It becomes the signature ACL (`v2/sign/service.go:1488`) the PR check resolves against. A second writer means two systems can disagree about which contributor owns a GitHub ID, and the casualty is a signature that gates merges.
2. **The hazard is already proven on this branch.** Commit `d0a4f81e0` exists specifically to guard enrichment against overwriting a bound GitHub ID. Adding SS as a caller multiplies the paths that can trip it.
3. **The design already puts linking elsewhere.** The mockup carries a dedicated **Identities** tab and the note *"You'll be asked to authorize the account you want to use (GitHub or GitLab) before EasyCLA opens"* — authorization in the hand-off, not pre-bound in SS.

For the record, the enrichment path that was considered and dropped: `GET /v4/user-from-token`'s fallback resolves by LF username then LF email and returns whatever record matches — *including one already bound to a different GitHub identity* (`cmd/server.go:1004-1042`). Blindly enriching it would overwrite that binding. A correct write path would therefore have needed both bind-to-unbound-record and create-record-bound-to-GitHub-ID, and the v1 `updateUser` API cannot serve as the primitive (its `githubUsername` branch returns 400 for an unseen username and updates without verifying the identity belongs to the caller — `users/handlers.go:84-107`). M2 builds none of this.

**Cost of the chosen path**: a contributor without a bound GitHub ID sees one OAuth screen they would otherwise have avoided — judged the correct trade against a second writer into merge-gating identity state. **M3**: reconciling a GitHub ID bound to a different EasyCLA record (log-only in M2).

---

## Mockup read (v16) — items the artifacts had missed

> **Superseded by v17 Final** (2026-08-14): the mockup was revised after this read to drop self-service invalidation and add signed-under identity, Request Removal/approval via a shared Contact-CLA-Manager modal, Manage-in-CCLA-Console, and the Revoked state. The v16-specific items below (typed-`INVALIDATE` modal, "Invalidated" pill, deep-link-style "Request approval") are **retired**; the Download-PDF and status-vocabulary-needs-three-fields findings still hold, adjusted for the new third state.

- **Download PDF is not a discrepancy.** ICLA rows render a real `<a>`; ECLA rows render a **disabled** `<span class="ccla-note">` with `cursor:not-allowed` and `title="Covered by Corporate CLA (CCLA)"`. The mockup already encodes the no-ECLA-PDF backend reality — now FR-011a, and no PM input is needed. Revoked rows additionally suppress download regardless of type (v17).
- **A row action originally uncovered by any FR: "Request approval."** Present only on the Needs-attention ECLA and removed once a row leaves that state. In v16 this was a deep link; **in v17 Final it opens the shared Contact-CLA-Manager modal in approval mode** instead (FR-011) — an in-app message to the manager(s), not a Console redirect.
- **Status vocabulary, which forces FR-010a — updated for v17.** v16 had `Valid` / `Needs attention` / `Invalidated` (self-invalidation copy). **v17 Final replaces the third pill with `Revoked`** — system-set, sanctions/OFAC, read-only, `Revoked · <date>`, no user actions. The middle state remains unrepresentable today: `row.Valid = sig.SignatureApproved && covered` (`v2/my_clas/service.go:288`) collapses approved-ness and coverage. Hence three independent fields still — the largest single M2 change to the read path — but the third field is now revocation provenance (Spike 2, superseded section) rather than self-/manager-invalidation attribution.
- v16's typed-`INVALIDATE` confirmation modal is **retired** — no self-service invalidation exists in v17 Final.
- Also noted (still true in v17): the ECLA type chip carries the company inline (`ECLA · IBM`).

---

## Net effect on the plan

- **Status (risk 1)** is unchanged in size, re-termed: the response-shape work is the same lift, now backing Revoked instead of self-/manager-invalidation attribution.
- **CLA-manager resolution + notification endpoint (risk 2)** replaces the retired ECLA-invalidation endpoint and is smaller: no durable-exclusion write, no signature mutation, no notification-template pair beyond the manager email. It is **not** a from-scratch build — it copies the existing `addCclaAllowlistRequest` pattern (resolve managers from the CCLA signature ACL → persist a request row → templated email via `utils.SendEmail()` → audit event; see spec.md Verified facts). M2 descopes that precedent's approve/reject half: the request record is a receipt, not a workflow.
- **Search (risk 3)** is unchanged: no reusable endpoint, a dead filter to fix, and a blocking cardinality measurement.
- **Proactive no-PR ICLA (risk 4)** is unchanged: shrinks to two branch conditions plus a request-shape addition. Keep the GitHub sign entry.
- **Account-binding (risk 6)** is retired by scope decision, not deferred — unchanged by the 2026-08-14 revision.
- **Retired entirely, not shrunk**: the ICLA-invalidation revision and the self-service ECLA-invalidation endpoint. Neither appears in the plan's risk list anymore.
- FR-002's hand-off needs no Console-side project fetch.
