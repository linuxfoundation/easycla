<!-- Copyright The Linux Foundation and each contributor to CommunityBridge.
SPDX-License-Identifier: CC-BY-4.0 -->

# M3 organization cleanup runbook (#2749, #2056)

Input: `org_import audit` reports (`cla-backend-go/cmd/org_import/README.md` §3.1). Everything here is done by hand; the tool never
deletes or merges rows. Run before the first prod `rewrite` tranche.

## 1. Unresolvable ids — `unresolvable.csv`

Active-CCLA rows whose `company_external_id` is empty, malformed, or a dead `001…`/`lf…` account (prod 2026-09-29 dry run: 42 empty,
16 dead `001…`, 3 dead `lf…`, 1 other). Per row decide one of:

- **Re-point** — a live Salesforce account exists (match by `company_name`/`website`): add `old_id,new_id,matched,true` to the mapping file;
  the row then migrates as a normal `rewrite` group. For an empty or malformed id use the row's `company_id` as `old_id`
  (`cmd/org_import/README.md` §4): the tool sets `company_external_id` on that single row and registers the Account — no hand-run
  `update-item`. Never guess by name; sales ops confirms or creates the Account first.
- **Leave** — record the decision (company, reason) in the #2749 comment. Rows referenced by CCLAs/ECLAs are never deleted.

## 2. Possible duplicates — `possible_duplicates.csv`

Two kinds of rows are listed; classify first (#2056):

- Same `company_external_id`, same (or empty) `signing_entity_name` → likely duplicate rows of one organization.
- Same `company_name` under different ids → possibly the same organization registered twice (legacy `lf…` + `001…`).

Distinct signing entities under one id are legitimate and migrate together; the tool never routes a group to `manual` for being a
duplicate. Review outcome (#3085) goes into the import's decisions file (`cmd/org_import/README.md` §4.1): `collapse` lets several old
ids land on one Account, `distinct` keeps them apart. Merging or deleting rows is post-import work with #2056 as the runbook, not a
tranche-1 prerequisite (#2749, 2026-09-29).
Confirmed duplicates: **default is to leave them unmerged** and document them. A merge is not a repointing of
`signature_reference_id`/`signature_user_ccla_company_id` only — signed PDF keys embed the original `company_id`
(`cla-backend-go/utils/s3.go`, upload `v2/sign/service.go`, download `v2/company/service.go`) and ACL/manager/invite references point at
the old row. If a merge is ever done: copy the PDFs, then verify PDF download, employee coverage and manager access before removing anything.

## 3. Companies without an active CCLA

Untouched by the tool and by this runbook (prod: ~1.4k). Disposition options to record in #2749 before Console retirement (not before
tranche 1): leave in EasyCLA until Console retirement, flag as archived, or delete after export.

## 4. Record keeping

Keep `audit.csv`, `unresolvable.csv`, `possible_duplicates.csv` and the decisions per row outside git (`org-import-out/` is ignored);
summarize counts and decisions in the #2749 comment.
