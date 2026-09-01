# Contract: EasyCLA endpoints consumed by M1 (as built)

Updated 2026-09-01. The originally planned per-endpoint composition (v3 user lookups + per-user signature queries + the `GET /v4/users/by-identity` contingency, preserved in §"Superseded plan" below) was replaced by a consolidated read surface in `cla-backend-go/v2/my_clas` — `GET /v4/my-clas` and `GET /v4/my-clas/{signatureID}/pdf` in [linuxfoundation/easycla#5125](https://github.com/linuxfoundation/easycla/pull/5125), `GET /v4/my-clas/identities` in [linuxfoundation/easycla#5128](https://github.com/linuxfoundation/easycla/pull/5128). The authoritative endpoint reference is [docs/MY_CLAS_API.md](../../../../docs/MY_CLAS_API.md).

All via lfx-gateway `/cla-service` prefix (`lfx-gateway/dynamic/services/cla-service.yaml`), with the caller's **user bearer token** (research R3 resolved in favor of user tokens). Additionally, the Self Serve server can be recognized as a trusted caller via in-handler JWT verification plus an `azp` allow-list read from SSM `cla-ss-trusted-client-ids-{stage}` . That parameter is unset in every environment today, so the path is disabled and SS is handled as an ordinary untrusted caller.

## 1. `GET /cla-service/v4/my-clas`

- Query parameters (all optional): `lfUsername` (defaults to the authenticated principal), repeated `email`/`secondaryEmail`, `githubId`/`githubUsername`, `gitlabId`/`gitlabUsername`, `gerritUsername`.
- For an **untrusted caller**, EasyCLA verifies every forwarded key against the caller's LF account before searching it — using its own user records (GSIs), the platform user-service profile/identities, and the Auth0 Management API (third source, [linuxfoundation/easycla#5172](https://github.com/linuxfoundation/easycla/pull/5172)). Unverifiable keys are not searched and are reported in `skippedIdentities`.
- For an **admin or trusted caller** — a verified JWT whose `azp` is on the `cla-ss-trusted-client-ids-{stage}` SSM allow-list — the forwarded keys are used **as supplied, with no per-key ownership verification** (`effectiveIdentity`). **This path is not active for Self Serve today**: the parameter is unset, and the token SS currently sends (`req.apiGatewayToken`, a `PCC_AUTH0_CLIENT_ID` refresh-token exchange) is handed to every logged-in user as `v1Token` by SS's `GET /api/profile/developer`, so allow-listing that client would let any user assert any identity. SS therefore runs on the **untrusted** path above until it uses a dedicated server-only client (see "Known limitations" in [docs/MY_CLAS_API.md](../../../../docs/MY_CLAS_API.md)). (The bypass exists because the trusted list is Auth0-derived and not re-derivable inside EasyCLA, and the historical GitHub-only signers this endpoint serves carry no `lf_username` on their EasyCLA records — so record-based verification would deny exactly the CLAs such a caller may see.)
- Internally resolves the matching EasyCLA user records, retrieves and deduplicates their ICLA/ECLA signatures, classifies them, and evaluates validity. M1 exposed the boolean `valid`; M2 extended the same endpoint with the computed five-value `status`, `invalidatedAt`, sanctions fields (`flagged`/`flaggedAt`/`flaggedCheck`), and `signedVia`/`signedAs`.
- One SS route consumes it: `GET /api/me/clas` → one upstream call with all session-derived keys.

## 2. `GET /cla-service/v4/my-clas/{signatureID}/pdf`

- Returns a presigned S3 URL (~15-min TTL, bucket `cla-signature-files-{stage}`) for **owned ICLAs only**; ECLAs have no PDF.
- Runs the same ownership resolution as §1 and returns **404 — never 403 — for anything not owned**, so the endpoint does not leak signature existence. SS fetches the URL on click (`GET /api/me/clas/:signatureId/pdf-url`), never on page load.
- A new endpoint was used instead of `GET /v4/signatures/{signatureID}/signed-document` because that route's access check does not fit a contributor-scoped caller.

## 3. `GET /cla-service/v4/my-clas/identities`

- Returns the identities EasyCLA can attach to the caller (the resolvable key set), available for identity-linking diagnostics. **Not consumed by the shipped M1 UI**: SS derives `githubLinked` from its own session/auth-service identities, and logs `skippedIdentities` from `GET /my-clas` as telemetry.

## Authorization boundary

**Signature** ownership enforcement moved **upstream into EasyCLA**: whatever identity set is resolved, the endpoints only ever return signatures owned by it, and the PDF route answers 404 — never 403 — for anything else. (Contrast with `GET /v4/signatures/user/{userID}`, which performs no ownership check and is no longer consumed by this feature.)

**Identity-key** trust is split by caller. For an untrusted caller EasyCLA verifies each forwarded key independently; for an admin, or for a caller whose `azp` is allow-listed, it does not (see §1). **As deployed, SS is on the untrusted side** — the allow-list parameter is unset and the client SS uses does not qualify for it — so every forwarded key is verified today. If and when the trusted path is switched on for a dedicated SS server-only client, the boundary moves: a compromised or buggy SS route could then submit another person's identity keys, and SS becomes responsible for deriving keys only from the authenticated session, with the upstream guarantee reduced to JWT/`azp` verification plus signature ownership.

---

## Superseded plan (2026-07-11, for the record)

The original contract composed existing endpoints with SS as the sole authorization boundary:

- **`GET /v4/users/by-identity?lfUsername=…&email=…&githubId=…`** (new, "expected required") — a thin union lookup over the `lf-username-index` / `lf-email-index` / `github-id-index` GSIs, because the only exposed generic search (`GET /v3/users/search`) is a DynamoDB table scan and GitHub-ID lookup had no HTTP surface. **Never built** — the my-clas module absorbed identity resolution wholesale.
- **`GET /v3/users/username/{userName}`** — interim LF-username lookup for the spike. No longer consumed.
- **`GET /v4/user-from-token`** — considered as a supplement; rejected because it matches/creates by `lf_username`/`lf_email` only and misses pre-LF-login GitHub history.
- **`GET /v4/signatures/user/{userID}`** — per-user signature pages, classified/deduplicated in SS. Replaced by the aggregated `GET /v4/my-clas`; its lack of an upstream ownership check was the main driver for moving enforcement into EasyCLA.
- **`GET /v4/signatures/{signatureID}/signed-document`** — presigned-PDF route, called after SS-side ownership re-verification. Replaced by `GET /v4/my-clas/{signatureID}/pdf` with upstream enforcement.

The spec's "at most one small read endpoint, no schema changes" allowance was exceeded in endpoint count (three read endpoints) but held in spirit: read-only, no schema changes, one new swagger-first module.
