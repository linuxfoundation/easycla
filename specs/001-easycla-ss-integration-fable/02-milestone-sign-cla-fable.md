# Milestone 2 — Sign-CLA entry + My CLAs actions in Self Serve (hands off to Contributor Console)

**Status**: Implemented (dark-launched behind the `my-clas-m2-enabled` flag) — implementation artifacts in [m2-sign-cla-handoff/](m2-sign-cla-handoff/) (PR [linuxfoundation/easycla#5144](https://github.com/linuxfoundation/easycla/pull/5144)); delivery epic [linuxfoundation/lfx-self-serve#1229](https://github.com/linuxfoundation/lfx-self-serve/issues/1229)
**Depends on**: M1 (identity mapping, SS `cla` module, My CLAs page) | **Retires**: nothing | **Effort**: M
**Spec**: [spec.md](spec.md) | **Overview**: [00-overview-fable.md](00-overview-fable.md)

## Merge note (2026-09-01)

This document replaces the former **02 (proactive ICLA sign entry)** and **03 (native ECLA signing in SS + Contributor Console retirement)** milestone briefs. M2 was implemented in a smaller, merged scope:

- The proactive sign entry hands off to the Contributor Console's **decision screen**, which covers **both the ICLA and ECLA/corporate paths** — so a separate "sign ECLA in SS" milestone is unnecessary.
- **Native signing in SS is not scheduled.** The Console keeps running the signing ceremony (DocuSign for ICLA, the corporate pre-check/acknowledgement flow for ECLA), and the PR-check remediation link is unchanged (no SSM flip).
- **Contributor Console and `easycla-landing-page` retirement is no longer scheduled within this milestone plan.** The old M3 bundled retirement with native ECLA signing; that framing was dropped as too disruptive (epic [#1229](https://github.com/linuxfoundation/lfx-self-serve/issues/1229), 2026-08-06). Retirement is deferred to a future product decision — the hand-off model keeps the Console load-bearing indefinitely.

Later milestones renumber accordingly: old M4 (Organization lens) → **M3**, old M5 (Project lens) → **M4**, old M6 (K8s V2 API) → **M5**.

## Goal

Extend M1's read-only My CLAs page (Me lens, Profile → CLAs tab) with four additive capabilities, per the [M2 mockup v17 Final](https://github.com/linuxfoundation/easyclav2-migration-planning/blob/main/Mockups/M2/EasyCLA_MyCLAs_Full_Prototype_Final.html):

1. **Sign a CLA** — a proactive, PR-independent entry: search for a project/CLA group/repo and be handed off to the Contributor Console to complete signing (ICLA or ECLA — the Console's decision screen owns that choice).
2. **CLA-manager-routed requests (ECLA only)** — Request approval / Request Removal / Contact CLA Manager, all emailing the resolved CLA managers; **no invalidation writes from SS** (legal decision, 2026-08-13/14).
3. **Status column** — per-row contributor-facing standing (Valid / Needs attention / Invalidated / Revoked), computed by the EasyCLA backend.
4. **Signed-under identity** — each row shows which platform and account the CLA was signed with.

## What was actually built

### Self Serve (lfx-self-serve, `apps/lfx-one`)

- The M1 page lives at `/profile/clas` (Profile-hub tab, renamed "CLAs" in the UI); the whole M2 overlay is dark-launched behind the `my-clas-m2-enabled` LaunchDarkly flag (M1's tab behind `my-clas-enabled`), both fail-open.
- **Sign a CLA**: "Sign CLA" button → `ClaGroupSelectComponent` live search (min 3 chars, debounced) via `GET /api/me/clas/sign-options` → routed by the CLA group's linked platforms:
  - **GitHub**: linked-account picker (`SignIdentitySelectComponent`; always shown, even for one account; empty state when none linked) → `POST /api/me/clas/prepare-sign` → full-page navigate to the returned Contributor Console `signUrl`. The server verifies the returned `githubId` matches the chosen account.
  - **Gerrit**: an LF-username confirmation card (Gerrit signs under LF SSO), then a direct browser redirect to the Console's Gerrit route — no BFF call.
  - **GitLab-only groups are blocked** with an explanatory dialog (SS cannot verify a GitLab identity yet) — a documented gap, not a bug.
  - The return hand-off is a host-validated absolute URL back to `/profile/clas`.
- **Manager requests**: one shared `ContactClaManagerComponent` modal and one endpoint for all three request types (`approval` / `removal` / `contact`); managers resolved via `GET /api/me/clas/:signatureId/cla-managers`, message required only for `contact`. ECLA rows only; blocked during impersonation. A `claManager: true` row additionally gets a **"Manage in CCLA Console"** deep link (navigation only).
- **Status pills**: `valid`, `needs_attention` (with a "no longer matches approval criteria" note), `invalidated`, `revoked` (sanctions; read-only — no row actions at all), `unknown` (rendered as an em-dash). Revoked ≠ Invalidated is deliberately enforced (see the [status matrix](https://github.com/linuxfoundation/easycla/pull/5155)).
- **Signed as** line under the Signed date, from the producer's `signedVia`/`signedAs` fields.

### EasyCLA backend (`cla-backend-go`, all under `/cla-service/v4`)

Documented in [docs/MY_CLAS_API.md](https://github.com/linuxfoundation/easycla/blob/dev/docs/MY_CLAS_API.md); M2 added, on top of M1's three read endpoints:

| Endpoint | Purpose |
|---|---|
| `GET /v4/my-clas/{signatureID}/cla-managers` | List the CLA managers of the CCLA covering an owned ECLA |
| `POST /v4/my-clas/{signatureID}/cla-manager-requests` | Email a removal/approval/contact request to selected managers; writes a request record + audit event, **never signature state** |
| `GET /v4/cla-group/search` | Case-insensitive search over CLA group / Salesforce project / linked org names + repo-URL resolution (in-process cache, ~30 min TTL) |
| `POST /v4/self-serve/prepare-sign` | Verifies the identity, creates the EasyCLA user record if missing, records the signing session, returns the Contributor Console hand-off `signUrl` |

Plus the status/metadata work surfaced through `GET /v4/my-clas`: computed `status`/`statusReason`, `invalidatedAt` (`date_invalidated`, [lfx-self-serve#1732](https://github.com/linuxfoundation/lfx-self-serve/issues/1732)), `flagged`/`flaggedAt`/`flaggedCheck` (sanctions screening; `sanctioned_date`, [lfx-self-serve#1370](https://github.com/linuxfoundation/lfx-self-serve/issues/1370)), and `signedVia`/`signedAs` identity stamping.

## Guardrails (held in the implementation)

- Self Serve runs **no signing ceremony** and makes **no signing-initiation calls** (`request-individual-signature`, `request-employee-signature`, …) — `prepare-sign` only prepares the hand-off; the Console calls the signing endpoints exactly as before.
- Self Serve makes **no invalidation writes of any kind**, for either ICLA or ECLA. Removal/invalidation happens in the Corporate Console by a CLA manager, or is system-set by sanctions screening.
- The Console is not cut over or retired; the PR-check remediation link is unchanged.
- DocuSign, webhooks, and PDF handling are untouched.

## What the old M3 scoped that remains unscheduled

The former M3 brief planned to move the Console's corporate flow natively into SS (company search/add, sanctions pre-checks, ECLA record, ICLA chaining, designee + invite-admin flows) and then retire the Contributor Console and `easycla-landing-page`. None of that is scheduled now:

- The corporate flow stays in the Contributor Console, reached via M2's hand-off (or the unchanged PR-check link).
- "Notify my CLA managers" exists in SS in a different shape: M2's Request-approval flow emails managers from the My CLAs row — but the in-Console not-on-list flow also remains.
- Console/landing-page retirement, the email-template re-pointing, and the decommission package move to a **future product decision**; they are not attached to any numbered milestone below.

## Exit criteria (met)

- A contributor can, from Self Serve with no PR context, find a CLA group by project/org/repo search and land in the Contributor Console ready to complete the ICLA or ECLA flow (GitHub and Gerrit; GitLab sign entry blocked by design).
- ECLA rows offer manager-routed approval/removal/contact requests that email the real CLA managers resolved from the CCLA signature ACL; no signature state changes from SS.
- Every row shows the computed status and signed-under identity from `GET /v4/my-clas`.
- No changes to DocuSign, webhook, or PDF handling; no SSM cutover of the PR-check remediation link.
