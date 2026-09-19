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
    define cla_admin: [user]
```

The relation is named **`cla_admin`** throughout this document and that is the name
proposed for the ADR. `cla_manager` was the alternative but is rejected: it collides
with the existing ACS `cla-manager` role, and the two are deliberately not the same
thing — the tuple is projected from `signature_acl`, not from the ACS role (§3).

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
| `cla_group` | M5 — project lens does not read CLA data through FGA before then |
| `cla_ecla`, `cla_icla` | M5 — Me lens works today on in-handler ownership checks, no tuples |

**Cost**: a second FGA model bump at M5, when the per-agreement types replace
`cla_admin`. Acceptable because the M5 bump happens under spec 044 regardless, and the
tuples are projections — migrating is a re-backfill from `signature_acl`, not a data
migration.

**Model shape, not independence**: the relation is defined on a specific object type.
As drafted that is `b2b_org`, which means the model still requires each EasyCLA company
to be represented by a B2B organization — without that record there is nothing to attach
the relation to. The earlier claim that this made M3 independent of the "EasyCLA orgs
become B2B Salesforce accounts" decision
([linuxfoundation/easycla#5210](https://github.com/linuxfoundation/easycla/pull/5210))
overstated it: what the single relation avoids is new *CLA object types*, not the org
record itself. Treat the B2B org record as a **precondition** of this model.

### 4.1 Blocking constraint: `b2b_org` tuples are reaped by member-service

**A `cla_admin` relation placed on `b2b_org` would be silently deleted**, and this must
be resolved before Variant B can be chosen. Verified in code:

- member-service publishes an `update_access` FGA message on every `b2b_org` create and
  update, from three paths — the writer orchestrator, the CDC (Salesforce change feed)
  consumer, and the org-settings writer — plus `/admin/reindex` and backfill
  ([`internal/service/messaging.go` `BuildB2BOrgFGAMessage`](https://github.com/linuxfoundation/lfx-v2-member-service/blob/main/internal/service/messaging.go)).
- fga-sync treats that message as a **full sync** of the object: `SyncObjectTuples`
  reads every live tuple on the object and deletes any not in the desired set
  ([`fga.go` `SyncObjectTuples`](https://github.com/linuxfoundation/lfx-v2-fga-sync/blob/main/fga.go)).
- Only two things survive a relation the publisher does not know about: membership in
  the message's `ExcludeRelations` list, or a `team:`-prefixed subject. `cla_admin`
  tuples carry `user:` subjects, so **neither applies**. The current exclude list is
  hardcoded to `parent`, `child`, and conditionally `global_org_admin`, `membership`,
  `writer`, `auditor`.

The failure mode is silent: managers lose lens access on the next unrelated org update,
with no error surfaced anywhere. Making Variant B safe therefore requires a
**member-service change** (add `cla_admin` to `ExcludeRelations` on every publish path),
a **deploy-order constraint** (that change ships before any tuple backfill), and a
**regression test** that an org update preserves CLA tuples. If member-service will not
own that, the manager grant belongs on a CLA-owned object instead — as §5 notes, a single
CLA-owned type in M3 avoids the reaping problem but reopens the "no CLA types before M5"
decision the same way Variant A does.

## 5. Explicitly out of scope for this model

Unchanged open items — this proposal solves none of them, under either model shape:

1. **CLA-only view** — UI work to hide membership sections for `cla_admin`-only users.
2. **Designees — and the pre-signing window.** The `cla-manager-designee` role itself
   exists only in ACS. But the earlier claim that there is "no `signature_acl` entry
   before the CCLA is signed" is **wrong**, and the projection must account for it: v4
   creates the CCLA row with `SignatureSigned: false` and writes the initiating user's
   LFID into `SignatureACL` on that same row
   ([sign/service.go:2941 and :2953](../../cla-backend-go/v2/sign/service.go#L2941)),
   persisting it via `populateSignURL`. A projection keyed on "appears in
   `signature_acl`" would therefore grant lens access to a **pending, unsigned** CCLA.

   **Therefore the projector gates on the active state, not on ACL membership alone:**
   project a `cla_admin` tuple only while the signature is `signature_signed = true`
   **and** `signature_approved = true`, and remove it when the company's last CCLA in
   that state exits it. This is the same active-CCLA predicate §2 uses for counting, so
   the tuple set and the population figures stay consistent by construction.

   With that gate, the designee case resolves cleanly: a designee's job is the
   pre-signing window (initiate DocuSign), which runs on the unscoped Sign CLA flow, not
   inside an org's lens, and the moment they have something to see in the lens (a signed
   CCLA) is the moment the gate opens for the ACL entry v4 already wrote for them.
3. **Signatory lens access** — no M3 work: signatories have no console access today
   (email-only DocuSign interaction; the `cla-signatory` ACS role is checked by no
   endpoint). Proper read access is spec 044's `cla_ccla#signatory` at M5. Do **not**
   fold signatories into `cla_admin` — the tab would offer manager actions v4 rejects.
4. **ACS/FGA parity** — FGA gates the UI, ACS enforces the API, for the whole M3→M5
   bridge period; both are fed by the same v4 write (`signature_acl` synchronous, ACS
   role asynchronous), and disagreement handling remains open item 6 there.

## 6. What v4 already enforces — two tiers, and M3 replicates both

"The API decides the contents" means the **existing** enforcement, which is not uniform.
Reads and writes sit at different widths:

| Tier | Gate | Effective scope |
|---|---|---|
| **Read / list** | `IsUserAuthorizedForOrganization(..., ALLOW_ADMIN_SCOPE)` | **Company-wide** |
| **Write — approval list / signature** | `IsUserAuthorizedForProjectOrganizationTree(..., DISALLOW_ADMIN_SCOPE)`, **then** `CurrentUserInACL` on the signature | **Per CLA group**, twice over |
| **Write — CLA-manager administration** | `IsUserAuthorizedForProjectOrganizationTree(..., DISALLOW_ADMIN_SCOPE)` **only** | **Per CLA group**, once |

The two write rows are not interchangeable. `CurrentUserInACL` is applied at exactly two
call sites in the backend — both on the approval-list/signature path
([`signatures/service.go:523`](../../cla-backend-go/signatures/service.go#L523),
[`v2/signatures/handlers.go:1499`](../../cla-backend-go/v2/signatures/handlers.go#L1499)).
CLA-manager create and delete enforce the project|organization tree and nothing else
([`v2/cla_manager/handlers.go:65`](../../cla-backend-go/v2/cla_manager/handlers.go#L65),
[`:122`](../../cla-backend-go/v2/cla_manager/handlers.go#L122)), so a manager scoped to a
CLA group can add or remove managers there without being in any signature's ACL. An M5
parity model must preserve this endpoint-specific difference rather than applying the
stricter ACL rule uniformly.

The read tier is company-wide because the shared scope matcher accepts a
`project|organization` scope on its **organization half alone**, ignoring the project
half ([lfx-kit `auth/user.go` `IsUserAuthorizedForOrganizationScope`](https://github.com/LF-Engineering/lfx-kit/blob/main/auth/user.go)),
while CLA-manager ACS roles are granted as `project|org` pairs
([`v2/dynamo_events/cla_manager.go:205`](../../cla-backend-go/v2/dynamo_events/cla_manager.go#L205)).
So a manager appointed for one CLA group at a company passes the org-scope check on that
company's other CLA groups: `GetCompanyClaGroups`
([`v2/company/handlers.go:134`](../../cla-backend-go/v2/company/handlers.go#L134)) and the
other listing endpoints return the company's full CLA set.

The write tier is narrow and cannot be widened by staff: the approval-list update checks
the project|org tree with **admin scope disallowed**
([`v2/signatures/handlers.go:121`](../../cla-backend-go/v2/signatures/handlers.go#L121)),
and the service layer then requires the caller to be in that specific signature's ACL
([`signatures/service.go:523`](../../cla-backend-go/signatures/service.go#L523),
[`v2/signatures/handlers.go:1499`](../../cla-backend-go/v2/signatures/handlers.go#L1499)).
Managing one CLA group's approval list therefore requires membership in *that* signature's
`signature_acl`; a manager on a sibling CLA group is refused.

**M3 replicates both tiers unchanged** — this is feature parity, and it is also the
cheapest option. The shipped M3 endpoints already encode exactly these rules: the org CLA
list documents auth as "`organization` scope for the `companySFID`, or any
`project|organization` scope whose organization half matches", and the manager/
acknowledgment write ops as "`project|organization` tree scope for the project/company
pair, LF admin disallowed" ([`docs/M3_ORG_LENS_API.md`](../M3_ORG_LENS_API.md)). Parity is
the result of *not* writing new code.

**Do not add per-manager filtering to v4.** Building new per-user filtering would diverge
from today's Corporate Console behavior and violate the program rule that Self Serve
mirrors v4's decisions rather than re-deriving them
([role-mapping-feasibility.md](role-mapping-feasibility.md) §3/§5). Nor is narrowing the
read tier an EasyCLA change at all — the breadth lives in the shared lfx-kit matcher used
by every LFX service, so it would mean changing shared platform code or bolting
CLA-specific filtering onto v4. The concrete version of this proposal, and why it fails on
its own terms, is recorded as a rejected alternative below.

**Cross-project visibility: settled as intended product behavior, not a gap.** Eric raised
it in review of this proposal as a possible legal concern — a CLA manager for one project
can see that their employer holds agreements with other projects. Scoped correctly it is a
**read-tier** question only; no cross-group writes are possible. Put to Product
(Heather Willson, 2026-09-18) as a choice between full read-only, listed-but-not-openable,
and hidden entirely: **full read-only is the decision**, on the grounds that a CLA's
information should not be hidden, precisely because someone may need to become a CLA
manager or get authorized under their company's CCLA — and they can do neither if they
cannot see the agreement exists and who manages it. No Legal escalation, and no M3 work:
this is what v4 already does.

**This constrains M5, it is not deferred to it.** An earlier revision of this section
parked the question for M5 on the assumption that `cla_ccla#auditor` would narrow reads per
agreement once FGA enforces. That reading is now wrong: since company-wide CLA visibility
is deliberate, M5's per-agreement read model must **preserve** it rather than narrow it —
`#auditor` has to keep admitting a company's non-manager CLA admins to read its other CLA
groups. Carry this into the M5 ADR as a requirement on the model, not an open item.

**Rejected alternative: filter the list per CLA group in v4.** Considered — the response
already carries `claManagers` per row from `signature_acl`
([`v2/company/service.go:1522`](../../cla-backend-go/v2/company/service.go#L1522)), so
filtering to "CLA groups where the caller is a manager" would cost ~10 lines and no extra
queries. Rejected on three counts, before the Product decision made it moot: it breaks
`needsClaManager` (an agreement with **zero** managers can match no caller, so the rows the
field exists to surface would vanish) and newly appointed managers (empty lens, no route
forward); it puts FGA and the API at different widths, since `cla_admin` is org-level by
construction; and it would make reads *narrower than writes* in the foundation case, where
the write gate is a project|org **tree** check — a manager appointed at foundation level can
write on a child project's agreement whose own ACL they are not in.

**Why `cla_admin` is deliberately coarser than the write gate.** The relation is projected
from `signature_acl` deduped across the org's CLA groups, so one ACL membership grants
lens entry to the org. That matches the read tier exactly, and it keeps the projection to
one tuple per user × org. A per-CLA-group relation would have to track v4's write gate,
giving two systems that can disagree — the parity problem in open item 6 of
[m3-org-visibility.md](m3-org-visibility.md) §5.

## 7. Decision venue

The spec-044 ADR review (open item 3 in
[m3-org-visibility.md](m3-org-visibility.md) §5): adopt this as the M3 model, with the
four CLA types moved to the M5 ADR where they become enforcing.

**Proposed form of the decision**, so the "no CLA types before M5" reopen is explicitly
scoped rather than implied:

> **Variant B (single `cla_admin` relation) for M3, conditional on §4.1 being resolved;
> Variant A (dedicated CLA types) for M5.**

The ADR should record three things alongside it:

1. **The §4.1 owner and resolution** — either member-service accepts the
   `ExcludeRelations` change with a deploy-order constraint and regression test, or the
   grant moves to a CLA-owned object. Variant B is not safe to build until this is
   answered.
2. **An owner for the ACS re-grant.** Under Path B of the org import
   ([m3-org-visibility.md](m3-org-visibility.md) open item 5), FGA would admit an
   existing manager while v4 returns 403, because their ACS scope is still pinned to the
   old company ID. This is currently buried in that open item and needs to be its own
   tracked work item. Under Path A (the working assumption — IDs are preserved) the
   problem does not arise, which is another reason to settle Path A/B first.
3. **The FGA-vs-ACS disagreement rule**, defined before cutover: what the system does
   when FGA lets someone into the lens but ACS refuses the API call. "FGA gates the UI,
   ACS gates the APIs" describes the split but does not say which wins, what the user
   sees, or how the drift is reported. Tracked as open item 6 in
   [m3-org-visibility.md](m3-org-visibility.md) §5.
