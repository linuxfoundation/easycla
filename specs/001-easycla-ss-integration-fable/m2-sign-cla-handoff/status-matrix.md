# M2 Status Matrix — My CLAs row status

**Parent spec**: [spec.md](https://github.com/linuxfoundation/easycla/blob/docs/easycla-ss-m2-speckit/specs/001-easycla-ss-integration-fable/m2-sign-cla-handoff/spec.md) (FR-010, in [easycla#5144](https://github.com/linuxfoundation/easycla/pull/5144)) | **Tickets**: [lfx-self-serve#1256](https://github.com/linuxfoundation/lfx-self-serve/issues/1256) (status column), [lfx-self-serve#1423](https://github.com/linuxfoundation/lfx-self-serve/issues/1423) (wire status/reason), [lfx-self-serve#1370](https://github.com/linuxfoundation/lfx-self-serve/issues/1370) (revocation metadata), [lfx-self-serve#1732](https://github.com/linuxfoundation/lfx-self-serve/issues/1732) (ICLA invalidation date)
**Validates against**: [lfx-self-serve#1440](https://github.com/linuxfoundation/lfx-self-serve/pull/1440)
**Created**: 2026-08-20 | **Status**: Draft — open decisions marked ⚠️

This is the single source of truth for which status pill a My CLAs row shows, derived from the underlying data signals. Every mapping below is grounded in verified backend behavior (file:line references in the last section). Engineers implementing #1256/#1423/#1440 and reviewers of those PRs validate against this table.

## Display states (pills)

Four user-facing pills, per the v17 prototype and the 2026-08-19 Slack decision (Heather):

| Pill | Applies to | Meaning | Dated? | Row actions |
|---|---|---|---|---|
| **Valid** | ICLA + ECLA | Signed, approved, (ECLA) covered by the company's current approval criteria | no | ICLA: PDF download. ECLA: Request Removal (#1574) |
| **Needs attention** | ECLA only | Still a live signature, but the contributor no longer matches the company's approval criteria — the next PR check fails | no | Note *"No longer matches {company}'s approval criteria."* + Request approval (#1372) |
| **Invalidated** | ICLA only | An admin invalidated the ICLA (`signature_approved=false`) | `Invalidated · <date>` from #1732; undated on pre-existing records | None (contributor remediation is an open legal question — see decisions) |
| **Revoked** | ECLA only | The signing company is sanctioned (OFAC/SSS or manual admin block) | `Revoked · <date>` — the moment EasyCLA set `is_sanctioned=true` (decided 2026-08-20); recorded by #1370, undated on records sanctioned before it lands | None — read-only: no Request Removal, no Request approval, no download |

Rows with `signature_signed=false` are **never returned** by `GET /v4/my-clas` — an in-progress signing is not a display state (confirmed as intended, Ahmed 2026-08-19). "Canceled"/"Invalid" are banned copy (Mike, 8/13); "Invalidated" was explicitly approved by Heather 2026-08-19.

## Input signals

| Signal | Lives on | Set by |
|---|---|---|
| `signature_signed` | signature | DocuSign completion |
| `signature_approved` | signature | `true` at signing; flipped `false` by (a) PCC admin ICLA invalidation, (b) CLA-manager approval-list removal — **both ICLAs and ECLAs** of the removed user |
| `is_sanctioned` (+ `sanction_origin`) | **company** | Live SSS screen at sign/request entry points, or a manual admin block. Never written to the signature |
| Approval-list coverage (`covered`) | computed live, ECLA only | `false` when: company sanctioned, OR company record missing, OR no approved+signed CCLA, OR user misses the current approval lists (GitLab quirk: defers to `signature_approved`) |
| `date_invalidated` | signature | **Does not exist yet** — #1732 (PCC ICLA path) |
| Revocation timestamp/reason | signature | **Does not exist yet** — #1370 (sanctions + manager-removal paths) |

## Decision table

Evaluation order matters: for ECLA, the sanction check wins over everything else (a sanctioned company's row is Revoked even if the contributor also fails, or still passes, the approval list).

### ICLA (`signature_user_company_id` empty)

| # | `signed` | `approved` | Pill | Wire `status` | Notes |
|---|---|---|---|---|---|
| I1 | false | — | *(row not returned)* | — | By design |
| I2 | true | true | **Valid** | `valid` | PDF download available |
| I3 | true | false | **Invalidated** | `invalidated` | Date from #1732 when present; pre-existing records render the pill undated. Causes: PCC admin invalidation, **or** approval-list removal (see decisions ⚠️) |

ICLA never evaluates coverage — it never emits `needs_attention`, `unknown`, or a listing note (#1423).

### ECLA (`signature_user_company_id` set)

| # | `signed` | `approved` | company sanctioned | covered | Pill | Wire `status` / `statusReason` | Notes |
|---|---|---|---|---|---|---|---|
| E1 | false | — | — | — | *(row not returned)* | — | By design |
| E2 | true | true | no | true | **Valid** | `valid` | Request Removal available |
| E3 | true | true | no | false — approval-list miss | **Needs attention** | `needs_attention` / `not_on_approval_list` | Note + Request approval (#1372 gates on this reason only) |
| E4 | true | true | **yes** | (forced false) | **Revoked** | `revoked` / `sanctioned` ⚠️ | Read-only, dated per #1370. **#1423 currently maps this to `unknown` — must be corrected** (see decisions) |
| E5 | true | **false** | no | — | **Needs attention** ⚠️ | `needs_attention` / `not_on_approval_list` | Proposed, not yet confirmed — see decisions. Cause: manager removed the contributor from the approval list, which flips `approved=false` |
| E6 | true | false | **yes** | — | **Revoked** | `revoked` / `sanctioned` | Sanction wins over E5 |
| E7 | true | true | no | false — company record missing, no approved+signed CCLA, or evaluation/lookup failure | *(no pill)* — render **—** | `unknown` / `unknown` | Per #1423: degrade the row, not the whole request; no invented copy, no Request approval |

## Open decisions ⚠️

1. **E5 — ECLA with `approved=false` (manager removal): proposed Needs attention, needs Heather's confirmation.** Heather stated (2026-08-19) she wasn't aware this state exists; **it does** — approval-list removal invalidates the removed user's ECLA (`signature_approved=false`, verified below). Semantically it is the same situation as E3 ("no longer matches approval criteria") and Request approval is the correct remedy, so Needs attention is proposed. The alternative — a distinct pill — would need new copy and new legal review. Note the backend consequence: `status` cannot be derived from `approved && covered` alone; E5 must map `approved=false` + not-sanctioned to `needs_attention`, not to `invalidated` (which is ICLA-only copy).
2. **I3 second cause — approval-list removal also invalidates ICLAs.** The Slack thread assumed PCC-admin-only. The same removal flow iterates ICLAs and flips `approved=false` on them too (verified below). The pill is the same either way; flagged so nobody "fixes" the copy to say "invalidated by a project admin" — the actor varies.
3. ~~Revoked has no date source today.~~ **Resolved 2026-08-20 (Michal):** the revocation date is **when EasyCLA sets `is_sanctioned=true`** — i.e. `UpdateCompanySanctionStatus(..., true, ...)` must record a timestamp when it flips the flag. #1370 implements recording and exposing it. No such write exists yet; companies sanctioned before #1370 lands render the Revoked pill undated.
4. **#1423 must add a `revoked` status token.** Its current AC maps "employer sanctioned" to `unknown` (render —), which contradicts the decided model ("Revoked is when organization is sanctioned"). The four-token set becomes five: `valid` / `needs_attention` / `invalidated` / `revoked` / `unknown`, with `statusReason: sanctioned` on revoked rows. Ticket needs updating.
5. **Invalidated ICLA — contributor remediation.** Legal question raised by Heather 2026-08-19: what should the contributor do / whom do they contact when their ICLA shows Invalidated? Pending; the pill ships with no action meanwhile. Related known gap: nothing prevents the contributor from simply signing a fresh auto-approved ICLA ([easycla#5154](https://github.com/linuxfoundation/easycla/issues/5154), not M2, deprioritized pending legal).
6. **Backfill.** Records invalidated/revoked before #1732/#1370 carry no dates; both the API and UI must tolerate the missing value and render the pill undated (#1370/#1732 both state this; restated here as the display rule).

## Verified backend facts (as of 2026-08-20, `dev`)

- Unsigned rows filtered: `cla-backend-go/v2/my_clas/service.go:172` (`!sig.SignatureSigned → continue`).
- ICLA validity: `service.go:199-202` — `Valid = SignatureApproved`, `PdfAvailable = true` (ICLA branch only; ECLA rows have no download).
- ECLA validity: `service.go:214-218` — `Valid = SignatureApproved && covered`.
- Coverage evaluation `eclaCoveredByCurrentApprovalList`: `service.go:665-715`. Sanction check is `companyModel == nil || companyModel.IsSanctioned → covered=false` (`:678-680`) — **the sanctioned cause is currently indistinguishable on the wire from an approval-list miss**; exposing the cause is exactly #1423's job. GitLab defer quirk at `:707-712`.
- Sanctions writes: company-level only — `UpdateCompanySanctionStatus` / `ClearCompanySanctionStatusIfSSS` (`company/repository.go:1285-1380`), called from the live SSS screen in `v2/sign/service.go:3098-3111`. A manual admin block (`sanction_origin` ≠ `sss`) is never auto-cleared by SSS. No signature attribute is written.
- Approval-list removal invalidates signatures: `signatures/repository.go:4029-4186` — `invalidateSignatures` (called from `UpdateApprovalList`, five call sites `:3421-3737`) walks the removed user's **ICLAs and ECLAs** and calls `InvalidateProjectRecord` (`:2089`), which sets `signature_approved=false` plus a human-readable `note`. No timestamp is recorded (hence #1732/#1370).
- PCC admin ICLA invalidation: `v2/signatures/service.go:355-423` → same `InvalidateProjectRecord` — `signature_approved=false`, note, contributor email, `InvalidatedSignature` event (event does not carry the signature ID, so the date is unrecoverable for old records — #1732).
