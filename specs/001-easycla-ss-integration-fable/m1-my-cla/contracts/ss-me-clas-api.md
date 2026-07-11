# Contract: Self Serve server API for "My CLAs" (consumed by the Angular module)

Base: `apps/lfx-one` Express server, session-authenticated (OIDC). All routes require an authenticated session; there is no user-ID input anywhere — identity comes from the session.

## GET /api/me/clas

Returns the logged-in user's agreements.

- **Auth**: session required (401 otherwise, standard middleware).
- **Response 200** (`MyClasResponse`):

```json
{
  "agreements": [
    {
      "id": "3c1e…",
      "kind": "ICLA",
      "projectName": "CNCF – Kubernetes CLA Group",
      "signedOn": "2025-11-03T10:22:00Z",
      "status": "valid",
      "documentVersion": "2.0",
      "pdfAvailable": true
    },
    {
      "id": "9ab2…",
      "kind": "ECLA",
      "projectName": "LF Energy",
      "companyName": "Example Corp",
      "signedOn": "2026-01-18T08:00:00Z",
      "status": "valid",
      "pdfAvailable": false
    }
  ],
  "identity": { "matchedUserIds": 2, "unmatched": false, "githubLinked": true }
}
```

- **Behavior**: ICLAs listed in any status with labels; ECLAs only when valid; sorted `signedOn` desc; empty `agreements` + `identity.unmatched=true` drives the "no CLA history found for your account" empty state.
- **GitHub-link CTA**: when `identity.githubLinked=false` (no GitHub account linked to the LF identity), the UI shows "Don't see your CLAs? Link your GitHub account" pointing into SS's existing identity-linking flow (`/social/callback` social-connection pattern); on return, the page re-fetches and resolution now includes the linked GitHub ID (research R2).
- **Errors**: 502 with `{ code: "UPSTREAM_ERROR" }` when EasyCLA is unavailable (UI shows retryable error); identity-resolution failure is not an error (returns empty + `unmatched`).

## GET /api/me/clas/:signatureId/pdf-url

Issues a short-lived download URL for a signed ICLA PDF.

- **Auth**: session required.
- **Guard**: `signatureId` MUST be an ICLA belonging to the session's resolved agreement set (server re-fetches/uses per-request resolution — 404 otherwise; never 403, to avoid existence oracle).
- **Response 200** (`PdfUrlResponse`): `{ "url": "https://s3…", "expiresInSeconds": 900 }` — client opens immediately (URL TTL ~15 min).
- **Errors**: 404 unknown/not-owned/ECLA id; 502 upstream failure.

## Feature flag

Both routes and the Angular module are gated by LaunchDarkly flag `my-clas-enabled` (name TBD per repo convention). Flag off ⇒ routes 404, sidebar item hidden, route guard redirects to Me dashboard.

## Non-goals (M1)

No POST/PUT/DELETE; no CCLA data; no arbitrary-user lookup; no PDF proxying/streaming through SS.
