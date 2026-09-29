# org_import — EasyCLA organization import/sync (M3)

Copyright The Linux Foundation and each contributor to CommunityBridge.

SPDX-License-Identifier: CC-BY-4.0

Tickets: linuxfoundation/lfx-self-serve#2750 (import), #2751 (no company rows created by reads), #2749 (cleanup).
Design: `docs/easycla-ss-migration/m3-b2b-org-import.md` (PR #5223). Cleanup runbook: `docs/easycla-ss-migration/m3-org-cleanup.md`.

## 1. What it does

For every EasyCLA company with an **active CCLA** (`signature_type=ccla`, `signature_reference_type=company`,
`signature_signed=true`, `signature_approved=true`) the tool makes the organization usable by LFX Self Serve:

| Route | When | What happens (apply mode) |
|---|---|---|
| `register` | `company_external_id` is a live Salesforce account id (`001…`, 15/18 chars) | `POST /b2b_orgs {"sfid": id}` on the LFX v2 member-service (idempotent) |
| `rewrite` | id is `lf…` (legacy console) or a dead `001…` account | needs a new Salesforce id from `--mapping` (or `--use-apex`): copy ACS grants old→new, rewrite `company_external_id` on every row of the group (old id kept in `previous_company_external_id`), re-key activity events, delete old grants, register the new id |
| `manual` | empty/invalid id, ambiguous/unapproved mapping, collision on the target Account without a decision (§4.1), no/shared website on the Apex path | nothing; listed with `manual_reason` and a suggested action |

A **group** is every company row sharing one `company_external_id` (signing entities of one organization); it is
eligible when at least one row has an active CCLA. Internal `company_id`s never change.

Never done by the tool: creating or deleting EasyCLA rows, merging companies, deleting Salesforce accounts or
org-service organizations, deleting roles, touching groups without an active CCLA, writing anything without `--apply`.
Rows sharing one `company_external_id` are one group and are never merged; merging duplicate companies is a post-import
runbook (linuxfoundation/lfx-self-serve#2056), the pre-import duplicate review (#3085) only feeds the decisions file (§4.1).

Safety rules:
- Default is **dry run**. `--apply` prompts you to type the stage name (`--yes` skips the prompt, for automation).
- Every rewrite step is idempotent and recorded in `--state` (required for a rewrite `--apply`; the first journal line precedes the first
  write and a journal write failure stops the group); re-running the same command converges.
- The row rewrite is a conditional write (`company_external_id = <exact stored value>`); a concurrent change stops that group only.
- Grants are copied before rows are rewritten and old grants deleted last, each old grant re-checked (and copied if missing) right before
  its deletion: managers never lose access.
- An apply that could not finish is refused before its first write: rewrites without `--state`, or any register/rewrite without member-service.
- `register` is stateless and needs no Salesforce input — it is the only route ever scheduled (§6).

## 2. Prerequisites

- Go 1.25+, `cd cla-backend-go && make build-org-import-linux` (Linux) or `make build-org-import-mac` → `bin/org-import[-mac]`.
- AWS credentials for the stage account (`dev.md`, "assume role" recipe; MFA). Either `AWS_PROFILE=lfproduct-dev AWS_SDK_LOAD_CONFIG=1`
  or exported `AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY/AWS_SESSION_TOKEN`. `AWS_REGION` defaults to `us-east-1`.
- `STAGE=dev|prod` (required).
- IAM: read of SSM `cla-*-{stage}` parameters; DynamoDB `Scan/Query/GetItem` on `cla-{stage}-companies`, `cla-{stage}-signatures`,
  `cla-{stage}-events`; `UpdateItem` on `cla-{stage}-companies` and `cla-{stage}-events` (rewrite route only). Same tables the API uses.
- SSM parameters read (all pre-existing except the first two):
  - `cla-member-service-base-url-{stage}`, `cla-member-service-auth0-audience-{stage}` — member-service; when missing the tool warns,
    every SFID group is `live=unverified` → `pending`/`crm_unverified` (org-service is **not** a liveness source) and an `--apply` with
    anything to register or rewrite is refused with `member-service is not configured` before writing.
    Values (from `lfx-v2-argocd/values/{stage}/lfx-platform.yaml`): dev `https://lfx-api.dev.v2.cluster.linuxfound.info` /
    `https://lfx-api.dev.v2.cluster.linuxfound.info/`, prod `https://lfx-api.v2.cluster.lfx.dev` / `https://lfx-api.v2.cluster.lfx.dev/`.
    Create as plain `String` parameters (`aws ssm put-parameter --type String …`); dev already has them, prod does not yet.
  - `cla-auth0-platform-*-{stage}` — M2M token (org-service, ACS, member-service).
  - `cla-api-gateway-url-{stage}`, `cla-acs-api-key-{stage}` — org-service and ACS.
  - `cla-salesforce-apex-base-url-{stage}`, `cla-salesforce-apex-token-{stage}` — only with `--use-apex`.
  - `cla-org-import-report-emails-{stage}` — optional, comma-separated recipients of the run report e-mail (§8); absent ⇒ no e-mail, noted in run.log.
    `cla-ses-sender-email-address-{stage}` (pre-existing) is the sender.
- Reports (§8) additionally need `logs:CreateLogGroup/CreateLogStream/PutLogEvents` on `/easycla/org-import/{stage}` and `ses:SendRawEmail`;
  `--no-aws-log` / `--no-email` switch them off (use both for a read-only look at prod from a laptop).
- Member-service access (ops, one-time per stage): (1) an Auth0 **client grant** for EasyCLA's M2M client (`cla-auth0-platform-client-id-{stage}`)
  on the lfx-api resource server (the audience above) — without it the token request fails with
  `authorization failed (403): Client "…" is not authorized to access resource server` (current state on dev); (2) the client must be in the
  `global_org_admin` team (member-service Heimdall ruleset) for `POST /b2b_orgs` and have `auditor` for `GET /b2b_orgs/{id}` (dry-run liveness).
  Check: `STAGE=dev bin/org-import ingest --routes register --ids <known live 001 id>` must print `live=live`; `live=error` means the GET
  is not permitted (never treated as dead) and the run exits 1.
- Nothing sensitive is ever written: report files contain company names/ids only; tokens stay in memory.

## 3. Commands

```
org_import audit  [--out-dir ./org-import-out] [report flags]
org_import ingest [--apply] [--yes] [--tranche N] [--ids id1,id2] [--mapping map.csv] [--decisions decisions.csv]
                  [--shared-domains domains.txt] [--state state.jsonl] [--routes register,rewrite] [--skip-wait]
                  [--use-apex] [--wait-max 40m] [--out-dir ./org-import-out] [report flags]
report flags:     [--email-to a@x,b@y] [--no-email] [--no-aws-log] [--aws-log-group /easycla/org-import/<stage>]
```

| Flag | Meaning |
|---|---|
| `--out-dir` | CSV reports directory (default `./org-import-out`, gitignored) |
| `--apply` | perform writes; without it nothing is written anywhere except report files |
| `--yes` | skip the "type the stage name" confirmation (automation) |
| `--tranche N` | act on at most N groups (planning still classifies everything) |
| `--ids` | only these groups; accepts old **or** new external ids (comma-separated) |
| `--mapping` | Salesforce mapping CSV (§4); required for the rewrite route unless `--use-apex` |
| `--decisions` | duplicate-review decisions CSV (§4.1): collapse/distinct verdicts for old ids landing on one Account |
| `--shared-domains` | file replacing the built-in shared-domain list (§4.2; one domain per line, `#` comments) |
| `--state` | append-only JSONL; one record per group per step; unfinished groups are replayed first on the next run. Required for `--apply` with the rewrite route (exit 2 otherwise) |
| `--routes` | `register`, `rewrite` or both (default both) |
| `--skip-wait` | check the new ids in org-service once instead of polling up to `--wait-max` |
| `--use-apex` | resolve new ids via the Salesforce Apex endpoint (`ORG_IMPORT_USE_APEX=true` equivalent); refused when the SSM params are missing |
| `--wait-max` | org-service propagation wait for new Salesforce accounts (default 40m, poll 30s) |
| `--email-to` | report recipients, overrides SSM `cla-org-import-report-emails-{stage}` |
| `--no-email` / `--no-aws-log` | do not send the report e-mail / do not copy run.log to CloudWatch Logs |
| `--aws-log-group` | CloudWatch log group (default `/easycla/org-import/{stage}`), created when missing |

Env: `STAGE` (required), `AWS_REGION` (default `us-east-1`), `LOG_LEVEL` (default `warn`; `debug` shows every AWS/HTTP call; `error` hides the
expected org-service "not found" warnings printed for every dead/unknown id).

Every run writes `<out-dir>/run.log` (everything printed, truncated per run) plus copies of the inputs it used
(`input-mapping.csv`, `input-decisions.csv`, `input-shared_domains.txt`, `input-state.jsonl` = the journal before the run; a `--state` outside
the out-dir is copied in after the run as `state.jsonl`) and ends with the report step (§8); report failures are logged and never change the exit
code. The apply commands in the report reference those copies as `"${RECORD:-<out-dir>}/input-*.csv"` and `"${RECORD:-<out-dir>}/state.jsonl"`:
run them where the dry run ran, or extract the e-mailed zip / Actions artifact elsewhere and `export RECORD=<that directory>` first.

### 3.1 `audit` (reads only)

```bash
cd cla-backend-go && make build-org-import-linux
export AWS_PROFILE=lfproduct-dev AWS_SDK_LOAD_CONFIG=1 STAGE=dev
bin/org-import audit --out-dir ./org-import-out/dev-$(date -u +%F)
```

Output line: `audit stage=dev companies=N eligible_groups=M MISSING_SFID=a INVALID_SFID_FORMAT=b SFID_OK=c SFID_DANGLING_OR_DELETED=d UNKNOWN=e register=r rewrite=w manual=m duplicates=k`
— the five tiers are 1:1 with `utils/audit_company_reachability.sh`; route counts are per row (only rows in eligible groups have a route).

Files (`--out-dir`):
- `audit.csv` — `company_id, company_name, signing_entity_name, company_external_id, id_shape(001|lf|empty|other), active_ccla, ccla_count, ecla_count, org_service(200|404|err), website, duplicate_sfid_group, route, manual_reason, tier`.
  `ecla_count` is filled only for active rows that are manual, duplicate or unresolvable (cheap); `-1` = count failed.
- `unresolvable.csv` — active rows with empty/invalid ids or dead `001…` ids (#2749 input).
- `possible_duplicates.csv` — rows sharing an id with the same (or empty) signing entity name, and same company name under different ids (#2056 input). Distinct signing entities under one id are **not** duplicates.

Runtime: full companies scan + one CCLA query + one org-service GET per distinct id (4 in parallel) — ~30 s on dev, ~10 min on prod (3.7k companies).

### 3.2 `ingest` dry run

```bash
STAGE=dev bin/org-import ingest                                  # all routes, no mapping: rewrite candidates are "pending"
STAGE=dev bin/org-import ingest --routes register                # what the sweep does
STAGE=dev bin/org-import ingest --routes rewrite --mapping map.csv --state state.jsonl
```

Annotated sample:

```
org_import ingest stage=dev mode=dry-run eligible_groups=4 register=1 rewrite=1 pending=1 skipped=0
register 0014100000Te0G7AAJ   shape=001   rows=2 live=live | Live Corp; Live Corp / Live Corp Asia
rewrite  lfbd1c2b3a4d5e6f7a8  shape=lf    rows=1 new_id=0014100000NewNewNe action=matched domain=legacy.example | Legacy Ltd
rewrite  0014100000DeadDead1  shape=001   rows=1 live=dead reason=no_mapping | Dead Corp        <- pending: needs a mapping row
manual   c0ffee00-...         shape=empty rows=1 reason=empty_external_id | Empty Inc
PLAN register 0014100000Te0G7AAJ: POST /b2b_orgs {sfid:0014100000Te0G7AAJ} (rows: c-live,c-live-sub)
PLAN rewrite lfbd1c2b3a4d5e6f7a8 -> 0014100000NewNewNe (matched): wait org-service; copy ACS grants; rewrite 1 row(s) c-lf; re-key events; delete old grants; POST /b2b_orgs {sfid:0014100000NewNewNe}
TARGETS (rewrite destinations grouped by Account):
target 0014100000NewNewNe groups=1 old_ids=lfbd1c2b3a4d5e6f7a8 existing_rows=0 status=ok
MANUAL ACTIONS (1):
c0ffee00-... [manual] empty_external_id: Fill company_external_id (…) or leave for cleanup (#2749); the tool never guesses an id.
stage=dev mode=dry-run eligible=4 registered=0 rewritten=0 pending=1 manual=1 failed=0
```

- `live=live|dead|error` — member-service `GET /b2b_orgs/{id}` (or org-service when member-service is unconfigured); `error` is never treated as dead.
- `pending` — rewrite candidates without a resolved new id (no mapping row, or a `register` POST answered 404 = dead account). They appear in `to_salesforce.csv`.
- `skipped` — eligible groups excluded by `--routes`, `--tranche`, or already `done` in `--state`.
- Summary line: `eligible` = groups after `--ids`; `registered`/`rewritten` = groups completed in apply mode; `manual`; `failed` = groups with an
  error — the run exits 1 in dry-run and apply mode alike (a dry run with `live=error` is not a clean dry run).

Runtime: ~10 s on dev, ~7 min on prod (one liveness GET per group).

Files (all rewritten after apply with the final state):
- `plan.csv` — `key, old_id, id_shape, route, manual_reason, live, org_service, website, domain, shared_domain, new_id, action, decision, reviewer, company_ids, company_names, error`.
- `manual_actions.csv` — one row per manual/pending/failed group with `reason` and `suggested_action` (what a human must do next).
- `targets.csv` — rewrite destinations grouped by Account: `target_sfid, groups, old_ids, company_ids, company_names, existing_rows, decision, reviewer, status(ok|needs_decision|distinct_conflict)`.
- `to_salesforce.csv` — `old_id, name, website, ccla_signed_date, domain, shared_domain` — the hand-off to sales ops (§4).

Manual reasons: `empty_external_id`, `invalid_id_shape`, `mapping_ambiguous`, `mapping_not_approved`, `mapping_same_id`,
`target_collision` (several old ids → one Account, or the Account already has EasyCLA rows, and no decision covers it — §4.1), `distinct_conflict`
(a `distinct` decision spans two ids resolved to the same Account), `missing_website` / `shared_domain` (Apex path only, §4.2),
`apex_match_needs_approval`, `apex_error`; pending reasons: `no_mapping`, `dead_account`, `crm_unverified` (no member-service for the stage).
With `--use-apex` a dry-run `created` has no Account id yet (`new_id=<apex-at-apply>`): the id is assigned by the real call at apply and a
different answer at apply time (`changed between dry run`) fails the group before any write; a failed resolution is never replayed as approved.

### 3.3 `ingest --apply`

```bash
# register only, 10 groups, interactive confirmation
STAGE=prod bin/org-import ingest --routes register --tranche 10 --apply
# rewrite tranche with mapping + state
STAGE=prod bin/org-import ingest --routes rewrite --mapping map.csv --state state.jsonl --tranche 10 --apply
# one group, by old or new id
STAGE=prod bin/org-import ingest --ids lfbd1c2b3a4d5e6f7a8 --mapping map.csv --state state.jsonl --apply
```

Order inside one run: all `register` POSTs → (Apex real calls) → **one** shared wait until org-service serves every new id (≤ `--wait-max`)
→ per rewrite group: `copy_grants` → `rewrite_rows` → `rekey_events` → `delete_old_grants` → `register` → `done`.
A failing step stops that group (state records it), the run continues with the next group and exits 1 with `FAILED …` lines.

Expected runtime: register = seconds per group; rewrite = the org-service wait (new Salesforce accounts take up to ~40 minutes to appear;
existing accounts are immediate) plus seconds per group.

## 4. Mapping file

CSV with header `old_id,new_id,action,approved`:

```
old_id,new_id,action,approved
lfbd1c2b3a4d5e6f7a8,0014100000NewNewNe,matched,true
0014100000DeadDead1,0014100000Fresh001,created,true
lf000000000000000001,,ambiguous,false
```

- `action`: `matched` (existing account found by domain/name — **requires `approved=true`**, a human checked it), `created` (new account created for this org, accepted as is), `ambiguous` (several candidates — stays manual).
- `new_id` must be a 15/18-char alphanumeric Salesforce id (`ambiguous` rows may leave it empty); `new_id == old_id` is refused (`mapping_same_id`).
- One row per `old_id`; two old ids pointing to the same `new_id` (or a `new_id` some EasyCLA row already carries) → `target_collision` until a
  `collapse` decision covers them (§4.1); `distinct` decisions cannot share an Account.
- Loader errors are fatal and name the line: `missing column`, `expected N columns`, `approved must be true|false`, `empty old_id`, `duplicate old_id`, `new_id … is not a Salesforce account id`, `action must be matched|created|ambiguous`.
- Rows with an empty or malformed `company_external_id` (`empty_external_id` / `invalid_id_shape` in `unresolvable.csv`, `manual_actions.csv`)
  have no organization id to key on, so `old_id` is the row's **`company_id`** instead (`<company_id>,001…,matched,true`). Such a row is a
  group of its own — blank or garbage values are never grouped —, the write is conditional on that one row still carrying its current value
  (`attribute_not_exists` / the malformed string), no `previous_company_external_id` is recorded for a blank value, ACS grants and events are
  not moved (nothing is keyed by the old value) and the new id is registered as usual. Two such rows on one Account are a `target_collision`
  until a `collapse` decision names both company ids (§4.1). `state.jsonl` records them under `key` = `company_id`.

How it is produced (sales ops, outside the tool): take `to_salesforce.csv` from a dry run, domain-match `website` against live accounts,
create the rest with Data Loader (unique external-id field = `old_id`, non-member record type, origin marker), export `old_id,new_id,action,approved`.
Mark `approved=true` only after a person confirmed each `matched` pair. Keep the file out of git (`org-import-out/` is ignored).

`--use-apex` replaces the file for `created` results: the tool does a `dryRun` call per group first, then the real call, and stops the group when
action/id differ between the two; `matched` results still need an approved mapping row with the same id. An approved/`created` mapping row wins
over Apex for its old id, and groups without a website or with a shared domain (§4.2) never reach Apex.

### 4.1 Decisions file (`--decisions`, lfx-self-serve#3085)

Outcome of the human duplicate review. CSV with header `decision,old_ids,target_sfid,reviewer,note`; `old_ids` separated by `;` or spaces:

```
decision,old_ids,target_sfid,reviewer,note
collapse,lfaaaa000000000000001;lfbbbb000000000000002,0014100000NewNewNe,michal,same company (Acme Inc / Acme GmbH)
distinct,lfcccc000000000000003;lfdddd000000000000004,,michal,different companies despite the shared domain
```

- `collapse` — the listed old ids (and any EasyCLA rows already on `target_sfid`) are one organization: the Account may receive all of them.
  Without it every Account that would end up with more than one old id — including one that already has EasyCLA rows — is `target_collision`.
- `distinct` — the listed old ids are different organizations; ≥2 ids, no target. If two of them still resolve to one Account the groups become
  `distinct_conflict` (fix the mapping). `reviewer` is required; an old id may appear in one decision only.
- Decisions never change the mapping: the Account comes from `--mapping` / Apex; the decision only approves the collision.

### 4.2 Shared domains (`--shared-domains`, Apex path only)

Groups whose website domain is in the shared list (`github.com`, `nowebsite.com`, `gmail.com`, `googlemail.com`, `yahoo.com`, `hotmail.com`,
`outlook.com`, `live.com`, `icloud.com`, `protonmail.com`, `qq.com`, `163.com`) or who have no website are `manual` (`shared_domain` /
`missing_website`) instead of being domain-matched by Apex. A file replaces the whole list (one domain per line, `#` comments). Mapping rows are
explicit human decisions and are not gated.

## 5. Tranche protocol (prod)

1. `audit` → compare tier counts with the last `utils/audit_company_reachability.sh` numbers; read `unresolvable.csv` / `possible_duplicates.csv`.
2. `ingest` dry run, all routes → read `plan.csv`; hand `to_salesforce.csv` to sales ops.
3. `ingest --routes register --tranche 10 --apply` → verify (below) → `--tranche 100` → rest.
4. With the mapping file: `ingest --routes rewrite --mapping map.csv --state state.jsonl --tranche 10 --apply` → verify → 100 → rest.
5. Re-run the same command: it must report nothing new (`registered=0 rewritten=0`, groups `skipped` as done).

Verification per tranche (pick 3 groups):
- `GET /b2b_orgs/{new id}` → 200 (member-service).
- org-service `GET /organizations/{new id}` → 200.
- ACS `GET /users/rolescopes/organization?orgid={new id}&scopetype=all` with `X-LFX-CACHE: false` → same users/roles as the old id had; old id → empty.
- `GET /v4/company/external/{new id}/cla-groups` lists the CCLA; `GET /v4/company/{companyID}/project/{projectSFID}/events` still shows the pre-rewrite history.
- DynamoDB row: `company_external_id = new`, `previous_company_external_id = old`.
- Corporate Console: a CLA manager of the company logs in, sees the company and its CCLA; Self Serve org lens shows the organization.

## 6. Sweep (`register` route only)

Manual: `STAGE=<stage> bin/org-import ingest --routes register [--apply --yes]` — stateless, idempotent, minutes.
It **never** rewrites and never touches ACS or Salesforce; new `lf…` (legacy console) companies only show up as `pending` — moving them is the
manual rewrite tranche of §5 (a named owner is required, see the design doc).

GitHub Actions `.github/workflows/org-import-sweep.yml`:
- Actions tab → "Org import sweep" → Run workflow: `stage` (`dev|prod`), `mode` (`dry-run|apply`), `routes` (default `register`),
  `tranche`, optional `ids`, optional `mapping` / `decisions` / `shared_domains` (the files of §4 pasted as text; rows separated by newlines or
  `|` — e.g. `old_id,new_id,action,approved|lf…,001…,matched,true`), `notify` (default true; false ⇒ `--no-email`). Without a mapping, rewrite
  candidates are only reported as pending. Same from a shell (the report e-mail of every dry run prints this line ready to paste):
  `gh workflow run org-import-sweep.yml -f stage=dev -f mode=dry-run -f routes=register,rewrite -f mapping="$(tr '\n' '|' < map.csv)"`.
- State: apply runs upload `state.jsonl` as artifact `org-import-state-<stage>`; the next run of the same stage restores the newest one first,
  so crashed rewrite groups are replayed; a state artifact that exists but cannot be downloaded fails the job (never start from an empty
  journal). Artifacts expire after 90 days: before that, copy `state.jsonl` from the newest `org-import-out-<stage>-*` artifact and keep it
  (it is also uploaded there). Without the journal, finished groups are re-classified as `register` on their new id, but a group that crashed
  mid-rewrite is not recognised as such — see §7 before re-running.
- One run per stage at a time (`concurrency: org-import-<stage>`): a manual dispatch queues behind a scheduled run instead of racing it.
- Schedule (`0 6 * * *`) is gated per stage by repository variables `ORG_IMPORT_SWEEP_DEV` / `ORG_IMPORT_SWEEP_PROD` ∈ `off|dry-run|apply`;
  unset or `off` (default) ⇒ the scheduled job does nothing. Scheduled runs are always `--routes register`. Manual runs ignore the variables.
- Promotion: ≥10 clean manual runs (dry-run, then apply) → set the variable to `dry-run`, read the artifacts for a week → `apply`.
  Rollback: set `off` (no deploy, no code change). The `prod` environment's protection rules (reviewers) apply to every run.
- Each run uploads `org-import-out/` (run.log and the CSVs of §3.2) as artifact `org-import-out-<stage>-<run id>`, copies run.log to CloudWatch
  Logs and e-mails the report (§8) — scheduled runs included.
- IAM: the `github-actions-deploy` role of each account must allow the DynamoDB/SSM access listed in §2 (it deploys the API, so it already does).

## 7. Failures and recovery

| Where | Message | Meaning / action |
|---|---|---|
| setup | `loading SSM config` / `STAGE is not set` | wrong account/profile or missing stage |
| planning | `live=error` | member-service GET failed (403 = missing `auditor`, network) — fix access; nothing is classified dead |
| planning | `live=unverified` / `crm_unverified` | no member-service params for the stage; groups stay pending, nothing is registered |
| apply | `rewrite apply requires --state` / `member-service is not configured` | refused before the first write; every planned group is reported as failed |
| any step | `state file …: cannot record` | the journal could not be written; the group stops (rows are never rewritten before their `start` line) |
| resolve | `changed between dry run` / `non-account id` | the live Apex answer differs from the plan; nothing written; re-run (not replayed) |
| planning | `mapping line N: …` | fix the CSV |
| planning | `state: unfinished group … has no valid new_id` | hand-edit the state file only if you know why; otherwise stop |
| register | `account … does not exist in Salesforce (rewrite candidate)` | POST 404: the id is dead; group is now `pending`/`dead_account` → mapping |
| register | `member-service is not configured` | SSM params missing for this stage |
| wait | `org-service does not serve … yet` | new Salesforce account not propagated within `--wait-max`; re-run later (state replays the group) |
| copy_grants | `granting … on …` | org-service create failed; nothing else was changed; re-run |
| rewrite_rows | `… conflict, group stopped` | a row's external id was changed by someone else meanwhile; investigate that row before re-running |
| rekey_events | `event …` | UpdateItem failed; re-run (already re-keyed events are skipped) |
| delete_old_grants | `deleting … grant …` | rows are already rewritten and new grants exist; re-run to finish |

Resume: re-run the same command with the same `--state`; unfinished groups are replayed first (pinned by their `company_ids`, so they are found
even after their rows already carry the new id). `--ids <old or new id>` narrows a run to one group.

Revert a rewrite by hand (only if really needed): for each row, `aws dynamodb update-item --table-name cla-{stage}-companies --key '{"company_id":{"S":"<id>"}}'
--update-expression 'SET company_external_id = :o REMOVE previous_company_external_id' --expression-attribute-values '{":o":{"S":"<old id>"}}'`,
then re-create the ACS grants on the old id (org-service `CreateRolescopes`, by username) and remove them from the new id. Events keep the new key
(run `events.RekeyRepository` the other way round if required). Stop and contact the EasyCLA maintainers before reverting more than one group.
`register` cannot be reverted through the API: member-service has no `DELETE /b2b_orgs/{id}`; a wrongly registered organization must be removed
by the member-service owners. Hence the register route is only ever applied to Salesforce-live ids, and never without member-service.

## 8. Run report (e-mail + CloudWatch Logs)

After every `audit`/`ingest` run (dry-run or apply, success or failure) the tool:
1. copies `run.log` to CloudWatch Logs group `/easycla/org-import/{stage}` (created when missing), stream `<start UTC>-<command>-<mode>-<runner>`;
2. e-mails `cla-org-import-report-emails-{stage}` (or `--email-to`) from `cla-ses-sender-email-address-{stage}` via SES: subject
   `[EasyCLA org-import][<stage>] <command> <mode>: <summary line> (OK|FAILED)`; body = run header (stage, mode, arguments, runner, build revision,
   Actions run/artifact links, CloudWatch stream), the **Manual actions** table with suggested actions, the **Targets** table, the full plan (≤2000
   rows inline) or audit tier counts, the ready-to-paste local and `gh workflow run` apply commands for a dry run, and the run.log tail; attachments =
   every CSV of the out-dir, run.log and a zip of the out-dir (≤6 MiB). To stay under SES's 10 MiB the largest text attachment is gzip-compressed
   (`run.log.gz`, full content) or, when not text, dropped — the zip last; every such change, an input or journal copy that failed, and a zip
   that could not be built or is over 6 MiB (it is the only carrier of the `input-*` copies and the journal) is announced at the top of the e-mail
   ("Delivery incomplete …") and in run.log; the complete record is always the out-dir / Actions artifact / CloudWatch stream.
   `audit` also prints one `audit row company_id=… route=… tier=…` line per company row into run.log.

Missing recipients parameter, SES or CloudWatch errors are written to run.log only; the exit code reflects the import itself.

## 9. Cleanup pointers

`unresolvable.csv` and `possible_duplicates.csv` feed `docs/easycla-ss-migration/m3-org-cleanup.md` (#2749, #2056): human review, nothing is deleted
or merged by the tool. Duplicate companies (same organization under several ids) are collapsed onto one Account by the import via the decisions
file (§4.1); deleting or re-pointing their EasyCLA rows is post-import work (#2056).
