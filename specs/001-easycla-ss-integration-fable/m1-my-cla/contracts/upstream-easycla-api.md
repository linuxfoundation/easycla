# Contract: EasyCLA endpoints consumed by M1 (existing APIs — no changes)

**Implementation update (PR #5125):** the endpoints below are superseded — that PR implements two consolidated read endpoints, `GET /v4/my-clas` and `GET /v4/my-clas/{signatureID}/pdf` (identity resolution, aggregation, ownership enforcement and validity evaluation inside EasyCLA; SS forwards all session-derived identity keys); see [docs/MY_CLAS_API.md](../../../../docs/MY_CLAS_API.md).

All via lfx-gateway `/cla-service` prefix (`lfx-gateway/dynamic/services/cla-service.yaml`), `secured` middleware. Token model per research R3 (spike: user bearer vs dedicated audience/M2M).

## 1. Identity lookup

Resolution unions three keys: LF username, verified emails, linked GitHub account(s) (research R2).

### NEW (expected required): GET /cla-service/v4/users/by-identity?lfUsername=…&email=…&githubId=…

- One small read endpoint to add in `cla-backend-go` (swagger-first in `cla.v2.yaml`, three-layer module pattern, read-only, no schema changes), wrapping the existing GSI-backed repository queries: `lf-username-index`, `lf-email-index` (+ `user_emails` match — verify `GetUsersByEmail` efficiency), `github-id-index` (`users/repository.go`; `GetUserByGitHubID` exists at service layer but is not exposed over HTTP today).
- Accepts repeated `email`/`githubId` params; returns the matched user records (union, deduped by `user_id`).
- Why new: the only exposed generic search, `GET /v3/users/search`, is a **DynamoDB table scan** (`users/repository.go:1279`) — unsuitable for per-request identity resolution; GitHub-ID lookup has no HTTP surface at all.
- Requires EasyCLA-team review; same no-ownership-check caveat as §2 — treat as internal/service API, SS is the authorization boundary.

### GET /cla-service/v3/users/username/{userName}

- Source: `cla-backend-go/users/handlers.go` (`GetUserByUserName`), swagger `cla.v1.yaml` `/users/username/{userName}`; GSI `lf-username-index`.
- Usable immediately for the LF-username key (spike/bring-up) while the by-identity endpoint lands.

### (Supplementary if audience works) GET /cla-service/v4/user-from-token

- Derives the EasyCLA user from the caller's JWT; only usable with the user-bearer token model. Cannot replace the union: it matches/creates by `lf_username`/`lf_email` only and never backfills `lf_username` onto GitHub-derived records (`cmd/server.go:930`), so it misses pre-LF-login history.

## 2. Agreements

### GET /cla-service/v4/signatures/user/{userID}?pageSize=…&nextKey=…

- Source: swagger `cla.v2.yaml` `/signatures/user/{userID}`; handler `v2/signatures/handlers.go` `SignaturesGetUserSignaturesHandler`.
- Out: paginated signature list (v2 models). SS classifies ICLA/ECLA per data-model.md and derives status per research R6.
- ⚠️ **No ownership check upstream** (verified): the handler queries whatever `userID` it is given. SS is the authorization boundary — `userID` values come only from step 1. Raised as an upstream hardening note (out of M1 scope).

## 3. Signed PDF

### GET /cla-service/v4/signatures/{signatureID}/signed-document

- Source: swagger `cla.v2.yaml` `/signatures/{signatureID}/signed-document`; returns presigned S3 URL (15-min TTL, bucket `cla-signature-files-{stage}`).
- Called only after SS re-verifies ownership (contract `ss-me-clas-api.md`). Alternative ICLA-specific route `/v4/signatures/project/{claGroupID}/icla/{signatureID}/pdf` available if response shape fits better — pick one in implementation, don't use both.

## Status of the "one small backend endpoint" allowance

Originally a contingency, `GET /v4/users/by-identity` (§1) is now **expected to be required** because `/v3/users/search` is scan-based and GitHub-ID lookup has no HTTP surface. Final confirmation at the end of the T1 spike; the endpoint stays within the spec's "at most one small read endpoint, no schema changes" boundary.
