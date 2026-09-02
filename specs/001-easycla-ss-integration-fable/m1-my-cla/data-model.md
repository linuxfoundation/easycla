# Data Model — Milestone 1: Read-only "My CLAs" (as built)

No persistent storage is introduced. These are view models and upstream-record mappings only.

Updated 2026-09-01 to match the implementation ([linuxfoundation/easycla#5125](https://github.com/linuxfoundation/easycla/pull/5125)): identity resolution, aggregation, deduplication, validity evaluation, and **signature**-ownership enforcement all happen **inside EasyCLA**, behind `GET /v4/my-clas` / `GET /v4/my-clas/{signatureID}/pdf` (see [contracts/upstream-easycla-api.md](contracts/upstream-easycla-api.md) and [docs/MY_CLAS_API.md](../../../docs/MY_CLAS_API.md)). SS does no per-user signature queries — it forwards session-derived identity keys and renders the response. It does receive the upstream `userIds`, but only to derive the `matchedUserIds` count; it neither persists them, queries by them, nor exposes them to the browser.

## Upstream records (read-only, EasyCLA-owned)

### EasyCLA User (`cla-{stage}-users`, resolved internally by the my-clas module)

| Field | Use in M1 |
|-------|-----------|
| `user_id` | key for signatures query (internal to EasyCLA) |
| `lf_username` | primary identity match |
| `user_emails[]`, `lf_email` | email identity match |
| `user_github_username`/`user_github_id` | identity-resolution keys (GitHub accounts linked to the LF identity) |

One LF person ⇒ 0..N EasyCLA user records (pre-LF-login history, multiple emails). The my-clas module unions them; SS never sees the individual records.

### Signature (`cla-{stage}-signatures`, read internally by the my-clas module)

| Field | Meaning |
|-------|---------|
| `signatureID` | key; input to the PDF endpoint |
| `projectID` / project name | CLA group signed against |
| `signatureType` (`cla`/`ecla`/`ccla`) + `signatureReferenceType` (`user`/`company`) | record class — `ecla` is the auto-created ECLA variant, see the classification rule below |
| `companyName` / `signature_user_ccla_company_id` | present ⇒ ECLA, identifies employer |
| `signed`, `approved` | validity inputs |
| `signedOn` / created | display date |

**Classification rule** (applied upstream): the query accepts `signature_type` of **both `cla` and `ecla`** — ICLAs and DocuSign-era ECLAs are stored as `cla`, while ECLAs auto-created from approval-list changes are stored as `ecla`. User-referenced rows are then classified by company-ID presence: `referenceType=user & cclaCompanyID empty` ⇒ **ICLA** (has PDF); `referenceType=user & cclaCompanyID set` ⇒ **ECLA** (no PDF, never offer download). `type=ccla` ⇒ corporate record — **excluded from M1** (Organization-lens data, M3).

## Upstream response item (`GET /v4/my-clas`)

Each item in the response carries (see MY_CLAS_API.md for the full shape): `signatureID`, `claType` (`icla`/`ecla`), project/CLA group identifiers and names, `companyName` (ECLA), `signedOn`, and validity. The endpoint still exposes the boolean `valid` that M1 shipped with; M2 added the computed five-value `status` (`valid`/`needs_attention`/`invalidated`/`revoked`/`unknown`) plus `signedVia`/`signedAs`. All agreements are returned regardless of validity (the "valid ECLAs only" filter from the original plan was dropped). Not every invalid row carries M2's Request-approval action: the shipped gate is an **ECLA whose `statusReason` is `not_on_approval_list`** (`canRequestClaApproval`, `packages/shared/src/utils/cla-manager-actions.utils.ts`) — which corresponds to `needs_attention`. `invalidated`, `revoked`, and `unknown` rows are informational.

## SS view models (TypeScript, server `cla` types + shared interface)

### `MyClaAgreement`

```ts
{
  id: string;                    // signatureID
  kind: 'ICLA' | 'ECLA';
  claGroupName: string;          // CLA group name (falls back to the CLA group UUID)
  projectName?: string;          // Salesforce project name; omitted when unresolved
  companyName?: string;          // ECLA only
  signedOn: string;              // ISO date
  status: ClaStatus;             // computed five-value status (M2 added the enum; see below)
  pdfAvailable: boolean;         // true only for ICLA
}
```

`claGroupName` and `projectName` are distinct: the CLA group name is *not* the Salesforce project name. The UI renders `projectName` as the primary line and `claGroupName` as its subtext, falling back to `claGroupName` alone when the project is unresolved. The interface carries further optional display/routing fields (`projectLogo`, `projectSfid`, `foundationSfid`, `claGroupId`, `claManager`, `signedVia`, `signedAs`, `statusReason`, `documentVersion`) — see `packages/shared/src/interfaces/cla.interface.ts` in `lfx-self-serve` for the authoritative shape.

Validation/invariants:

- `kind='ECLA'` ⇒ `pdfAvailable=false` and `companyName` required (FR-002).
- All rows are shown with their validity/status; nothing is filtered out client- or server-side in SS.
- Deduplication happens upstream by `signatureID`; distinct signatures for the same project are all shown (they are distinct legal records).

### `MyClasResponse`

```ts
{
  agreements: MyClaAgreement[];    // sorted signedOn desc
  identity: {
    matchedUserIds: number;        // count of EasyCLA user records matched to the session identity
    unmatched: boolean;            // true ⇒ show "history may be incomplete" hint
    githubLinked: boolean;         // false ⇒ show "Don't see your CLAs? Link your GitHub account" CTA
  }
}
```

(`unmatched` is derived from the match count — `userIds.length === 0` — not from `skippedIdentities`, which the SS server logs separately as telemetry. The SS server does receive the upstream `userIds` before reducing them to the `matchedUserIds` count; it is the **browser** that never sees raw EasyCLA user IDs.)

### `PdfUrlResponse`

```ts
{ url: string; expiresInSeconds: number }   // presigned S3 URL, ~15 min TTL
```

## Server-side identity forwarding (in-memory, per request)

SS derives the identity keys from the session — LF username, verified emails, GitHub IDs/usernames from the Auth0 identities array — and forwards them as query parameters to `GET /v4/my-clas`. Nothing is persisted and nothing is client-supplied.

How EasyCLA treats those keys depends on the caller (`effectiveIdentity`, `cla-backend-go/v2/my_clas/service.go`):

- **Untrusted callers** have every forwarded key verified against their own LF account — EasyCLA's own user records, the platform user-service, and the Auth0 Management API (per [linuxfoundation/easycla#5172](https://github.com/linuxfoundation/easycla/pull/5172)) — before it is searched. Unverifiable keys come back in `skippedIdentities`.
- **Admins and trusted callers** — a verified JWT whose `azp` is on the `cla-ss-trusted-client-ids-{stage}` SSM allow-list — supply their keys **directly, with no per-key ownership verification**. **SS is not such a caller today**: the parameter is unset, and the token SS sends is also handed to users as `v1Token`, so its client must not be allow-listed (see "Known limitations" in [docs/MY_CLAS_API.md](../../../docs/MY_CLAS_API.md)); enabling this for SS needs a dedicated server-only client first. The bypass itself is deliberate: the trusted list is Auth0-derived and not re-derivable inside EasyCLA, and the historical GitHub-only signers this endpoint exists to serve carry no `lf_username` on their EasyCLA records, so verifying against those records would deny exactly the CLAs the caller is entitled to see.

**If the trusted path is ever activated for SS, SS becomes the authorization boundary for the identity keys it forwards** — trusted to send only keys derived from the authenticated session. Until then EasyCLA verifies each key itself. What moved into EasyCLA is *signature* ownership: the PDF route passes the `signatureID` straight to `GET /v4/my-clas/{signatureID}/pdf`, which re-runs the ownership check itself and returns 404 (never 403) for anything the resolved identity doesn't own.

## State transitions

None owned by M1 (read-only). Validity is computed upstream per request.
