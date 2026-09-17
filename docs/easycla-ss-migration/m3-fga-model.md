<!-- Copyright The Linux Foundation and each contributor to CommunityBridge.
SPDX-License-Identifier: CC-BY-4.0 -->

# M3 FGA model — one org-level relation, UI-only

**Status**: proposal, for the spec-044 ADR review (simplifies the M3 slice of the model agreed on the 2026-09-10 architecture call)
**Owner**: Michal (engineering)
**Related**: [m3-org-visibility.md](m3-org-visibility.md) §4.2 (the four-type variant this simplifies) · [role-mapping-feasibility.md](role-mapping-feasibility.md)

**Spec 044** = the [lfx-v2-cla-service plan](https://docs.google.com/document/d/1hyWZUE_kofeAjVSmeXsRXtPSxSsj7uWvRj3tTNgwdik/edit)
(Luis, rev 5, 2026-09-10): a new platform CLA service that mirrors EasyCLA's DynamoDB into
OpenSearch/OpenFGA and proxies writes back to v4. Its spec package
`specs/044-lfx-v2-cla-service/` (branch `044-lfx-v2-cla-service`) is not yet published in
any `linuxfoundation` repo; the recap doc above is the citable source.

**In one line**: M3 needs exactly one thing from OpenFGA — *CLA managers can enter their org in the Org Lens and see the EasyCLA tab* — so M3 adds exactly one relation, and everything below the tab stays with the existing v4 APIs under ACS.

**Division of labor — FGA gates the UI, ACS gates the APIs.** OpenFGA decides only what
the Self Serve UI shows (org selector entry, EasyCLA tab). Every EasyCLA API call is
authorized by ACS, unchanged: gateway → ACS warden → `X-ACL` header → v4 scope check.
FGA never authorizes an API call; ACS never decides what the UI renders. This holds for
the whole M3→M5 bridge period.

---

## 1. The one requirement

Most CLA managers hold no membership-based grant, so without a new FGA grant they open
Self Serve and have no organization to select (gate 2 in
[m3-org-visibility.md](m3-org-visibility.md) §1). The tab's *contents* don't need FGA in
M3: the CLA pages run on the bridge (Self Serve → EasyCLA v4 via the API gateway), where
every request is already enforced by ACS.

## 2. The model

One new relation on the existing org type. No new object types in M3.

```
type b2b_org
  relations
    ...existing (writer, auditor, ...)
    define cla_admin: [user]          # working name; "cla_manager" also fits
```

| Question | Answered by | How |
|---|---|---|
| Does org X appear in the user's selector? | FGA | user holds any org relation, `cla_admin` now included |
| Does the EasyCLA tab render for org X? | FGA | `cla_admin` (or an org read relation) |
| Which CCLAs are listed, which actions work? | ACS via the v4 bridge | unchanged — gateway warden check + `X-ACL` scopes, per request |

`cla_admin` grants **nothing else**: no org read, no People/membership access. A user
holding only `cla_admin` gets the CLA-only view — the org appears in their selector and
the EasyCLA tab renders; every other section (People, memberships, key contacts,
meetings) is hidden or empty, because query-service fails closed without a `b2b_org`
read relation and member-service routes require `auditor`/`writer`. On the API side they
still reach CLA data only: v4 authorizes them via their existing ACS `cla-manager` role
over the bridge; non-CLA org APIs deny them. Org admin ≠ CLA manager stays true in both
directions ("never grant org-wide read to CLA managers", spec 044 Q4).

## 3. Tuple lifecycle

- **Source**: the CCLA signature's `signature_acl` (DynamoDB) — the synchronous write,
  same source for backfill and ongoing projection. Never derived from ACS.
- **Projection**: managers of any of an org's CCLAs → one `b2b_org:<org>#cla_admin` tuple
  per user × org (deduped across the org's CLA groups). Remove the tuple when the user
  leaves the last `signature_acl` of that org.
- **Cross-check**: the read-only ACS drift report (spec 044 item 22) compares ACS
  `cla-manager` roles against the tuples; run it periodically, not once — ACS keeps being
  written by v4 for as long as the bridge exists.

## 4. What this defers, and what it costs

Spec 044's four CLA types (`cla_group`, `cla_ccla` with `manager`/`signatory`,
`cla_ecla`, `cla_icla`) earn their keep only when FGA starts **enforcing** — per-agreement
read authorization on the query plane, at M5. Nothing in M3 uses them:

| Spec-044 type | First actually needed |
|---|---|
| `cla_ccla` per-agreement relations | M5 — query-plane enforcement (`#auditor`) |
| `cla_group` | M4 — project lens |
| `cla_ecla`, `cla_icla` | M5 — Me lens works today on in-handler ownership checks, no tuples |

**Cost**: a second FGA model bump at M5, when the per-agreement types replace
`cla_admin`. Acceptable because the M5 bump happens under spec 044 regardless, and the
tuples are projections — migrating is a re-backfill from `signature_acl`, not a data
migration.

**Side benefit**: the relation sits on whatever org object exists, so the M3 model no
longer depends on the open "EasyCLA orgs become B2B Salesforce accounts" decision
([linuxfoundation/easycla#5210](https://github.com/linuxfoundation/easycla/pull/5210)).

## 5. Explicitly out of scope for this model

Unchanged open items — this proposal solves none of them, under either model shape:

1. **CLA-only view** — UI work to hide membership sections for `cla_admin`-only users.
2. **Designees** — `cla-manager-designee` exists only in ACS, with no `signature_acl`
   entry before the CCLA is signed, so no tuple can be projected — and none is needed:
   a designee's only job is the pre-signing window (initiate DocuSign), which runs on the
   unscoped Sign CLA flow, not inside an org's lens. The moment they have something to
   see in the lens (a signed CCLA) is the moment `signature_acl` — and therefore their
   `cla_admin` tuple — exists, because v4 writes the initiating designee as the ACL's
   sole initial entry ([sign/service.go:2949](../../cla-backend-go/v2/sign/service.go#L2949)).
3. **Signatory lens access** — no M3 work: signatories have no console access today
   (email-only DocuSign interaction; the `cla-signatory` ACS role is checked by no
   endpoint). Proper read access is spec 044's `cla_ccla#signatory` at M5. Do **not**
   fold signatories into `cla_admin` — the tab would offer manager actions v4 rejects.
4. **ACS/FGA parity** — FGA gates the UI, ACS enforces the API, for the whole M3→M5
   bridge period; both are fed by the same v4 write (`signature_acl` synchronous, ACS
   role asynchronous), and disagreement handling remains open item 6 there.

## 6. Do not add per-manager filtering to v4

"The API decides the contents" means the **existing** enforcement: an org-scoped user
sees the org's CLA list; writes are gated per project|org scope. Building new per-user
filtering into v4 would diverge from today's Corporate Console behavior and violate the
program rule that Self Serve mirrors v4's decisions rather than re-deriving them
([role-mapping-feasibility.md](role-mapping-feasibility.md) §3/§5).

## 7. Decision venue

The spec-044 ADR review (open item 3 in
[m3-org-visibility.md](m3-org-visibility.md) §5): adopt this as the M3 model, with the
four CLA types moved to the M5 ADR where they become enforcing.
