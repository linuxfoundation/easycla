# Quickstart — Milestone 1: Read-only "My CLAs"

## Prerequisites

- `lfx-self-serve` running locally (`apps/lfx-one`): Node 22+, `.env` per `apps/lfx-one/.env.example`, Auth0 dev tenant login working.
- Network access to EasyCLA **dev**: lfx-gateway dev URL for `/cla-service/v3|v4` (set `CLA_SERVICE_BASE_URL`), plus whichever token config the R3 spike selected (user bearer passthrough, or exchange/M2M client credentials).
- A dev test user with CLA history. To create one: use the dev Contributor Console flow against a dev CLA-gated repo (sign an ICLA; for an ECLA, use a dev company with a signed CCLA and approval-list entry), or pick an existing dev identity from the `cla-dev-signatures` table.
- LaunchDarkly dev flag `my-clas-enabled` (or repo-convention name) turned on for your user.

## Run

```bash
cd lfx-self-serve
pnpm install            # or repo-standard package manager
pnpm dev                # starts lfx-one with SSR server
```

1. Log in with the test user.
2. Open **Me lens → My CLAs** (`/me/clas`).
3. Expect: ICLA row(s) with status + **Download PDF**; ECLA row(s) with company name and no download; empty state if the account has no history.

## Verify (maps to acceptance scenarios)

| Check | How |
|-------|-----|
| US1-AS1: ICLAs listed + PDF works | Click Download PDF → browser gets S3 URL, file opens; compare list against `GET /v4/signatures/user/{userID}` called directly with a dev token |
| US1-AS2: ECLA listed, no PDF | Confirm ECLA row renders company + date, no download affordance |
| US1-AS3: empty state | Log in with a CLA-less user → explanatory empty state |
| US1-AS4: read-only | No signing affordances anywhere; "need to sign?" links to dev Contributor Console |
| FR-005: multi-record aggregation | Test user with two EasyCLA records (e.g., pre-LF-login GitHub-only record + linked record) shows union of agreements |
| FR-006: no cross-user access | `GET /api/me/clas/<someone-else's signatureId>/pdf-url` → 404 |
| SC-001 sampling | Run the comparison script (tasks phase) across N dev users: SS list == direct EasyCLA query |

## Useful direct calls (dev, with token)

```bash
# identity
curl -H "Authorization: Bearer $TOK" "$GW/cla-service/v3/users/username/<lf-username>"
# agreements
curl -H "Authorization: Bearer $TOK" "$GW/cla-service/v4/signatures/user/<userID>"
# pdf url
curl -H "Authorization: Bearer $TOK" "$GW/cla-service/v4/signatures/<signatureID>/signed-document"
```
