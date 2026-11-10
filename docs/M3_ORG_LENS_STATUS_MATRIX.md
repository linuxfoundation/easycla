# M3 Status Matrix — Org lens CLA statuses

Copyright The Linux Foundation and each contributor to CommunityBridge.

SPDX-License-Identifier: CC-BY-4.0

**Parent spec**: milestone brief [03-milestone-ccla-org-lens-fable.md](../specs/001-easycla-ss-integration-fable/03-milestone-ccla-org-lens-fable.md); backend endpoints [M3_ORG_LENS_API.md](M3_ORG_LENS_API.md)
**Companion**: [MY_CLAS_STATUS_MATRIX.md](MY_CLAS_STATUS_MATRIX.md) — the shipped M2 status model for the **Me** lens
**Status of this document**: **the target model.** It describes the statuses the Org lens
should show, not what any build renders. Written against the M3 UI prototype and the M3
backend on `dev` (2026-09-14; acknowledgment statuses re-checked 2026-11-10). Implementation is compared against this document, not the
other way round; where they differ, [Not yet implemented](#not-yet-implemented) records it.

Single source of truth for the statuses the **Org lens** EasyCLA module shows once the
Corporate CLA Console moves into LFX Self Serve. Two surfaces carry a status: the **CLA
entry** (one signing entity × CLA group) and each **employee acknowledgment** row beneath
it. Every status names its **backend basis** — several have none yet.

### Terminology

Two objects, and the names this document uses for them:

| Object | This document | Backend / code | Do not use |
|---|---|---|---|
| A signing entity's agreement with one CLA group | **CLA entry**, **corporate agreement** | CCLA, `signature_signed`, `corporate-signature` | — |
| One employee's coverage under that agreement | **employee acknowledgment**, **acknowledgment** | ECLA, `employee_signature`, `autoCreateECLA` | **"Employee CLA"**, **"ECLA"** in prose |

An employee does not sign a separate agreement — they **acknowledge** the company's corporate
agreement. "Employee CLA" implies otherwise and is not used in user-facing text: documentation,
emails and UI all say *employee acknowledgment*. **ECLA** survives only as an internal
abbreviation in code identifiers, URL paths and schema names (`autoCreateECLA`,
`/cla-group/{id}/ecla/{sigID}/invalidate`), which this document quotes verbatim where it cites
them. Spelling is American throughout — *acknowledgment*, not *acknowledgement*.

## CLA entry statuses (corporate agreement)

Shown on the CLA group card and on its detail page header.

| Status | What it means | Backend basis | Dated? |
|---|---|---|---|
| **Signed** | The signing entity has a corporate agreement in force for this CLA group. | `signed = true` on the list entry | yes — *Signed by {name} on {date}*, from `signedBy` + `signedOn` |
| **Revoked** | The signing entity is under a sanctions block, so the agreement cannot be relied on and every write is refused. Set by the system, never by a person in the product. | `sanctioned = true` on the list entry | no — see [Not yet implemented](#not-yet-implemented) |
| **Not started** | No agreement exists for this CLA group yet. | none — **not a listable state**, see below | no |

Rules:

- **Only signed agreements are listed.** `GET /v4/company/external/{companySFID}/cla-groups`
  filters on `signature_signed = true` **and** `signature_approved = true`, so an unsigned or
  invalidated corporate agreement produces no entry at all — mirroring M2's rule that unsigned
  agreements are never shown. Consequently `signed` is always `true` on a returned entry: a
  sanity check, not a discriminator.
- **Not started exists only as a preview.** It is reachable solely through **Sign CLA**, the
  unscoped catalog search backed by [`GET /v4/cla-group/search`](M3_ORG_LENS_API.md), which
  carries no signature or sanctions fields. Picking an unsigned CLA group there opens a
  transient detail page with managers and approval tabs locked; it is never a row in the org's
  CLA list and a reload loses it. Treating it as a persisted status would imply the list API
  can return it.
- **Revoked wins over every other status.** Revoked means the signing entity is under a
  sanctions block — a fact about the whole organization, not about any one agreement — so it
  outranks Signed and Not started alike and is evaluated first. M2 applies the same reasoning
  where it ranks Revoked above Invalidated: the sanction is the more consequential fact and
  Revoked is the more restrictive status, since the entry becomes read-only.
- **Signing a CCLA is blocked while the signing entity is sanctioned**, so a **Revoked** entry
  can never become **Signed**, and *Sign CLA* must be unavailable from a **Not started**
  preview for a sanctioned entity. `v2/sign/service.go` enforces this at two points via
  `checkCompanyCompliance` — before the DocuSign envelope is created, and again in the
  completion callback before `signature_signed` is set. That method prefers a live Sanctions
  Screening Service result over the stored flag:
    - **A manual/admin block** (`sanction_origin != "sss"`) short-circuits first, with no SSS
      call. SSS-origin blocks fall through to a live call, so a now-clean result can clear them.
    - **A cached decision wins next.** The cache is **service-scoped with a five-minute TTL**,
      not per-request, so a decision can be reused across requests for up to five minutes.
    - **When no live result is available** — unconfigured client, no external ID, unresolvable
      domain, SSS unreachable, or an ambiguous status — the modes diverge: **optional** mode
      honors the stored `is_sanctioned`; **required** mode returns an error and blocks signing,
      failing closed rather than falling back to the flag.

  **One exception**: `cla-sss-enabled=false` returns "not sanctioned" *before* the cache and
  before any stored SSS-origin flag, so with screening disabled a company blocked by SSS can
  sign — only manual/admin blocks still refuse. "A Revoked entry can never become Signed" holds
  only while screening is enabled. The M3 self-serve endpoint delegates to this same method and
  inherits both gates and the exception.
- **Invalidated and Revoked must never share wording**, and **"Canceled" and "Invalid" remain
  banned copy**. Both carry over from M2 unchanged.
- **A name is shown only when a real one was recorded.** `signedBy` is omitted when the CCLA
  carries no `SignatoryName` — no CLA-manager fallback — so the line degrades to *Signed on
  {date}* rather than naming the wrong person. **The date now holds to that standard too**:
  the Org-lens list service (`storedSignedOn`) reads the CCLA's stored `SignedOn` directly and
  returns no date when it is absent, rather than substituting `signature_created` — the shared
  v1 converter's fallback, which is a corporate-console contract this lens does not inherit.

### Which status a CLA entry gets

First matching row wins.

| Condition | In the data | Status |
|---|---|---|
| No signed+approved CCLA | not returned by the list endpoint | *entry does not exist* |
| Signing entity is sanctioned | company `is_sanctioned = true` → `sanctioned = true` | **Revoked** |
| A corporate agreement is in force | `signed = true` | **Signed** |
| Reached only via **Sign CLA** search | — | **Not started** (preview only) |

A signing entity that is **sanctioned and has never signed** cannot appear in the org's CLA
list at all, because the list is built from signatures. The prototype shows exactly that case,
reachable only through the **Sign CLA** preview — so the preview must apply the sanctions check
itself rather than inheriting it from an entry that does not exist. Signing is gated
server-side regardless, so an admin who gets that far is refused at the API.

## Acknowledgment statuses (employee acknowledgment)

Shown per contributor row in the employee acknowledgment table of a CLA entry.

| Status | What it means | Backend basis | Dated? |
|---|---|---|---|
| **Authorized** | The employee acknowledged the corporate agreement and the company's approval criteria still cover them. | `signatureSigned = true` **and** `signatureApproved = true` | no |
| **Not Authorized** | The acknowledgment is intact, but an approval-list criterion that covered the contributor was removed. Nobody voided it deliberately. **Recoverable**: re-adding a matching entry returns the row to **Authorized**, with no re-acknowledgment. | `signatureApproved = false` with approval-list-removal evidence only — `invalidationReason` *approved list removal (…)*, or the legacy removal note — classified by `approvalListRemoval()` in the Self Serve BFF | no |
| **Invalidated** | The acknowledgment was made void deliberately, by a CLA manager or admin. **Not recoverable** by re-adding the contributor; they must acknowledge again. | `signatureApproved = false` without removal-only evidence | yes — *Invalidated · date*, from `invalidatedAt` |

Rules:

- **Unsigned acknowledgments are never shown** — a row exists only once
  `signatureSigned = true`, aligning with M2's filter and retiring today's console state
  **"Not set up"** (`!approved && !signed`). The backend enforces this: `corporateContributorFilter`
  requires `signature_signed = true` in both the page and the `totalCount` query of
  `GetClaGroupCorporateContributors`, and keeps signed rows with `signature_approved = false` —
  the **Not Authorized** and **Invalidated** rows.
- **Not Authorized is recoverable; Invalidated is not.** That distinction is the whole point of
  splitting them: one is a removed criterion the company can put back, the other a voided
  agreement it cannot. Since [easycla#5231](https://github.com/linuxfoundation/easycla/pull/5231)
  the backend honors it: on an approval-list add, `restoreRemovalInvalidatedEmployeeSignatures`
  re-approves the signed acknowledgments that **only** an approval-list removal had invalidated
  and whose user the added entries cover, with or without `autoCreateECLA`; deliberate
  invalidations are never touched. Copy and severity must not blur them — the prototype renders
  the first amber, the second red.
- **`autoCreateECLA` now honors that rule.** `processEmployeeSignatures` checks
  `employeeModel.Invalidated` first and leaves an invalidated record alone, logging that it
  needs an explicit re-approval; only a record that was never invalidated and has either flag
  false is re-approved, via `ValidateProjectRecordUnlessInvalidated`. An **Invalidated** row does
  not return to **Authorized** on an approval-list edit; the only way back for an unapproved row
  is the re-add recovery above, which applies to **Not Authorized** rows alone.
- **Only the invalidation date is shown; the row names nobody.** The API exposes the full set —
  `corporate-contributor` carries `invalidatedAt`, `invalidatedBy`, `invalidationReason`,
  `invalidationNote` and the legacy `note`, all populated by `GetClaGroupCorporateContributors`
  — but the Org lens renders the **date alone**, matching the M3 prototype and M2's
  *Invalidated · date* pill. Actor and reason stay available for support and the activity log,
  not the status cell. Older records carry nothing at all, so attribution is per-record and
  never assumed — the same constraint as M2.

### Which status an acknowledgment row gets

| Condition | In the data | Status |
|---|---|---|
| Not acknowledged | `signatureSigned = false` | *row is not shown* (filtered server-side) |
| Invalidated only by an approval-list removal | `signatureApproved = false` with removal evidence only | **Not Authorized** |
| Voided deliberately | `signatureApproved = false`, any other invalidation | **Invalidated** |
| Acknowledgment intact and covered | `signatureApproved = true` | **Authorized** |

### What Not Authorized still misses

Removing an approval criterion invalidates the matching acknowledgments with removal evidence —
`invalidateSignatures` sets `signature_approved = false` and `invalidation_reason`
*approved list removal (…)* — so the row lands in **Not Authorized**, and re-adding a matching
entry restores it ([lfx-self-serve#2980](https://github.com/linuxfoundation/lfx-self-serve/issues/2980)).
The prototype's recovery hint — *add the user to the approval list, or Invalidate to remove for
good* — is therefore truthful.

Not Authorized names only coverage loss that an approval-list removal recorded. Where coverage
went stale and nothing recorded it, the row keeps reading **Authorized**: membership drift in an
approved GitHub org (no approval-list edit occurs, so nothing triggers a re-check) and records
the sweep skipped (missing user record, or a panic recovered per acknowledgment). GitLab groups
are not a source of residue because they are never evaluated in either direction — they are out
of scope for M3 entirely. Naming that residue needs a live coverage verdict from the backend —
an open product decision, not a copy fix. The specific backend defects are listed in
[Not yet implemented](#not-yet-implemented).

## Cross-lens naming map

The same underlying state is named differently in each lens — deliberately, since a contributor
reads "what do I need to do" and an org admin reads "is this person covered" — but support
needs to translate.

| Underlying state | Me lens (M2, shipped) | Today's Corporate Console | Org lens (M3, target) |
|---|---|---|---|
| Acknowledged and covered | **Valid** | Authorized | **Authorized** |
| Acknowledged, not covered by criteria | **Needs attention** | Not Authorized | **Not Authorized** |
| Acknowledgment voided | **Invalidated** | Not Authorized *(collapsed)* | **Invalidated** |
| Coverage could not be determined | **—** (dash) | *never determined — no coverage check runs* | *no equivalent — see gaps* |
| Company under sanctions | **Revoked** (on the acknowledgment row) | "Unable to Sign" / "Unable to Prepare CCLA" error states | **Revoked** (on the CLA entry) |
| Not acknowledged / not signed | *row hidden* | Not set up | *row hidden* |

Two divergences are load-bearing:

- **"Needs attention" ≡ "Not Authorized".** Same state, different audience — an instruction to
  the contributor versus a fact about a person. Neither name is being changed.
- **"Revoked" sits on a different object in each lens** — one contributor's acknowledgment in
  the Me lens, the whole signing entity's CLA entry in the Org lens. The sanction is
  company-level in both; only the object carrying the label differs.

## What changes from today's console

| Surface | Corporate CLA Console today | Org lens (M3, target) |
|---|---|---|
| Acknowledgment statuses | Authorized / Not Authorized / Not set up, from two booleans | Authorized / Not Authorized / Invalidated; unsigned rows dropped rather than labelled |
| Voided vs not-covered | Indistinguishable — both read "Not Authorized" | Split, with different colors, copy and recoverability |
| CLA entry status | Signed / Not Signed per project | Signed / Revoked, with unsigned reachable only via the **Sign CLA** preview |
| Sanctions detection | `isSanctioned` plus a substring test for `"sanctioned"` on the error text — `project-active-cla.component.ts` and `ccla-dialog.component.ts` both carry a `TODO(#5078)` to replace it | Typed `code: "company_sanctioned"` on gated write ops; stored `sanctioned` flag on reads; live SSS re-screening on the signing path |
| Sanctions copy | Two titles — *Unable to Sign* and *Unable to Prepare CCLA* — sharing one body string | one state, one wording — **Revoked** on the entry |

## Not yet implemented

The Org-lens EasyCLA module is live in `lfx-self-serve` (list, card, detail page, approval
list, contributor acknowledgments, Sign CLA flow); its `org-lens-cla-m3-enabled` flag was
removed in [lfx-self-serve#5231](https://github.com/linuxfoundation/lfx-self-serve/issues/5231).
It diverges from this document in the first two rows below. The acknowledgment table renders all
three [acknowledgment statuses](#acknowledgment-statuses-employee-acknowledgment).

| Gap | Effect in the Org lens | Backend status |
|---|---|---|
| **Shipped build shows no pill for the sanctions state** | Since [lfx-self-serve#3532](https://github.com/linuxfoundation/lfx-self-serve/pull/3532) a sanctioned entry carries no status pill on the card or the detail header: legal review ruled out *Sanctioned*, and its replacement is pending that review. The detail heading still reads *Unavailable*. **Revoked** is this document's proposal, matching the Me lens and the M3 prototype, and is subject to the same review. | frontend copy only — pending legal review of the wording |
| **Shipped build derives the entry status from two booleans in the BFF** | `signed` and `sanctioned` are collapsed in the Self Serve server layer, relocating the two-boolean derivation this matrix criticizes. The Me lens consumes an authoritative `status` from the producer. | open — whether the entry status should become a producer-side field |
| **No coverage verdict for unrecorded drift** | **Not Authorized** renders only when an approval-list removal recorded the loss. A contributor who leaves an approved GitHub org, or a record the removal sweep skipped, keeps reading **Authorized**: `corporate-contributor` carries the signature flags and the invalidation attributes, but nothing stating whether the approval criteria still cover the contributor — no equivalent of the M2 coverage check exists. | needs filing — a per-row coverage verdict, or a documented decision that unrecorded drift stays unreported |
| **The prototype formats the invalidation date differently from M2** | The M3 prototype renders a full timestamp under the pill — *on Sep 17, 2026, 3:42:07 PM* — where the shipped Me lens shows *Invalidated · Jun 3, 2026*. One field, two formats across the lenses. | backend done — `invalidatedAt` is on the row; frontend copy only, adopt M2's date-only *Invalidated · date* |
| **The DocuSign-completion signing refusal is not machine-readable** | `SignRequestCorporateSignatureHandler` already maps a sanctions block to the typed `403 company_sanctioned` body, but `SignCclaCallbackHandler` still returns a plain `400` with no typed error, so the Org lens would have to string-match on that one path — the same fragility as `TODO(#5078)`. | needs filing — return the typed sanctions error from the callback path too |
| **Revoked has no date** | The entry shows the status alone; the list entry carries only the boolean `sanctioned`, and the M3 prototype shows no date either. The Me lens model dates it from `flaggedAt` — the company's stored `sanctioned_date`, stamped when EasyCLA first detected the block and re-stamped if a cleared entity is flagged again, so it is EasyCLA's detection date and not the sanctioning authority's listing date. | needs filing — expose the stored date on the list entry |
| **GitLab group criteria are stored but never evaluated, so they are out of scope for M3** | Group membership cannot be read without a per-group installed OAuth token, so EasyCLA evaluates a GitLab group entry in neither direction: `EvaluateUserApproval` has no group check when granting coverage, and `verifyUserApprovals` has no `GitlabOrgCriteria` branch when removing it. The entry is stored and listed, but grants nothing and, on removal, invalidates nothing. This is long-standing backend behavior ([easycla#3081](https://github.com/linuxfoundation/easycla/pull/3081) added the lists), not new — the Corporate Console has always offered the criterion on the same terms, and the Self Serve approval list ([lfx-self-serve#2257](https://github.com/linuxfoundation/lfx-self-serve/pull/2257)) inherits it by exposing the same producer field. Carried over from M2, which recorded the same limitation. | needs filing against the backend — a group entry must either be evaluated or be rejected at the API. Until then the Org lens should not offer **GitLab group**, since a stored-but-inert criterion reads as working coverage. GitLab *username* is unaffected and stays supported |
| **GitLab-org removal iterates the wrong acknowledgment set** | `UpdateApprovalList` mutates one shared `ApprovalList`. The GitHub-org removal branch now loads its own removed-org ECLAs and assigns `.ECLAs` explicitly before invalidating, so a standalone GitHub-org removal sweeps correctly. The GitLab-org branch still never assigns `.ECLAs`, so a standalone GitLab-org removal sweeps nothing; combined with a domain removal in the same request it sweeps the domain-derived set under the GitLab-org criterion instead. | needs filing — backend bug scoped to the GitLab-org branch; it must load the acknowledgment set the removed GitLab org selects before calling `invalidateSignatures`, matching the GitHub-org branch's fix |
| **An invalidated CCLA disappears silently** | The entry vanishes from the list, so an admin cannot tell a never-signed CLA group from one whose agreement was voided. | open product question |
| **Sanctioned + never signed is not listable** | Only the **Sign CLA** preview can show it, and `GET /v4/cla-group/search` carries no `sanctioned` field, so the preview must resolve the sanctions state separately. | needs filing — add the flag to the search result, or have the preview resolve the company |
| **Invalidating existing acknowledgments on sanction is blocked** | A newly sanctioned company keeps **Authorized** acknowledgment rows while every write is refused. | blocked pending [lfx-self-serve#2051](https://github.com/linuxfoundation/lfx-self-serve/issues/2051) |
| **`needsClaManager` and `autoCreateECLA` are flags, not statuses** | Both are on the list entry and render as their own affordances on the card; they do not participate in the status model. | done — out of scope |

## Related tickets

- [lfx-self-serve#2149](https://github.com/linuxfoundation/lfx-self-serve/issues/2149) — org CLA groups list (`signed`, `signedOn`, `sanctioned`, counts)
- [lfx-self-serve#2231](https://github.com/linuxfoundation/lfx-self-serve/issues/2231) — `signedBy` on the list entry
- [lfx-self-serve#2222](https://github.com/linuxfoundation/lfx-self-serve/issues/2222) — `approvalCriteriaCount`
- [lfx-self-serve#2150](https://github.com/linuxfoundation/lfx-self-serve/issues/2150) — self-serve corporate signature (the *Sign CLA* path)
- [lfx-self-serve#2151](https://github.com/linuxfoundation/lfx-self-serve/issues/2151) — CLA manager requests + acknowledgment invalidate (`reason` enum)
- [lfx-self-serve#2153](https://github.com/linuxfoundation/lfx-self-serve/issues/2153) — sanctioned-company write gating
- [lfx-self-serve#2186](https://github.com/linuxfoundation/lfx-self-serve/issues/2186) — email removal invalidates all matching acknowledgments
- [lfx-self-serve#2051](https://github.com/linuxfoundation/lfx-self-serve/issues/2051) — invalidating existing acknowledgments when a company becomes sanctioned
