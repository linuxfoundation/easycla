# Phase 0 Research: M2 — backend spikes

**Input**: [spec.md](spec.md) open questions 2 and 4 (Spikes 1–2) and 1 and 5 (Addendum) | [plan.md](plan.md) Complexity Tracking risks 1–5
**Date**: 2026-08-11, addendum 2026-08-12 | **Status**: **all five spec open questions resolved**; one measurement outstanding (CLA-group cardinality, FR-001a). Recommendations below feed the contracts.

Spikes 1 and 2 (2026-08-11) trace the two `[NEEDS CLARIFICATION]` items that blocked M2's sign-entry and ECLA-invalidation paths. The [addendum](#addendum-2026-08-12--open-questions-1-and-5-resolved-mockup-read-in-full) (2026-08-12) closes the remaining two open questions and records a full read of the Final/v16 mockup. Neither spike was answerable from the requirements text — both needed the code traced. Every claim cites the file and line it was read from (`cla-backend-go`, `easycla-contributor-console`, and `lfx-self-serve` at `dev`/`main`; mockup at `easyclav2-migration-planning@86f5a35`).

---

## Spike 1 — No-PR ICLA signing on the GitHub path (open question 4, risk 2)

### The blocker, precisely located

There are **two independent PR-context dependencies**, one per repo. Both must be addressed; fixing either alone leaves the path broken.

**(a) Console — `findActiveSignature()` hard-stops before any request is made.**
`IndividualDashboardComponent.ngOnInit` branches on the `hasGerrit` storage flag (`individual-dashboard.component.ts:40-46`):

```
if (this.hasGerrit) { this.postIndivdualRequestSignature(); }   // Gerrit: straight through
else               { this.findActiveSignature(); }              // GitHub: gated
```

`findActiveSignature()` calls `getUserActiveSignature(userId)` and, on an empty response, sets `status = 'Failed'` and shows: *"Whoops, It looks like you don't have any signatures in progress. Try going back to your pull request and restarting the signing process from your pull request if necessary."* (`:49-66`). The active-signature record is created by the PR flow, so a proactively-arriving contributor gets exactly this dead end — the failure mode the spec predicted, confirmed verbatim.

It also supplies `return_url` from `activeSignatureModel.return_url` (`:73`), so the active signature is both a gate *and* a data source.

**(b) Backend — `getIndividualSignatureCallbackURL` requires PR metadata.**
For `return_url_type=github`, `v2/sign/service.go:1414-1419` unconditionally calls `getIndividualSignatureCallbackURL`, which reads `activeSignatureMetadata` and errors out when either key is missing (`:2882-2906`):

```
errors.New("missing pull_request_id in metadata")   // :2890
errors.New("missing repository_id in metadata")     // :2903
```

Those values then produce the DocuSign callback via `getInstallationIDFromRepositoryID` → `github.GetReturnURL(installationID, repositoryID, pullRequestID)` (`:2909-2915`).

### The Gerrit precedent, and what it actually proves

Gerrit's no-PR flow works because it avoids **both** dependencies rather than satisfying them:

- Console: `GerritDashboardComponent` sets `HAS_GERRIT = true` (`gerrit-dashboard.component.ts:39`), which routes past `findActiveSignature()`, and passes `return_url` from the stored `redirect` value instead of an active signature (`individual-dashboard.component.ts:74`).
- Backend: `return_url_type` is neither `github` nor `gitlab`, so *neither* callback-URL branch runs (`v2/sign/service.go:1414-1427`) and `callBackURL` stays empty. The `acl` field is likewise only set for github/gitlab (`:1432-1436`).

So the precedent is real and bounds the work — but note it proves a **weaker** claim than the spec implies. Gerrit does not demonstrate "GitHub ICLA signing without a PR works"; it demonstrates "the DocuSign ceremony completes fine with no PR callback." The GitHub-specific pieces (PR-comment callback, `acl` = `github:{githubID}`) are what M2 must decide about, not merely re-route.

### Recommendation

**Introduce an explicit no-PR (proactive) ICLA mode rather than special-casing on absent metadata.** Concretely:

1. **Backend** (`v2/sign/service.go`): make the github branch tolerate missing PR metadata *by intent, not by accident* — when the request carries no PR context, skip `getIndividualSignatureCallbackURL` and fall back to the caller-supplied `return_url`, mirroring the Gerrit path's shape. Prefer an explicit signal in the request (e.g. a proactive flag or a distinct `return_url_type`) over inferring it from an empty metadata map, so a genuine PR-flow bug can't silently degrade into a callback-less signature.
2. **Keep `acl = github:{user.GithubID}`.** This is orthogonal to PR context and is the identity binding the PR check later matches on — it must not be dropped in the proactive path. This is why FR-003/FR-004's GitHub-ID binding is load-bearing: `user.GithubID` must already be on the record before signing, or the proactive signature is bound to an empty ACL.
3. **Console** (`individual-dashboard.component.ts`): generalize the `hasGerrit` branch into a "no active signature required" condition covering the proactive entry, and source `return_url` from the stored redirect in that mode. The existing structure already supports this — it's a widening of an existing branch, not a new code path.

**Cost**: small and bounded in both repos — one branch condition plus a request-shape addition in the backend, one branch condition plus a `return_url` source change in the Console. **No DocuSign, schema, or webhook changes.**

**Consequence for the trade-off in plan.md**: the "narrow to Gerrit-only" fallback saves genuinely little now that the delta is located. Recommend keeping the GitHub path in M2.

### Constraint carried from open question 1

The no-PR shape must not require a live GitHub OAuth session — identity is bound server-side by SS before hand-off. Nothing in the traced code conflicts with this: neither `postIndivdualRequestSignature` nor the backend's proactive path needs a GitHub token, only `user.GithubID` already stored on the record.

### Deep-linkability of the decision screen — confirmed, no gap

An intermediate version of this document claimed the decision screen does not fetch its project and would render both buttons disabled on a cold deep link. **That was wrong and is retracted** — it came from reading `cla-dashboard.component.ts` without its template.

The fetch is in the embedded child component. `cla-dashboard.component.html:3` wires `<app-project-title (successEmitter)="setProject($event)" (errorEmitter)="hasErrorPresent($event)">`, and `ProjectTitleComponent.getProject()` fetches the project by ID and emits it back into the container (`project-title.component.ts:46-58`). The container's storage read (`cla-dashboard.component.ts:63`) is a fallback the child overwrites. **FR-002's hand-off works as specified.**

Two ordering/data dependencies worth carrying into the contracts:

- `ProjectTitleComponent.getProject()` sources `projectId` from **storage**, not the route. The container writes `PROJECT_ID` from the route param in `ngOnInit` (`:65`) and the child reads it in `ngAfterViewInit`, so the ordering is correct — but the hand-off depends on writable browser storage. `ClaDashboardComponent` already surfaces a private-window warning (`:69-75`), so this is a pre-existing property of the Console, not something M2 introduces.
- `getUser()` emits an error and shows "There is an invalid user ID in the URL" when `USER_ID` is absent (`project-title.component.ts:84-101`). It is written from the route's `:userId`, so FR-002's URL shape satisfies it — but a placeholder or unresolved ID would fail here. This **reinforces FR-003**: SS must resolve a real `userID` server-side before redirecting.

Method note: the original error was a single-file read where the component's template was load-bearing. Component-level claims about this Console need the `.html` read alongside the `.ts`.

---

## Spike 2 — ECLA durable self-exclusion (open question 2, risk 1)

### Why a flag flip is insufficient — confirmed, with the exact trigger

`InvalidateProjectRecord` writes **only** two attributes (`signatures/repository.go:2089-2130`): `signature_approved = false` and `note`. No timestamp, no reason, no actor — which independently confirms FR-008b's rationale (the invalidation date is unreconstructable today).

That flip is then undone by two separate paths:

1. **`auto_create_ecla` re-validation.** The employee-signature loop calls `ValidateProjectRecord` under exactly the condition a flip creates (`signatures/service.go:898`):
   `if !employeeSignatureModel.SignatureApproved || !employeeSignatureModel.SignatureSigned`
   — with the note *"signed and approved employee acknowledgement since auto_create_ecla feature flag set to true"*. So any Approved-List update re-approves the self-invalidated signature. The spec's durability caveat is not a precaution; it is the live code path.
2. **PR-time re-authorization.** `ProcessEmployeeSignature` sets `hasSigned = true` whenever `UserIsApproved` succeeds (`:1578-1583`), *independent of the signature's own approved flag* — so PR gating would pass even without (1).

### The decisive design constraint

`UserIsApproved(ctx, user, cclaSignature)` (`signatures/service.go:1607`) receives **only the user and the CCLA**. It has no reference to the employee signature record. It matches the user against the CCLA's GitHub/GitLab username, email, and domain approval lists (`:1623-1660+`).

This rules out the most obvious implementation: **a marker stored on the employee signature cannot be honored inside `UserIsApproved` without changing its signature.** Two viable options follow.

**Option A — exclusion checked at the call sites (recommended).** Keep `UserIsApproved` as the pure Approved-List predicate it is, and have both consumers consult the employee signature's self-exclusion marker *before* treating approval as coverage:
- `signatures/service.go:898` — add the marker to the re-validation guard so a self-excluded record is skipped rather than re-approved.
- `ProcessEmployeeSignature` `:1578` — require `!selfExcluded` alongside `userApproved` before `hasSigned = true`.
- `v2/my_clas/service.go:214` — `eclaCoveredByCurrentApprovalList` already sits at the third consumer; the marker feeds FR-010's status directly.

Three call sites, each a one-condition change, and `UserIsApproved`'s contract is untouched. The marker is the FR-008b reason/actor field, per spec — one concept, one field.

**Option B — thread the employee signature into `UserIsApproved`.** Centralizes the check but changes a widely-called signature (it's on the service interface, `:70`, and mocked) and conflates "is on the Approved List" with "has self-excluded." Not recommended.

### Recommendation

New swagger-first endpoint in `cla-backend-go` for self-service ECLA invalidation, per FR-008/008a/008b:

- **Targeting**: by `signatureID` (the row's own ID, per FR-007's precedent). `InvalidateProjectRecord` is already signature-ID-keyed (`:2112-2116`), so no repository change is needed for targeting — only for the additional attributes.
- **Ownership**: resolve through M1's existing `authorizeIdentity` (`v2/my_clas/service.go:383`) rather than a new check — see plan.md's Security gate.
- **Write**: extend the invalidation write to set `signature_approved = false`, a contributor-framed `note`, an **invalidation timestamp**, and the **reason/actor** marker (`self`) atomically — one `UpdateItem`, as today. Note `InvalidateProjectRecord` does **not** set `date_modified`; the new timestamp attribute is what makes the date recoverable (FR-008b).
- **Honor the marker at the three call sites above.**
- **Notify**: user (self-service-framed template, FR-008a) + the company's CLA managers (FR-008). No Approved List mutation.
- **Additive schema only** — pre-M2 records carry empty values; consumers must tolerate that (FR-008b's known limitation).

**Reuse for ICLA (FR-007)**: the same attributes and the same contributor-framed note serve the revised ICLA endpoint. The ICLA revision additionally needs signature-ID targeting to avoid `GetIndividualSignature`'s `sigs[0]` behaviour on multiple matches (`signatures/repository.go:895-898`) — the ECLA endpoint sidesteps this by being signature-ID-targeted from the start.

---

## Net effect on the plan

- **Risk 2 shrinks** — the no-PR delta is two branch conditions plus a request-shape addition, in known locations. Keep the GitHub sign entry; don't narrow to Gerrit-only for schedule reasons.
- **Risk 1 is well-defined but genuinely new work** — endpoint + three call-site guards + two notification templates + additive attributes. Still the largest single M2 backend item.
- **No new items.** An intermediate draft added a fifth risk for a supposed Console empty-project gap; it was retracted on further tracing (see Spike 1's deep-linkability section). FR-002's hand-off needs no Console-side project fetch.

### Remaining open

All three items listed here on 2026-08-11 were resolved on 2026-08-12 — see the addendum below.

---

## Addendum (2026-08-12) — open questions 1 and 5 resolved; mockup read in full

Two verification checks plus a full read of the Final/v16 mockup. Sources: `cla-backend-go` and `lfx-self-serve` at their working trees, and `easyclav2-migration-planning@86f5a35`.

### Check A — GitHub identity types are compatible (resolves a caveat on open question 1)

Spike 1 left an unverified caveat: whether SS's GitHub identity and EasyCLA's `user.GithubID` are the same type. They are, and the comparison key is the **numeric GitHub ID**:

- EasyCLA declares `user_github_id` as a Go `string` (`users/models.go:18`) but stores and queries it as a **number**: `GetUserByUserName` strips the `github:` prefix, `strconv.Atoi`s it, and queries the `github-id-index` GSI with an integer (`users/repository.go:719-728`). Its comment states the ACL format outright — *"Username for GitHub comes in as github:123456"*. This is the only consumer of the `github:` ACL prefix in the repo.
- SS already produces the same shape: `githubIds` via `normalizeGithubId()`, which reduces Auth0's `github|13434323` form to the bare numeric ID, keeping `githubUsernames` as a separate list (`server/services/cla.service.ts:193-196`).

**Consequence, and a new rule**: SS carries `githubIds` as an **array** — multiple linked GitHub identities are possible — while a signature ACL binds exactly one. The sign flow needs a picker when several are linked (now spec FR-004), which is also what the mockup's authorize-the-account note implies.

### Open question 1 — resolved by *not* building a binding operation

The open `[NEEDS CLARIFICATION]` was the shape of a backend-owned identity-binding operation (create-vs-enrich branches, gateway auth). **Recommendation adopted: build neither branch.** SS runs a read-only pre-flight against the already-shipped `GET /v4/my-clas/identities`, gates per repo-source type (Gerrit needs no GitHub ID), and when the identity is missing hands off to the Console's existing GitHub OAuth, which binds it as it does today.

Why the write path was rejected rather than designed:

1. **`user.GithubID` is merge-gating state.** It becomes the signature ACL (`v2/sign/service.go:1433`) that the PR check resolves against. A second writer means two systems can disagree about which contributor owns a GitHub ID, and the casualty is a signature that gates merges.
2. **The hazard is already proven on this branch.** Commit `d0a4f81e0` exists specifically to guard enrichment against overwriting a bound GitHub ID. Adding SS as a caller multiplies the paths that can trip that guard.
3. **The design already puts linking elsewhere.** The mockup carries a dedicated **Identities** tab and the note *"You'll be asked to authorize the account you want to use (GitHub or GitLab) before EasyCLA opens"* — authorization in the hand-off, not pre-bound in SS.

**Cost**: a contributor without a bound GitHub ID sees one OAuth screen they would have avoided. Judged the correct trade against a second writer into merge-gating identity state. **Deferred to M3**: reconciling a GitHub ID bound to a *different* EasyCLA record — log-only in M2, since reconciliation has real support implications.

### Open question 5 — resolved: a new endpoint is required, and the current one's search is dead

The question assumed an existing listing endpoint could be reused. None can:

- `listClaGroupsUnderFoundation` (`GET /foundation/{projectSFID}/cla-groups`) and `listFoundationClaGroups` (`GET /foundation-mapping`) are both **scope-bound** by foundation/project SFID; the Sign CLA modal searches with no scope.
- `GetCLAGroupByName` (`project/repository/repository.go:408-440`) is **exact-match only** via the `project-name-lower-search-index` GSI on `project_name_lower`.
- **`GetCLAGroups` (`:525`) accepts `SearchField`/`SearchTerm`/`FullMatch` and ignores them.** It logs them at `:529-533`, then builds `expression.NewBuilder().WithProjection(buildProjection()).Build()` at `:538` — projection only, no `WithFilter` — so `expr.Filter()` at `:547` is nil. The endpoint is an unfiltered full-table `Scan` whose search parameters are decorative. **This is a latent bug**, and it has a direct bearing on the caching question below: search performance on this table has never actually been measured, so any claim that "DynamoDB can't do this" is inherited, not tested.

Scope is also wider than the question assumed — the mockup searches **four** sources (`onSearchInput()`, mockup script `:540-544`): project name, CLA-group name, org name (with GitHub/GitLab/Gerrit provenance rendered by `orgDisplay()`), and repo URL. The earlier `RepositoryNameIndex`/`GitHubGetRepositoryByName` resolver remains right for the pasted-URL case — now one of four inputs.

### On building a cache for the search (recommendation: not in M2)

The proposal was a simple cache over DynamoDB to make search instant. Recommendation is **measure first, and if measurement demands a layer, use OpenSearch rather than a bespoke cache**:

1. **Correctness outranks latency here.** A stale entry means a contributor signs against the **wrong CLA Group** — legal-correctness, not perf.
2. **Invalidation spans four write paths, once per search source** — CLA-group create/rename, project enroll/unenroll (`unenrollProjects` exists today), org and repo additions.
3. **Lambda undercuts in-process caching** — cold containers start empty; concurrency yields N copies with N staleness windows. A real cache therefore means shared infrastructure (ElastiCache/DAX/OpenSearch): new IaC, cost, failure mode, on-call surface — disproportionate in a milestone whose other backend deltas are single-condition changes.
4. **A cache doesn't provide what "instant search" needs** — no ranking, no fuzzy matching, no prefix-across-fields. It buys invalidation complexity without search capability.
5. **Cheaper levers first**: fix the dead filter, ~200 ms client debounce, server-side result cap, dropdown-only projection.

**Explicitly unverified**: the CLA-group row count. The assumption is hundreds to low thousands (staff-created legal constructs, not user-generated content), which is where a projected scan is comfortably fast. It **could not be measured** in this session — no `lfproduct-*` profile exists in the local `~/.aws/config` (only `lfx-*`), and those SSO tokens were expired with no interactive browser flow available. Recorded in spec.md under *Performance assumptions* as an assumption to verify, not a finding. If per-keystroke **repo-URL** search is required at full fidelity, cardinality is genuinely much larger and the fallback is exact/suffix GSI matching before any indexing layer.

### Mockup read in full — three items the artifacts were missing

Previously only copy strings had been extracted. Reading Final/v16 end to end surfaced:

- **The Download PDF "discrepancy" does not exist.** ICLA rows render a real `<a>` (rows 1/2/5/6); ECLA rows render a **disabled** `<span class="ccla-note">` with `cursor:not-allowed` and `title="Covered by Corporate CLA (CCLA)"` (rows 3/4). The mockup already encodes the no-ECLA-PDF backend reality. The item flagged for PM on 2026-08-08 was a misreading; **no PM input needed** — now spec FR-011a.
- **A row action no FR covered: "Request approval."** Present only on the Needs-attention ECLA (mockup `:333`) and **removed** when the row becomes Invalidated (`confirmInvalidate()` removes `.approve-group`, `:674-675`). Now specified in FR-011 as conditional on that state.
- **Exact status vocabulary, which pins FR-010 and forces FR-010a.** Three pills — `Valid`; `Needs attention` with *"No longer matches {company}'s approval criteria."*; `Invalidated` with *"You confirmed you no longer work at {company}."* The middle state is unrepresentable today: `row.Valid = sig.SignatureApproved && covered` (`v2/my_clas/service.go:218`) collapses approved-ness and coverage, and telling self- from manager-invalidation needs a third input (FR-008b's reason/actor). Hence FR-010a's three independent fields — the largest single M2 change to the My CLAs read path, and coupled to Spike 2's marker since they are the same field.

Also noted: the ECLA type chip carries the company inline (`ECLA · IBM`), and the invalidate modal requires typing `INVALIDATE` (`:439`) — a friction gate the spec's copy did not mention.

### Net effect

All five spec open questions are now resolved. **Nothing remains blocked on clarification** — the one outstanding item is a *measurement* (CLA-group cardinality, for FR-001a). Two decisions deliberately keep M2's Simplicity gate intact: no SS-owned identity-binding operation, and no bespoke cache or new datastore. Risk 4 (status) grew from "small SS-side delta" to a joint-largest backend item and is now coupled to risk 1 through the shared provenance field; risk 5 (search endpoint) is new.
