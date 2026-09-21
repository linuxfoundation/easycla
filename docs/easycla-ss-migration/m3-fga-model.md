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
- **Projection**: managers of any of an org's **signed** CCLAs (`signature_signed = true`;
  see §5 item 2 for why `signature_approved` is excluded) → one `b2b_org:<org>#cla_admin`
  tuple per user × org (deduped across the org's CLA groups). Remove the tuple when the
  user leaves the last such `signature_acl` of that org.
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

### 4.2 Second blocking constraint: the tuple alone does not reach the consumer

A `cla_admin` tuple is necessary but **not sufficient** — nothing in Self Serve reads it
today, and two specific gates would still refuse a manager who holds only that tuple.
Verified in `lfx-self-serve` at the time of writing:

- **The org selector** builds its roster from `b2b_org_settings` `member:<username>`
  documents and classifies each entry as **`writer` or `auditor` only**
  ([`org-role-grants.service.ts:401-403`](https://github.com/linuxfoundation/lfx-self-serve/blob/main/apps/lfx-one/src/server/services/org-role-grants.service.ts#L401-L403)).
  A `cla_admin` grant produces no roster entry, so the org never appears in the picker.
- **The CLA BFF route** is guarded by `requireOrgLensAccess`
  ([`org-clas.route.ts`](https://github.com/linuxfoundation/lfx-self-serve/blob/main/apps/lfx-one/src/server/routes/org-clas.route.ts)),
  which delegates to `assertOrgLensRead`. That helper accepts a roster grant **or** a
  direct `b2b_org:<uid>#auditor` answer from the authorizer, and nothing else
  ([`org-lens-read-access.helper.ts`](https://github.com/linuxfoundation/lfx-self-serve/blob/main/apps/lfx-one/src/server/helpers/org-lens-read-access.helper.ts)).
  A `cla_admin`-only caller gets a 403.

So Variant B needs **three** changes, not one: the relation, a selector
discovery/materialization path that surfaces `cla_admin` orgs into the picker, and a
CLA-route-specific `cla_admin` gate **that retains the existing auditor gate for non-CLA
lens routes** — a CLA manager must not thereby acquire read access to meetings, ROI or
the people roster, which is exactly the "never grant org-wide read to CLA managers"
constraint in §2.

This does not sink Variant B, but it does change the comparison in §7: the "one relation
versus four types" framing understates Variant B's cost, because the consumer-side work
is real and lands in a third repo. It must be weighed against Variant A with these items
included on Variant B's side of the ledger.

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

   **Therefore the projector gates on `signature_signed`, and on that alone:** project a
   `cla_admin` tuple once the signature is `signature_signed = true`, and remove it when
   the ACL entry is removed or the signature is deleted.

   **`signature_approved` is deliberately *not* part of the gate**, even though §2's
   counting predicate uses it. Invalidation sets `signature_approved = false` and the
   `note` field without touching `signature_acl`
   ([`signatures/repository.go` `InvalidateProjectRecord`](../../cla-backend-go/signatures/repository.go)),
   while the ACS updater reacts only to ACL differences
   ([`v2/dynamo_events/signatures.go:366`](../../cla-backend-go/v2/dynamo_events/signatures.go#L366),
   which logs "No changes in ACL" and exits otherwise). An approval-gated projection
   would therefore strand managers at exactly the wrong moment: v4 still authorizes them
   via ACS, but FGA removes their only route into the lens — so they cannot inspect the
   invalidated agreement's history or initiate a replacement CCLA. Signed-but-unapproved
   is precisely the state in which a manager most needs the surface.

   The consequence is that the tuple set is **not** identical to §2's active-CCLA
   population: it is the slightly larger "has ever signed, ACL intact" set. That
   divergence is intentional and must be stated wherever the two numbers are compared.

   With this gate, the designee case still resolves cleanly: a designee's job is the
   pre-signing window (initiate DocuSign), which runs on the unscoped Sign CLA flow, not
   inside an org's lens, and the moment they have something to see in the lens (a signed
   CCLA) is the moment the gate opens for the ACL entry v4 already wrote for them.
3. **Signatory lens access** — no `cla_admin` tuple, but *not* "no M3 work". Do **not**
   fold signatories into `cla_admin`: the tab would offer manager actions v4 rejects.
   Proper read access is spec 044's `cla_ccla#signatory` at M5.

   Two things are nevertheless true of M3 and must not be read away by the paragraph
   above. First, **M3 ships the signatory flow**: [`spec.md`](../../specs/001-easycla-ss-integration-fable/spec.md)
   FR-030 puts "CCLA signing initiation (signatory flow, including send-by-email)" in
   the parity inventory, and [linuxfoundation/lfx-self-serve#2150](https://github.com/linuxfoundation/lfx-self-serve/issues/2150)
   registers `self_serve_request_corporate_signature:create` with an ACS policy on
   `cla-manager-designee` **and `cla-signatory`**. So the role does get an M3
   enforcement point on the write path, even though it gets no FGA relation. Second,
   FR-031 governs org-lens access by "CLA-manager/signatory authority", which the
   manager-only selector union does not satisfy.

   The unresolved part is therefore **how a signatory reaches the lens at all**, not
   whether they may read the agreement once inside (spec 044 computes
   `cla_ccla#auditor` as `manager or signatory or …`). That is an open Product +
   Architecture decision tracked in
   [`m3-org-visibility.md` §4.2](m3-org-visibility.md) and narrowed by §4.5 — if CCLA
   signing starts from a dedicated CLA landing page rather than the Org Lens, a
   signatory never needs selector access. It is not decided here, and the claim that
   the `cla-signatory` ACS role "is checked by no endpoint" describes only today's
   code: the two current mentions in
   [`v2/sign/handlers.go:153`](../../cla-backend-go/v2/sign/handlers.go#L153) and
   [`v2/self_serve_sign/handlers.go:132`](../../cla-backend-go/v2/self_serve_sign/handlers.go#L132)
   are error-message strings on an org-service lookup failure, not authorization
   checks — and [lfx-self-serve#2150](https://github.com/linuxfoundation/lfx-self-serve/issues/2150) changes that.
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

**This requirement is in tension with Variant A's read model, and that tension is
unresolved.** Raised by Luis Moriguerra in review (2026-09-21). Spec 044 computes
`cla_ccla#auditor` as `manager or signatory or auditor from b2b_org or auditor from
cla_group` — a **per-agreement** relation
([m3-org-visibility.md](m3-org-visibility.md) §4.2). A manager of CLA group X therefore
gets no read on the same company's sibling group Y unless they hold `b2b_org#auditor`,
which §4.2 of that document explicitly forbids for CLA managers ("Never org-wide read for
CLA managers"). So Variant A as written does not satisfy the requirement above, while
Variant B preserves today's behaviour in M3 through the bridge but defines no M5 read
model at all. Neither variant, as currently specified, carries company-wide read into M5.

**The gap is not marginal.** Measured read-only against the prod DynamoDB mirror
(2026-09-21, appendix method): **318 organizations hold more than one signed CCLA group**,
and within those, **849 of 1,033 manager × org pairs (82%) manage only a subset of their
organization's groups**. Under Variant A as written, that 82% loses visibility it has
today. Luis measured 287 orgs and 815 of 980 pairs (83%) with slightly different filters;
the two derivations agree on the conclusion.

**Grain: per Salesforce org, not per EasyCLA company row.** `GetCompanyClaGroups` resolves
its rows through `GetCompaniesByExternalID(ctx, companySFID, true)`
([`v2/company/service.go:1325`](../../cla-backend-go/v2/company/service.go#L1325)), which
fans out over every company record sharing that SFID — so a single Salesforce org can
carry multiple signing entities with divergent ACLs. Any per-company read relation has to
be keyed on the SFID to match what v4 returns today. Luis counts 4 SFIDs carrying multiple
signing entities, 3 with divergent ACLs across 19 groups.

**Proposed resolution, for the ADR.** Luis's conclusion is that a **per-company CLA read
relation is needed in both M3 and M5**, which reframes §7's question from "Variant A or
Variant B" to *where that relation lives*:

- **On `b2b_org`** (`cla_admin`, extended as `... or cla_admin from b2b_org`) — carries
  §4.1's member-service coupling forward into M5.
- **On a CLA-owned object** (`cla_company#manager`, extended as `... or manager from
  cla_company`) — no member-service coupling, but adds a type and reopens P2's "no CLA
  object types before M5" on its own terms.

Not decided here. This is for the spec-044 ADR review (§7), which is the venue that can
settle it.

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

> **Variant B (single `cla_admin` relation) for M3, conditional on §4.1 and §4.2 being resolved;
> Variant A (dedicated CLA types) for M5.**

**This form may be the wrong shape of question.** §6 records a tension neither variant
resolves: Variant A's per-agreement read model does not preserve the company-wide read
Product has confirmed as intended, and Variant B defines no M5 read model at all. If the
ADR accepts that a per-company CLA read relation is needed in both milestones, the
decision becomes *where that relation lives* — on `b2b_org` or on a CLA-owned object —
rather than a choice between the two variants. The recommendation above stands as the
starting position; §6 states what would displace it.

The ADR should record three things alongside it:

1. **The §4.1 owner and resolution** (and, with it, the §4.2 consumer-side work — selector
   materialization plus a CLA-route gate, which lands in `lfx-self-serve`) — either member-service accepts the
   `ExcludeRelations` change with a deploy-order constraint and regression test, or the
   grant moves to a CLA-owned object. Variant B is not safe to build until this is
   answered.
2. **The bridge ID-translation contract — and only then an ACS re-grant owner.** Under
   Path B of the org import ([m3-org-visibility.md](m3-org-visibility.md) open item 5)
   the lens holds a new B2B SFID while ACS scopes and `company_external_id` still carry
   the old one. Whether that needs an ACS migration depends entirely on a contract
   nobody has written down yet:

   - **Bridge translates (new → old) before calling v4** — as
     [lfx-self-serve#2750](https://github.com/linuxfoundation/lfx-self-serve/issues/2750)
     describes: requests still match existing ACS scopes, **no re-grant is needed**, and
     the cost is that every bridged path must translate without exception.
   - **New ID passed through untranslated** — v4 returns 403 for managers FGA has
     already admitted, and an ACS-scope migration with a named owner becomes mandatory.

   **Decide and document the boundary contract first**; assign the ACS re-grant owner
   only if the second option is chosen. Under Path A (the working assumption — IDs are
   preserved) neither arises **for companies that store a real old-org SFID**, which is
   a further reason to settle Path A/B first. Path A is not a blanket exemption: the
   530 `lf`-shaped IDs have no Salesforce record to preserve, and the 374 domain-linked
   orgs point at a B2B account whose ID differs from the stored value
   ([m3-org-visibility.md](m3-org-visibility.md) §2.1 and §2.3). Both sets need a rewrite
   or translation under either path, and both therefore still need an owner.
3. **The FGA-vs-ACS disagreement rule**, defined before cutover: what the system does
   when FGA lets someone into the lens but ACS refuses the API call. "FGA gates the UI,
   ACS gates the APIs" describes the split but does not say which wins, what the user
   sees, or how the drift is reported. Tracked as open item 6 in
   [m3-org-visibility.md](m3-org-visibility.md) §5.
