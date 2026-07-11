<!-- SPDX-License-Identifier: CC-BY-4.0 -->
# Milestone 1: Read-Only ICLA/ECLA Agreements in Me Lens

**Status**: Draft
**Depends on**: nothing functionally, but establishes patterns Milestones 2–5 reuse
**Retires**: nothing (additive)

## User story

As a contributor who has signed one or more CLAs, I want to see all my signed ICLAs and active ECLAs in one place in my Self Serve "Me" lens, so I don't have to remember which project's Contributor Console to visit to check my CLA status. If I want to sign something new, I understand I'll be sent to the existing Contributor Console for now.

## Scope

**In scope:**
- A new "Agreements" (or similarly named) view under the Me lens listing:
  - All ICLAs the user has signed, across all projects/CLA groups.
  - All ECLAs (employee acknowledgments) currently valid for the user, across all companies/projects.
- Read-only: no sign, revoke, or edit actions.
- A visible "Sign a new agreement" affordance that deep-links out to the existing Contributor Console (not a Self Serve feature yet — Milestone 2 replaces this for ICLA).
- Download of the signed PDF for any listed ICLA. Also **download the signed PDF for any CCLA the user can see in this view** if the user happens to be surfaced as party to one (e.g., a CLA Manager viewing their own manager-signed CCLA) — confirmed CCLAs do have signed PDFs, not just ICLAs (see overview §2, R10). If, in practice, this view never surfaces CCLA records to an individual contributor, this is moot; confirm during design.
- ECLAs have **no PDF** to download (confirmed: pure DB record, no DocuSign envelope) — the UI must not offer a download action for ECLA rows.

**Out of scope:**
- Signing anything (Milestone 2, 3).
- CCLA management as a CLA Manager (Milestone 4).
- Any write action.

## Prerequisite: user-identity resolution (R4)

EasyCLA identifies signatures by an internal EasyCLA user id, and separately stores linked GitHub username, GitLab username, LF username, and email per user record. Self Serve identifies the logged-in user via its own auth/session. Before this milestone can list "my" agreements, the BFF needs a reliable way to resolve **Self Serve user → EasyCLA user record(s)**, including the case where a person has multiple linked identities (e.g., signed once with a GitHub-linked account and separately has an LF username-linked record).

This resolution step is designed once here and reused by every later milestone that needs "which EasyCLA records belong to this Self Serve user."

**Recommended approach:** resolve via a shared identifier already present in both systems (email, or LF/SSO identity) rather than inventing a new identity-linking system. Confirm during design whether EasyCLA's existing `/v4` user-lookup endpoints support lookup by the identifier Self Serve's auth exposes; if not, this may require an additive, narrowly-scoped EasyCLA API endpoint.

## Functional requirements

1. The system MUST display, for the logged-in user, every ICLA they have signed across all CLA groups/projects, with project/CLA-group name, signing date, and status.
2. The system MUST display, for the logged-in user, every currently-valid ECLA (employee acknowledgment), with the covering company name, project, and effective date.
3. The system MUST allow downloading the signed PDF for any listed ICLA.
4. The system MUST allow downloading the signed PDF for any listed CCLA the user is entitled to see in this view, if such records appear here (confirm scope during design per R10 above).
5. The system MUST NOT offer a PDF download for ECLA rows, since no such document exists.
6. The system MUST provide a clear path to the existing Contributor Console for signing new agreements, since signing is out of scope for this milestone.
7. The system MUST resolve the logged-in Self Serve user to the correct EasyCLA user record(s) even when the user has multiple linked identities, without showing another person's records or silently omitting the user's own records under a different linked identity.
8. If the user has no signed agreements, the system MUST show a clear empty state rather than an error.

## Data flow

```
Me lens "Agreements" component
  → Self Serve BFF route (new)
    → identity resolution (Self Serve user → EasyCLA user id(s))
    → EasyCLA /v4 signature-lookup endpoint(s) (existing, read-only)
  ← list of ICLA/ECLA records + PDF download links (existing /v4 PDF endpoints)
```

No EasyCLA backend changes are required if existing `/v4` read endpoints already support "list signatures for user X" and "download signed PDF for signature Y" — confirm both exist during design; if a combined "all my agreements across projects" endpoint doesn't exist, this may require a small additive read-only EasyCLA endpoint rather than N calls per project.

## Risks specific to this milestone

| Risk | Mitigation |
|---|---|
| No existing EasyCLA endpoint aggregates "all signatures for this user across all projects/companies" in one call — today's consoles are typically scoped per-project. | Confirm during design; if absent, scope a small additive read endpoint rather than having the BFF fan out to every project. |
| Identity resolution (R4) undercounts or overcounts a user's agreements if linked-identity data is incomplete or inconsistent. | Treat identity resolution as its own testable unit; write test cases for multi-identity users explicitly. |
| Users expect to sign here and are confused by the redirect-out to Contributor Console. | Clear, explicit UI messaging ("to sign a new agreement, continue to the Contributor Console") rather than a silent link. |

## Effort

**M** — mostly new read-only BFF plumbing and the identity-resolution groundwork; UI itself is a straightforward list view. Identity resolution is the long pole.
