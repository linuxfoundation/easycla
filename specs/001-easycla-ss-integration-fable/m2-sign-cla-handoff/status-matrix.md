# M2 Status Matrix — My CLAs row status

**Parent spec**: [spec.md](spec.md) (FR-010, in [easycla#5144](https://github.com/linuxfoundation/easycla/pull/5144)) | **Tickets**: [lfx-self-serve#1256](https://github.com/linuxfoundation/lfx-self-serve/issues/1256) (status column), [lfx-self-serve#1423](https://github.com/linuxfoundation/lfx-self-serve/issues/1423) (wire status/reason), [lfx-self-serve#1370](https://github.com/linuxfoundation/lfx-self-serve/issues/1370) (revocation metadata), [lfx-self-serve#1732](https://github.com/linuxfoundation/lfx-self-serve/issues/1732) (ICLA invalidation date)
**Created**: 2026-08-20 | **Last verified against shipped code**: 2026-08-24 | **Status**: ✅ = verified shipped behavior; ⚠️ = proposed or unresolved

Single source of truth for which status pill a My CLAs row shows and why. **Both halves have now shipped** — the backend status/reason fields ([easycla](https://github.com/linuxfoundation/easycla) `dev`, `assignMyClaStatus`) and the frontend rendering ([lfx-self-serve#1440](https://github.com/linuxfoundation/lfx-self-serve/pull/1440), merged 2026-08-21).

This document is a **conformance reference**, and it mixes two kinds of statement. Rows marked ✅ record **verified shipped behavior**, with `file:line` references under "Verified backend facts". Rows and items marked ⚠️ are **proposals or open questions that are not settled** — where one exists, the shipped behavior is stated alongside it and is what to implement against today. Nothing here should be read as ratified unless it is marked ✅ or cites shipped code.

## Display states (pills)

| Pill | Applies to | Meaning | Dated? | Row actions |
|---|---|---|---|---|
| **Valid** | ICLA + ECLA | Signed, approved, (ECLA) covered by the company's current approval criteria | no | ICLA: PDF download. ECLA: Request Removal ([#1574](https://github.com/linuxfoundation/lfx-self-serve/issues/1574)) |
| **Needs attention** | ECLA only | Still `signature_approved=true`, but a **completed** check proved the ECLA does not cover the contributor — either they fell off the approval list (`not_on_approval_list`) or the employer has no active CCLA (`no_active_ccla`, not yet built) | no | Note; Request Removal and Contact CLA Manager always; Request approval ([#1372](https://github.com/linuxfoundation/lfx-self-serve/issues/1372)) on `not_on_approval_list` **only** |
| **Invalidated** | ICLA + ECLA | `signature_approved=false` — the signature itself was invalidated externally | ⚠️ **Undated today — no path writes an invalidation timestamp.** `InvalidateProjectRecord` writes only `signature_approved` and `note`. A dated `Invalidated · <date>` for PCC ICLA invalidation is proposed in [#1732](https://github.com/linuxfoundation/lfx-self-serve/issues/1732) (**open**); Approved List removal and CLA-group deletion are not in its scope and would stay undated | ICLA: none. ECLA: Request Removal is retained. Contributor remediation is an open legal question — see decisions |
| **Revoked** | ECLA only | The employer carries `is_sanctioned=true`. That comes from a live SSS screen **or from a manual administrator block** — an admin block (`sanction_origin != "sss"`) is authoritative and short-circuits the live screen entirely, so not every Revoked row is a screening result | `Revoked · <date>` from the company's `sanctioned_date`. ⚠️ Backend only — the merged frontend does not render the date yet ([#1370](https://github.com/linuxfoundation/lfx-self-serve/issues/1370)) | None — read-only; the producer forces `valid` and `claManager` false |
| **—** (`unknown`) | ECLA only | Coverage could not be evaluated — a transient fault, not an answer. Plain text, **not a named pill** | no | Request Removal is retained (it is gated only on "ECLA and not Revoked") |
| **Superseded** | reserved | An older document version than the CLA group's current. **Not produced today** — the endpoint does not expose the current version; the type and pill exist for forward compatibility | no | — |

Rows with `signature_signed=false` are **never returned** by `GET /v4/my-clas`. "Canceled"/"Invalid" are banned copy (Mike, 8/13); "Invalidated" was approved by Heather 2026-08-19.

**Row actions are gated by three independent predicates**, not by the pill (`packages/shared/src/utils/cla-manager-actions.utils.ts`, merged):

| Action | Shipped gate |
|---|---|
| Request approval ([#1372](https://github.com/linuxfoundation/lfx-self-serve/issues/1372)) | ECLA **and** `statusReason === 'not_on_approval_list'` |
| Request Removal ([#1574](https://github.com/linuxfoundation/lfx-self-serve/issues/1574)) | ECLA **and** `status !== 'revoked'` — so Valid, Needs attention, Invalidated and `unknown` all keep it |
| Contact CLA Manager | ECLA **and** `status === 'needs_attention'` |

Every ICLA row therefore carries no CLA-manager action, and Revoked is the only ECLA state with none.

## Input signals

| Signal | Lives on | Set by |
|---|---|---|
| `signature_signed` | signature | DocuSign completion |
| `signature_approved` | signature | `true` at signing; flipped `false` by PCC admin ICLA invalidation, CLA-manager Approved List removal, or CLA-group deletion. Removal invalidates **both ICLAs and ECLAs** of the removed user, but **only for the email, email-domain, GitHub-username and GitHub-org criteria** — GitLab username/org removals flip nothing (see the writers bullet below) |
| `is_sanctioned` / `sanctioned_date` / `sanction_origin` | **company** | Live SSS screen or a manual admin block. Never written to the signature — sanctions do **not** flip `signature_approved` |
| Coverage (`covered`, `unevaluable`) | computed live, ECLA only | `EvaluateUserApproval` against the employer's approved+signed CCLA. Distinguishes a *completed* miss from an *unevaluable* check |

## Decision table

**Evaluation order as shipped** (`assignMyClaStatus`, `v2/my_clas/service.go:1118-1136`): `Flagged` (sanctions) → `!Approved` → ICLA → `unevaluable` → `covered` → default.

### ICLA (`signature_user_company_id` empty)

| # | `signed` | `approved` | Pill | Wire `status` | Notes |
|---|---|---|---|---|---|
| I1 | false | — | *(row not returned)* | — | By design |
| I2 | true | true | **Valid** | `valid` | PDF download available |
| I3 | true | false | **Invalidated** | `invalidated` | Causes: PCC admin invalidation, Approved List removal, or CLA-group deletion. Undated today; [#1732](https://github.com/linuxfoundation/lfx-self-serve/issues/1732) would add a date for the PCC path only |

ICLA only ever produces `valid` or `invalidated` — never `needs_attention` or `unknown`.

### ECLA (`signature_user_company_id` set)

| # | `approved` | employer flagged | coverage | Pill | Wire `status` / `statusReason` | Conforms? |
|---|---|---|---|---|---|---|
| E1 | *(unsigned)* | — | — | *(row not returned)* | — | ✅ |
| E2 | true | no | covered | **Valid** | `valid` | ✅ |
| E3 | true | no | completed miss | **Needs attention** | `needs_attention` / `not_on_approval_list` | ✅ |
| E4 | true | **yes** | (not reached) | **Revoked** | `revoked` | ✅ |
| E5 | **false** | no | (not reached) | **Invalidated** | `invalidated` | ✅ |
| E6 | **false** | **yes** | (not reached) | ⚠️ **unresolved** — ships as **Revoked**; Invalidated proposed | currently `revoked`; proposed `invalidated` | ⚠️ **open** — see decision 1 |
| E7a | true | no | **confirmed** absent: the employer holds no approved+signed CCLA for the CLA group | **Needs attention** | `needs_attention` / `no_active_ccla` | ⚠️ **not built** — ships as `unknown`/"—"; see decision 5 |
| E7b | true | no | unevaluable: company or CCLA record unreadable, approval check failed, or GitLab group membership (never checkable) | **—** | `unknown` / `unknown` | ✅ |

## Open decisions ⚠️

1. **E6 — precedence unresolved; the current pill is Revoked.** The proposal on the table (Michal, 2026-08-24, **not yet ratified with Heather**) is that `signature_approved=false` means "invalidated externally" and should win **regardless of sanction status**. It has not been accepted, so **`revoked` remains the current behavior and the correct thing to implement against today.** The shipped `assignMyClaStatus` checks `row.Flagged` **first**, so an invalidated ECLA at a sanctioned employer renders **Revoked**, not Invalidated. Only E6 differs — E5 (not flagged) already renders Invalidated correctly. Swapping the first two cases in the switch implements the decision. **Worth weighing before changing it:** the shipped order arguably has the stronger safety argument — a sanctioned employer is a compliance fact that should be visible regardless of the signature's flag, and Revoked is the more restrictive pill (no actions at all). Invalidated-at-a-sanctioned-employer is also a rare intersection. Decide explicitly; either way the code and this table must agree.
2. **I3 / E5 second cause — approval-list removal invalidates both ICLAs and ECLAs**, and CLA-group deletion is a third cause. The pill attributes nothing about the actor, which the shipped code documents deliberately. Flagged so nobody "fixes" the copy to name a specific actor.
3. **Revoked date — backend shipped, display not yet.** `sanctioned_date` is stamped on the company at the first live detection (`persistLiveSanction`) and surfaced as `flaggedAt`; the My CLAs path deliberately does not re-stamp it on later listings, so the value stops moving. When no date is stored and stamping fails, the response time is reported instead. **Two caveats before treating `Revoked · <date>` as done:** the merged Self Serve model does not consume `flaggedAt` yet ([#1440](https://github.com/linuxfoundation/lfx-self-serve/pull/1440) ships the pill without a date), and the signing compliance path can still advance `sanctioned_date` on an already-SSS-flagged company (see the backend facts below). Both are worth closing out under [#1370](https://github.com/linuxfoundation/lfx-self-serve/issues/1370).
4. ~~[#1423](https://github.com/linuxfoundation/lfx-self-serve/issues/1423) must add a `revoked` token.~~ **Resolved and shipped** — the wire enum is `valid` / `needs_attention` / `revoked` / `invalidated` / `unknown`, with reasons `not_on_approval_list` / `unknown`. The ticket's original "sanctioned → `unknown`" mapping was superseded.
5. **E7 split — proposed 2026-08-24 (Michal), not yet built.** Copy and the reason token still need Heather's sign-off; until then the shipped `unknown` behavior stands. Today one `unknown` bucket renders "—" for both a lapsed CCLA and a failed lookup. A missing or lapsed CCLA is a **completed check with a definitive answer** — the ECLA covers nothing and the next PR check *will* fail — while a lookup failure is a transient fault where the answer is genuinely unknown. They split as follows:
   - **E7a → `needs_attention` / `no_active_ccla`**, note *"{company}'s corporate agreement is no longer active."* (copy pending Heather's sign-off). **Request approval MUST NOT appear** — the remedy is the company signing a new CCLA, not an approval-list edit, so the [#1372](https://github.com/linuxfoundation/lfx-self-serve/issues/1372) action stays gated on `not_on_approval_list` alone. A Request-approval button here would ask a manager to approve someone onto an agreement that does not exist.
   - **E7b stays `unknown` / `unknown`**, rendered "—". After the split `unknown` means only "we could not check", so a reader of the API knows a retry may change it.

   Implementation notes: `statusReason` gains a third token (`no_active_ccla`); the enum is documented as extensible, so this is additive rather than breaking. The backend change is deeper than an enum value — the missing-CCLA path and the error paths currently converge on the same `unevaluable` signal, so the coverage evaluation must report **why** it could not confirm coverage. Both repos need it (`swagger/common/my-cla.yaml` + `assignMyClaStatus`, then the SS label/note mapping). Known trade-off accepted: **Needs attention** now carries two meanings — an individual approval-list miss and a company-wide lapse — distinguished by the note and the presence of the action. **Follow-up, not an M2 blocker**; current `unknown` behavior is conservative rather than wrong.
6. **Invalidated — contributor remediation.** Legal question raised by Heather 2026-08-19: whom does a contributor contact when their CLA shows Invalidated? Applies to both ICLA and ECLA. Pending; the pill ships with no action. Related gap: nothing prevents signing a fresh auto-approved ICLA ([easycla#5154](https://github.com/linuxfoundation/easycla/issues/5154), not M2, deprioritized pending legal).
7. **`superseded` is reserved but unreachable.** The type and pill exist; nothing produces the status because `GET /v4/my-clas` does not expose the CLA group's current document version. Either wire it up or drop it — a permanently unreachable branch invites the assumption that version staleness is handled when it is not.

## Verified backend facts (`dev`, 2026-08-24)

All references are `cla-backend-go/`.

- Status assignment: `v2/my_clas/service.go:1118-1136` (`assignMyClaStatus`) — order is `Flagged` → `!Approved` → ICLA → `unevaluable` → `covered` → `needs_attention`. Status is set **independently of** `valid`; the swagger notes the two can legitimately disagree.
- Row assembly: `service.go:240-284` — `Valid = SignatureApproved && coverage.covered` (ECLA), `Valid = SignatureApproved` + `PdfAvailable` (ICLA). `Flagged`, `FlaggedCheck`, `FlaggedAt` are ECLA-only.
- Sanctions: `companySanctions` / `persistLiveSanction` `service.go:1080-1113`, screening in `v2/my_clas/sanctions.go`. **An administrator block wins without a screen**: `ScreenCompany` returns `flagged=true, check=stored` immediately when `IsSanctioned && SanctionOrigin != "sss"` (`sanctions.go:87-90`), mirroring `v2/sign`'s `checkCompanyCompliance`. So Revoked means "the company is flagged", not "screening flagged the company". Otherwise: live screen where available, else the stored flag. `sanctioned_date` stamped once at first live detection; `flaggedCheck` reports `live` / `stored` / `unavailable`. Company-level writes only (`company/repository.go:1311-1313`) — no signature attribute is touched.
- Coverage: `evaluateApproval` `service.go:1140-1160`. `EvaluateUserApproval` returns covered, a GitHub-org-lookup-failed flag, and an error; a failed check is `unevaluable`, so `covered=false` never silently means "no longer approved". GitLab group membership cannot be evaluated (needs per-group OAuth), so those rows are `covered=true, unevaluable=true` — deferring to `signature_approved`.
- Coverage resolution: `prefetch.go:288-299` (`claData.coverage`) — **this is where the E7 split lands**. A missing company record, a missing CCLA (`d.cclas[...] == nil`, `:292`) and an unresolved approval entry all return the same `eclaCoverage{unevaluable: true}`. Note the conflation is twofold: `:292` cannot currently distinguish "the employer has no approved+signed CCLA" (E7a, a definitive answer) from "the CCLA lookup did not complete" (E7b), because the prefetch stores a nil for both. Implementing `no_active_ccla` therefore requires the prefetch to record *why* the CCLA is absent, not just that it is.
- Wire contract: `swagger/common/my-cla.yaml:58-77` — `status` enum (five values, documented as extensible) and `statusReason` enum (`not_on_approval_list`, `unknown`). The proposed `no_active_ccla` reason (E7a) is **not in the enum yet**.
- Invalidation writers: `signatures/repository.go` `invalidateSignatures` (Approved List removal; walks the removed user's ICLAs **and** ECLAs → `InvalidateProjectRecord` → `signature_approved=false` + note, no timestamp) and `v2/signatures/service.go` PCC admin ICLA invalidation (same record write, plus contributor email and an `InvalidatedSignature` event that does not carry the signature ID — hence [#1732](https://github.com/linuxfoundation/lfx-self-serve/issues/1732)).
- **Approved List removal does not cover every criterion.** `verifyUserApprovals` (`signatures/repository.go:4111-4186`) branches only on `EmailDomainCriteria`, `GitHubOrgCriteria`, `GitHubUsernameCriteria` and `EmailCriteria`. Removals under the GitLab username and GitLab organization criteria reach `invalidateSignatures` but match no branch, so `InvalidateProjectRecord` is never called and `signature_approved` stays `true`. A contributor removed from a GitLab approval list therefore does **not** land in I3/E5; their row falls through to the coverage check instead — and because GitLab group membership is itself unevaluable (`covered=true, unevaluable=true`), that row keeps rendering **Valid**. Treat this as a backend gap, not as documented behavior.
- Sanction date stability: the My CLAs path is stable — `persistLiveSanction` returns early when the company already carries `is_sanctioned` **and** a `sanctioned_date`, so listings never move the date. The signing compliance path in `v2/sign` is not guarded the same way: `buildSanctionUpdate` stamps `sanctioned_date` on every sanctioned write and its condition (`attribute_not_exists(#S) OR #S = :false OR #O = :o`) admits an already-SSS-flagged company, so a later flagged screen there can advance the date.

## Frontend conformance ([lfx-self-serve#1440](https://github.com/linuxfoundation/lfx-self-serve/pull/1440), merged)

- `ClaStatus` = `valid | needs_attention | revoked | invalidated | unknown | superseded` (`packages/shared/src/interfaces/cla.interface.ts`).
- Labels (`cla-view.utils.ts`): Valid / Needs attention / Revoked / Invalidated / "—" / Superseded. The code explicitly documents that Invalidated and Revoked **must never share a label**, matching this table.
- `unknown` renders as plain text "—", not a named pill — matching E7.
