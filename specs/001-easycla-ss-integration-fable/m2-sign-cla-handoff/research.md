# Phase 0 Research: M2 — backend spikes

**Input**: [spec.md](spec.md) Decisions 1–5 | [plan.md](plan.md) Complexity Tracking
**Date**: 2026-08-11, addendum 2026-08-12

Four spikes tracing the design questions that could not be answered from the requirements text. Every claim cites the file and line it was read from — `cla-backend-go`, `easycla-contributor-console`, and `lfx-self-serve` at `dev`/`main`; mockup at `easyclav2-migration-planning@86f5a35`.

Outcome: all five spec design questions settled. One **measurement** outstanding — CLA-group cardinality for FR-001a (Spike 3).

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
For `return_url_type=github`, `v2/sign/service.go:1414-1419` unconditionally calls it, and it errors when either key is missing (`:2882-2906`):

```
errors.New("missing pull_request_id in metadata")   // :2890
errors.New("missing repository_id in metadata")     // :2903
```

Those values produce the DocuSign callback via `getInstallationIDFromRepositoryID` → `github.GetReturnURL(...)` (`:2909-2915`).

### The Gerrit precedent, and what it actually proves

Gerrit works because it avoids **both** dependencies rather than satisfying them: `GerritDashboardComponent` sets `HAS_GERRIT = true` (`gerrit-dashboard.component.ts:39`), routing past the Console gate and sourcing `return_url` from the stored `redirect` value (`individual-dashboard.component.ts:74`); and its `return_url_type` matches neither callback branch (`v2/sign/service.go:1414-1427`), so `callBackURL` stays empty and `acl` is never set (`:1432-1436`).

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

## Spike 2 — ECLA durable self-exclusion

### Why a flag flip is insufficient

`InvalidateProjectRecord` writes **only** `signature_approved = false` and `note` (`signatures/repository.go:2089-2130`) — no timestamp, reason, or actor, which independently confirms FR-008b's rationale.

That flip is then undone by two separate paths:

1. **`auto_create_ecla` re-validation.** The employee-signature loop calls `ValidateProjectRecord` under exactly the condition a flip creates (`signatures/service.go:898`): `if !employeeSignatureModel.SignatureApproved || !employeeSignatureModel.SignatureSigned`, with the note *"signed and approved employee acknowledgement since auto_create_ecla feature flag set to true"*. Any Approved-List update re-approves the self-invalidated signature. **The durability caveat is the live code path, not a precaution.**
2. **PR-time re-authorization.** `ProcessEmployeeSignature` sets `hasSigned = true` whenever `UserIsApproved` succeeds (`:1578-1583`), *independent of the signature's own approved flag* — so PR gating would pass even without (1).

### The decisive design constraint

`UserIsApproved(ctx, user, cclaSignature)` (`signatures/service.go:1607`) receives **only the user and the CCLA**. It has no reference to the employee signature record; it matches the user against the CCLA's GitHub/GitLab username, email, and domain approval lists (`:1623-1660+`).

This rules out the obvious implementation: a marker stored on the employee signature **cannot** be honored inside `UserIsApproved` without changing its signature.

**Option A — exclusion checked at the call sites (recommended).** Keep `UserIsApproved` as the pure Approved-List predicate it is, and have consumers consult the marker *before* treating approval as coverage:

- `signatures/service.go:898` — add the marker to the re-validation guard so a self-excluded record is skipped, not re-approved.
- `ProcessEmployeeSignature` `:1578` — require `!selfExcluded` alongside `userApproved` before `hasSigned = true`.
- `v2/my_clas/service.go:214` — `eclaCoveredByCurrentApprovalList` is the third consumer; the marker feeds FR-010's status directly.

Three call sites, each a one-condition change, `UserIsApproved`'s contract untouched. The marker is FR-008b's reason/actor field — one concept, one field.

**Option B — thread the employee signature into `UserIsApproved`.** Centralizes the check but changes a widely-called interface method (`:70`, and mocked) and conflates "is on the Approved List" with "has self-excluded." **Not recommended.**

### Recommendation

New swagger-first endpoint for self-service ECLA invalidation:

- **Targeting**: by `signatureID`. `InvalidateProjectRecord` is already signature-ID-keyed (`:2112-2116`), so no repository change is needed for targeting — only for the added attributes.
- **Ownership**: resolve through M1's existing `authorizeIdentity` (`v2/my_clas/service.go:383`), not a new check.
- **Write**: `signature_approved = false`, a contributor-framed `note`, an invalidation timestamp, and the reason/actor marker (`self`) — atomically, one `UpdateItem`. `InvalidateProjectRecord` does **not** set `date_modified`, so the new timestamp is what makes the date recoverable.
- **Honor the marker at the three call sites above.**
- **Notify**: user (self-service template, FR-008a) + the company's CLA managers. No Approved List mutation.
- **Additive schema only** — pre-M2 records carry empty values.

**Reuse for ICLA (FR-007)**: the same attributes and contributor-framed note serve the revised ICLA endpoint, which additionally needs signature-ID targeting to avoid `GetIndividualSignature`'s `sigs[0]` behaviour on multiple matches (`signatures/repository.go:895-898`). The ECLA endpoint sidesteps this by being signature-ID-targeted from the start.

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

**A new rule follows**: SS carries `githubIds` as an **array** — multiple linked identities are possible — while a signature ACL binds exactly one. The sign flow needs a picker when several are linked (FR-004), which is also what the mockup's authorize-the-account note implies.

### Why the write path was rejected rather than designed

The open question was the shape of a backend-owned identity-binding operation (create-vs-enrich branches, gateway auth). **Recommendation adopted: build neither branch.** SS runs a read-only pre-flight against the shipped `GET /v4/my-clas/identities`, gates per repo-source type (Gerrit needs no GitHub ID), and when the identity is missing hands off to the Console's existing GitHub OAuth.

1. **`user.GithubID` is merge-gating state.** It becomes the signature ACL (`v2/sign/service.go:1433`) the PR check resolves against. A second writer means two systems can disagree about which contributor owns a GitHub ID, and the casualty is a signature that gates merges.
2. **The hazard is already proven on this branch.** Commit `d0a4f81e0` exists specifically to guard enrichment against overwriting a bound GitHub ID. Adding SS as a caller multiplies the paths that can trip it.
3. **The design already puts linking elsewhere.** The mockup carries a dedicated **Identities** tab and the note *"You'll be asked to authorize the account you want to use (GitHub or GitLab) before EasyCLA opens"* — authorization in the hand-off, not pre-bound in SS.

For the record, the enrichment path that was considered and dropped: `GET /v4/user-from-token`'s fallback resolves by LF username then LF email and returns whatever record matches — *including one already bound to a different GitHub identity* (`cmd/server.go:1004-1042`). Blindly enriching it would overwrite that binding. A correct write path would therefore have needed both bind-to-unbound-record and create-record-bound-to-GitHub-ID, and the v1 `updateUser` API cannot serve as the primitive (its `githubUsername` branch returns 400 for an unseen username and updates without verifying the identity belongs to the caller — `users/handlers.go:84-107`). M2 builds none of this.

**Cost of the chosen path**: a contributor without a bound GitHub ID sees one OAuth screen they would otherwise have avoided — judged the correct trade against a second writer into merge-gating identity state. **M3**: reconciling a GitHub ID bound to a different EasyCLA record (log-only in M2).

---

## Mockup read (v16) — items the artifacts had missed

- **Download PDF is not a discrepancy.** ICLA rows render a real `<a>` (rows 1/2/5/6); ECLA rows render a **disabled** `<span class="ccla-note">` with `cursor:not-allowed` and `title="Covered by Corporate CLA (CCLA)"` (rows 3/4). The mockup already encodes the no-ECLA-PDF backend reality — now FR-011a, and no PM input is needed.
- **A row action no FR covered: "Request approval."** Present only on the Needs-attention ECLA (`:333`) and **removed** when the row becomes Invalidated (`confirmInvalidate()` removes `.approve-group`, `:674-675`). Now FR-011.
- **Exact status vocabulary, which forces FR-010a.** Three pills — `Valid`; `Needs attention` with *"No longer matches {company}'s approval criteria."*; `Invalidated` with *"You confirmed you no longer work at {company}."* The middle state is unrepresentable today: `row.Valid = sig.SignatureApproved && covered` (`v2/my_clas/service.go:218`) collapses approved-ness and coverage, and telling self- from manager-invalidation needs a third input. Hence three independent fields — the largest single M2 change to the read path, coupled to Spike 2's marker since they are the same field.
- Also noted: the ECLA type chip carries the company inline (`ECLA · IBM`), and the invalidate modal requires typing `INVALIDATE` (`:439`).

---

## Net effect on the plan

- **Risk 1 (ECLA endpoint)** is well-defined but genuinely new work: endpoint + three call-site guards + two notification templates + additive attributes.
- **Risk 2 (status)** grew from a small SS-side delta to a joint-largest backend item, and is coupled to risk 1 through the shared provenance field — design them together.
- **Risk 3 (search)** is new: no reusable endpoint, a dead filter to fix, and a blocking cardinality measurement.
- **Risk 4 (no-PR ICLA)** shrinks to two branch conditions plus a request-shape addition. Keep the GitHub sign entry.
- **Risk 6 (binding)** is retired by scope decision, not deferred.
- FR-002's hand-off needs no Console-side project fetch.
