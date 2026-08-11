# Phase 0 Research: M2 — backend spikes

**Input**: [spec.md](spec.md) open questions 2 and 4 | [plan.md](plan.md) Complexity Tracking risks 1 and 2
**Date**: 2026-08-11 | **Status**: both spikes resolved; recommendations below feed `/speckit.clarify` and the contracts

These are the two `[NEEDS CLARIFICATION]` items that block M2's sign-entry and ECLA-invalidation paths. Neither was answerable from the requirements text — both needed the code traced. Every claim below cites the file and line it was read from (`cla-backend-go` and `easycla-contributor-console` at `dev`/`main`, 2026-08-11).

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

- FR-001's listing endpoint (open question 5, narrowed — which existing endpoint serves the name search).
- Open question 1's identity-binding operation shape (create-vs-enrich branches, gateway auth). Spike 1 confirms *why* it's load-bearing: `acl = github:{user.GithubID}` at `v2/sign/service.go:1433` means the GitHub ID must be bound **before** signing, or the proactive signature carries an empty ACL.
- ECLA "Download PDF" mockup discrepancy — still awaiting PM.
