# M3 Status Matrix — Org lens CLA statuses

Copyright The Linux Foundation and each contributor to CommunityBridge.

SPDX-License-Identifier: CC-BY-4.0

**Parent spec**: milestone brief [03-milestone-ccla-org-lens-fable.md](../specs/001-easycla-ss-integration-fable/03-milestone-ccla-org-lens-fable.md); backend endpoints [M3_ORG_LENS_API.md](M3_ORG_LENS_API.md)
**Companion**: [MY_CLAS_STATUS_MATRIX.md](MY_CLAS_STATUS_MATRIX.md) — the shipped M2 status model for the **Me** lens
**Status of this document**: **target model, not shipped.** Written against the M3 UI prototype and the M3 backend as it stands on `dev` (2026-09-14).

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
  signing one. Picking an unsigned CLA group there opens a transient detail page whose
  managers and approval tabs are locked. It is never a row in the org's CLA list, and a
  reload without the search loses it. Treating it as a persisted status would imply the
  API can return it.
- **Revoked wins over Signed.** A sanctioned signing entity with a signed agreement shows
  **Revoked**. It is the more consequential fact and the more restrictive status — the
  entry becomes read-only — so it takes precedence, exactly as in M2.
- **Signing a CCLA is blocked while the signing entity is sanctioned**, so a **Revoked**
  entry can never become **Signed** and *Sign CLA* must be unavailable from a **Not
  started** preview for a sanctioned entity. The backend enforces this at two points in
  `v2/sign/service.go`, both via `checkCompanyCompliance`, which re-screens against the
  Sanctions Screening Service rather than trusting the stored flag: once **before** the
  DocuSign envelope is created, and again in the **completion callback** before
  `signature_signed` is set — so a company that becomes blocked mid-signing does not get
  a finalized CCLA. Manual/admin blocks short-circuit without an SSS call; SSS-origin
  blocks fall through so a now-clean result can clear them. The M3 self-serve endpoint
  delegates to this same method and inherits both gates.
- **Invalidated and Revoked must never share wording**, and **"Canceled" and "Invalid"
  remain banned copy**. Both rules carry over from M2 unchanged.
- **A date is shown only when a real one was recorded.** `signedBy` is omitted when the
  CCLA carries no `SignatoryName`, and there is no CLA-manager fallback — the line then
  degrades to *Signed on {date}* rather than naming the wrong person.

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
| **Invalidated** | The acknowledgment itself was made void — deliberately by a CLA manager, or as a side effect of an approval-criterion removal. **Not recoverable** by re-adding the contributor; they must acknowledge again. | `signatureApproved = false` | yes — *Invalidated · date* (field not exposed on this row today) |

Rules:

- **Unsigned acknowledgments should never be shown** — a row should exist only once the
  employee has acknowledged, `signatureSigned = true`, aligning the org lens with M2's
  filter and retiring today's console state **"Not set up"** (`!approved && !signed`).
  **This is a proposal, not current behavior**: unlike the CCLA query, the employee
  signature query filters only on company and project, so unsigned records *are* returned
  today and the frontend must drop them. See
  [Not yet implemented](#not-yet-implemented).
- **Not Authorized is amber and recoverable; Invalidated is red and deliberate.** The
  distinction is the whole point of splitting them: one is a coverage drift the company
  can undo, the other is a voided agreement it cannot. Copy must not blur them.
- **Invalidated does not name who did it** unless the record actually says so. New
  invalidations store `InvalidationMetadata` (`InvalidatedBy`, `Reason`) and the M3
  invalidate endpoint takes a `reason` enum, but older records carry nothing — so
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

Removing an approval criterion **invalidates matching acknowledgments immediately**.
`invalidateSignatures` in `cla-backend-go/signatures/repository.go` sets
`signature_approved = false` with invalidation metadata for removals of email, email
domain, GitHub username, GitHub org, and GitLab username criteria — each guarded by a
`userStillApproved` veto so a contributor covered by another criterion is left alone. The
row therefore lands in **Invalidated**, not in a lingering recoverable state.

So a genuine **Not Authorized** can only arise from:

- **GitLab group removals**, which are not handled at all (the same gap M2 records) —
  group membership cannot be checked without per-group tokens.
- **Membership drift** — a contributor leaving an approved GitHub org or GitLab group,
  which changes coverage without any approval-list edit and is never re-evaluated.
- **Records the invalidation sweep skipped**, e.g. where the underlying user record is
  gone.

The prototype's recovery hint — *add the user to the Approval list, or Invalidate to
remove for good* — is only truthful for those cases. For the common case (a manager
removes a criterion) re-adding the criterion does **not** restore the row, because the
acknowledgment was already voided. Either the row needs a live coverage verdict from the
backend, or removal must stop auto-invalidating. This is an open product decision, not a
copy fix.

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
| Company under sanctions | **Revoked** (on the ECLA row) | "Unable to Sign" error state | **Revoked** (on the CLA entry) |
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
| Sanctions detection | `isSanctioned` plus string matching on error messages — `project-active-cla.component.ts` and `ccla-dialog.component.ts` both carry a `TODO(#5078)` to replace it | Typed `code: "company_sanctioned"` on the gated write ops; stored `sanctioned` flag on reads; live SSS re-screening on the CCLA signing path |

## Not yet implemented

Everything above is the intended model. These parts are not backed end-to-end yet — some
have no backend at all, others store the data but do not expose it.

| Gap | Effect in the Org lens | Backend status |
|---|---|---|
| **Unsigned acknowledgments are not filtered server-side** | Rows with `signatureSigned = false` come back from the API, so the Org lens must drop them itself or it will show statusless rows. The CCLA list query filters on signed+approved; the employee signature query filters only on company and project. | needs filing — either filter in the query or confirm the frontend owns it |
| **No coverage verdict on an acknowledgment row** | **Not Authorized** cannot be rendered for the case the prototype describes. `corporate-contributor` carries only `signatureSigned` and `signatureApproved`; there is no equivalent of the M2 coverage check. | needs filing — a per-row coverage verdict, or a documented decision that removal stops auto-invalidating |
| **Invalidation date, reason and actor are not exposed** | **Invalidated** renders without its date or cause, even where `InvalidationMetadata` recorded both. | metadata is stored; the row model exposes none of it |
| **The signing refusal is not machine-readable** | The two CCLA signing gates return a plain error (`company requires further review for trade compliance`), not the typed `403 company_sanctioned` body the gated write ops return. The Org lens would have to string-match to tell a sanctions refusal from any other signing failure — the same fragility as today's console `TODO(#5078)`. | needs filing — return the typed sanctions error from the signing path too |
| **Revoked has no date** | The CLA entry shows the status alone. The Me lens dates it from `flaggedAt`; the list entry carries only the boolean `sanctioned`. | needs filing |
| **GitLab group removals do not invalidate** | A contributor removed from an approved GitLab group keeps an **Authorized** row. Carried over from M2 unchanged. | needs filing |
| **An invalidated CCLA disappears silently** | The CLA entry vanishes from the list with no trace, so an org admin cannot tell a never-signed CLA group from one whose agreement was voided. | open product question — whether an invalidated CCLA deserves its own entry status |
| **Sanctioned + never signed is not listable** | The prototype's Revoked-without-agreement case has no list entry; only the catalog preview can show it, and it must run its own sanctions check. | open — depends on the preview's data source |
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
