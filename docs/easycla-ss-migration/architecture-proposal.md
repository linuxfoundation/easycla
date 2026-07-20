<!-- Copyright The Linux Foundation and each contributor to CommunityBridge.
SPDX-License-Identifier: CC-BY-4.0 -->

# EasyCLA → LFX Self Serve: Architecture Proposal

**Status**: Proposed — for architecture review (not yet decided)
**Owner**: Michal (engineering) | **Date**: 2026-07-20
**Program specs**: [specs/001-easycla-ss-integration-fable/](../../specs/001-easycla-ss-integration-fable/spec.md) (overview, six milestone docs, M1 plan)
**Supporting analysis**: [role-mapping-feasibility.md](role-mapping-feasibility.md)

## What the program is

Migrate all EasyCLA user-facing functionality (Contributor Console, Corporate CLA Console + its GraphQL BFF, PCC EasyCLA module, landing page) into **LFX Self Serve** under its Me / Organization / Project lenses, in six independently shippable milestones (M1 read-only "My CLAs" → M5 project administration), with the backend re-platform (M6: Lambda → Kubernetes V2 service, optional DynamoDB → Postgres) as a **separately gated decision**.

## Already settled at the leadership review (2026-07-15) — context, not up for review

| # | Decision |
|---|---|
| L1 | **UI-first sequencing approved**: M1–M5 build on the existing EasyCLA APIs; M6 gated separately |
| L2 | Target timeline: **Q3 / early Q4 2026** |
| L3 | **M5 is decision-gated**: PCC EasyCLA admin moves to SS *or stays in PCC* — open product decision (Kieran/Manish/Heather) |
| L4 | EasyCLA **landing page retired** (added to the M3 decommission package) |
| L5 | LFID account is a prerequisite for signing (already the status quo in the consoles) |
| L6 | Post-signing redirect to the SS profile (collect GitHub linking *after* signing, no friction before) |

## Proposed architecture (for review)

| # | Proposal | Rationale (short) | Detail |
|---|---|---|---|
| P1 | **Strangler pattern: SS is a new client of the existing EasyCLA v4/v3 APIs.** One SS server-side `cla` module; no business logic reimplemented; enforcement (roles, approval lists, sanctions) stays in EasyCLA until M6 | One source of truth; follows the proven crowdfunding integration shape in SS | [overview §3](../../specs/001-easycla-ss-integration-fable/00-overview-fable.md) |
| P2 | **Roles: bridge, don't migrate.** No CLA object types in OpenFGA before M6. SS gates UI via EasyCLA's own signals (signature-ACL-backed manager list + optimistic v4 calls); every write is still enforced by the gateway/ACS + v4 | An FGA copy would be non-enforcing, with sync lag on an already-async pipeline; verified feasible with user tokens, no gateway or EasyCLA changes | [role-mapping-feasibility.md](role-mapping-feasibility.md) |
| P3 | **Tokens: user-scoped, via SS's existing api-gw-audience refresh-token exchange.** No M2M by default, no token infrastructure work | The whole v4 chain keys on user identity, not client/audience — verified in code; two curl spikes remain (role-less-user read access; per-env Auth0 grant) | [feasibility §4, §7](role-mapping-feasibility.md) |
| P4 | **DocuSign never moves in M2–M4.** SS fetches a `sign_url` from v4 and redirects, exactly as the consoles do today; no DocuSign bridge service | Webhooks, PDF storage, and envelope state already live in `v2/sign`; duplicating them adds risk with no user value | [milestone 02](../../specs/001-easycla-ss-integration-fable/02-milestone-sign-icla-fable.md) |
| P5 | **Cutover per milestone is a config flip** (SSM parameter for the PR-check redirect base; lens feature flags for org/project) — instant rollback | Reversibility is a program success criterion (SC-007) | [overview §3.4](../../specs/001-easycla-ss-integration-fable/00-overview-fable.md) |
| P6 | **All three git platforms in scope** via per-platform sub-milestones (M2a/M3a GitHub, M2b/M3b GitLab, M2c/M3c Gerrit), each with its own cutover switch and parity checklist | Prevents Gerrit/GitLab slipping and blocking console retirement late | [spec.md Q2](../../specs/001-easycla-ss-integration-fable/spec.md) |
| P7 | **The legacy `/v1`/`/v2` Go API surface (`cla-backend-legacy`) stays until M6** and remains in scope for parity/contract testing — contributor flows still call it | Second API codebase inside the blast radius even for "UI-only" milestones; absorbed/retired at M6 | [overview §3, §5](../../specs/001-easycla-ss-integration-fable/00-overview-fable.md) |
| P8 | **Email-based CCLA signatory signing is preserved** — the signatory signs via an emailed DocuSign link and is never forced into SS/LF SSO | Documented product behavior; a distinct UX path that must survive M4 | [milestone 04](../../specs/001-easycla-ss-integration-fable/04-milestone-ccla-org-lens-fable.md) |

## What the review should challenge

1. **P2/P3 rest on two unverified operational facts** — the spikes in [feasibility §7](role-mapping-feasibility.md) (role-less users through the gateway ACS check; SS Auth0 client's audience grant per environment). If spike 2 fails, the fallback is M2M + server-side subject binding — still Option A, slightly less clean.
2. **The hybrid-strangler alternative**: stand up a small CLA read/query V2 service after M2 for M4/M5 to consume. Rejected for now (spreads M6 risk but adds a service before the M6 go/no-go); worth revisiting if M6 is committed early ([spec.md Q3](../../specs/001-easycla-ss-integration-fable/spec.md)).
3. **UX consistency of the role bridge**: CLA grants are async (Salesforce-coupled, 30-min ACS warden cache — revocations linger too) while SS-native grants are near-instant; CLA roles won't appear in SS's People/Access views. Design consequences are tabled in [feasibility §6.1](role-mapping-feasibility.md).
4. **Hardening observation (independent of this program)**: EasyCLA v4 trusts the gateway-injected `X-ACL` header unconditionally and no authorizer/API-key was found on the v4 stack itself — see feasibility §3 and spike 4.
