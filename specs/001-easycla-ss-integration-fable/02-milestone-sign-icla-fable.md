# Milestone 2 — Proactive CLA signing entry in Self Serve (hands off to Contributor Console)

**Status**: Draft | **Depends on**: M1 (identity mapping, SS `cla` module) | **Retires**: nothing | **Effort**: M
**Spec**: [spec.md](spec.md) | **Overview**: [00-overview-fable.md](00-overview-fable.md)

## Revision note (2026-08-04)

Prior drafts of this milestone had SS take over the PR-redirect flow end-to-end (native DocuSign signing in SS, Contributor Console cut over via the `CLAContributorv2Base` SSM flip). Per Heather (PM) on Slack, 2026-08-04, that is **not** the M2 scope:

- **Contributor Console stays as-is.** The PR-check remediation link keeps pointing at the Console; there is no SSM cutover in M2.
- **M2 instead adds a new, additive entry point**: a contributor can proactively start signing from Self Serve — no failing PR required — by picking a CLA Group (and possibly a GitHub org/repo) from a dropdown and choosing ICLA or ECLA.
- **Signing itself still happens in the Contributor Console.** Confirmed by Heather: "I envisioned it to hand off to the existing contributor console flow for this milestone, for both the ICLA and CCLA path." SS's job in M2 is the picker + hand-off, not the signing ceremony.
- Native in-SS signing (DocuSign ceremony run inside SS, replacing the Console) is **not scheduled** — it remains a possible future milestone if/when reuse of M2's picker work makes it worthwhile.

The rest of this document describes the revised scope. Text about a PR-redirect cutover, SSM flip, or native ICLA signing in SS from earlier drafts has been removed or superseded below.

## Goal

A contributor who wants to sign a CLA — without first hitting a failing PR check — can go to Self Serve, pick which CLA Group (and, where relevant, GitHub org/repo) they need to sign for, choose ICLA or ECLA, and be hopped off to the Contributor Console to complete the actual signing there. The PR-check remediation path is untouched: it still points at the Console today, same as before M2.

## Current state (facts)

- The failed status check links to `{CLAContributorv2Base}/#/cla/project/{claGroupID}/user/{userID}?redirect=<PR URL>`. This is unchanged by M2 — no cutover, no SSM flip.
- Contributor Console individual flow, in order: `GET /v2/user/{id}/active-signature` (legacy Go backend), `POST /v4/request-individual-signature` (Go v4) → response contains a DocuSign **embedded-signing `sign_url`** → `window.open(sign_url, '_self')` → DocuSign redirects the browser to `return_url` → DocuSign webhook hits the EasyCLA backend, which stores the signed PDF to S3 and flips the signature to approved.
- The console **never talks to DocuSign** — the Go `v2/sign` package owns envelopes, webhooks, PDF storage. M2 does not change this; SS does not talk to DocuSign either, since SS never reaches the signing step.
- The console requires **LF SSO login for all flows** — matches SS's own login requirement, so no new friction from the hand-off.
- The individual-vs-corporate decision today is gated by the CLA group's `project_icla_enabled` / `project_ccla_enabled` flags (`GET /v2/project/{id}`, legacy Go backend) — the new SS picker reuses the same flags to determine which sign types to offer for a given CLA Group.

## Scope

### In

1. **New SS entry point** (e.g. surfaced from the Me lens — "Sign a CLA"), reachable without any PR context. Not a deep-link/redirect route; a normal authenticated SS page.
2. **Picker UI**: user selects a CLA Group. Where a CLA Group spans multiple GitHub orgs/repos, the picker additionally lets the user narrow to a specific org/repo (exact scope TBD in design — see Open questions).
3. **Sign-type choice**: ICLA or ECLA, gated by the selected CLA Group's `project_icla_enabled` / `project_ccla_enabled` flags (same source as the Console's decision screen).
4. **Hand-off to Contributor Console**: once CLA Group (+ org/repo) and sign type are chosen, SS redirects the user into the existing Console flow to complete signing — for **both the ICLA and ECLA/CCLA paths** (confirmed by Heather). SS does not call `request-individual-signature`, `request-employee-signature`, or any other signing-initiation endpoint itself in M2; the Console does, exactly as it does today.
5. **CLA Group discovery data**: SS needs a way to list CLA Groups (and their org/repo scope) available to present in the dropdown — likely `GET /v2/project/{id}` / CLA-group listing endpoints already used elsewhere (e.g. M1's read-only lens); confirm during design whether a new listing endpoint is needed or an existing one suffices.

### Out

- Any DocuSign/webhook/PDF backend changes.
- Native signing ceremony in SS (no `request-individual-signature` / `request-employee-signature` calls from SS in M2).
- PR-redirect cutover (`CLAContributorv2Base` SSM flip) — out of scope; the Console remains the destination for PR-check remediation links, unchanged.
- Corporate org-selection UX refinement for the CCLA path — Heather flagged this needs revisiting in M3 (see M3 doc); M2's picker should not over-invest in org-selection polish for the CCLA path ahead of that.

## Open questions (for design)

- **Org/repo scope in the picker**: how far does "possibly GitHub org/repo" go for a CLA Group that spans many orgs/repos — full tree picker, flat list, or defer to the Console after CLA-Group selection? Needs a design pass; not resolved by Heather's Slack answer.
- **Hand-off contract**: what context does SS need to pass the Console (CLA group ID, org/repo, sign type, user) so the Console lands the user on the right screen without re-asking? Likely a URL with query params analogous to today's PR-redirect (`?redirect=<...>`), but there is no PR URL to preserve here — needs its own contract.
- **CLA-Group listing source**: confirm which existing endpoint (if any) can enumerate CLA Groups + org/repo scope for a user to browse, or whether this needs new API support from `cla-backend-go`/`cla-backend-legacy`.

## Risks

| Risk | Notes |
|------|-------|
| Hand-off contract undefined | No existing "deep-link into Console for signing, no PR" contract exists — design this explicitly, don't assume the PR-redirect querystring shape works unchanged |
| Org/repo picker scope creep | "Possibly GitHub org/repo" is loosely scoped; cap M2 to what's needed for a working hand-off, defer richer org-selection UX to M3 per Heather's note |
| CLA-Group discovery endpoint may not exist yet | Verify before committing effort estimates; could require new API work, which changes this milestone's size |
| Confusion between two entry points | Contributors reaching SS via the (unchanged) PR-redirect land in the Console directly; contributors reaching SS via the new proactive picker also land in the Console — copy/UX should make it clear these are the same destination, avoid parallel divergent Console entry contracts |

## Exit criteria

- A contributor can, from Self Serve with no PR context, pick a CLA Group (+org/repo where applicable) and ICLA/ECLA, and land in the Contributor Console ready to complete that specific signing flow.
- No changes to DocuSign, webhook, or PDF handling; no SSM cutover of the PR-check remediation link.
