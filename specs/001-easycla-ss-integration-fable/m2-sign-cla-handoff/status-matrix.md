# M2 Status Matrix — My CLAs row status

**Parent spec**: [spec.md](https://github.com/linuxfoundation/easycla/blob/docs/easycla-ss-m2-speckit/specs/001-easycla-ss-integration-fable/m2-sign-cla-handoff/spec.md) (FR-010, in [easycla#5144](https://github.com/linuxfoundation/easycla/pull/5144)) | **Tickets**: [lfx-self-serve#1256](https://github.com/linuxfoundation/lfx-self-serve/issues/1256) (status column), [lfx-self-serve#1423](https://github.com/linuxfoundation/lfx-self-serve/issues/1423) (wire status/reason), [lfx-self-serve#1370](https://github.com/linuxfoundation/lfx-self-serve/issues/1370) (revocation metadata), [lfx-self-serve#1732](https://github.com/linuxfoundation/lfx-self-serve/issues/1732) (ICLA invalidation date)
**Created**: 2026-08-20 | **Last verified against shipped code**: 2026-08-24 | **Status**: Describes shipped behavior; open decisions marked ⚠️

Single source of truth for which status pill a My CLAs row shows and why. **Both halves have now shipped** — the backend status/reason fields ([easycla](https://github.com/linuxfoundation/easycla) `dev`, `assignMyClaStatus`) and the frontend rendering ([lfx-self-serve#1440](https://github.com/linuxfoundation/lfx-self-serve/pull/1440), merged 2026-08-21). This document is therefore a **conformance reference**: it records the intended model, marks where shipped code matches it, and flags the one place it does not.

## Display states (pills)

| Pill | Applies to | Meaning | Dated? | Row actions |
|---|---|---|---|---|
| **Valid** | ICLA + ECLA | Signed, approved, (ECLA) covered by the company's current approval criteria | no | ICLA: PDF download. ECLA: Request Removal ([#1574](https://github.com/linuxfoundation/lfx-self-serve/issues/1574)) |
| **Needs attention** | ECLA only | Still `signature_approved=true`, but a **completed** approval-list check proved the contributor is no longer covered | no | Note + Request approval ([#1372](https://github.com/linuxfoundation/lfx-self-serve/issues/1372)) |
| **Invalidated** | ICLA + ECLA | `signature_approved=false` — the signature itself was invalidated externally | `Invalidated · <date>` from [#1732](https://github.com/linuxfoundation/lfx-self-serve/issues/1732); undated on pre-existing records | None (contributor remediation is an open legal question — see decisions) |
| **Revoked** | ECLA only | The employer is flagged by sanctions screening | `Revoked · <date>` — `sanctioned_date`, stamped at first detection | None — read-only; the producer forces `valid` and `claManager` false |
| **—** (`unknown`) | ECLA only | Coverage could not be evaluated. Plain text, **not a named pill** | no | None |
| **Superseded** | reserved | An older document version than the CLA group's current. **Not produced today** — the endpoint does not expose the current version; the type and pill exist for forward compatibility | no | — |

Rows with `signature_signed=false` are **never returned** by `GET /v4/my-clas`. "Canceled"/"Invalid" are banned copy (Mike, 8/13); "Invalidated" was approved by Heather 2026-08-19.

## Input signals

| Signal | Lives on | Set by |
|---|---|---|
| `signature_signed` | signature | DocuSign completion |
| `signature_approved` | signature | `true` at signing; flipped `false` by PCC admin ICLA invalidation, CLA-manager approval-list removal (**both ICLAs and ECLAs** of the removed user), or CLA-group deletion |
| `is_sanctioned` / `sanctioned_date` / `sanction_origin` | **company** | Live SSS screen or a manual admin block. Never written to the signature — sanctions do **not** flip `signature_approved` |
| Coverage (`covered`, `unevaluable`) | computed live, ECLA only | `EvaluateUserApproval` against the employer's approved+signed CCLA. Distinguishes a *completed* miss from an *unevaluable* check |

## Decision table

**Evaluation order as shipped** (`assignMyClaStatus`, `v2/my_clas/service.go:1118-1136`): `Flagged` (sanctions) → `!Approved` → ICLA → `unevaluable` → `covered` → default.

### ICLA (`signature_user_company_id` empty)

| # | `signed` | `approved` | Pill | Wire `status` | Notes |
|---|---|---|---|---|---|
| I1 | false | — | *(row not returned)* | — | By design |
| I2 | true | true | **Valid** | `valid` | PDF download available |
| I3 | true | false | **Invalidated** | `invalidated` | Causes: PCC admin invalidation, approval-list removal, or CLA-group deletion. Date from [#1732](https://github.com/linuxfoundation/lfx-self-serve/issues/1732) when present |

ICLA only ever produces `valid` or `invalidated` — never `needs_attention` or `unknown`.

### ECLA (`signature_user_company_id` set)

| # | `approved` | employer flagged | coverage | Pill | Wire `status` / `statusReason` | Conforms? |
|---|---|---|---|---|---|---|
| E1 | *(unsigned)* | — | — | *(row not returned)* | — | ✅ |
| E2 | true | no | covered | **Valid** | `valid` | ✅ |
| E3 | true | no | completed miss | **Needs attention** | `needs_attention` / `not_on_approval_list` | ✅ |
| E4 | true | **yes** | (not reached) | **Revoked** | `revoked` | ✅ |
| E5 | **false** | no | (not reached) | **Invalidated** | `invalidated` | ✅ |
| E6 | **false** | **yes** | (not reached) | **Invalidated** *(per the 2026-08-24 decision)* | `invalidated` | ❌ **ships as `revoked`** — see decision 1 |
| E7 | true | no | unevaluable | **—** | `unknown` / `unknown` | ✅ but see decision 5 |

## Open decisions ⚠️

1. **E6 — shipped precedence contradicts the 2026-08-24 decision.** You decided that `signature_approved=false` means "invalidated externally" and should win **regardless of sanction status**. The shipped `assignMyClaStatus` checks `row.Flagged` **first**, so an invalidated ECLA at a sanctioned employer renders **Revoked**, not Invalidated. Only E6 differs — E5 (not flagged) already renders Invalidated correctly. Swapping the first two cases in the switch implements the decision. **Worth weighing before changing it:** the shipped order arguably has the stronger safety argument — a sanctioned employer is a compliance fact that should be visible regardless of the signature's flag, and Revoked is the more restrictive pill (no actions at all). Invalidated-at-a-sanctioned-employer is also a rare intersection. Decide explicitly; either way the code and this table must agree.
2. **I3 / E5 second cause — approval-list removal invalidates both ICLAs and ECLAs**, and CLA-group deletion is a third cause. The pill attributes nothing about the actor, which the shipped code documents deliberately. Flagged so nobody "fixes" the copy to name a specific actor.
3. ~~Revoked has no date source.~~ **Resolved and shipped.** `sanctioned_date` is stamped on the company at the first live detection (`persistLiveSanction`) and surfaced as `flaggedAt`; it is deliberately not re-stamped on later listings so the displayed date stops moving. When no date is stored and stamping fails, the response time is reported instead.
4. ~~[#1423](https://github.com/linuxfoundation/lfx-self-serve/issues/1423) must add a `revoked` token.~~ **Resolved and shipped** — the wire enum is `valid` / `needs_attention` / `revoked` / `invalidated` / `unknown`, with reasons `not_on_approval_list` / `unknown`. The ticket's original "sanctioned → `unknown`" mapping was superseded.
5. **E7 — `unknown` still merges a lapsed CCLA with a lookup failure.** A missing or lapsed CCLA is a **real business state** (the next PR check *will* fail) while a lookup failure is a transient fault, yet both render "—". Recommendation: split them, giving the lapsed-CCLA case **Needs attention** with a distinct note (*"{company}'s corporate agreement is no longer active."*) and **no Request approval** — the remedy is the company re-signing the CCLA, not an approval-list change — and leaving only genuine failures as `unknown`. That needs a new `statusReason` token (`no_active_ccla`) plus Heather's copy sign-off. Counter-argument for leaving it: "—" is honest about a state the contributor cannot act on anyway, and a new reason token means another wire change. **Not blocking M2**; raise as a follow-up.
6. **Invalidated — contributor remediation.** Legal question raised by Heather 2026-08-19: whom does a contributor contact when their CLA shows Invalidated? Applies to both ICLA and ECLA. Pending; the pill ships with no action. Related gap: nothing prevents signing a fresh auto-approved ICLA ([easycla#5154](https://github.com/linuxfoundation/easycla/issues/5154), not M2, deprioritized pending legal).
7. **`superseded` is reserved but unreachable.** The type and pill exist; nothing produces the status because `GET /v4/my-clas` does not expose the CLA group's current document version. Either wire it up or drop it — a permanently unreachable branch invites the assumption that version staleness is handled when it is not.

## Verified backend facts (`dev`, 2026-08-24)

All references are `cla-backend-go/`.

- Status assignment: `v2/my_clas/service.go:1118-1136` (`assignMyClaStatus`) — order is `Flagged` → `!Approved` → ICLA → `unevaluable` → `covered` → `needs_attention`. Status is set **independently of** `valid`; the swagger notes the two can legitimately disagree.
- Row assembly: `service.go:240-284` — `Valid = SignatureApproved && coverage.covered` (ECLA), `Valid = SignatureApproved` + `PdfAvailable` (ICLA). `Flagged`, `FlaggedCheck`, `FlaggedAt` are ECLA-only.
- Sanctions: `companySanctions` / `persistLiveSanction` `service.go:1080-1113`. Live screen where available, else the stored flag. `sanctioned_date` stamped once at first live detection; `flaggedCheck` reports `live` / `stored` / `unavailable`. Company-level writes only (`company/repository.go:1311-1313`) — no signature attribute is touched.
- Coverage: `evaluateApproval` `service.go:1140-1160`. `EvaluateUserApproval` returns covered, a GitHub-org-lookup-failed flag, and an error; a failed check is `unevaluable`, so `covered=false` never silently means "no longer approved". GitLab group membership cannot be evaluated (needs per-group OAuth), so those rows are `covered=true, unevaluable=true` — deferring to `signature_approved`.
- Wire contract: `swagger/common/my-cla.yaml:58-77` — `status` enum (five values, documented as extensible) and `statusReason` enum (`not_on_approval_list`, `unknown`).
- Invalidation writers: `signatures/repository.go` `invalidateSignatures` (approval-list removal; walks the removed user's ICLAs **and** ECLAs → `InvalidateProjectRecord` → `signature_approved=false` + note, no timestamp) and `v2/signatures/service.go` PCC admin ICLA invalidation (same record write, plus contributor email and an `InvalidatedSignature` event that does not carry the signature ID — hence [#1732](https://github.com/linuxfoundation/lfx-self-serve/issues/1732)).

## Frontend conformance ([lfx-self-serve#1440](https://github.com/linuxfoundation/lfx-self-serve/pull/1440), merged)

- `ClaStatus` = `valid | needs_attention | revoked | invalidated | unknown | superseded` (`packages/shared/src/interfaces/cla.interface.ts`).
- Labels (`cla-view.utils.ts`): Valid / Needs attention / Revoked / Invalidated / "—" / Superseded. The code explicitly documents that Invalidated and Revoked **must never share a label**, matching this table.
- `unknown` renders as plain text "—", not a named pill — matching E7.
