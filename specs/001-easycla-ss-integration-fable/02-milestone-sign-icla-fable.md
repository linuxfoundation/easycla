# Milestone 2 — Sign ICLAs in Self Serve

**Status**: Draft | **Depends on**: M1 (identity mapping, SS `cla` module) | **Retires**: nothing | **Effort**: M
**Spec**: [spec.md](spec.md) | **Overview**: [00-overview-fable.md](00-overview-fable.md)

## Goal

A contributor clicking "Signed Agreement Missing" on a GitHub PR lands in Self Serve, chooses "Individual contributor", completes the DocuSign ceremony, and returns to a green PR — Contributor Console no longer involved for the individual path.

## Current state (facts)

- The failed status check links to `{CLAContributorv2Base}/#/cla/project/{claGroupID}/user/{userID}?redirect=<PR URL>`. `CLAContributorv2Base` is an SSM parameter per environment → **cutover to SS and rollback are config flips, no code release**.
- Contributor Console individual flow, in order: `GET /v2/user/{id}/active-signature` (legacy Go backend), `POST /v4/request-individual-signature` (Go v4) → response contains a DocuSign **embedded-signing `sign_url`** → `window.open(sign_url, '_self')` → DocuSign redirects the browser to `return_url` (the PR) → DocuSign webhook hits the EasyCLA backend, which stores the signed PDF to S3 and flips the signature to approved → PR check re-evaluates.
- The console **never talks to DocuSign** — the Go `v2/sign` package owns envelopes, webhooks, PDF storage.
- The console requires **LF SSO login for all flows** — the entry dashboard forces `login()` for unauthenticated users (added recently; commit "feat: added lf login"). SS's login requirement therefore introduces no new contributor friction. Historical signatures predating this may lack LF usernames (covered by M1's identity mapping).
- The individual-vs-corporate decision screen is informational (two cards + guidance), gated by the CLA group's `project_icla_enabled` / `project_ccla_enabled` flags (`GET /v2/project/{id}`, legacy Go backend).

## The DocuSign question, answered critically

The brief asked: *"SS will need to talk with DocuSign… maybe introduce a small service in Kubernetes to help integrate SS with DocuSign? Be critical."*

**Recommendation: neither. SS should not integrate with DocuSign at all, and no new bridge service should be built.**

- The premise is off: today's console doesn't integrate with DocuSign either. It calls one EasyCLA endpoint and redirects the browser to the returned `sign_url`. SS can do exactly the same from its Express server.
- A new "DocuSign bridge" service would have to duplicate what `v2/sign` already does — JWT auth to DocuSign, envelope creation from the CLA group's template/version, webhook endpoint + validation, signed-PDF download to S3, signature-record state transitions. That state lives in EasyCLA's DynamoDB; a bridge would either share the tables (tight coupling, two writers) or introduce a second signature store (split-brain). Both are worse than the status quo.
- The only defensible timing for extracting a signing service is **M6**, when the surrounding service is being rewritten anyway — and even then it's an internal module of the new CLA V2 service, not an SS helper.
- Residual risk to verify in design (not a reason for a bridge): `request-individual-signature` authentication. If the v4 endpoint requires an Auth0 audience SS doesn't hold, follow the crowdfunding precedent (dedicated audience/token exchange) or accept SS's M2M with a server-derived user context. Cheap either way.

## Scope

### In

1. **Entry route in SS** accepting the PR-check redirect context (CLA group, user, return URL) — a public/deep-link route that survives the login redirect (context must not be lost through Auth0).
2. **Decision screen** (individual vs corporate) at guidance parity; "corporate" links to the Contributor Console until M3.
3. **Individual flow**: project/CLA-group load, active-signature check, request-individual-signature, redirect to `sign_url`, truthful pending state ("signature processing — your PR check updates automatically") for the webhook-latency window.
4. **Config cutover mechanism**: flip `CLAContributorv2Base` per environment; documented rollback.
5. **Endpoint inventory**: the flow's `/v2` endpoints (`active-signature`, `project`, …) are served by the legacy Go backend (`cla-backend-legacy`) — keep calling them as-is (no port needed). Verify SS's network path to them early: they live on the `api.*` domains via the `cla-backend` stack, and only `/v3`/`/v4` are confirmed routed through lfx-gateway.
6. **Baseline metrics**: instrument Contributor Console completion rate *before* cutover so SC-002 is measurable.

### Out

- Corporate/ECLA path (M3) — the decision screen simply deep-links to the console.
- Any DocuSign/webhook/PDF backend changes.
- Later platform sub-milestones are out of scope for earlier ones (see split below); the console remains reachable for platforms not yet cut over.

## Sub-milestone split (all three platforms in scope)

- **M2a — GitHub**: the volume path; everything in this document.
- **M2b — GitLab**: same DocuSign/return-URL model, but the remediation URL is constructed server-side in `v2/gitlab_sign` and identity arrives via GitLab OAuth — needs its own config switch and entry contract.
- **M2c — Gerrit**: different entry entirely (per-instance links, `POST /v1/user/gerrit` identity, LF login long-standing); no PR status check — cutover means re-pointing Gerrit-facing links/config per instance.

Sequence by traffic (GitHub ≫ GitLab > Gerrit). Two cautions: (1) verify each platform's redirect is **independently switchable** before starting its sub-milestone — GitHub's SSM parameter is confirmed; GitLab's and Gerrit's mechanisms must be verified in `v2/gitlab_sign` and the gerrit-instance config respectively; (2) keep all sub-milestones inside one release train — the console cannot retire until M3c, and every month of split operation is dual-maintenance.

## Parity details from the product documentation

- **Embargo/OFAC attestation**: a mandatory checkbox ("I am not from an embargoed country") gates the Sign button — must exist in the SS signing UI (docs: `embargo-compliance-for-secure-cla-signing.md`).
- **Multi-PR redirect**: after signing, EasyCLA updates only the **earliest open PR**; other PRs re-check via the `/easycla` comment command. Documented behavior — preserve it and say so in the SS completion copy.
- **Manual signing fallback**: CLA templates display a project contact email for offline signing outside DocuSign; the SS page should keep surfacing template contact info.
- ICLA statuses upstream are Active/Incomplete/Disabled/Invalidated — the "pending" state in SS maps to Incomplete.

## Identity: no longer an open question

Earlier drafts flagged anonymous ICLA signing as a product decision. Verified in current console code: LF login is already mandatory for every contributor flow, so SS matches the status quo. Two residual notes: (a) preserve the deep-link context (CLA group, user, return URL) through the OIDC round-trip — the console solves this with local storage around the Auth0 redirect; (b) pre-login-era records may be unlinked from the LF account — M1's mapping and telemetry cover this.

## Risks

| Risk | Notes |
|------|-------|
| Deep-link context lost through Auth0 redirect | Design the entry route around the OIDC round-trip early (state param / server session), it's the fiddliest part of this milestone |
| Webhook latency reads as failure | Pending state + "check your PR in a minute" copy; no polling loops against v4 |
| Split traffic during partial rollout | The SSM flip is all-or-nothing per environment; if gradual rollout is wanted, EasyCLA needs a small change to vary the redirect URL (e.g., per CLA group) — decide whether that's in scope |
| Legacy `/v1`/`/v2` reachability from SS | Endpoints live on the `api.*` domains (cla-backend stack), not confirmed behind lfx-gateway — verify network path/CORS/auth early |
| GitLab/Gerrit cutover switches don't exist yet | Verify per-platform redirect configurability before committing M2b/M2c dates |

## Exit criteria

- SC-002 met on dev/staging + a production canary period, per platform sub-milestone.
- Rollback rehearsed (flip back to console) once per platform switch.
