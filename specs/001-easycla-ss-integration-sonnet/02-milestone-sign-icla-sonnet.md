<!-- SPDX-License-Identifier: CC-BY-4.0 -->
# Milestone 2: Sign ICLA in Self Serve

**Status**: Draft
**Depends on**: Milestone 1 (identity resolution, BFF seam)
**Retires**: nothing yet — Contributor Console still needed for the individual-vs-corporate choice and ECLA/corporate flows until Milestone 3

## User story

As a contributor who clicks "Signed Agreement Missing" on a GitHub PR (or Gerrit/GitLab equivalent) and is an individual contributor, I want to sign my ICLA directly in Self Serve, without being redirected to a separate Contributor Console app, so the experience is faster and more consistent with the rest of my LFX tools.

## Scope

**In scope:**
- A sign-ICLA flow in the Me lens: user reaches a "sign this agreement" screen (either by following a re-pointed PR link, or from the Milestone 1 agreements list), confirms details, and is redirected to DocuSign to sign.
- After signing, DocuSign redirects back to a Self Serve return URL; Self Serve shows a confirmation and refreshes the agreements list (Milestone 1's view).
- Support for the GitHub, Gerrit, and GitLab entry paths, not GitHub-only (R7) — confirm each provider's current link/redirect mechanics with the Contributor Console before building, since Gerrit in particular has console-specific handling (e.g., a Gerrit login/logout step around signing).
- Re-pointing the "signed agreement missing" deep link (emitted by the Go backend's PR/Gerrit/GitLab status-check code) to Self Serve instead of the Contributor Console, for the individual-contributor path only.

**Out of scope:**
- The individual-vs-corporate choice screen — retained in a (possibly minimal) surviving Contributor Console per the user's note, since corporate contributors still need to decide "individual or corporate" before this flow applies. Confirm during design whether that choice screen can move into Self Serve too, or must remain external for this milestone.
- ECLA signing (Milestone 3).
- CCLA signing/management (Milestone 4).
- Any change to DocuSign integration itself — reuse the existing `/v4` signing API unchanged (see below).

## Why no new DocuSign integration or microservice is needed

Confirmed in code (`cla-backend-go/v2/sign/docusign.go:481-529`): DocuSign envelope creation and the embedded-signing URL are entirely generated server-side by the existing Go backend. The frontend's only job, in both existing consoles, is to redirect the browser to the returned `signUrl` and later handle the `returnUrl` callback. This means:

- Self Serve's BFF calls the same existing `/v4` sign-request endpoint the Contributor Console calls today.
- Self Serve's frontend redirects the browser to the returned `signUrl`, exactly as the Contributor Console does.
- **No new DocuSign integration, credentials, or microservice is needed in Self Serve or Kubernetes.** The "should we build a small DocuSign-bridging service" question from the original brief is answered: no — that would duplicate working logic and add an unnecessary moving part, especially since Milestone 6 will restructure this same code anyway.

## Functional requirements

1. The system MUST let an individual contributor initiate ICLA signing from Self Serve without visiting the Contributor Console.
2. The system MUST call the existing EasyCLA `/v4` sign-request endpoint to obtain a DocuSign `signUrl`, and redirect the browser to it.
3. The system MUST handle the DocuSign return-URL callback and show the user a clear success/failure state.
4. The system MUST support this flow for GitHub-, Gerrit-, and GitLab-originated "signed agreement missing" links, matching each provider's existing parity requirements (R7) — not just GitHub.
5. The Go backend's PR/Gerrit/GitLab-comment link target MUST become configurable (or otherwise support a staged rollout) so it can point at Self Serve for individual contributors while other flows (corporate choice, ECLA) still point at the Contributor Console until their respective milestones land (R6).
6. After signing, the system MUST reflect the newly-signed ICLA in the Milestone 1 read-only agreements view without requiring a manual refresh workaround (e.g., cache-bust or re-fetch on return).
7. If the DocuSign signing session fails, expires, or is abandoned, the system MUST allow the user to retry without duplicate envelope creation, consistent with existing Contributor Console behavior.

## The deep-link / return-URL contract (R6)

This is the least visible but most consequential piece of this milestone. Today, a PR/Gerrit/GitLab check that finds a missing signature emits a link containing project and user identifiers pointing at the Contributor Console (e.g., `.../#/cla/project/{projectId}/user/{userId}`). Moving the ICLA-signing entry point into Self Serve means:

- The Go backend's link-generation code (in the PR-status-check / Gerrit-comment / GitLab-note logic) needs to conditionally emit a Self Serve URL for the individual-ICLA case.
- The DocuSign `returnUrl` passed when creating the envelope needs to point back to a Self Serve route instead of (or in addition to) the Contributor Console's.
- This is a **coordination point across repos** (easycla's Go backend + Self Serve), not a frontend-only change, and needs a staged/flagged rollout so it can be reverted per-provider or per-project if issues surface.

## Risks specific to this milestone

| Risk | Mitigation |
|---|---|
| Gerrit's console-specific login/logout handling around signing is missed, breaking parity for Gerrit-based projects. | Explicitly test the Gerrit path during design and build, not just GitHub (R7). |
| Changing the PR-comment link target is a platform-wide, externally-visible change (contributors across many projects see this link) — a bug here is high-blast-radius. | Stage the rollout (e.g., behind a flag, cohort, or project allowlist) rather than a single global cutover. |
| The individual-vs-corporate choice screen's fate is ambiguous — if it must stay external, the flow has an awkward hop (Self Serve → mini Contributor Console → back to Self Serve) that could confuse users. | Resolve this explicitly during design before building; it affects the UX shape of this milestone materially. |

## Effort

**M** — the signing call itself reuses existing APIs, but the deep-link/return-URL coordination and multi-provider (GitHub/Gerrit/GitLab) parity work are real, cross-repo effort.
