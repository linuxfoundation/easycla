<!-- Copyright The Linux Foundation and each contributor to CommunityBridge. -->
<!-- SPDX-License-Identifier: CC-BY-4.0 -->

# M3: EasyCLA Orgs in the Self Serve Org Lens — Visibility & Access

**Status:** Updated after the 2026-09-10 architecture call · aligned with `specs/044-lfx-v2-cla-service/` (rev 5)
**Problem:** The Self Serve Org Lens only shows LF **member** organizations. Most EasyCLA customers are **not** members — so most CLA managers would open Self Serve and see nothing.

## The gap, quantified (prod data, Snowflake)

| Metric | Count |
|---|---|
| EasyCLA companies with a Salesforce ID (SFID) | 3,531 |
| …visible in Self Serve today (member orgs) | **880 (25%)** |
| …invisible (non-member orgs) | **2,651 (75%)** |
| Orgs with an **active signed CCLA** | 2,277 |
| …of those, invisible in Self Serve | **1,581 (69%)** |

Membership was measured with the exact gate member-service uses: Salesforce `Account` having an `Asset` with `Product2.Family = 'Membership'`.

> Eric's B2B-ingest analysis measures differently (domain matching against Salesforce; ~1,700 accounts to bring in, ~50% already mapping to existing Accounts). The two counts need reconciling in his proposal, but they describe the same gap.

## Why they are invisible — two independent gates

```mermaid
flowchart LR
    SF["Salesforce Accounts<br/>(~100k)"] -->|"gate 1: has Membership Asset<br/>(~5,265 accounts)"| MS["member-service"]
    MS -->|"b2b_org docs"| QS["query-service<br/>(OpenSearch)"]
    MS -->|"writer / auditor tuples"| FGA["OpenFGA"]
    QS -->|"org list"| SS["Self Serve<br/>Org Lens"]
    FGA -->|"gate 2: user must hold<br/>a grant on the org"| SS
    CLA[("EasyCLA DynamoDB<br/>3,531 orgs, 2,651 non-member")] -.->|"no path"| SS
```

1. **The org record does not exist.** member-service only ingests Accounts with a Membership Asset, so a non-member org has no `b2b_org` document — nothing to list, search, or attach permissions to.
2. **The user has no grant.** Org Lens eligibility is an OpenFGA relation resolved against `b2b_org` / CLA objects. EasyCLA CLA-manager roles live in ACS/Org Service scopes — OpenFGA knows nothing about them.

## Direction agreed on the 2026-09-10 architecture call

**EasyCLA companies are B2B engagements — they become real Salesforce B2B accounts.** There will be no separate "EasyCLA organization" entity, no new B2C org type, and no parallel org catalogue. A CCLA attaches to a B2B account, the same way membership does. Eric is drafting the proposal to sales ops (Mindy) to approve ingesting the non-member CLA companies as B2B accounts — tracked in his "Consolidate B2B backend, ingest EasyCLA companies as Salesforce accounts" ticket. **This approval is the critical-path dependency for M3.** If sales ops pushes back, a different approach is needed (explicitly acknowledged on the call).

The technical shape converges with the CLA-service plan (spec 044, rev 5):

```mermaid
flowchart LR
    subgraph v1 ["EasyCLA v1 (AWS)"]
        DDB[("DynamoDB<br/>companies + signatures<br/>(signature_acl = managers)")]
        V4["EasyCLA v4 API"]
    end
    subgraph platform ["LFX One platform"]
        SH["lfx-v1-sync-helper<br/>(streams + Meltano)"]
        KV["NATS KV<br/>cla-v1-objects"]
        MS["member-service<br/>predicate: membership<br/>OR CLA-referenced"]
        FGA["OpenFGA<br/>cla_group / cla_ccla /<br/>cla_ecla / cla_icla"]
        QS["query-service"]
    end
    DDB --> SH --> KV
    KV -->|"projector: ACL diff →<br/>manager / signatory tuples"| FGA
    KV -.->|"b2b_org_ensure for<br/>unknown accounts"| MS
    MS -->|"b2b_org docs + reindex"| QS
    SS["Self Serve"] -->|"org list + selector union<br/>(org read OR cla_ccla manager)"| QS
    SS -->|"/access-check"| FGA
    SS -->|"CLA tabs: bridge (v4 via gateway)<br/>until parity gate flips flags"| V4
```

- **Org catalogue:** member-service widens its `b2b_org` predicate from "has a Membership Asset" to "membership **or** CLA-referenced", plus a `lfx.member.b2b_org_ensure` request so an unknown account can be onboarded on demand; full `b2b_org` reindex afterwards. member-service remains the one Salesforce org service.
- **Permissions:** new FGA types per spec 044 — `cla_group`, `cla_ccla` (`manager`, `signatory`), `cla_ecla`, `cla_icla` — confirmed on the call as needed **in this milestone** regardless of where the data plane lands. Manager grants derive from the signature row's `signature_acl` (the synchronous write), not from ACS; ACS roles feed a dry-run drift report only. Org admin ≠ CLA manager.
- **Lens entry:** the org selector becomes a union — org read (`writer`/`auditor` on `b2b_org`) **or** `manager` on any `cla_ccla` — with a CLA-only view for managers who hold nothing else. Never org-wide read for CLA managers.
- **Console cutover:** hard cut, no parallel operation of the Corporate Console and the Org Lens (agreed on the call). That removes the need for a live ACS↔FGA dual sync; the drift report suffices. The earlier one-time ACS→FGA backfill predates the newly admitted accounts, so a new backfill pass over CLA roles is required (spec 044 story H).

### What changed vs. the earlier version of this proposal

| Earlier proposal (this doc, pre-call) | Now |
|---|---|
| New `cla_manager` relation on `b2b_org` | Superseded: dedicated CLA FGA types (`cla_ccla#manager` etc.), manager from `signature_acl` |
| Admit EasyCLA orgs "tagged non-member" | Confirmed and sharpened: they become real B2B accounts (sales-ops approval pending); member-service predicate widened |
| CLA tabs call v4 via gateway; no replication in M3 | Converges with spec 044's own rollout: pages run on the bridge behind per-env flags and flip to the CLA service only at zero parity differences |

## Open items

1. **Sales-ops approval** — Eric → Mindy; the blocker for the catalogue change. Heather and Michal to be invited to that conversation.
2. **Architecture review of spec 044's four ADRs** — CLA FGA types, dedicated `cla-v1-objects` bucket, read-plane-first for an external system of record, catalogue predicate + selector rule. Gates the FGA model bump and member-service PR.
3. **M3 sequencing** — the minimum for M3 is the catalogue change, the FGA model + `cla_ccla` tuple projection/backfill, and the selector union (spec 044 stories A, B, C9, D), with the CLA tabs shipping on the existing bridge. The full read-plane migration (search projections, activity log, My CLAs on the service) follows behind the parity-gated flags and is not an M3 dependency.

## Prerequisites already tracked

Data cleanup, needed regardless of the outcome ([parent story](https://github.com/linuxfoundation/lfx-self-serve/issues/2043)):

- [lfx-self-serve#2054](https://github.com/linuxfoundation/lfx-self-serve/issues/2054) — 230 companies with missing/invalid SFID are unreachable under any design
- [lfx-self-serve#2055](https://github.com/linuxfoundation/lfx-self-serve/issues/2055) — legacy `POST /v1/company` still creates companies with no Salesforce link
- [lfx-self-serve#2056](https://github.com/linuxfoundation/lfx-self-serve/issues/2056) — duplicate company rows per SFID break resolution
