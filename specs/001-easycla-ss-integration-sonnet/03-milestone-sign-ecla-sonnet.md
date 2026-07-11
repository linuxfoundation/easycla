<!-- SPDX-License-Identifier: CC-BY-4.0 -->
# Milestone 3: Sign ECLA in Self Serve

**Status**: Draft
**Depends on**: Milestone 0 (role adapter), Milestone 1 (identity resolution), Milestone 2 (deep-link/return-URL pattern established)
**Retires**: Contributor Console (candidate, after this milestone reaches parity and burns in)

## User story

As a contributor who works for a company that already has a CCLA in place, when I click "Signed Agreement Missing" on a PR, I want to acknowledge the ECLA directly in Self Serve rather than being sent to the Contributor Console, so I never need to leave the LFX product surface I already use.

## Scope

**In scope:**
- An ECLA acknowledgment flow in the Me lens: user reaches a screen confirming their employer's existing CCLA covers them, and confirms/agrees. Since ECLA has **no DocuSign envelope and no PDF** (confirmed: pure DB record with `auto_create_ecla`), this is a simpler "confirm and record" action, not a document-signing redirect.
- Re-pointing the relevant "signed agreement missing" deep link for the corporate-employee case to Self Serve.
- Introducing CLA-role awareness into Self Serve for the first time: this flow needs to check whether the user's claimed company/employer relationship is valid, and may surface a "request to be added to the approval list" path if the user isn't yet recognized, which in turn may need to notify or route to a CLA Manager/Signatory for that company. This is the first milestone that actually exercises the Milestone 0 role adapter.

**Out of scope:**
- CCLA management itself, i.e. anything a CLA Manager does to configure or maintain a CCLA (Milestone 4).
- Any change to how the CLA Manager/Signatory/Program Manager roles are granted — this milestone only *reads* role/approval-list state via the Milestone 0 adapter, it doesn't manage it.

## Functional requirements

1. The system MUST let a contributor acknowledge (agree to) an ECLA under their employer's existing CCLA directly in Self Serve.
2. The system MUST NOT present a document-signing (DocuSign) step for ECLA, since none exists today — only a confirmation/agreement action.
3. The system MUST NOT offer a PDF download for the resulting ECLA record, consistent with Milestone 1's finding that ECLAs have no PDF.
4. If the contributor's employer/company relationship cannot be automatically confirmed (e.g., they are not yet on the company's approval list), the system MUST provide a clear path forward (e.g., request-to-be-added, or guidance to contact their CLA Manager), matching current Contributor Console behavior — not a dead end.
5. The system MUST re-point the corporate-contributor "signed agreement missing" deep link to Self Serve for this flow, coordinated with the Milestone 2 deep-link mechanism (R6) rather than inventing a second parallel mechanism.
6. The system MUST use the Milestone 0 role adapter for any check involving CLA Manager/Signatory/approval-list state — never infer authorization from Self Serve's generic lens access (R2, hard requirement).
7. The system MUST support this flow for GitHub-, Gerrit-, and GitLab-originated links (R7), matching Milestone 2's provider-parity bar.

## Retiring the Contributor Console

Once this milestone reaches parity and has burned in with real traffic, the Contributor Console can be retired **provided**:
- The individual-vs-corporate choice screen (noted as a possible carve-out in Milestone 2) has either moved into Self Serve or has a confirmed, sustainable home.
- Any Contributor-Console-only edge cases (e.g., a project/provider combination not yet covered) are inventoried and either covered or explicitly accepted as a gap before shutdown.
- **PR/Gerrit/GitLab gating itself is not being retired** — only the UI that contributors use to satisfy that gate. This is a recurring point of confusion (R3) worth restating here since this is the actual console-retirement milestone.

This is a go/no-go decision separate from "the code merged" — recommend a defined parity-verification and burn-in period (e.g., real usage across a representative sample of projects/providers) before decommissioning.

## Risks specific to this milestone

| Risk | Mitigation |
|---|---|
| This is the first milestone to depend on the Milestone 0 role adapter; if that adapter's design has gaps, they surface here first. | Treat Milestone 0 as a hard prerequisite, not a parallel-track nice-to-have; do not start this milestone's role-dependent work until Milestone 0's adapter is validated against at least this milestone's actual needs. |
| The "not yet on the approval list" edge case is where most contributor confusion and support load already originates in the current Contributor Console — a regression here is highly visible. | Explicitly test and design this path, not just the happy path; compare against current Contributor Console support-ticket patterns if available. |
| Retiring the Contributor Console prematurely (before the individual/corporate choice screen has a home) breaks the entry point for all contributors, not just corporate ones. | Make the choice-screen resolution (flagged as open in Milestone 2) a explicit go/no-go gate for retirement, not an afterthought. |

## Effort

**L** — combines a new (if conceptually simpler) confirmation flow with the first real exercise of the role adapter, the approval-list edge case, and the actual decommissioning of a production app used by every corporate contributor across every EasyCLA-enabled project.
