# EasyCLA / LFX Self Serve end-to-end suite

Manual, re-runnable end-to-end validation of EasyCLA and its Self Serve (SS) integration points on **dev** or
**prod**: Milestones M1 (Me lens), M2 (signing), M3 (Organization lens → EasyCLA) with the Organization-lens
bridge cases, the legacy consoles and the legacy API (`cla-backend-legacy`), the hardened `POST /v4/user/{id}/company` route,
the read ACL matrix, the identity boundary, the GitHub PR webhook pipeline and read-only Lambda log scans.
It is **not** wired into CI: run it from a workstation whenever a deployment or a role/permission change has to
be validated. Everything is "as code": the same checks run the same way every time, and every run leaves a
self-contained evidence directory with a `SUMMARY.md`.

```sh
cd tests/e2e
./run.sh prod                     # everything that is allowed on prod (see gates)
./run.sh dev                      # everything, including DocuSign sandbox signing and the GitHub PR flow
./run.sh prod --no-writes         # pure read-only pass (useful during an incident)
./run.sh dev --group api,legacy   # only the HTTP groups
./run.sh prod --module m3_chain   # one module
./run.sh prod --list              # modules per group
```

Exit code `1` when any check `FAIL`ed, `2` when `--group`/`--module` selected no module at all. Console output is one line per check:
`<UTC> [<module>] PASS|FAIL|SKIP|INFO <CHECK-ID> - <note>`.

## Requirements

* `bash`, `curl`, `python3` (3.9+, standard library only); `shellcheck` is only needed when editing.
  The shell parts avoid GNU-only flags (no `date -d`), so Linux, macOS and FreeBSD all work.
* Browser group: Node.js ≥ 20 (Playwright's requirement). `run.sh` runs `npm install` (Playwright) and `playwright install chromium`
  on first use; the dependency lives in `tests/e2e/package.json` only.
* Per-environment configuration and credentials in `secrets/` (git-ignored) — see
  [`secrets/README.md`](secrets/README.md): `secrets/<env>.env.secret` (copy
  [`config/example.env`](config/example.env) and fill in your accounts and QA fixtures), the Auth0 client id and
  `AUTH0_USERNAME/AUTH0_PASSWORD` of the two suite identities per environment. Tokens are minted through the
  repository's `utils/get_auth0_token.sh` (staged in a private temp dir, nothing in `utils/` is modified).
* Optional: an AWS profile (`AWS_PROFILE_E2E`, `lfproduct-dev|prod`) for the Lambda scans and the ACS
  cross-checks (missing → `SKIP`, never `FAIL`); `gh` logged in with push access to `GITHUB_REPO` for the
  dev-only PR flow.

## Identities and fixtures

| name | config key | role | used for |
|---|---|---|---|
| ID1 (`lgpm19820716`) | `ID1*` | org admin of `ID1_ADMIN_ORG_SFIDS`, CLA manager of the fixture companies, GitHub `ID1_GITHUB` | the "positive" account: writes, manager-side flows, B9 |
| ID2 (`morgothv8`) | `ID2*` | no organization roles; the chains grant and revoke its CLA-manager role on the fixture company | the "negative" account: 403/404 expectations, contributor side, B1–B8, B12 |

All writes happen on the QA fixture only: `WRITE_*` (company × CLA group where ID1 is in the CCLA
`signatureACL`), the M3 fixture `M3_*` (same company on prod) and ID2's own records. Every write module captures
a baseline first, brings the fixture to the state the chain needs (pre-flight), and compares the end state
with the baseline (`F-*` checks) — so modules are re-runnable in any order and from any leftover state.
`--keep-state` leaves the produced state in place; the baseline comparison then reports what differs.

Configuration is `secrets/dev.env.secret` and `secrets/prod.env.secret` (plain `KEY=VALUE`, parsed, never
sourced; both files must declare the same keys — `config/example.env` is the documented template, the suite
never reads it). Gates per environment, as set in those files:

| gate | dev | prod | guards |
|---|---|---|---|
| `ALLOW_FIXTURE_WRITES` | 1 | 1 | approval list / auto-ECLA / manager add+remove / invalidation on the fixture (always baseline-restored) |
| `ALLOW_EMAIL` | 1 | 1 | the one contact-manager request that e-mails ID1's own mailbox |
| `ALLOW_DOCUSIGN` | 1 | 0 | completing DocuSign sandbox envelopes (`m2_signing`, `org_sign`) |
| `ALLOW_GITHUB_PR` | 1 | 0 | opening a throw-away PR on `GITHUB_REPO` |
| `ALLOW_COMPANY_CREATE` | 0 | 0 | the one positive company-creation case of the hardened route |

`--no-writes` overrides every gate: the shell/python modules skip non-GET calls, the browser modules run with an
empty write allow-list (the guard blocks and records any attempted write instead of sending it).

## Modules

| group | module | what it proves |
|---|---|---|
| api | `health.sh` | v4/v3 health, legacy health + `X-EasyCLA-Backend`, legacy alias redirect |
| api | `hardened_route.sh` | `POST /v4/user/{id}/company` hardening (ops#343 / acs-cli#1148 / lfx-gateway#116): anonymous 401, foreign user 403 for uuid/username/`auth0|` sub, own user validation and duplicate rules, nothing created; neighbours (`invite-company-admin`) |
| api | `read_matrix.py` | read ACL matrix (ops#382) for ID1, ID2 and anonymous: own company 200, group-wide / other company / sanctioned 403, anonymous 401; ID2's expectations follow its live ACS scopes; identity reads and CCLA shape |
| api | `identity_boundary.py` | `my-clas` identity filters never leak the other identity's rows, foreign acknowledgment sub-resources 404, prepare-sign with foreign identities 403 |
| api | `fixture_writes.py` | approval list (own + six criteria types, invalid e-mail refused, duplicate idempotent, remove), auto-ECLA toggle, designee/manager reads, sanctioned company refuses every write, removal-only invalidation of ID1's own ECLA, baseline restored |
| api | `interactions.py` | two-identity chain: criteria added → ID2 prepare-sign → auto-ECLA creates ID2's ack → Me-lens reads → manager invalidates (409 on repeat, event names manager) → no resurrection on re-add / auto-ECLA → ID2 made CLA manager (ACL + ACS scope, writes 200) → removed (writes 403) → baseline |
| legacy | `legacy_api.sh` | legacy API (`cla-backend-legacy`, Go): backend header, v1 readers need the legacy Auth0 claim set (api-gw tokens 401 on prod), v2 last-signature carries no DocuSign secrets, anonymous public v2 readers 200, allowlist request with markup refused, `lfcla.com` alias only redirects |
| browser | `ld_flags.mjs` | LaunchDarkly flags as the SS front end evaluates them for both identities (M3 kill switch baseline) |
| browser | `ss_identity.mjs` | SS login, BFF reads (role-grants, me/clas, personas, nav), Me lens, organization selector, `/org/easycla` landing, deep links of own / other / CLA-only organizations |
| browser | `m3_org_lens.mjs` | M3 agreement detail as ID1: approval list (validation, add, duplicate, delete-cancel, remove), auto-ECLA on/off, acknowledgment invalidation, CLA-manager roster (add/remove ID2), activity log, end state = baseline |
| browser | `m3_chain.mjs` | M3 chain through the UI with both identities: criteria → auto-ECLA → ID2 acknowledges (Me lens → contributor console → corporate click-through) → manager invalidates (dialog also removes the matching criterion) → ID2 sees it invalidated → no resurrection → ID2 made manager → opens the organization EasyCLA page (bridge) → role removed → baseline |
| browser | `bridge.mjs` | Organization-lens bridge cases B1–B13 and lfx-self-serve#3631 for both identities (table below) |
| browser | `consoles.mjs` | legacy consoles read-only smoke: corporate console company dashboard (v1 on prod, LFX on dev), contributor console, PCC (+ v1 PCC on dev) |
| browser | `m2_signing.mjs` | dev only: ICLA through DocuSign sandbox and ECLA corporate click-through from the Me lens; verifies the existing rows when already signed |
| browser | `org_sign.mjs` | dev only: Organization lens CCLA signing (picker → manager question → attestation → BFF sign → DocuSign sandbox → row signed → ID1 on the roster); verifies the end state when already signed |
| github | `pr_flow.sh` | dev only, **creates a branch, a commit and a pull request on `GITHUB_REPO`** (gate `ALLOW_GITHUB_PR`): commit attributed to ID1's GitHub identity, PR opened, EasyCLA commit status reaches a terminal state and the bot comment arrives, status and the comment's badge verdict agree, sign link 302 to GitHub OAuth; PR closed and branch deleted (closed PR kept as proof) |
| scans | `lambda_scan.sh` | CloudWatch scan of every EasyCLA Lambda for the run window: panics / runtime errors / timeouts FAIL, error counts, INIT failures and Errors/Throttles sums INFO, SSM high-throughput setting |

### Bridge cases (`bridge.mjs`)

| case | check ids | expectation |
|---|---|---|
| B1 | `B1-EASYCLA-ONLY-PAGE-*` | CLA manager without org admin opens the company EasyCLA tab; navigation shows only EasyCLA |
| B2 | `B2-*` | … can add and remove approval-list entries (one synthetic domain, removed again) |
| B3 | `B3-*` | CLA manager of several companies switches between them and sees only the selected company's agreements |
| B4 | `B4-NO-OVERVIEW/PEOPLE/MEMBERSHIPS/PROJECTS-*` | … cannot open any other page of the company organization view (redirected to EasyCLA) |
| B5 | `B5-*` | … cannot open another company's organization view |
| B6 | `interactions.py` W3/W4, `m3_chain` C10 | removal as CLA manager revokes access (ACL + ACS, observed immediately; product promise ≤ 30 min) |
| B7 | `B7-ODD-ID-*` | company on an old / odd id gets a limited "not found" page, not an error |
| B8 | `B8-NON-MEMBER-*` | non-member company gets an EasyCLA-only page |
| B9 | `B9-ADMIN-AND-CLA-MANAGER` | org admin who is also a CLA manager sees the organization once with full access |
| B10 | `B10-*` | org admin who is not a CLA manager sees the Organization lens as before |
| B11 | `B11-*` | CLA signatory / designee only, no org access → no Organization lens entry |
| B12 | `B12-*` | user with no organization or CLA role → no Organization lens |
| B13 | `B13-ADMIN-PLUS-CLA-ONLY-*` | org admin of some organizations and CLA manager only of another: the selector lists the admin organizations with the full sidebar, the CLA-managed company is not a CLA-only entry (SS lists CLA-only companies only for callers without any Org Lens grant); its deep link renders a limited page or is denied, never a crash |
| LD | `B-LD-KILL-SWITCH-*` | M3 LaunchDarkly flag off for the identity → the EasyCLA tab is not served (observed, never forced) |
| #3631 | `B10-3631-*` | org-lens writer without an EasyCLA ACS scope opens Organization → EasyCLA: no crash, clear empty state |

Expectations are derived from the live `/api/orgs/me/role-grants` of each identity, so the cases stay correct
whichever role state the fixture is in (e.g. right after a `--keep-state` chain run).

## Evidence

Default run directory: `e2e-tests/runs/suite/<env>-<UTC stamp>/` (git-ignored; `--run-dir` overrides):

```
results.jsonl          one JSON row per check: ts, module, id, status, note, data
SUMMARY.md             per-module PASS/FAIL/SKIP/INFO table, failed and skipped checks with notes
run.log                the console output
<module>/              per-module evidence: *.private.json HTTP bodies (0600, tokens redacted),
                       receipts.private.jsonl (python modules: one line per HTTP call), *.png screenshots,
                       network.private.json (api/blocked: METHOD host/path + status; writes: the
                       business writes made, with their request bodies; network-<identity>.private.json
                       for a module's second identity)
```

Nothing in the evidence contains a token or password; still treat the directory as private (it holds
e-mail addresses and fixture ids).

## Expected INFO / SKIP rows (not defects)

* `legacy_api` `LG-V2-LAST-SIGNATURE-*`: the DocuSign callback URL is still part of the last-signature body
  (known, tracked separately); only secrets are asserted absent.
* `interactions.py` W3: after the manager role is revoked, org-level *reads* may stay 200 for a while
  (contact role on the company and the warden permission cache); writes are 403 immediately and that is what
  is asserted. ID2's user-service permissions feed lags ACS by up to 30 minutes — the suite reads ACS directly.
* Each `interactions.py` / `m3_chain` run leaves one more manager-invalidated acknowledgment of ID2 on the
  fixture group (intentional proof; Auto-ECLA never resurrects a manager-invalidated acknowledgment, which the
  chain asserts). The SS invalidate dialog offers to remove the criterion added for that contributor
  (default on) — the chain records which criteria the dialog removed (`C5B-*`) and removes the rest itself.
* `lambda_scan`: INIT_REPORT / SSM cold-start noise and non-zero error counts are INFO with the top messages;
  only panics, runtime errors and timeouts fail. `/api/changelog/unseen` 500s in the SS network log are SS
  noise and ignored by the guard.
* Approval-list `PUT` is not atomic on the backend: a request mixing valid and invalid entries can apply the
  valid part; the suite therefore sends one entry per request.
* `read_matrix` `W-ID2-NEGATIVE-WRITES`, `identity_boundary` `*-FOREIGN-INVALIDATE-REFUSED`: skipped while ID2
  legitimately holds the fixture CLA-manager scope (e.g. a chain left with `--keep-state`).
* prod: the GitLab organization fixture is invalid upstream (INFO), the sanctioned-company auto-create route
  answers 404 (INFO), `hardened_route` creation-capable cases and all signing/GitHub modules are gated off.
* dev: `m2_signing` ICLA and `org_sign` take the "already signed" path once ID1 has signed the fixture group
  (an ICLA is never invalidated by the user); point `ICLA_GROUP` / `ORGSIGN_GROUP` at a fresh group to drive
  DocuSign again. The GitHub verdict (`GH-VERDICT`) depends on ID1's authorization on the enrolled CLA group
  and is recorded, not asserted.

## Layout

```
run.sh                 runner (groups, modules, gates, evidence dir, summary)
config/example.env     template for secrets/<env>.env.secret (endpoints, identities, QA fixtures, gates)
secrets/               <env>.env.secret configuration, credentials, generated tokens / browser state (git-ignored)
lib/common.sh          shell helpers: config, secrets, http, checks, gates, redaction
lib/e2e.py             python helpers (stdlib only): config, tokens, http, checks, ACS/identity state
lib/token.sh           token minting / reuse for both identities
lib/summary.py         SUMMARY.md renderer
api/ legacy/ scans/ github/   HTTP, legacy, Lambda-scan and GitHub modules
browser/lib/e2e.mjs    Playwright harness: login (cached state), network guard (declared writes only,
                       DocuSign sandbox allow-list), LD capture, screenshots, results
browser/lib/m3.mjs     Organization lens → EasyCLA page helpers (approval list, auto-ECLA, roster, invalidation)
browser/lib/signing.mjs  Me lens hand-off, contributor console, corporate click-through, DocuSign sandbox
browser/*.mjs          browser modules
```

Adding a check: call `e2e_result <ID> <PASS|FAIL|SKIP|INFO> <note> [json]` (shell), `e2e.check(...)` /
`e2e.info(...)` / `e2e.skip(...)` (python) or `check(...)` / `info(...)` / `skip(...)` (node); ids are
upper-case, module-prefixed and stable so runs can be diffed. Adding a module: drop the file into its group
directory and register it in the `MODULES` array of `run.sh`. Browser modules must declare every write they
intend to make in `launch({ writes: [...] })`; anything else is blocked by the guard and reported.
