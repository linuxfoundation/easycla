# M2 Status Matrix — My CLAs row status

**Parent spec**: FR-010 in `spec.md`, which lands with [easycla#5144](https://github.com/linuxfoundation/easycla/pull/5144) — **not yet on `dev`**, so it sits beside this file only once that PR merges ([read it there](https://github.com/linuxfoundation/easycla/blob/docs/easycla-ss-m2-speckit/specs/001-easycla-ss-integration-fable/m2-sign-cla-handoff/spec.md) meanwhile)
**Last verified against shipped code**: 2026-08-25

Single source of truth for the status a My CLAs row shows. Everything below is **shipped in both repos and settled** — the backend status fields (`easycla` `dev`) and the frontend rendering ([lfx-self-serve#1440](https://github.com/linuxfoundation/lfx-self-serve/pull/1440), merged 2026-08-21). Where a future change is intended, it is called out as such so nobody builds to it by mistake.

## The five statuses

| Status | Applies to | What it means | Dated? | What the contributor can do |
|---|---|---|---|---|
| **Valid** | ICLA + ECLA | Signed and in force. For an ECLA, the employer's agreement covers the contributor. | no | ICLA: download the PDF. ECLA: Request Removal |
| **Needs attention** | ECLA only | The agreement is intact, but a completed check proved it does **not** cover the contributor — they are no longer on the employer's approved list. | no | Request approval, Contact CLA Manager (not working yet), Request Removal |
| **Invalidated** | ICLA + ECLA | The agreement itself was made void — by a CLA manager removing the contributor from an approved list, a project admin invalidating an ICLA, or a CLA group being deleted. In the data this is `signature_approved = false`. | no — see [Known limitations](#known-limitations) | ICLA: nothing. ECLA: Request Removal |
| **Revoked** | ECLA only | The employer is under a sanctions block, so the agreement cannot be relied on. Set by the system, never by a person in the product. | no — see [Known limitations](#known-limitations) | Nothing — the row is read-only |
| **—** | ECLA only | Coverage could not be confirmed. Shown as a plain dash, **not** a labelled pill, because this is an absence of information rather than a verdict. | no | Request Removal |

Three rules that hold across the table:

- **Unsigned agreements are never shown.** A row only exists once the contributor has signed — in the data, `signature_signed = true`. Records with `signature_signed = false` are filtered out before any status is worked out, so they have no status at all rather than a hidden one.
- **Invalidated and Revoked must never share wording.** They are different situations: Revoked is about the employer being sanctioned, Invalidated is about the agreement being voided. Conflating them would tell a contributor their company is sanctioned when it is not.
- **"Canceled" and "Invalid" are banned copy** (Mike, 2026-08-13). "Invalidated" is the approved term (Heather, 2026-08-19).

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
- The employer has **no active corporate agreement**. This one is a definitive answer rather than a failure, and the intended end state is to show it as **Needs attention** with its own explanation, since the remedy is for the company to sign a new agreement. That split is a **later change** — today it shows as "—".

A failed sanctions check does **not** land here. If screening is unavailable the last known answer is used, so the row still resolves normally.

## Actions in detail

Actions are driven by the underlying situation, not by the status label:

| Action | Offered when |
|---|---|
| **Request approval** | ECLA, and the contributor is specifically off the approved list |
| **Request Removal** | Any ECLA except Revoked — so Valid, Needs attention, Invalidated and "—" all keep it |
| **Contact CLA Manager** | ECLA showing Needs attention — but the button does not yet send anything, see [Known limitations](#known-limitations) |

Consequences worth stating plainly:

- **ICLA rows never offer a CLA-manager action** — there is no employer or manager involved.
- **Revoked is the only ECLA state with no actions.**
- **Request approval is deliberately withheld** from a row whose employer has no active agreement. There would be nothing to be approved onto, so the button would send the contributor to a manager who cannot help.

## Known limitations

Current gaps. None blocks the status model; each is worth tracking.

| Limitation | Effect on the contributor | Tracking |
|---|---|---|
| **No dates on Invalidated or Revoked** | Neither status shows when it happened. For Revoked a date exists but is not reliable in every case, and for Invalidated no date is recorded at all — so showing one risks displaying a wrong date, which is worse than showing none. | [#1370](https://github.com/linuxfoundation/lfx-self-serve/issues/1370) (Revoked), [#1732](https://github.com/linuxfoundation/lfx-self-serve/issues/1732) (Invalidated) |
| **GitLab approved-list removals have no effect** | A contributor removed from a GitLab approved list is not marked Invalidated. Their row shows Needs attention or "—" instead, and they may still appear covered. A real backend gap — **no ticket yet.** | needs filing |
| **Contact CLA Manager does not send a message** | The contributor picks managers, writes a message and clicks Send — and is told *"Message not sent"*. Nothing reaches the manager. A visible dead end on the one status that most needs a human contact path. | later change |
| **Invalidated offers no way forward** | The contributor sees the status with no next step and no one to contact. Because the cause is not recorded, no single contact would be right. Open product question: leave as-is, or add one line of static guidance such as *"This agreement is no longer active. To contribute to this project, you may need to sign a new CLA."* | open |
| **"No active corporate agreement" is not distinguished** | It currently shows as "—" alongside genuine failures, so a contributor cannot tell a definite problem from a temporary one. | later change |
| **A "Superseded" status exists in the frontend but is unreachable** | Nothing can produce it, because the version of the agreement a contributor signed is not compared against the current one. **Document-version staleness is not handled at all** — do not assume otherwise. | later change |

⚠️ **If a "sign a new CLA" link is ever added to an Invalidated row**, note that nothing currently prevents signing a fresh, automatically-approved ICLA ([easycla#5154](https://github.com/linuxfoundation/easycla/issues/5154)) — which would silently undo a deliberate invalidation. Static guidance is safe; a working Sign button is not, until that is closed.

## Related tickets

- [lfx-self-serve#1256](https://github.com/linuxfoundation/lfx-self-serve/issues/1256) — status column
- [lfx-self-serve#1423](https://github.com/linuxfoundation/lfx-self-serve/issues/1423) — status values on the API
- [lfx-self-serve#1370](https://github.com/linuxfoundation/lfx-self-serve/issues/1370) — revocation date
- [lfx-self-serve#1732](https://github.com/linuxfoundation/lfx-self-serve/issues/1732) — invalidation date
- [lfx-self-serve#1372](https://github.com/linuxfoundation/lfx-self-serve/issues/1372) — Request approval
- [lfx-self-serve#1574](https://github.com/linuxfoundation/lfx-self-serve/issues/1574) — Request Removal
