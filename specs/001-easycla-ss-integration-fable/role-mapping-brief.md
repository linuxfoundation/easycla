# Task Brief: Roles/Permissions Mapping Feasibility — EasyCLA ↔ LFX Self Serve

**For**: a fresh Claude Code session (this file is your instruction set — read fully before acting)
**Requested by**: Michal (action item from the 2026-07-15 EasyCLA-migration leadership review)
**Deliverable**: `specs/001-easycla-ss-integration-fable/role-mapping-feasibility.md` — a memo (~2 pages + appendix) for Heather (PM) and Kieran (strategy), committed to branch `001-easycla-ss-integration`

## Git logistics (important)

- Work in this repo (`easycla`). The main checkout may be on a different branch (PR work in flight) — **check `git branch --show-current` first**. A linked worktree pinned to the right branch exists at `.claude/worktrees/spec`; prefer working there, or create your own worktree. Do not switch the main checkout's branch.
- Commit with `--signoff` (DCO) and push to `origin/001-easycla-ss-integration`.
- Docs need `SPDX-License-Identifier: CC-BY-4.0` only if placed outside `specs/` (specs/ files here don't carry it — match the existing spec files).

## Context (read these first, in order)

1. `specs/001-easycla-ss-integration-fable/00-overview-fable.md` — §2.4 (roles today) and §3 (bridge-don't-migrate strategy)
2. `specs/001-easycla-ss-integration-fable/04-milestone-ccla-org-lens-fable.md` — the role-mapping options table (A: bridge, B: OpenFGA copy, C: org-admin mapping) and the recommendation
3. `specs/001-easycla-ss-integration-fable/m1-my-cla/research.md` — R3 (token model spike) and the no-ownership-check finding
4. `specs/001-easycla-ss-integration-fable/spec.md` — "Program review outcomes" section

The program is migrating EasyCLA UIs into LFX Self Serve (UI-first approved). The open question this memo answers: **can Self Serve feasibly use the existing EasyCLA v3/v4 APIs given the role-model differences, and how should CLA authority be evaluated/enforced during M3–M5?**

## Facts already verified (do not re-derive; spot-check if suspicious)

- EasyCLA roles `cla-manager`, `cla-signatory`, `cla-manager-designee` are hardcoded in ACS (`acs/userrole/repository.go:101-102`), Salesforce-backed Postgres, scopes `organization` and `project|organization`.
- EasyCLA assigns them via organization-service/ACS APIs (`easycla/cla-backend-go/v2/organization-service/client.go:62-103,209-246`; `v2/cla_manager/service.go`); assignment is **asynchronous** (console polls up to 30×).
- The Corporate Console receives permission strings like `signature_approval_list:update:project|organization:{projectId}|{companyId}` resolved from ACS.
- LFX V2 platform authz = OpenFGA relations via `lfx-v2-fga-sync` (NATS); **no CLA object types exist in the FGA model** (`lfx-v2-fga-sync/docs/fga-catalog.md`).
- SS Org lens gates on `b2b_org#writer` via member-service; `lfx-v2-auth-service` is identity/metadata only (not roles).
- EasyCLA v3/v4 route through lfx-gateway (`lfx-gateway/dynamic/services/cla-service.yaml`) with `secured` middleware; the gateway has an ACS authorizer Traefik plugin (`traefik-acs-authorizer-middleware`) — its exact role for cla-service routes is **unverified**.
- v4 `GET /signatures/user/{userID}` performs **no per-user ownership check** — an example that v4 endpoint-level authorization is uneven.
- Documented known issue: a user's CLA role attaches to one company at a time.

## Questions to answer (the actual research)

1. **Where does EasyCLA v4 enforce CLA roles, exactly?** Trace enforcement per endpoint group in `easycla/cla-backend-go`: approval-list PUT, cla-manager POST/DELETE, request-corporate-signature, ecla-auto-create toggle, project-level admin ops. Look at the `auth`/`user_authorizer` packages, `authUser` usage in v2 handlers, and any warden/ACS calls. Produce a table: endpoint group → what's checked → where (token claims vs ACS lookup vs nothing).
2. **What token does v4 expect?** How are the Corporate Console's Auth0 claims/permissions produced (custom claims? gateway-injected? per-request ACS lookup)? Determine whether a Self Serve user token (different Auth0 client, LFX audience) would carry what v4 needs — check `cla-backend-go` auth middleware and the lfx-gateway ACS-authorizer plugin config. This decides whether SS can call v4 with user tokens or needs token exchange/M2M (crowdfunding precedent in `lfx-self-serve`).
3. **Read path for UI gating**: how would SS learn "user X has CLA authority over company Y / CLA group Z"? Inventory candidate endpoints (`/company/{id}/project/{id}/cla-managers`, ACS rolescopes API, the `cla-{stage}-user-permissions` DynamoDB table, token claims). Assess each for latency, auth requirements, and staleness.
4. **Evaluate the three options** from milestone 04 (bridge / FGA modeling / org-admin mapping) against the evidence from 1–3: effort, failure modes (esp. ACS async + Salesforce coupling), UX consequences (multi-company limitation), and what each implies for M6 later. Confirm or overturn the current recommendation (A: bridge). Be critical — if the evidence says the bridge is harder than assumed (e.g., v4 won't accept SS tokens without gateway changes), say so plainly.
5. **Concrete spike list**: end with the 3–5 cheapest experiments (dev-environment curl-level) that would convert remaining unknowns into facts, each with expected outcome.

## Repos (all local)

- `/Users/michal/src/github/linuxfoundation/easycla` (this repo; `cla-backend-go`, `cla-backend-legacy`)
- `/Users/michal/src/github/linuxfoundation/acs`, `acs-cli`
- `/Users/michal/src/github/linuxfoundation/lfx-gateway` (ACS authorizer plugin), `lfx-gateway-terraform`
- `/Users/michal/src/github/linuxfoundation/lfx-v2-fga-sync`, `lfx-v2-auth-service`
- `/Users/michal/src/github/linuxfoundation/lfx-self-serve` (token handling: `apps/lfx-one/src/server/middleware/auth.middleware.ts`, crowdfunding token exchange)
- `/Users/michal/src/github/linuxfoundation/lfx-corp-cla-console` (how the console obtains/uses permissions today)

## Memo shape

Executive answer first (feasible / feasible-with-conditions / not feasible, one paragraph). Then: enforcement-point table, token-path finding, read-path recommendation, options assessment with recommendation, spike list. Cite `file:line` for every load-bearing claim. Distinguish "verified in code" from "inferred" — the audience will make staffing decisions on this.
