# M3 Status Matrix — Org lens CLA statuses

Copyright The Linux Foundation and each contributor to CommunityBridge.

SPDX-License-Identifier: CC-BY-4.0

**Parent spec**: milestone brief [03-milestone-ccla-org-lens-fable.md](../specs/001-easycla-ss-integration-fable/03-milestone-ccla-org-lens-fable.md); backend endpoints [M3_ORG_LENS_API.md](M3_ORG_LENS_API.md)
**Companion**: [MY_CLAS_STATUS_MATRIX.md](MY_CLAS_STATUS_MATRIX.md) — the shipped M2 status model for the **Me** lens
**Status of this document**: **the target model.** It describes the statuses the Org lens
should show, not what any build renders. Written against the M3 UI prototype and the M3
backend on `dev` (2026-09-14). Implementation is compared against this document, not the
other way round; where they differ, [Not yet implemented](#not-yet-implemented) records it.

Single source of truth for the statuses the **Org lens** EasyCLA module shows once the
Corporate CLA Console moves into LFX Self Serve. Two surfaces carry a status: the **CLA
entry** (one signing entity × CLA group) and each **employee acknowledgment** row beneath
it. Every status names its **backend basis** — several have none yet.

## CLA entry statuses

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
  unscoped catalog search backed by `GET /cla-group/search`, which carries no signature or
  sanctions fields. Picking an unsigned CLA group there opens a transient detail page with
  managers and approval tabs locked; it is never a row in the org's CLA list and a reload
  loses it. Treating it as a persisted status would imply the list API can return it.
- **Revoked wins over Signed**, exactly as in M2 — the more consequential fact and the more
  restrictive status, since the entry becomes read-only.
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
  {date}* rather than naming the wrong person. **The date does not yet hold to that standard**:
  the list service substitutes `signature_created` when the signature has no `SignedOn`, so
  *Signed on {date}* can present a creation date as a signing date, breaking M2's "a wrong date
  is worse than none" rule. See [Not yet implemented](#not-yet-implemented).

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

## Acknowledgment statuses

Shown per contributor row in the acknowledgments (employee CLA) table of a CLA entry.

| Status | What it means | Backend basis | Dated? |
|---|---|---|---|
| **Authorized** | The employee acknowledged the corporate agreement and the company's approval criteria still cover them. | `signatureSigned = true` **and** `signatureApproved = true` | no |
| **Not Authorized** | The acknowledgment is intact, but the contributor is no longer covered by the approval criteria. Nobody revoked access deliberately — a criterion they matched was removed, or their membership of an approved org or group changed. **Recoverable**: adding them back restores them. | **none today** — no coverage verdict is computed, see [Not yet implemented](#not-yet-implemented) | no |
| **Invalidated** | The acknowledgment itself was made void — deliberately by a CLA manager, or as a side effect of an approval-criterion removal. **Not recoverable** by re-adding the contributor; they must acknowledge again. One exception today: `autoCreateECLA`, below. | `signatureApproved = false` | yes — *Invalidated · date* (not exposed on this row today) |

Rules:

- **Unsigned acknowledgments should never be shown** — a row should exist only once
  `signatureSigned = true`, aligning with M2's filter and retiring today's console state
  **"Not set up"** (`!approved && !signed`). **This is a proposal, not current behavior**:
  unlike the CCLA query, the employee signature query filters only on company and project, so
  unsigned records *are* returned today. See [Not yet implemented](#not-yet-implemented).
- **Not Authorized is recoverable; Invalidated is not.** That distinction is the whole point of
  splitting them: one is coverage drift the company can undo, the other a voided agreement it
  cannot. Copy and severity must not blur them — the prototype renders the first amber, the
  second red.
- **`autoCreateECLA` breaks that rule today.** With auto-create enabled, an approval-list
  update calls `CreateOrUpdateEmployeeSignature` → `ValidateProjectRecord`, which sets
  `signature_approved = true` on every acknowledgment where either flag is false. So an
  **Invalidated** row silently returns to **Authorized** on the next approval-list edit,
  including one a CLA manager invalidated deliberately. The target model treats Invalidated as
  final; see [Not yet implemented](#not-yet-implemented).
- **Invalidated does not name who did it** unless the record says so. New invalidations store
  `InvalidationMetadata` (`InvalidatedBy`, `Reason`, `Note`) and the M3 invalidate endpoint
  takes a `reason` enum plus a free-text `note`, but older records carry nothing — attribution
  is per-record, never assumed. Same constraint as M2, narrower blast radius.

### Which status an acknowledgment row gets

| Condition | In the data | Status |
|---|---|---|
| Not acknowledged | `signatureSigned = false` | *row is not shown* (filtered client-side today) |
| The acknowledgment was voided | `signatureApproved = false` | **Invalidated** |
| Acknowledgment intact, criteria do not cover the contributor | *not computed today* | **Not Authorized** |
| Acknowledgment intact and covered | `signatureApproved = true` | **Authorized** |

### Why Not Authorized is unreachable today

Removing an approval criterion **invalidates matching acknowledgments immediately** for most
criteria — `invalidateSignatures` sets `signature_approved = false` with invalidation metadata
— so the row lands in **Invalidated** rather than in a recoverable state. What **Not
Authorized** is meant to name is the residue: cases where coverage went stale and nothing
recorded it, so the row keeps reading **Authorized**. Those are GitLab group removals
(which invalidate nothing), membership drift in an approved GitHub org or GitLab group (no
approval-list edit occurs, so nothing triggers a re-check), and records the sweep skipped
(missing user record, or a panic recovered per acknowledgment). The specific backend defects
behind each are listed in [Not yet implemented](#not-yet-implemented).

Consequently the prototype's recovery hint — *add the user to the approval list, or Invalidate
to remove for good* — is untruthful for the common case: the acknowledgment is already voided,
so re-adding the criterion does not restore the row. The hint only describes reality where
`autoCreateECLA` is enabled, and there it works by re-approving invalidated records
indiscriminately rather than by any coverage logic. Either the row needs a live coverage
verdict from the backend, or removal must stop auto-invalidating. This is an open product
decision, not a copy fix.

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
| Company under sanctions | **Revoked** (on the ECLA row) | "Unable to Sign" / "Unable to Prepare CCLA" error states | **Revoked** (on the CLA entry) |
| Not acknowledged / not signed | *row hidden* | Not set up | *row hidden* |

Two divergences are load-bearing:

- **"Needs attention" ≡ "Not Authorized".** Same state, different audience — an instruction to
  the contributor versus a fact about a person. Neither name is being changed.
- **"Revoked" sits on a different object in each lens** — one contributor's ECLA in the Me lens,
  the whole signing entity's CLA entry in the Org lens. The sanction is company-level in both;
  only the object carrying the label differs.

## What changes from today's console

| Surface | Corporate CLA Console today | Org lens (M3, target) |
|---|---|---|
| Acknowledgment statuses | Authorized / Not Authorized / Not set up, from two booleans | Authorized / Not Authorized / Invalidated; unsigned rows dropped rather than labelled |
| Voided vs not-covered | Indistinguishable — both read "Not Authorized" | Split, with different colors, copy and recoverability |
| CLA entry status | Signed / Not Signed per project | Signed / Revoked, with unsigned reachable only via the **Sign CLA** preview |
| Sanctions detection | `isSanctioned` plus a substring test for `"sanctioned"` on the error text — `project-active-cla.component.ts` and `ccla-dialog.component.ts` both carry a `TODO(#5078)` to replace it | Typed `code: "company_sanctioned"` on gated write ops; stored `sanctioned` flag on reads; live SSS re-screening on the signing path |
| Sanctions copy | Two titles — *Unable to Sign* and *Unable to Prepare CCLA* — sharing one body string | one state, one wording — **Revoked** on the entry |

## Not yet implemented

A first Org-lens EasyCLA build exists in `lfx-self-serve` behind the `org-lens-cla-m3-enabled`
feature flag (list, card, detail page, approval list, Sign CLA flow). It is not user-visible
and diverges from this document in the first two rows below. Acknowledgment statuses have no UI
there at all, so everything in [Acknowledgment statuses](#acknowledgment-statuses) is unbuilt.

| Gap | Effect in the Org lens | Backend status |
|---|---|---|
| **Shipped build labels the sanctions state "Sanctioned", not "Revoked"** | Card pill reads *Sanctioned*, detail heading reads *Unavailable* — one company-level fact, three words across two lenses. | frontend copy only — rename to **Revoked** |
| **Shipped build derives the entry status from two booleans in the BFF** | `signed` and `sanctioned` are collapsed in the Self Serve server layer, relocating the two-boolean derivation this matrix criticizes. The Me lens consumes an authoritative `status` from the producer. | open — whether the entry status should become a producer-side field |
| **Unsigned acknowledgments are not filtered server-side, and the count disagrees with the page** | Rows with `signatureSigned = false` come back from the API. Worse, the page query filters on company only while `totalCount` also filters `signature_approved` and `signature_signed`, so unsigned and invalidated records consume page slots and cursors while being excluded from the total. Client-side dropping yields short pages and a mismatched count. | needs filing — both queries must select the **same** row set: require `signature_signed = true` in each and **keep** signed rows with `signature_approved = false`, which are the **Invalidated** rows the lens must show. A frontend-only fix is insufficient |
| **`autoCreateECLA` resurrects invalidated acknowledgments** | The target model treats **Invalidated** as final; it is not. A deliberately invalidated row returns to **Authorized** on the next unrelated approval-list edit, with only a `note` recording it. | needs filing — likely backend bug; auto-create should not re-approve invalidated records |
| **`signedOn` may be a creation date** | The list service substitutes `signature_created` when the signature has no `SignedOn`, and nothing flags the difference — breaking M2's "a wrong date is worse than none". | needs filing — omit the field when no signing timestamp exists, or mark it approximate |
| **No coverage verdict on an acknowledgment row** | **Not Authorized** cannot be rendered. `corporate-contributor` carries only `signatureSigned` and `signatureApproved`; no equivalent of the M2 coverage check exists. | needs filing — a per-row coverage verdict, or a documented decision that removal stops auto-invalidating |
| **Invalidation date, reason and actor are not exposed** | **Invalidated** renders without its date or cause, even where `InvalidationMetadata` recorded both. | metadata is stored; the row model exposes none of it |
| **The signing refusal is not machine-readable** | Both CCLA signing gates return a plain error, not the typed `403 company_sanctioned` body the write ops return, so the Org lens would have to string-match — the same fragility as `TODO(#5078)`. | needs filing — return the typed sanctions error from the signing path |
| **Revoked has no date** | The entry shows the status alone; the list entry carries only the boolean `sanctioned`. The Me lens dates it from `flaggedAt`. | needs filing |
| **The Sign CLA search endpoint is undocumented** | `GET /cla-group/search` backs the **Not started** preview but has no section in [M3_ORG_LENS_API.md](M3_ORG_LENS_API.md). | needs filing — document the endpoint |
| **GitLab group removals invalidate nothing** | A contributor removed from an approved GitLab group keeps an **Authorized** row: `verifyUserApprovals` has no `GitlabOrgCriteria` branch, and the GitLab-org block never populates the acknowledgments to iterate. Carried over from M2. | needs filing |
| **Org removals iterate the wrong acknowledgment set** | `UpdateApprovalList` mutates one shared `ApprovalList`; only the domain block assigns `ECLAs` and neither org block clears it. An org removal alone sweeps nothing; combined with a domain removal in the same request it sweeps the domain-derived set under the org criterion — the only way the GitHub-org branch runs at all. | needs filing — backend bug; the fix has two halves. Isolating per-block state stops the wrong-set sweep but **alone makes a standalone org removal invalidate nothing**, since the org blocks never load acknowledgments. They must also load the set the removed org selects before calling `invalidateSignatures` |
| **GitHub-org removal over-invalidates** | When it runs, the `GitHubOrgCriteria` branch checks only the email and GitHub-username lists instead of the full `userStillApproved` re-check, comparing case-sensitively against a single `getBestEmail(user)`. A contributor still covered by a domain rule, a GitLab username, a differently-cased entry or a secondary email is invalidated anyway. | needs filing — backend bug, not a display gap |
| **An invalidated CCLA disappears silently** | The entry vanishes from the list, so an admin cannot tell a never-signed CLA group from one whose agreement was voided. | open product question |
| **Sanctioned + never signed is not listable** | Only the **Sign CLA** preview can show it, and `GET /cla-group/search` carries no `sanctioned` field, so the preview must resolve the sanctions state separately. | needs filing — add the flag to the search result, or have the preview resolve the company |
| **Invalidating existing ECLAs on sanction is blocked** | A newly sanctioned company keeps **Authorized** acknowledgment rows while every write is refused. | blocked pending [lfx-self-serve#2051](https://github.com/linuxfoundation/lfx-self-serve/issues/2051) |
| **`needsClaManager` and `autoCreateECLA` are flags, not statuses** | Both are on the list entry and render as their own affordances on the card; they do not participate in the status model. | done — out of scope |

## Related tickets

- [lfx-self-serve#2149](https://github.com/linuxfoundation/lfx-self-serve/issues/2149) — org CLA groups list (`signed`, `signedOn`, `sanctioned`, counts)
- [lfx-self-serve#2231](https://github.com/linuxfoundation/lfx-self-serve/issues/2231) — `signedBy` on the list entry
- [lfx-self-serve#2222](https://github.com/linuxfoundation/lfx-self-serve/issues/2222) — `approvalCriteriaCount`
- [lfx-self-serve#2150](https://github.com/linuxfoundation/lfx-self-serve/issues/2150) — self-serve corporate signature (the *Sign CLA* path)
- [lfx-self-serve#2151](https://github.com/linuxfoundation/lfx-self-serve/issues/2151) — CLA manager requests + ECLA invalidate (`reason` enum)
- [lfx-self-serve#2153](https://github.com/linuxfoundation/lfx-self-serve/issues/2153) — sanctioned-company write gating
- [lfx-self-serve#2186](https://github.com/linuxfoundation/lfx-self-serve/issues/2186) — email removal invalidates all matching acknowledgments
- [lfx-self-serve#2051](https://github.com/linuxfoundation/lfx-self-serve/issues/2051) — invalidating existing ECLAs when a company becomes sanctioned
