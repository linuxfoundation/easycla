<!-- Copyright The Linux Foundation and each contributor to CommunityBridge.
SPDX-License-Identifier: CC-BY-4.0 -->

# Spike Runbook — Spike 1 (dev), with Spike 2 retained as a connectivity check

Ready-to-run steps against the **dev** environment.

**Only Spike 1 is still a decision gate**: confirm a Self-Serve-minted user token reaches EasyCLA v4 through lfx-gateway (the api-gw grant + secured-call token path feeding P3). **Spike 2 — whether a role-less user is allowed through — was resolved by shipped M1**, which ships an any-authenticated-user policy on the My CLAs read endpoints; its call is retained below purely as a route-sync/connectivity diagnostic, not as an open investigation. See [role-mapping-feasibility.md §7](role-mapping-feasibility.md).

## What you need

- The **dev api-gw audience**: `https://api-gw.dev.platform.linuxfoundation.org/` (`lfx-self-serve/apps/lfx-one/.env.example:146`).
- The **Auth0 issuer, client_id, client_secret** the SS server uses for the exchange — the `PCC_AUTH0_*` values in the dev SS deployment's env (`.env.example:9,11`; real values in the dev secret store / running config, not the repo).
- A **user refresh token** for each test user (see step 1 for options). Requires the `offline_access` scope.
- Two dev test users: **(A)** a known CLA manager for some company × CLA group; **(B)** a user with **no** ACS CLA role.
- Gateway base URL (dev): `https://api-gw.dev.platform.linuxfoundation.org` — the `/cla-service/v4/*` prefix routes to the EasyCLA v4 Lambda.

## Step 1 — get a user access token for the api-gw audience

This reproduces what SS's `extractApiGatewayToken()` does (`auth.middleware.ts:230-249` → `refresh-token-exchange.util.ts:104-120`): a `refresh_token` grant asking for the api-gw audience.

```bash
ISSUER="https://<dev-auth0-issuer>"          # PCC_AUTH0_ISSUER_BASE_URL (dev)
CLIENT_ID="<dev-client-id>"                  # PCC_AUTH0_CLIENT_ID (dev)
CLIENT_SECRET="<dev-client-secret>"          # PCC_AUTH0_CLIENT_SECRET (dev)
AUDIENCE="https://api-gw.dev.platform.linuxfoundation.org/"
REFRESH_TOKEN="<user-A-refresh-token>"       # swap for user B in the role-less run

TOKEN=$(curl -s -X POST "$ISSUER/oauth/token" \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode "grant_type=refresh_token" \
  --data-urlencode "refresh_token=$REFRESH_TOKEN" \
  --data-urlencode "client_id=$CLIENT_ID" \
  --data-urlencode "client_secret=$CLIENT_SECRET" \
  --data-urlencode "audience=$AUDIENCE" \
  | jq -r .access_token)

echo "$TOKEN" | cut -d. -f2 | base64 -d 2>/dev/null | jq .   # inspect claims
```

**Checkpoint (this is spike 1's core prerequisite):** the decoded token must contain `http://lfx.dev/claims/username`. If it's present, Auth0 is granting the api-gw audience to this client and stamping the claim the gateway needs — the prerequisite is met (spike 1 is only "passed" once the secured `cla-managers` call below returns `200`; see `role-mapping-feasibility.md` line 227). If the exchange returns an error (e.g. `invalid_grant`, `access_denied`, or unauthorized audience), spike 1 has found the gap: the SS Auth0 client isn't authorized for that audience in dev — that's an Auth0 client-grant config item, not a code change.

> Getting a refresh token: log into dev SS and read it from the session store (`req.appSession.refresh_token`) via a controlled inspection, or run a one-off authorization-code+PKCE login against the dev client with `scope=openid offline_access`. Either way the token must carry `offline_access`. The refresh token (and the client secret above) are reusable credentials — do not log or persist them, and revoke/discard the token once the spike is done.

## Step 2 — call a secured v4 endpoint

### Spike 1 (user A = CLA manager): expect **200**

```bash
COMPANY_ID="<dev-company-internal-id>"
PROJECT_SFID="<dev-project-sfid>"            # a project A manages CLAs for

curl -s -o /dev/null -w "%{http_code}\n" \
  "https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/company/$COMPANY_ID/project/$PROJECT_SFID/cla-managers" \
  -H "Authorization: Bearer $TOKEN"
```

- **200** → the whole chain works: SS token → gateway (issuer check) → ACS warden allow → X-ACL injected → v4 scope check passes. Spikes 1's happy path confirmed.
- **403** → capture the body. If the gateway rejects (`User <name> does not have access to resource or path ...`), it's an ACS warden/policy result. If the body is a v4 payload, the scopes didn't match the resource. Note which.

### Spike 2 (user B = no CLA role): resolved by the shipped M1

> **Superseded.** This spike was written against `/v4/signatures/user/{userID}` — an older operation that also filters out ECLAs — to decide whether M1 could use user tokens. M1 shipped a purpose-built `GET /v4/my-clas` instead, whose ACS policy admits any authenticated user, with ownership enforced inside the endpoint. User tokens work as designed; no M2M fallback was needed. Kept below as a connectivity check against the **actual** M1 route — a 403 here means a missing or unsynced M1 ACS route, not an M1 policy gap.

Re-run step 1 with user B's refresh token, then hit the M1 read endpoint. No EasyCLA userID is needed — the endpoint resolves the caller's identity itself:

```bash
# $TOKEN here is user B's token — re-run Step 1 with B's refresh token first
curl -s -o /dev/null -w "%{http_code}\n" \
  "https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/my-clas" \
  -H "Authorization: Bearer $TOKEN"
```

- **200** → **role-less users pass the secured router.** M1/M2 (contributor-facing reads; M3 in the pre-2026-09-01 numbering) use user tokens exactly as proposed (P3); no fallback needed. Best outcome.
- **403 at the gateway** → the M1 ACS route is missing or unsynced in this environment (the shipped policy admits any authenticated user). Capture the warden response and file the route sync — this is an environment gap, not the M1 policy gap the original spike hypothesised, and it does not call for the M2M fallback.

## Recording results

For each call log: user, endpoint, HTTP status, and (on 403) the response body and whether it looks gateway-issued or v4-issued. For the **Spike 1** call those four fields feed the P3 decision. The **role-less** call is now only a route-sync check — a 403 there indicates a missing or unsynced ACS route, not an open policy question — so record it as a diagnostic and do not read a decision into it.

## Notes / gotchas

- **`clagroup` vs `claGroupID`**: unrelated to these spikes, but if you also try the write path (spike 3), the approval-list segment is `.../clagroup/{claGroupID}/approval-list` — lowercase `clagroup`.
- A **404** on the cla-managers call usually means wrong company-internal-id vs SFID — the path wants the internal company ID, not the Salesforce ID.
- The gateway only reads `http://lfx.dev/claims/username` from the token; a token missing that claim 403s at the ACS plugin regardless of audience, so the step-1 checkpoint matters before you debug step 2.
