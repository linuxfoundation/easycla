# Contract: EasyCLA endpoints consumed by M1 (existing APIs — no changes)

All via lfx-gateway `/cla-service` prefix (`lfx-gateway/dynamic/services/cla-service.yaml`), `secured` middleware. Token model per research R3 (spike: user bearer vs dedicated audience/M2M).

## 1. Identity lookup

### GET /cla-service/v3/users/username/{userName}

- Source: `cla-backend-go/users/handlers.go` (`GetUserByUserName`), swagger `cla.v1.yaml` `/users/username/{userName}`.
- In: LF username from SS session. Out: EasyCLA user model (`user_id`, `lf_username`, `user_emails`, github identity) or 404.

### GET /cla-service/v3/users/search?searchTerm=…&searchField=…

- Source: swagger `cla.v1.yaml` `/users/search`; used with email fields to catch pre-LF-login records.
- Note: verify auth constraints and matching semantics in the T1 spike (handler requires a CLAUser context; emails are stored lowercased per recent backend change — lowercase the query).

### (Preferred if audience works) GET /cla-service/v4/user-from-token

- Derives the EasyCLA user from the caller's JWT — least-privilege identity resolution; only usable with the user-bearer token model.

## 2. Agreements

### GET /cla-service/v4/signatures/user/{userID}?pageSize=…&nextKey=…

- Source: swagger `cla.v2.yaml` `/signatures/user/{userID}`; handler `v2/signatures/handlers.go` `SignaturesGetUserSignaturesHandler`.
- Out: paginated signature list (v2 models). SS classifies ICLA/ECLA per data-model.md and derives status per research R6.
- ⚠️ **No ownership check upstream** (verified): the handler queries whatever `userID` it is given. SS is the authorization boundary — `userID` values come only from step 1. Raised as an upstream hardening note (out of M1 scope).

## 3. Signed PDF

### GET /cla-service/v4/signatures/{signatureID}/signed-document

- Source: swagger `cla.v2.yaml` `/signatures/{signatureID}/signed-document`; returns presigned S3 URL (15-min TTL, bucket `cla-signature-files-{stage}`).
- Called only after SS re-verifies ownership (contract `ss-me-clas-api.md`). Alternative ICLA-specific route `/v4/signatures/project/{claGroupID}/icla/{signatureID}/pdf` available if response shape fits better — pick one in implementation, don't use both.

## Contingency (expected NOT needed)

If `users/search` cannot serve email-based lookup for SS (auth or index constraints found in the spike): add one read endpoint to `cla-backend-go` — `GET /v4/users/by-identity?email=…` — swagger-first in `cla.v2.yaml`, three-layer module pattern, read-only, no schema changes. Decision point: end of T1 spike; requires EasyCLA-team review.
