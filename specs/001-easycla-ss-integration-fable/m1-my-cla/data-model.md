# Data Model — Milestone 1: Read-only "My CLAs" (Phase 1)

No persistent storage is introduced. These are view models and upstream-record mappings only.

**Implementation update (PR #5125):** SS forwards session-derived identity keys to `GET /v4/my-clas` / `GET /v4/my-clas/{signatureID}/pdf`, which resolve users, aggregate signatures and enforce ownership inside EasyCLA; `signature_type` may also be `ecla`, GitHub/GitLab fields are identity-resolution keys, and M1 status is the boolean `valid` (no superseded detection). SS-side mappings below (v3 user endpoints, `easyclaUserIds`, per-user signature/PDF authorization, the `status` enum) are superseded — see [contracts/upstream-easycla-api.md](contracts/upstream-easycla-api.md).

## Upstream records (read-only, EasyCLA-owned)

### EasyCLA User (`cla-{stage}-users` via v3 users endpoints)

| Field | Use in M1 |
|-------|-----------|
| `user_id` | key for signatures query |
| `lf_username` | primary identity match |
| `user_emails[]`, `lf_email` | fallback identity match |
| `user_github_username`/`user_github_id` | display only (identity provenance) |

One LF person ⇒ 0..N EasyCLA user records (pre-LF-login history, multiple emails). M1 unions them.

### Signature (`cla-{stage}-signatures` via `GET /v4/signatures/user/{userID}`)

| Field | Meaning |
|-------|---------|
| `signatureID` | key; input to PDF-URL endpoint |
| `projectID` / project name | CLA group signed against |
| `signatureType` (`cla`/`ccla`) + `signatureReferenceType` (`user`/`company`) | record class |
| `companyName` / `signature_user_ccla_company_id` | present ⇒ ECLA, identifies employer |
| `signed`, `approved` | validity inputs (see rules) |
| `signedOn` / created | display date |
| document major/minor version | superseded detection (if exposed; else display-only) |

**Classification rule**: `type=cla & referenceType=user & cclaCompanyID empty` ⇒ **ICLA** (has PDF); `type=cla & referenceType=user & cclaCompanyID set` ⇒ **ECLA** (no PDF, never offer download); `type=ccla` ⇒ corporate record — **excluded from M1** (Organization-lens data, M4).

## SS view models (TypeScript, `server/types/cla.types.ts` + shared interface)

### `MyClaAgreement`

```ts
{
  id: string;                    // signatureID
  kind: 'ICLA' | 'ECLA';
  projectName: string;           // CLA group name
  companyName?: string;          // ECLA only
  signedOn: string;              // ISO date
  status: 'valid' | 'superseded' | 'inactive';  // per research R6
  documentVersion?: string;
  pdfAvailable: boolean;         // true only for ICLA
}
```

Validation/invariants:
- `kind='ECLA'` ⇒ `pdfAvailable=false` and `companyName` required (FR-002).
- ECLA entries with `status != 'valid'` are filtered out server-side (FR-002); ICLAs are kept with status labels (FR-001).
- Deduplication key across merged EasyCLA users: `(kind, projectID, companyID?, signatureID)` — identical `signatureID` wins once; distinct signatures for the same project are all shown (they are distinct legal records).

### `MyClasResponse`

```ts
{
  agreements: MyClaAgreement[];    // sorted signedOn desc
  identity: {
    matchedUserIds: number;        // count only — no raw EasyCLA IDs to client
    unmatched: boolean;            // true ⇒ show "history may be incomplete" hint
    githubLinked: boolean;         // false ⇒ show "Don't see your CLAs? Link your GitHub account" CTA
  }
}
```

### `PdfUrlResponse`

```ts
{ url: string; expiresInSeconds: number }   // presigned S3 URL, ~15 min TTL
```

## Server-side session→identity mapping (in-memory, per request)

`ResolvedClaIdentity`: `{ lfUsername, emails[], githubIds[], easyclaUserIds[] }` — GitHub IDs come from the Auth0 identities array (linked social accounts, numeric ID preferred over username); computed per request (optionally memoized in the session for its lifetime), never persisted, never client-supplied. All upstream signature/PDF queries iterate only over `easyclaUserIds`; the PDF route re-verifies the requested `signatureID` is contained in the fetched agreement set before asking EasyCLA for the presigned URL (authorization boundary per research R3).

## State transitions

None owned by M1 (read-only). Status derivation is a pure function of upstream fields (research R6).
