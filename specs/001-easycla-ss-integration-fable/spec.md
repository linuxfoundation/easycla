# Feature Specification: EasyCLA → LFX Self Serve Integration

**Feature Branch**: `001-easycla-ss-integration`
**Created**: 2026-07-11
**Status**: Reviewed 2026-07-15 — UI-first approach approved; see "Program review outcomes" below
**Stakeholders**: Heather (lead PM) · Nuno (UI/UX design) · Michal (engineering) · Kieran (strategy)
**Input**: User description: "Integrate EasyCLA with LFX Self Serve: migrate Contributor Console and Corporate CLA Console functionality into Self Serve with feature parity, evaluate moving EasyCLA APIs to Kubernetes as a V2 service, and evaluate DynamoDB → Postgres migration. Break into six milestones."

**Companion documents** (this directory):

| Doc | Milestone |
|-----|-----------|
| [00-overview-fable.md](00-overview-fable.md) | Program overview, current-state architecture, cross-cutting risks |
| [01-milestone-read-only-me-lens-fable.md](01-milestone-read-only-me-lens-fable.md) | M1 — Read-only "My CLAs" in Me lens |
| [02-milestone-sign-icla-fable.md](02-milestone-sign-icla-fable.md) | M2 — Proactive CLA signing entry in Self Serve (hands off to Contributor Console; revised 2026-08-04) |
| [03-milestone-sign-ecla-fable.md](03-milestone-sign-ecla-fable.md) | M3 — Sign ECLAs in Self Serve; retire Contributor Console (scope under revision, see doc) |
| [04-milestone-ccla-org-lens-fable.md](04-milestone-ccla-org-lens-fable.md) | M4 — CCLA management in Organization lens; retire Corporate Console |
| [05-milestone-project-lens-pcc-fable.md](05-milestone-project-lens-pcc-fable.md) | M5 — EasyCLA project administration in Project lens; remove from PCC |
| [06-milestone-k8s-v2-api-fable.md](06-milestone-k8s-v2-api-fable.md) | M6 — EasyCLA API on Kubernetes as a V2 service; DynamoDB → Postgres evaluation |

## User Scenarios & Testing *(mandatory)*

### User Story 1 (Milestone 1) - Contributor views their CLAs in Self Serve (Priority: P1)

A contributor logs into LFX Self Serve and, under the Me lens, opens "My CLAs". They see every ICLA they have signed and every currently valid ECLA (employee acknowledgement), each showing the project/CLA group, signing date, and status. For signed ICLAs they can download the signed PDF. Actions that create new agreements (signing) still route to the existing Contributor Console.

**Why this priority**: Lowest-risk, read-only slice that delivers immediate user value, establishes the SS↔EasyCLA integration pattern (auth, identity mapping, API plumbing) that every later milestone reuses, and requires no changes to signing, roles, or consoles.

**Independent Test**: Sign an ICLA and an ECLA in the existing Contributor Console with a test user, then log into Self Serve as that user and verify both agreements appear with correct metadata and the ICLA PDF downloads.

**Acceptance Scenarios**:

1. **Given** a logged-in user who has signed at least one ICLA, **When** they open My CLAs in the Me lens, **Then** each signed ICLA is listed with project name, date signed, and a working signed-PDF download.
2. **Given** a logged-in user with a valid ECLA, **When** they open My CLAs, **Then** the ECLA is listed with company name, project, and acknowledgement date, and no PDF download is offered (ECLAs have no signed document).
3. **Given** a logged-in user with no CLA history, **When** they open My CLAs, **Then** an empty state explains what CLAs are and links to documentation.
4. **Given** any user viewing My CLAs, **When** they look for signing actions, **Then** none exist in Self Serve; any "sign" pointers link out to the Contributor Console.

---

### User Story 2 (Milestone 2) - Contributor proactively starts CLA signing from Self Serve, hands off to Contributor Console (Priority: P2)

**Revised 2026-08-04** (per Heather, PM, Slack): the Contributor Console is **not** cut over or retired in M2, and the PR-check remediation link keeps pointing at the Console unchanged. Instead, M2 adds a new, additive entry point: a contributor can go to Self Serve without any PR context, pick a CLA Group (and possibly a GitHub org/repo) from a dropdown, choose whether they want to sign an ICLA or acknowledge an ECLA, and be handed off to the existing Contributor Console to complete the signing there — for both the ICLA and CCLA paths. SS does not run the signing ceremony itself in M2.

**Why this priority**: Delivers a self-initiated signing path (no failing PR required) using the Console's existing, already-working signing flow — lower-risk than moving DocuSign/ECLA logic natively into SS, while still giving contributors a reason to start using Self Serve.

**Independent Test**: As a logged-in Self Serve user with no open PR, use the new "Sign a CLA" entry to pick a CLA Group and ICLA/ECLA, and verify landing in the Contributor Console pre-scoped to that CLA Group/sign type, ready to complete signing there.

**Acceptance Scenarios**:

1. **Given** a logged-in Self Serve user with no PR context, **When** they open the new proactive signing entry, **Then** they can pick from CLA Groups available to them (narrowing by GitHub org/repo where applicable).
2. **Given** a user who picked a CLA Group and ICLA, **When** they proceed, **Then** they are handed off to the Contributor Console's individual-signing flow, scoped to that CLA Group.
3. **Given** a user who picked a CLA Group and ECLA/CCLA, **When** they proceed, **Then** they are handed off to the Contributor Console's corporate flow, scoped to that CLA Group.
4. **Given** the existing PR-check status link, **When** a contributor clicks it, **Then** they still land directly in the Contributor Console as today — unaffected by the new SS entry point.

---

### User Story 3 (Milestone 3) - Employee acknowledges a corporate CLA via Self Serve (Priority: P3)

**Note (2026-08-04)**: this story's premise (native ECLA signing in SS, Console retirement) was written assuming M2 moved ICLA signing natively into SS. M2 has since been revised to hand off to the Console instead (see User Story 2) and the Console is not cut over in M2 — so this story's scope needs to be revisited with Heather before it's current. Heather has also flagged that, for the CCLA path specifically, **how the user selects their organization in this flow may need to change** — noted here as an open question, not yet resolved. The narrative below is left as originally drafted and should be treated as unconfirmed pending that follow-up.

A contributor whose employer has signed a CCLA clicks the failed PR check, lands in Self Serve, chooses "Corporate contributor", selects their company, and — being on the company's Approved List — confirms the employee acknowledgement (ECLA). If they are not on the Approved List they can notify their company's CLA managers; if their company has not signed a CCLA they can start the CLA-manager designation / company-admin invitation flow. After this milestone the Contributor Console is retired.

**Why this priority**: Completes contributor-side parity but depends on the M2 plumbing and touches the EasyCLA role machinery (CLA manager designee, signatory invitations), which is the highest-complexity contributor flow.

**Independent Test**: With a dev company holding a signed CCLA and an Approved List, drive a test contributor through the corporate flow in Self Serve end-to-end; separately drive the not-on-list and no-CCLA branches.

**Acceptance Scenarios**:

1. **Given** an approved employee of a CCLA-signed company, **When** they confirm the acknowledgement, **Then** an ECLA record is created and the PR check passes.
2. **Given** an employee not on the Approved List, **When** they request authorization, **Then** selected CLA managers are notified and the contributor sees a confirmation.
3. **Given** a company with no signed CCLA, **When** the contributor initiates CLA setup, **Then** they can either become CLA manager designee (with LF login) or invite a company admin, with the same outcomes as today's console.
4. **Given** a CLA group that requires an ICLA in addition to the CCLA, **When** the employee completes the ECLA, **Then** they are prompted to sign the ICLA before being returned to the PR.
5. **Given** a sanctioned company (per sanctions screening), **When** an employee attempts the corporate flow, **Then** the flow is blocked with the same messaging as today.

---

### User Story 4 (Milestone 4) - CLA manager administers their company's CLAs in the Organization lens (Priority: P4)

A CLA manager opens the Organization lens in Self Serve and manages everything they do in the Corporate Console today: view signed CCLAs per project, sign new CCLAs (as/with a CLA signatory), maintain Approved Lists (email, domain, GitHub org/username, GitLab), view employee acknowledgements, add/remove CLA managers, toggle auto-create-ECLA, view activity logs, and export CSVs. After parity is reached the Corporate Console is retired.

**Why this priority**: Large surface (the Corporate Console is ~160 frontend components plus its own GraphQL BFF) and depends on role-mapping decisions, but is used by a smaller, expert audience — parity risk is more contained than contributor flows.

**Independent Test**: Execute the full Corporate Console regression suite (sign CCLA, edit Approved List, add/remove manager, view acknowledgements, export CSV) against the Organization lens implementation and compare outcomes.

**Acceptance Scenarios**:

1. **Given** a user holding the CLA manager role for a company, **When** they open the Organization lens, **Then** they see CLA management for exactly the companies/projects their role covers.
2. **Given** a CLA manager editing an Approved List, **When** they add a domain entry with auto-create-ECLA enabled, **Then** matching employees gain acknowledgements exactly as the Corporate Console produces today.
3. **Given** a CLA signatory, **When** they initiate CCLA signing, **Then** the e-signature ceremony completes and the signed CCLA (with PDF) appears for the company/project.
4. **Given** a user with no CLA role in a company, **When** they open the Organization lens, **Then** CLA management functions for that company are not accessible.

---

### User Story 5 (Milestone 5) - Project admin configures EasyCLA in the Project lens (Priority: P5)

A project administrator manages EasyCLA setup from Self Serve's Project lens instead of PCC: create/edit CLA groups (ICLA/CCLA/ICLA-required flags), manage CLA templates (preview and regenerate PDFs), connect GitHub organizations (EasyCLA GitHub App) and enroll repositories, manage Gerrit instances and GitLab groups, view/invalidate signatures, and view events. EasyCLA functionality is then removed from PCC.

**Why this priority**: Admin-only audience, smallest user base, and PCC v1 continues to work meanwhile; sequenced after the org lens because it reuses its role-mapping and reporting patterns.

**Independent Test**: Recreate a full project onboarding (create CLA group, select template, connect GitHub org, enroll repos) in the Project lens on dev and verify PR gating works on an enrolled repo.

**Acceptance Scenarios**:

1. **Given** a project admin, **When** they create a CLA group with a template in the Project lens, **Then** the CLA group functions identically to one created in PCC (PR gating, signing, PDFs).
2. **Given** a connected GitHub organization, **When** the admin enrolls/unenrolls repositories, **Then** EasyCLA enforcement follows within the same latency as PCC today.
3. **Given** a user without project-admin authority, **When** they open the Project lens, **Then** EasyCLA administration is not accessible.

---

### User Story 6 (Milestone 6) - Platform operates EasyCLA APIs as a V2 Kubernetes service (Priority: P6)

The platform team runs EasyCLA's APIs as an LFX V2 service on Kubernetes (replacing Lambdas behind API Gateway), with platform-standard authentication/authorization, observability, and deployment. Optionally, storage moves from DynamoDB to Postgres. No user-visible behavior changes.

**Why this priority**: Pure re-platforming; enormous scope; only worth committing after UI migrations prove the integration surface. Explicitly presented to the architecture board as a decision with with/without-database options.

**Independent Test**: Run the existing functional/E2E suites (PR gating, signing, consoles' API contract tests) against the Kubernetes deployment and compare results and latency against the Lambda deployment.

**Acceptance Scenarios**:

1. **Given** the K8s deployment serving the same API contract, **When** the full EasyCLA E2E suite runs, **Then** results match the Lambda deployment.
2. **Given** a cutover, **When** GitHub/GitLab/Gerrit webhooks and DocuSign callbacks fire, **Then** they are processed with no lost events.

---

### Edge Cases

- A contributor's historical EasyCLA records (signed before the console required LF login, or under unlinked emails/GitHub identities) may not be reachable from their LF account — My CLAs must handle unmatched history gracefully.
- A user has multiple EasyCLA user records (multiple emails/GitHub IDs) — My CLAs must aggregate or the user sees partial history.
- ICLA signed against an old CLA-group document version (superseded major version) — display and gating must distinguish valid vs out-of-date signatures.
- Employee changes company: prior ECLA remains historical; new acknowledgement needed — read views must not present a stale ECLA as authorizing.
- DocuSign ceremony completed but webhook delayed/lost — PR stays red; users need a truthful "processing" state and support path.
- Company appears in EasyCLA but has no Salesforce/organization-service record (or vice versa) — org-lens mapping must handle mismatches.
- Approved List entry removed while an employee's ECLA exists — signature invalidation semantics must match current behavior.
- Gerrit and GitLab contributors follow different entry paths than GitHub — each platform cuts over independently (sub-milestones), and the console stays reachable for platforms not yet migrated.
- Self Serve unavailable while consoles are retired — PR remediation links would dead-end; rollback path (re-pointing the redirect base URL) must stay viable.
- Sanctioned-company block must be enforced server-side in every new flow, not only in UI.

## Requirements *(mandatory)*

### Functional Requirements

**Milestone 1 — read-only My CLAs (Me lens)**

- **FR-001**: Self Serve MUST display, for the logged-in user, all ICLAs they have signed (any status: valid, superseded, expired), with project/CLA group name, date signed, and validity status.
- **FR-002**: Self Serve MUST display all of the user's currently valid ECLAs with company name, project/CLA group, and acknowledgement date, and MUST NOT offer a PDF for ECLAs (none exists).
- **FR-003**: Users MUST be able to download the signed PDF for each of their signed ICLAs via a time-limited link.
- **FR-004**: The view MUST be read-only; any signing affordance MUST link out to the existing Contributor Console.
- **FR-005**: The system MUST resolve the Self Serve identity (LF SSO) to the user's EasyCLA user record(s), including users with multiple linked emails/GitHub identities, and aggregate agreements across them.
- **FR-006**: Users MUST see only their own agreements; no access to other users' signature data through this surface.

**Milestone 2 — proactive CLA signing entry in Self Serve, hands off to Contributor Console** *(revised 2026-08-04, per Heather/PM; supersedes the original FR-010–FR-014 below)*

- **FR-010**: The GitHub PR status-check remediation link stays pointed at the Contributor Console — no cutover, no per-environment switch, in M2. *(Superseded: previously required a Console↔SS switch; removed per the revision — the Console is not cut over in M2.)*
- **FR-011**: Self Serve MUST offer a new, PR-independent entry point where a logged-in user picks a CLA Group (and, where applicable, a GitHub org/repo) and chooses ICLA or ECLA.
- **FR-012**: Self Serve MUST hand off the user to the existing Contributor Console, pre-scoped to the chosen CLA Group and sign type, to complete the actual signing — for both the ICLA and CCLA paths. Self Serve MUST NOT call the signing-initiation endpoints (`request-individual-signature`, `request-employee-signature`, etc.) itself in M2.
- **FR-013**: *(Deferred — no longer M2 scope)* Per-platform (GitHub/GitLab/Gerrit) sub-milestones for a native SS signing ceremony are not scheduled; M2's picker/hand-off applies uniformly regardless of platform, since the Console — not SS — completes the platform-specific ceremony.
- **FR-014**: *(Removed — not applicable)* Signature completion / PR re-evaluation behavior is unchanged because SS does not run the ceremony; the Console's existing completion behavior applies as-is.

**Milestone 3 — sign ECLA in Self Serve; retire Contributor Console** *(scope under revision — see [03-milestone-sign-ecla-fable.md](03-milestone-sign-ecla-fable.md) revision note; FR-020–FR-026 below are unconfirmed pending follow-up with Heather)*

- **FR-020**: Self Serve MUST support company search/selection, including "add my company" with the same downstream org-creation behavior as today.
- **FR-021**: Self Serve MUST run the pre-checks (company sanctioned, CCLA missing, Approved List) and route each outcome to the equivalent flow: acknowledge, request authorization, or CLA setup.
- **FR-022**: Employees on the Approved List MUST be able to record an ECLA without any e-signature ceremony.
- **FR-023**: Contributors not on the Approved List MUST be able to notify selected CLA managers.
- **FR-024**: Contributors at companies without a CCLA MUST be able to (a) become CLA manager designee (requires LF login) or (b) invite a company admin — with identical role assignments and notifications as today.
- **FR-025**: Where the CLA group requires ICLA-with-CCLA, the flow MUST chain into the ICLA flow before returning to the PR.
- **FR-026**: The Contributor Console MUST be retired only after all three platform entry paths (GitHub, GitLab, Gerrit) have Self Serve equivalents (i.e., after M3c).

**Milestone 4 — Organization lens CCLA management; retire Corporate Console**

- **FR-030**: The Organization lens MUST reach feature parity with the Corporate Console inventory: CCLA signing initiation (signatory flow, including send-by-email), Approved List CRUD across all five criteria types, employee-acknowledgement views with search/pagination, CLA manager add/remove/designee, auto-create-ECLA toggle, signed-CLA views (foundation and project level), CCLA PDF access, activity logs with CSV export, and CLA metrics.
- **FR-031**: Access to org-lens CLA functions MUST be governed by the user's EasyCLA CLA-manager/signatory authority for that company (however mapped — see Open Decision on role bridging in milestone docs), and enforcement MUST be server-side.
- **FR-032**: Role assignments made through Self Serve MUST remain consistent with the system of record used by EasyCLA's enforcement (approval emails, PR gating, notifications must keep working).

**Milestone 5 — Project lens EasyCLA administration; remove from PCC**

- **FR-040**: The Project lens MUST reach parity with PCC EasyCLA administration: CLA group lifecycle (create/edit/delete, enrollment of projects), template selection/preview/regeneration, GitHub org connection (GitHub App install) and repository enrollment incl. enforce-all, Gerrit instance management, GitLab group management, ICLA/CCLA signature reporting incl. invalidation, approved-contributor reports, and events log with CSV export.
- **FR-041**: Project-lens EasyCLA administration MUST be gated by project-level administrative authority equivalent to PCC's permission matrix.

**Milestone 6 — V2 service on Kubernetes (+ optional Postgres)**

- **FR-050**: The migrated API MUST preserve the externally consumed contract (or provide a versioned replacement with a deprecation window) for: consoles/SS surfaces, GitHub/GitLab/Gerrit webhooks, DocuSign callbacks, and any third-party API consumers.
- **FR-051**: Event-driven behaviors currently attached to the data layer (audit events, notifications, cache invalidation, zip building) MUST be preserved or explicitly re-architected.
- **FR-052**: If the database migrates, a verified dual-run/backfill strategy MUST demonstrate zero signature-record loss before cutover; signed PDFs in object storage are unaffected.

### Key Entities

- **CLA Group**: The unit of CLA policy for one or more projects; flags for ICLA/CCLA enabled and "CCLA requires ICLA"; owns document templates/versions.
- **Signature (ICLA)**: Individual agreement; belongs to an EasyCLA user and CLA group; has a signed PDF; has validity/approved/signed flags and document version.
- **Signature (CCLA)**: Corporate agreement; belongs to a company and CLA group; signed by a CLA signatory; has a signed PDF; carries the Approved Lists and auto-create-ECLA flag.
- **Signature (ECLA / employee acknowledgement)**: Record that an employee acknowledged their company's CCLA for a CLA group; references user + company; **no PDF**; may be auto-created from Approved List changes.
- **EasyCLA User**: Contributor identity aggregating emails, GitHub/GitLab IDs, and optional LF username; distinct from the LF SSO account — the mapping is many-to-one and sometimes missing.
- **Company (Signing Entity)**: Organization able to sign CCLAs; linked to the platform organization record (Salesforce ID); may have multiple signing entities.
- **Approved List**: Per company+CLA group criteria (email, domain, GitHub org, GitHub username, GitLab) that authorize employees.
- **CLA Roles**: cla-manager, cla-signatory, cla-manager-designee — company- and project+company-scoped authorities that gate corporate CLA operations.
- **Repository / Git Platform Enrollment**: GitHub orgs+repos, GitLab groups, Gerrit instances attached to CLA groups; drive PR/change gating.
- **Event**: Audit-log entry for CLA activity, surfaced in consoles and exports.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001** (M1): 100% of a sampled user population's signed ICLAs and valid ECLAs visible in the Contributor Console's data are also visible in Self Serve; ICLA PDF download success rate ≥ 99%.
- **SC-002** (M2): *(revised 2026-08-04 — M2 no longer runs the signing ceremony in SS)* ≥ 95% of contributors who start the new proactive picker in Self Serve are successfully handed off to the Contributor Console pre-scoped to the right CLA Group/sign type, without support intervention. *(Original text, now superseded: "≥ 95% of contributors who start the ICLA flow from a PR link in Self Serve complete it without support intervention, matching or beating the Contributor Console's current completion rate; median time from PR link click to green check unchanged or better.")*
- **SC-003** (M3): *(unconfirmed pending M3 scope revisit — see 03-milestone-sign-ecla-fable.md)* All contributor journeys (individual, corporate-approved, corporate-not-approved, corporate-no-CCLA) completable in Self Serve; Contributor Console traffic reaches ~0 and the console is decommissioned.
- **SC-004** (M4): Corporate Console regression checklist passes 100% in the Organization lens; Corporate Console decommissioned with no increase in CLA-related support tickets over the following quarter.
- **SC-005** (M5): A new project can be fully CLA-onboarded in the Project lens without touching PCC; PCC EasyCLA module removed.
- **SC-006** (M6): API cutover with zero lost webhook/callback events, error rate and p95 latency equal or better than the Lambda baseline over a 30-day window.
- **SC-007** (program): No period during the migration in which a contributor is unable to sign or a PR is permanently blocked due to the migration (rollback switch demonstrated per milestone).

## Assumptions

- The existing EasyCLA v3/v4 APIs remain the system of record and enforcement point through M1–M5; Self Serve integrates with them (as it already does with the crowdfunding backend) rather than reimplementing business logic. M6 is where reimplementation happens, if approved.
- E-signature (DocuSign) integration stays inside the EasyCLA backend, unchanged, through M4: the Contributor Console (not Self Serve) requests the signing URL and hands the browser to the ceremony, for both the PR-redirect path and M2's new proactive-picker path; envelope creation, webhooks, and PDF storage do not move. *(Revised 2026-08-04: in the original framing SS itself requested the signing URL from M2 onward — that is no longer the plan; SS hands off to the Console before reaching that step. No new "DocuSign bridge" service is needed either way — analysis in milestone 02 doc.)*
- The PR status-check remediation URL remains pointed at the Contributor Console through M2; no cutover happens in M2. *(Revised 2026-08-04: previously this assumption described an M2 console→SS cutover via a centrally-configured, per-environment parameter — that cutover is no longer M2 scope. Whether/when it happens is now bundled with M3's unresolved scope.)*
- ECLAs have no signed PDF (confirmed in the data model); ICLAs and CCLAs do, stored in object storage with time-limited download links.
- Role bridging strategy for M3–M5: EasyCLA/ACS remains the system of record for cla-manager/cla-signatory/cla-manager-designee; Self Serve consults/act-through EasyCLA APIs (server-side enforcement stays in EasyCLA). Modeling CLA roles natively in the platform's fine-grained-authorization system is deferred to M6 scope. There are currently no CLA object types in the platform authorization model.
- PCC v1 remains operational until M5 completes; no EasyCLA work is done in PCC v2 (it has none today).
- The legacy `/v1`/`/v2` API surface is served by Go (`cla-backend-legacy`, deployed from the `cla-backend` stack; the Python backend has been removed). Contributor flows keep calling these endpoints through M2–M3 (via the Contributor Console); the surface is absorbed into the main service in M6.
- Gerrit and GitLab remain supported platforms. *(Revised 2026-08-04: M2's own per-platform split no longer applies — see Q2 above. M3's per-platform split and console-retirement-requires-all-three assumption stand as originally written but are themselves under revisit pending M3 scope follow-up.)*
- Corporate Console's GraphQL BFF logic (aggregation, Salesforce lookups) is absorbed into Self Serve's Express server or EasyCLA APIs during M4; the BFF is retired with the console.
- Sanctions screening (SSS) checks remain enforced in the backend for corporate flows regardless of UI.

## Program review outcomes *(leadership review, 2026-07-15)*

- **UI-first approved**: migrate UIs onto the existing EasyCLA backend APIs first; backend modernization (M6) in later phases — confirms Q3 below.
- **Timeline**: target completion **Q3 / early Q4 2026** (tighter than the original end-of-Q4 framing) — argues for parallelizing M4/M5 design work early (Nuno adding SS placement placeholders now).
- **M5 is decision-gated**: whether PCC EasyCLA administration moves to Self Serve **or stays in PCC** is an open product decision (Kieran/Manish/Heather) — M5 scope is conditional until resolved; avoid SS/PCC duplication.
- **Landing page retirement**: the standalone EasyCLA landing page (`easycla-landing-page`) was judged redundant — users reach the correct flow via the PR-check link directly. Added to the M3 decommission package.
- **Post-signing profile redirect**: after signing, redirect contributors to their SS profile to collect additional data (e.g., GitHub identity linking) without adding friction *before* signing — UX requirement candidate for M2/M3; synergizes with M1's identity mapping.
- **LFID prerequisite confirmed**: account creation precedes CLA signing (matches the verified code behavior; Q1 below stands).
- **Open engineering action (Michal)**: formalize the roles/permissions mapping feasibility between EasyCLA/ACS and Self Serve (basis: [00-overview](00-overview-fable.md) §2.4/§3 and milestone 04's role-bridge options).

## Resolved Decisions *(after review feedback, 2026-07-11)*

- **Q1 — Contributor login**: RESOLVED — already the status quo. The Contributor Console requires LF login for all flows, including ICLA (verified: the entry dashboard triggers `login()` for unauthenticated users; commit "feat: added lf login"). Self Serve's login requirement is therefore not a regression and no anonymous path is needed. Residual work: M1 identity mapping must still handle historical signatures created before the login requirement (EasyCLA user records without LF usernames).
- **Q2 — Git platform scope**: RESOLVED for M3 as originally scoped — all three platforms are in scope, split into per-platform sub-milestones (M3a GitHub, M3b GitLab, M3c Gerrit), each with its own cutover switch and parity checklist; the Contributor Console retires only after M3c. *(Revised 2026-08-04: M2's own per-platform split, M2a/M2b/M2c, no longer applies — M2 is now a single proactive picker/hand-off flow that works the same way regardless of git platform, since the Contributor Console, not SS, runs the platform-specific ceremony. M3's per-platform split and Console-retirement timing remain as stated here but are themselves under revisit — see the M3 doc's revision note.)*
- **Q3 — Sequencing of the platform rewrite**: RESOLVED — UI-first: M1–M5 build against the existing v4 APIs with deliberately thin adapters (single SS server module); M6 remains a separately gated decision. Noted alternative worth revisiting at the M6 go/no-go: the hybrid strangler (stand up a CLA read/query V2 service after M2 for M4/M5 to consume) spreads M6 risk across the program and reduces adapter rework if M6 is committed early.
