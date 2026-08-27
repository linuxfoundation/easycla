# M2 Status Matrix — My CLAs row status

**Parent spec**: FR-010 in `spec.md`, which lands with [easycla#5144](https://github.com/linuxfoundation/easycla/pull/5144) — **not yet on `dev`**, so it sits beside this file only once that PR merges ([read it there](https://github.com/linuxfoundation/easycla/blob/docs/easycla-ss-m2-speckit/specs/001-easycla-ss-integration-fable/m2-sign-cla-handoff/spec.md) meanwhile)
**Last verified against shipped code**: 2026-08-27

Single source of truth for the status a My CLAs row shows. The status model is **shipped and settled** — the backend status fields (`easycla` `dev`, including [easycla#5156](https://github.com/linuxfoundation/easycla/pull/5156)) and the frontend rendering ([lfx-self-serve#1440](https://github.com/linuxfoundation/lfx-self-serve/pull/1440), merged 2026-08-21). [Not yet implemented](#not-yet-implemented) lists everything a contributor cannot do or see today, so nobody builds to it by mistake.

## The five statuses

| Status | Applies to | What it means | Dated? | What the contributor can do |
|---|---|---|---|---|
| **Valid** | ICLA + ECLA | Signed and in force. For an ECLA, the employer's agreement covers the contributor. | no | ICLA: download the PDF. ECLA: Contact CLA Manager, Request Removal |
| **Needs attention** | ECLA only | The agreement is intact, but a completed check proved it does **not** cover the contributor — they are no longer on the employer's approved list. | no | Request approval, Contact CLA Manager, Request Removal |
| **Invalidated** | ICLA + ECLA | The agreement itself was made void — by a CLA manager removing the contributor from an approved list, a project admin invalidating an ICLA, or a CLA group being deleted. In the data this is `signature_approved = false`. | yes — *Invalidated · date* | ICLA: nothing. ECLA: Request Removal |
| **Revoked** | ECLA only | The employer is under a sanctions block, so the agreement cannot be relied on. Set by the system, never by a person in the product. | not yet — see the date rules below | Nothing — the row is read-only |
| **—** | ECLA only | Coverage could not be confirmed. Shown as a plain dash, **not** a labelled pill, because this is an absence of information rather than a verdict. | no | Request Removal |

Rules that hold across the table:

- **Unsigned agreements are never shown.** A row only exists once the contributor has signed — in the data, `signature_signed = true`. Records with `signature_signed = false` are filtered out before any status is worked out, so they have no status at all rather than a hidden one.
- **Invalidated and Revoked must never share wording.** They are different situations: Revoked is about the employer being sanctioned, Invalidated is about the agreement being voided. Conflating them would tell a contributor their company is sanctioned when it is not.
- **"Canceled" and "Invalid" are banned copy.** "Invalidated" is the approved term.
- **A date is shown only when a real one was recorded.** Invalidated reads its date from `invalidatedAt` — the stored `date_invalidated`, absent on records invalidated before the field existed, so those rows show the status on its own. A wrong date is worse than none.
- **Revoked cannot be dated from `flaggedAt` as it stands.** `flaggedAt` is stamped with the current time the first time a live screen flags an employer (`persistLiveSanction`), so it means *when EasyCLA first noticed*, not when the employer was sanctioned. An employer already on a sanctions list before that code shipped gets the date of the first screen after deploy, and nothing on the wire distinguishes that from a genuine one. Presenting it as *Revoked · date* would state a revocation date the system does not hold. Either relabel the sub-line to what the value means, or leave Revoked undated — a product call, not an implementation detail.

### Why Invalidated never names who did it

Three different events produce Invalidated, and the stored record is **identical** for all three — it does not capture which one occurred. So the status deliberately attributes nothing.

This is a constraint, not a copy choice: wording like "your CLA manager removed you" would be wrong whenever the cause was actually a project admin or a deleted CLA group. Naming a cause requires a backend change first.

### Revoked wins over Invalidated

If an agreement is both invalidated *and* the employer is sanctioned, the row shows **Revoked**. The sanction is the more consequential fact and Revoked is the more restrictive status — it offers no actions at all — so it takes precedence.

## Which status a row gets

Read top to bottom; the first matching row wins.

| Condition | In the data | Status |
|---|---|---|
| Not signed | `signature_signed = false` | *row is not shown* |
| Employer is sanctioned | company `is_sanctioned = true` | **Revoked** |
| The agreement was voided | `signature_approved = false` | **Invalidated** |
| It is an ICLA | no employer on the signature | **Valid** |
| Coverage could not be checked | — | **—** |
| Coverage confirmed | — | **Valid** |
| Coverage checked, contributor not covered | — | **Needs attention** |

An **ICLA** can therefore only ever be Valid or Invalidated — never Needs attention, Revoked or "—", since all three depend on an employer.

### What lands in "—"

The dash covers several situations that share one property: the check did not reach a conclusion.

- The employer or their corporate agreement could not be read.
- The approved-list check itself failed.
- The employer has **no active corporate agreement**. This one is a definitive answer rather than a failure, and belongs under Needs attention instead — see [Not yet implemented](#not-yet-implemented).

A failed sanctions check does **not** land here. If screening is unavailable the last known answer is used, so the row still resolves normally.

## Actions in detail

Actions are driven by the underlying situation, not by the status label:

| Action | Offered when |
|---|---|
| **Request approval** | ECLA, and the contributor is specifically off the approved list |
| **Request Removal** | Any ECLA except Revoked — so Valid, Needs attention, Invalidated and "—" all keep it |
| **Contact CLA Manager** | ECLA showing **Valid or Needs attention**. Sends a free-form message to the chosen managers and changes nothing — the email says so explicitly |

Consequences worth stating plainly:

- **ICLA rows never offer a CLA-manager action** — there is no employer or manager involved.
- **Revoked is the only ECLA state with no actions.**
- **Request approval is deliberately withheld** from a row whose employer has no active agreement. There would be nothing to be approved onto, so the button would send the contributor to a manager who cannot help.

Contact is offered only where a manager could actually help. A **Valid** row has nothing wrong, but the contributor may still have a question (a team change, an acquisition, another project) — a message-only action fits that. It is withheld from **Invalidated**, because a manager's only lever is the approved list and removal from it produces Needs attention, not Invalidated; and from **"—"**, because the system could not work out what is wrong, and where no company or corporate agreement exists no managers resolve at all.

## Not yet implemented

Everything above describes the intended behavior. These parts are not live for a contributor yet.

**Waiting on the frontend only** — the backend ships the data, the console does not use it yet:

| Gap | What the contributor sees today | Backend status |
|---|---|---|
| **The Invalidated date is not displayed** | Invalidated renders without a date, even where one was recorded. | Done — `invalidatedAt` is on the row ([easycla#5156](https://github.com/linuxfoundation/easycla/pull/5156)); the console's row model does not carry it. Revoked is a separate case and is **not** waiting on the frontend — see the date rules above. |

**Still open:**

| Gap | Effect on the contributor | Tracking |
|---|---|---|
| **GitLab approved-list removals do not invalidate** | A contributor removed from a GitLab approved list keeps a covered-looking row instead of Invalidated. GitLab group membership cannot be checked without per-group tokens, so the row stays unevaluable. | needs filing |
| **"No active corporate agreement" is not distinguished** | It shows as "—" alongside genuine check failures, so a contributor cannot tell a definite problem from a temporary one. It should be **Needs attention** with its own explanation, since the remedy is for the company to sign. | later change |
| **Invalidated offers no way forward** | The contributor sees the status with no suggested next step. The cause is not recorded, so no single contact would be right for every case. Signing a replacement ICLA is **not** blocked yet, so guidance must not point the contributor back at signing — a fresh ICLA would silently undo a deliberate invalidation. | open product question; the block is [lfx-self-serve#1859](https://github.com/linuxfoundation/lfx-self-serve/issues/1859) |
| **`Superseded` is unreachable** | Nothing can produce it — the version a contributor signed is never compared against the current one. Document-version staleness is not handled at all. | later change |

## Related tickets

- [lfx-self-serve#1256](https://github.com/linuxfoundation/lfx-self-serve/issues/1256) — status column
- [lfx-self-serve#1423](https://github.com/linuxfoundation/lfx-self-serve/issues/1423) — status values on the API
- [lfx-self-serve#1370](https://github.com/linuxfoundation/lfx-self-serve/issues/1370) — revocation date (backend done)
- [lfx-self-serve#1732](https://github.com/linuxfoundation/lfx-self-serve/issues/1732) — invalidation date (backend done)
- [lfx-self-serve#1372](https://github.com/linuxfoundation/lfx-self-serve/issues/1372) — Request approval
- [lfx-self-serve#1574](https://github.com/linuxfoundation/lfx-self-serve/issues/1574) — Request Removal
