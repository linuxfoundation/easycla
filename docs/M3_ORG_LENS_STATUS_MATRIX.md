# M3 Status Matrix — Org lens CLA statuses

Copyright The Linux Foundation and each contributor to CommunityBridge.

SPDX-License-Identifier: CC-BY-4.0

**Parent spec**: milestone brief [03-milestone-ccla-org-lens-fable.md](../specs/001-easycla-ss-integration-fable/03-milestone-ccla-org-lens-fable.md); backend endpoints [M3_ORG_LENS_API.md](M3_ORG_LENS_API.md)
**Companion**: [MY_CLAS_STATUS_MATRIX.md](MY_CLAS_STATUS_MATRIX.md) — the shipped M2 status model for the **Me** lens
**Status of this document**: **the target model.** It describes the statuses the Org lens
should show, not what any build currently renders. Written against the M3 UI prototype and
the M3 backend as it stands on `dev` (2026-09-14). Implementation is compared against this
document, not the other way round — where the two differ,
[Not yet implemented](#not-yet-implemented) records it.

Single source of truth for the statuses the **Org lens** EasyCLA module shows once the
Corporate CLA Console moves into LFX Self Serve. Two surfaces carry a status: the **CLA
entry** (one signing entity × CLA group) and each **employee acknowledgment** row beneath
it. Every status below names its **backend basis** — the field or computation it reads —
because several statuses in the prototype have no backend basis yet.
[Not yet implemented](#not-yet-implemented) lists those gaps, so nobody builds to them by
mistake.

Where this document and the current Corporate CLA Console disagree, the console is the
old behavior and this document is the intent — see
[What changes from today's console](#what-changes-from-todays-console).

## CLA entry statuses

Shown on the CLA group card and on its detail page header.

| Status | What it means | Backend basis | Dated? |
|---|---|---|---|
| **Signed** | The signing entity has a corporate agreement in force for this CLA group. | `signed = true` on the list entry | yes — *Signed by {name} on {date}*, from `signedBy` + `signedOn` |
| **Revoked** | The signing entity is under a sanctions block, so the agreement cannot be relied on and every write is refused. Set by the system, never by a person in the product. | `sanctioned = true` on the list entry | no — see [Not yet implemented](#not-yet-implemented) |
| **Not started** | No agreement exists for this CLA group yet. | none — **not a listable state**, see below | no |

Rules that hold across the table:

- **Only signed agreements are listed.** `GET /v4/company/external/{companySFID}/cla-groups`
  reads its signatures through a query that filters on `signature_signed = true` **and**
  `signature_approved = true`, so an unsigned or invalidated corporate agreement produces
  no entry at all. This mirrors M2's rule that unsigned agreements are never shown. A
  consequence: `signed` is always `true` on a returned entry, so it is a sanity check
  rather than a discriminator.
- **Not started exists only as a preview.** It is reachable solely through **Sign CLA** —
  the search in the Org lens that lets an admin look up any CLA group in the LF project
  catalog, including ones their organization has no agreement with, in order to start
  signing one. That search is backed by `GET /cla-group/search` (`searchClaGroups`), which
  is unscoped by organization — it matches on CLA group, project and foundation names and
  on linked GitHub org / GitLab group / Gerrit names, and carries no signature or sanctions
  fields at all. Picking an unsigned CLA group there opens a transient detail page whose
  managers and approval tabs are locked. It is never a row in the org's CLA list, and a
  reload without the search loses it. Treating it as a persisted status would imply the
  list API can return it.
- **Revoked wins over Signed.** A sanctioned signing entity with a signed agreement shows
  **Revoked**. It is the more consequential fact and the more restrictive status — the
  entry becomes read-only — so it takes precedence, exactly as in M2.
- **Signing a CCLA is blocked while the signing entity is sanctioned**, so a **Revoked**
  entry can never become **Signed** and *Sign CLA* must be unavailable from a **Not
  started** preview for a sanctioned entity. The backend enforces this at two points in
  `v2/sign/service.go`, both via `checkCompanyCompliance`: once **before** the DocuSign
  envelope is created, and again in the **completion callback** before `signature_signed`
  is set — so a company that becomes blocked mid-signing does not get a finalized CCLA.
  `checkCompanyCompliance` prefers a live Sanctions Screening Service result over the
  stored flag, but falls back to the stored flag in four cases: a manual/admin block
  (`sanction_origin != "sss"`) short-circuits without an SSS call, a cached result inside
  the request is reused, an unconfigured SSS client in optional mode returns
  `is_sanctioned`, and an unreachable SSS does the same. SSS-origin blocks do fall through
  to a live call, so a now-clean result can clear them. **One operational exception matters
  for this rule**: `cla-sss-enabled=false` returns "not sanctioned" *before* any stored
  SSS-origin flag is consulted, so with screening disabled a company blocked by SSS can
  sign — only manual/admin blocks still refuse, because they short-circuit above that
  check. So "a Revoked entry can never become Signed" holds only while screening is
  enabled. The M3 self-serve endpoint delegates to this same method and inherits both
  gates and this exception.
- **Invalidated and Revoked must never share wording**, and **"Canceled" and "Invalid"
  remain banned copy**. Both rules carry over from M2 unchanged.
- **A name is shown only when a real one was recorded.** `signedBy` is omitted when the
  CCLA carries no `SignatoryName`, and there is no CLA-manager fallback — the line then
  degrades to *Signed on {date}* rather than naming the wrong person. **The date does not
  yet hold to the same standard**: the list service substitutes `signature_created` when the
  signature carries no `SignedOn`, so *Signed on {date}* can present a creation date as a
  signing date, and the entry carries no flag distinguishing the two. That breaks M2's "a
  wrong date is worse than none" rule — see [Not yet implemented](#not-yet-implemented).

### Which status a CLA entry gets

Read top to bottom; the first matching row wins.

| Condition | In the data | Status |
|---|---|---|
| No signed+approved CCLA | not returned by the list endpoint | *entry does not exist* |
| Signing entity is sanctioned | company `is_sanctioned = true` → `sanctioned = true` | **Revoked** |
| A corporate agreement is in force | `signed = true` | **Signed** |
| Reached only via **Sign CLA** search | — | **Not started** (preview only) |

A consequence worth stating plainly: a signing entity that is **sanctioned and has never
signed** cannot appear anywhere in the org's CLA list, because the list is built from
signatures. The prototype shows exactly this case (a sanctioned CLA group with no
agreement), and it is only reachable through the **Sign CLA** preview. That preview must
therefore apply the sanctions check itself rather than inheriting it from a list entry
that does not exist — and because signing is gated server-side regardless, an admin who
gets that far is refused at the API rather than silently creating an envelope.

## Acknowledgment statuses

Shown per contributor row in the acknowledgments (employee CLA) table of a CLA entry.

| Status | What it means | Backend basis | Dated? |
|---|---|---|---|
| **Authorized** | The employee acknowledged the corporate agreement and the company's approval criteria still cover them. | `signatureSigned = true` **and** `signatureApproved = true` | no |
| **Not Authorized** | The acknowledgment is intact, but the contributor is no longer covered by the approval criteria. Nobody revoked their access deliberately — a criterion they matched was removed, or their membership of an approved org or group changed. **Recoverable**: adding them back to the approval list restores them. | **none today** — no coverage verdict is computed for this row, see [Not yet implemented](#not-yet-implemented) | no |
| **Invalidated** | The acknowledgment itself was made void — deliberately by a CLA manager, or as a side effect of an approval-criterion removal. **Not recoverable** by re-adding the contributor; they must acknowledge again. One exception today: see the `autoCreateECLA` rule below. | `signatureApproved = false` | yes — *Invalidated · date* (field not exposed on this row today) |

Rules:

- **Unsigned acknowledgments should never be shown** — a row should exist only once the
  employee has acknowledged, `signatureSigned = true`, aligning the org lens with M2's
  filter and retiring today's console state **"Not set up"** (`!approved && !signed`).
  **This is a proposal, not current behavior**: unlike the CCLA query, the employee
  signature query filters only on company and project, so unsigned records *are* returned
  today and the frontend must drop them. See
  [Not yet implemented](#not-yet-implemented).
- **Not Authorized is recoverable; Invalidated is not.** The distinction is the whole point
  of splitting them: one is a coverage drift the company can undo by re-adding the
  contributor, the other is a voided agreement it cannot. Copy and severity must not blur
  them — the prototype renders the first amber and the second red.
- **`autoCreateECLA` breaks that rule today.** When the CCLA has auto-create enabled, an
  approval-list update calls `CreateOrUpdateEmployeeSignature`, which runs
  `ValidateProjectRecord` against every acknowledgment where `signature_approved` or
  `signature_signed` is false — and that sets `signature_approved = true`. So an
  **Invalidated** row silently returns to **Authorized** on the next approval-list edit,
  including one a CLA manager invalidated deliberately. The target model treats Invalidated
  as final; see [Not yet implemented](#not-yet-implemented).
- **Invalidated does not name who did it** unless the record actually says so. New
  invalidations store `InvalidationMetadata` (`InvalidatedBy`, `Reason`, `Note`) and the M3
  invalidate endpoint takes a `reason` enum plus a free-text `note`, but older records carry
  nothing — so
  attribution is per-record, never assumed. Same constraint as M2, with a narrower blast
  radius.

### Which status an acknowledgment row gets

| Condition | In the data | Status |
|---|---|---|
| Not acknowledged | `signatureSigned = false` | *row is not shown* (filtered client-side today) |
| The acknowledgment was voided | `signatureApproved = false` | **Invalidated** |
| Acknowledgment intact, criteria do not cover the contributor | *not computed today* | **Not Authorized** |
| Acknowledgment intact and covered | `signatureApproved = true` | **Authorized** |

### Why Not Authorized is nearly unreachable today

This is the central gap in the M3 status model, and it is worth being precise about.

Removing an approval criterion **invalidates matching acknowledgments immediately**, for
most criteria. `invalidateSignatures` in `cla-backend-go/signatures/repository.go` walks
the CCLA's acknowledgments and delegates each one to `verifyUserApprovals`, which sets
`signature_approved = false` with invalidation metadata (`InvalidatedBy` and
`Reason: "approved list removal (<criteria>)"`; `Note` is left empty on this path). The row
therefore lands in **Invalidated**, not in a lingering recoverable state.

`verifyUserApprovals` branches on which criterion was removed, and the branches are **not
uniform** — which matters, because the gaps live in the differences:

| Removed criterion | Behavior | Veto protecting a still-covered contributor |
|---|---|---|
| Email, GitHub username, GitLab username | Invalidates | `userStillApproved` — full re-check across emails, domain patterns, and both username lists |
| Email domain | Invalidates, but only if the user's emails match a *removed* domain pattern | `userStillApproved` |
| GitHub org | Invalidates if the user's GitHub username is in the removed org's member list — but only reached at all via the shared-struct leak below | **narrower** — only checks the email and GitHub-username approval lists; a contributor covered solely by a domain rule or a GitLab username is invalidated anyway |
| **GitLab group** | **Invalidates nothing on its own** — see below | n/a |

`userStillApproved` deliberately does **not** re-check GitHub or GitLab org membership, so
a contributor covered only by *another* org rule is invalidated when one org is removed.

Both **org** branches depend on a quirk worth naming, because it decides whether they run
at all. `UpdateApprovalList` declares **one** `ApprovalList` struct and mutates it in place
as it walks the removal blocks in order (email → domain → GitHub username → GitHub org →
GitLab username → GitLab org). The three username/email blocks each build their own
per-entry copy and reset `ECLAs` to `nil` first, so they stay isolated. The **domain** block
does not: it assigns `ECLAs` on the shared struct, and neither org block clears it. So the
two org blocks iterate whatever the domain block left behind — meaning an org removal
invalidates nothing when it arrives alone, but when a single request removes **both** a
domain entry and an org entry, the org block runs the sweep over the *domain*-derived
acknowledgment set while `Criteria` has been overwritten to the org criterion. This is the
only path by which the GitHub-org branch executes at all, and the set it judges is not the
one that criterion selected.

What remains are the cases that leave an acknowledgment's coverage **stale** — the
contributor is no longer covered, but nothing recorded it. These are the conditions
**Not Authorized** is meant to name; today they produce no status change at all, so the row
keeps reading **Authorized**. The status is not merely rare, it is unreachable until a
coverage verdict exists:

- **GitLab group removals**, which invalidate nothing on their own. The path does call
  `invalidateSignatures`, but nothing reaches `verifyUserApprovals` with a verdict: the
  GitLab-org block never populates the `ECLAs` to iterate, and `verifyUserApprovals` has no
  branch for `GitlabOrgCriteria` — the sixth criterion falls through all three branches and
  returns `invalidated = false`. So even when acknowledgments *are* iterated, via the
  shared-struct leak above, a GitLab-group removal still invalidates none of them, and a
  contributor removed from an approved GitLab group keeps a fully **Authorized**-looking
  row. (M2 records the same gap from the contributor's side.)
- **Membership drift** — a contributor leaving an approved GitHub org or GitLab group.
  Coverage changes with no approval-list edit at all, so nothing triggers a re-check.
- **Records the sweep skipped.** `verifyUserApprovals` returns early without invalidating
  when the user record is missing (`GetUser` returns no record — deliberate, since
  invalidating what cannot be re-checked would be destructive), and each acknowledgment is
  processed in a goroutine with a `recover()` that logs and skips on panic.

The prototype's recovery hint — *add the user to the approval list, or Invalidate to
remove for good* — is untruthful for the common case. When a manager removes a criterion
the acknowledgment is already voided, so re-adding the criterion does not restore the row;
the contributor must acknowledge again. The hint only describes reality where
`autoCreateECLA` happens to be enabled, and there it works by re-approving invalidated
records indiscriminately rather than by any coverage logic. Either the row needs a live
coverage verdict from the backend, or removal must stop auto-invalidating. This is an open
product decision, not a copy fix.

## Cross-lens naming map

The same underlying state is named differently in the Me lens and the Org lens. That is
deliberate — a contributor reads "what do I need to do", an org admin reads "is this
person covered" — but support needs to translate between them.

| Underlying state | Me lens (M2, shipped) | Today's Corporate Console | Org lens (M3, target) |
|---|---|---|---|
| Acknowledged and covered | **Valid** | Authorized | **Authorized** |
| Acknowledged, not covered by criteria | **Needs attention** | Not Authorized | **Not Authorized** |
| Acknowledgment voided | **Invalidated** | Not Authorized *(collapsed)* | **Invalidated** |
| Coverage could not be determined | **—** (dash) | *never determined — no coverage check runs* | *no equivalent — see gaps* |
| Company under sanctions | **Revoked** (on the ECLA row) | "Unable to Sign" / "Unable to Prepare CCLA" error states | **Revoked** (on the CLA entry) |
| Not acknowledged / not signed | *row hidden* | Not set up | *row hidden* |

Two divergences are load-bearing:

- **"Needs attention" ≡ "Not Authorized".** Same state, different audience. The Me-lens
  wording is an instruction to the contributor; the org-lens wording is a fact about a
  person. Neither name is being changed.
- **"Revoked" sits on a different object in each lens.** In the Me lens it labels one
  contributor's ECLA; in the Org lens it labels the CLA entry for a whole signing entity.
  The sanction is a company-level fact in both cases — only the object carrying the label
  differs.

## What changes from today's console

| Surface | Corporate CLA Console today | Org lens (M3, target) |
|---|---|---|
| Acknowledgment statuses | Authorized / Not Authorized / Not set up, from two booleans | Authorized / Not Authorized / Invalidated; unsigned rows dropped rather than labelled |
| Voided vs not-covered | Indistinguishable — both read "Not Authorized" | Split, with different colors, copy and recoverability |
| CLA entry status | Signed / Not Signed per project | Signed / Revoked, with unsigned reachable only via the **Sign CLA** preview |
| Sanctions detection | `isSanctioned` plus a substring test for `"sanctioned"` on the error message text — `project-active-cla.component.ts` and `ccla-dialog.component.ts` both carry a `TODO(#5078)` to replace it with a machine-readable code | Typed `code: "company_sanctioned"` on the gated write ops; stored `sanctioned` flag on reads; live SSS re-screening on the CCLA signing path |
| Sanctions copy | Two different titles: *Unable to Sign* on the project CLA page, *Unable to Prepare CCLA* in the CCLA dialog, sharing one body string | one state, one wording — **Revoked** on the entry |

## Not yet implemented

Everything above is the intended model. These parts are not backed end-to-end yet — some
have no backend at all, others store the data but do not expose it.

A first Org-lens EasyCLA build already exists in `lfx-self-serve` behind the
`org-lens-cla-m3-enabled` feature flag (list, card, detail page, approval list, Sign CLA
flow). It is not user-visible, and it diverges from this document in two places — the first
two rows below. Acknowledgment statuses have no UI there at all, so everything in
[Acknowledgment statuses](#acknowledgment-statuses) is still unbuilt.

| Gap | Effect in the Org lens | Backend status |
|---|---|---|
| **The shipped build labels the sanctions state "Sanctioned", not "Revoked"** | The card pill reads *Sanctioned* and the detail heading reads *Unavailable*, where this matrix and the M2 Me lens both name the same company-level fact **Revoked**. One state, three words across two lenses. | frontend copy only — rename to **Revoked** to match the Me lens |
| **The shipped build derives the entry status from two booleans in the BFF** | `signed` and `sanctioned` are collapsed into one status in the Self Serve server layer, so the Org lens repeats the two-boolean derivation this matrix criticizes in the old console — just relocated. The Me lens by contrast consumes an authoritative `status` from the producer. | open — whether the entry status should become a producer-side field like the Me lens's |
| **Unsigned acknowledgments are not filtered server-side, and the count disagrees with the page** | Rows with `signatureSigned = false` come back from the API, so the Org lens must drop them itself or it will show statusless rows. Worse, the two queries disagree: the page query filters on company only, while `totalCount` also filters `signature_approved` and `signature_signed`. Unsigned and invalidated records therefore consume page slots and cursors while being excluded from the reported total, so client-side dropping yields short pages and a count that does not match the rows. Dropping them in the frontend cannot fix the pagination. | needs filing — filter in **both** queries; a frontend-only fix is not sufficient |
| **`autoCreateECLA` resurrects invalidated acknowledgments** | The target model treats **Invalidated** as final. It is not: on a CCLA with auto-create enabled, every approval-list update calls `CreateOrUpdateEmployeeSignature`, which runs `ValidateProjectRecord` over each acknowledgment where `signature_approved` or `signature_signed` is false and sets `signature_approved = true`. A row a CLA manager invalidated deliberately silently returns to **Authorized** on the next unrelated approval-list edit, with only a `note` recording it. | needs filing — a likely backend bug; auto-create should not re-approve records that were invalidated |
| **`signedOn` may be a creation date, not a signing date** | *Signed on {date}* is not guaranteed to be a signing date: the list service substitutes `signature_created` when the signature carries no `SignedOn`, and the entry carries no flag distinguishing the two. This breaks the M2 rule that a wrong date is worse than none. | needs filing — omit the field when no signing timestamp exists, or mark it approximate |
| **No coverage verdict on an acknowledgment row** | **Not Authorized** cannot be rendered for the case the prototype describes. `corporate-contributor` carries only `signatureSigned` and `signatureApproved`; there is no equivalent of the M2 coverage check. | needs filing — a per-row coverage verdict, or a documented decision that removal stops auto-invalidating |
| **Invalidation date, reason and actor are not exposed** | **Invalidated** renders without its date or cause, even where `InvalidationMetadata` recorded both. | metadata is stored; the row model exposes none of it |
| **The signing refusal is not machine-readable** | The two CCLA signing gates return a plain error (`company requires further review for trade compliance`), not the typed `403 company_sanctioned` body the gated write ops return. The Org lens would have to string-match to tell a sanctions refusal from any other signing failure — the same fragility as today's console `TODO(#5078)`. | needs filing — return the typed sanctions error from the signing path too |
| **Revoked has no date** | The CLA entry shows the status alone. The Me lens dates it from `flaggedAt`; the list entry carries only the boolean `sanctioned`, with no date field at all. | needs filing |
| **The Sign CLA search endpoint is undocumented** | `GET /cla-group/search` backs the **Not started** preview but has no section in [M3_ORG_LENS_API.md](M3_ORG_LENS_API.md), so the one status that depends on it has no documented contract. | needs filing — document the endpoint |
| **GitLab group removals invalidate nothing** | A contributor removed from an approved GitLab group keeps an **Authorized** row. `verifyUserApprovals` has no `GitlabOrgCriteria` branch, so the criterion falls through and reports nothing invalidated — and the GitLab-org block never populates the acknowledgments to iterate in the first place. Carried over from M2 unchanged. | needs filing |
| **Org removals iterate the wrong acknowledgment set** | `UpdateApprovalList` mutates one shared `ApprovalList` in place; only the domain block assigns `ECLAs` on it, and neither org block clears it. An org removal alone sweeps nothing; an org removal **combined with a domain removal in the same request** sweeps the domain-derived set under the org criterion. This is also the only way the GitHub-org branch runs at all. | needs filing — a backend bug; reset `ECLAs` per block or give each block its own struct |
| **GitHub-org removal over-invalidates** | When it does run (see the row above), the `GitHubOrgCriteria` branch checks only the email and GitHub-username approval lists before invalidating, instead of the full `userStillApproved` re-check its sibling branches use — and it compares with exact, case-sensitive `StringInSlice` against a single `getBestEmail(user)`, where `userStillApproved` folds case across *all* the user's emails. A contributor still covered by a domain rule, a GitLab username, a differently-cased entry, or a secondary email is invalidated anyway, landing in **Invalidated** while genuinely still approved. | needs filing — a backend bug, not a display gap |
| **An invalidated CCLA disappears silently** | The CLA entry vanishes from the list with no trace, so an org admin cannot tell a never-signed CLA group from one whose agreement was voided. | open product question — whether an invalidated CCLA deserves its own entry status |
| **Sanctioned + never signed is not listable** | The prototype's Revoked-without-agreement case has no list entry; only the **Sign CLA** preview can show it. That preview is backed by `GET /cla-group/search`, whose result model carries no `sanctioned` field, so the preview must resolve the sanctions state separately rather than reading it from the search result. | needs filing — either add the flag to the search result or have the preview resolve the company |
| **Invalidating existing ECLAs on sanction is blocked** | A newly sanctioned company keeps **Authorized** acknowledgment rows while every write is refused. | blocked pending [lfx-self-serve#2051](https://github.com/linuxfoundation/lfx-self-serve/issues/2051) |
| **`needsClaManager` and `autoCreateECLA` are flags, not statuses** | Both are on the list entry and render as their own affordances on the card. They deliberately do not participate in the status model above. | done — out of scope for this matrix |

## Related tickets

- [lfx-self-serve#2149](https://github.com/linuxfoundation/lfx-self-serve/issues/2149) — org CLA groups list (`signed`, `signedOn`, `sanctioned`, counts)
- [lfx-self-serve#2231](https://github.com/linuxfoundation/lfx-self-serve/issues/2231) — `signedBy` on the list entry
- [lfx-self-serve#2222](https://github.com/linuxfoundation/lfx-self-serve/issues/2222) — `approvalCriteriaCount`
- [lfx-self-serve#2150](https://github.com/linuxfoundation/lfx-self-serve/issues/2150) — self-serve corporate signature (the *Sign CLA* path)
- [lfx-self-serve#2151](https://github.com/linuxfoundation/lfx-self-serve/issues/2151) — CLA manager requests + ECLA invalidate (`reason` enum)
- [lfx-self-serve#2153](https://github.com/linuxfoundation/lfx-self-serve/issues/2153) — sanctioned-company write gating
- [lfx-self-serve#2186](https://github.com/linuxfoundation/lfx-self-serve/issues/2186) — email removal invalidates all matching acknowledgments
- [lfx-self-serve#2051](https://github.com/linuxfoundation/lfx-self-serve/issues/2051) — invalidating existing ECLAs when a company becomes sanctioned
