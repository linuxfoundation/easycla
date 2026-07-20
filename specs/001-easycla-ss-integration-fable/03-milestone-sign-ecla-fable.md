# Milestone 3 — Sign ECLAs in Self Serve; retire the Contributor Console

**Status**: Draft | **Depends on**: M2 | **Retires**: Contributor Console | **Effort**: L
**Spec**: [spec.md](spec.md) | **Overview**: [00-overview-fable.md](00-overview-fable.md)

## Goal

The corporate contributor path moves to Self Serve: employees of CCLA-signed companies acknowledge the ECLA in SS; not-on-list and no-CCLA branches (notify managers, become designee, invite admin) work at parity. With both paths in SS — and GitLab/Gerrit covered per Q2 — the Contributor Console is decommissioned.

## Current state (facts)

The console's corporate flow, verified in code:

1. **Company search** `GET /v3/organization/search?companyName=…`; company detail `GET /v3/company/external/{sfid}` (includes persisted `isSanctioned`); "add company" uses `GET /v4/company/lookup?websiteName=…` (Clearbit) then `POST /v4/user/{id}/company`, with **up to 12 retries** waiting for the Salesforce record to materialize.
2. **Pre-check** `POST /v2/check-prepare-employee-signature` (legacy Go backend) returns typed errors: `sanctioned` (live SSS check, authoritative), `missing_ccla`, `ccla_approval_list`.
3. **Happy path**: `POST /v2/request-employee-signature` (legacy Go backend) creates the ECLA — a pure record, **no DocuSign** — then `GET /v2/user/{id}/project/{id}/last-signature` decides whether the CLA group's `ccla_requires_icla` forces chaining into the ICLA flow; otherwise redirect to the PR.
4. **Not on list**: route to request-authorization: `GET /v4/company/{companyId}/cla-group/{claGroupId}/cla-managers` → user picks managers → `POST /v4/notify-cla-managers`.
5. **No CCLA**: two sub-flows —
   - *Become CLA manager designee* (requires LF login): `POST /v4/company/{companyId}/claGroup/{claGroupId}/cla-manager-designee`, then poll `…/is-cla-manager-designee` **up to 30 times** (ACS/Salesforce role assignment is async), then hand off to the Corporate Console.
   - *Invite someone*: `GET /v4/company/{sfid}/admin` → `POST /v4/user/{id}/invite-company-admin` (contact admin, or name+email a CLA manager candidate).
6. **Roles touched**: `cla-manager-designee` (assigned here), `cla-signatory`/`cla-manager` (assigned downstream when the CCLA gets signed) — all ACS roles, hardcoded in ACS.
7. **Gerrit** has its own entry (`/cla/gerrit/project/{id}/{type}`, `POST /v1/user/gerrit`, LF login mandatory). **GitLab** entry is constructed by `v2/gitlab_sign` on the backend.

## Role-model reality check

The brief flags role differences as the challenge here. Precisely scoped:

- **What M3 actually does with roles**: *assigns* `cla-manager-designee` via the existing v4 endpoint and *reads* CLA manager lists. It does not evaluate CLA roles for UI gating. The EasyCLA backend + ACS keep doing assignment and enforcement — **SS does not need OpenFGA/CLA modeling for M3**.
- **What is genuinely hard**: the *asynchrony* (retry loops against asynchronous ACS role assignment) and the *hand-off target*. Today the designee flow ends by sending the user to the Corporate Console; if M4 isn't done yet, M3 must still hand off to the Corporate Console — an awkward SS→legacy-console hop that argues for keeping M3 and M4 close together on the roadmap, or accepting the hop temporarily.
- `lfx-v2-auth-service` (confirmed): NATS-based identity/user-metadata service — relevant for identity enrichment, **not** a role system. EasyCLA role management lives in **ACS** (roles hardcoded there; assignment via organization-service; EasyCLA v4 is routed through lfx-gateway).

## Scope

### In

1. Corporate path in SS mirroring flows 1–5 above (company search/add, pre-checks incl. sanctions messaging, ECLA record, ICLA chaining, request-authorization, designee + invite flows) — all via existing v2/v3/v4 endpoints; retry/async patterns live in the SS server, not the browser.
2. **Per-platform sub-milestones — M3a (GitHub), M3b (GitLab), M3c (Gerrit)**: all three platforms are in scope; console retirement requires M3c (FR-026). Same cautions as M2's split: verify each platform's cutover switch exists, and keep the sub-milestones in one release train.
3. **Email/notification re-pointing**: manager-notification and invite emails contain console URLs; templates must point at SS.
4. **Decommission package**: redirect stub at the console's domain (bookmarks, old PR comments, docs links), docs updates, then infra teardown — **including retirement of `easycla-landing-page`** (judged redundant at the 2026-07-15 review: users reach the correct flow via the PR-check link directly).

### Out

- CCLA signing / approval-list management (M4 — but see hand-off note above).
- Changing role semantics, ACS, or approval-list evaluation.
- Auto-create-ECLA changes (backend behavior, untouched).

## Design notes

- **All users are logged in** — the console requires LF login for every flow today, so there is no auth delta in M3.
- ECLA creation is a plain API call — the "signing" UX is a consent screen with the agreement text; render the CLA group's current corporate document text as the console does. The acknowledgement records **which CCLA version** it was made under (documented behavior) — the API handles this; don't cache document text across versions.
- Sanctions: never trust the persisted flag alone; the live pre-check is authoritative (mirrors console logic). Enforcements stay server-side; SS only renders outcomes.
- The company-add retry choreography (Clearbit → create → poll Salesforce) belongs in one SS server-side orchestration with sensible timeouts, not client-side retry loops.

## Risks

| Risk | Notes |
|------|-------|
| Designee hand-off to Corporate Console while M4 unfinished | Accept the hop (documented) or reorder/overlap M4; do not block M3 on M4 |
| Gerrit/GitLab parity underestimated | Separate entry contracts and identity behavior; per-platform sub-milestones with their own test plans |
| Email templates missed | Grep the backend's email bodies for console URLs as a checklist item |
| ACS/Salesforce async failures surface as SS bugs | Same failure modes exist today; carry over the retry budgets and messaging, add tracing |
| Long-tail console URLs in old PR comments | EasyCLA posts/edits PR comments with links; old comments keep old URLs → redirect stub is mandatory, not optional |

## Exit criteria

- SC-003: all four contributor journeys pass E2E in SS on staging + production canary; console traffic ~0; console decommissioned with redirect stub live.
- Each platform sub-milestone (M3a–M3c) has a signed-off parity checklist before its cutover.
