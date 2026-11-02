# E2E suite secrets

Everything in this directory except this README is ignored by git (`tests/e2e/.gitignore`: `secrets/*`,
`!secrets/README.md`; the repository `.gitignore` also ignores every `*.secret`). Keep the directory private:

```sh
chmod 700 tests/e2e/secrets && chmod 600 tests/e2e/secrets/*.secret
```

Nothing in the suite ever prints a secret: tokens are redacted from logs and evidence, HTTP evidence files are
written `0600`, and the browser modules record only `METHOD host/path` for requests.

## Files you create

| File | Content | Used by |
|---|---|---|
| `<env>.env.secret` | the whole per-environment configuration (endpoints, the two identities, QA fixture ids, gates): copy `../config/example.env` and fill it in; keys missing a value turn the checks that need them into `SKIP` | everything (`lib/common.sh`, `lib/e2e.py`, `browser/lib/e2e.mjs`) |
| `auth0-<env>-client-id.secret` | one line: the Auth0 client id `utils/get_auth0_token.sh` uses for `<env>` (falls back to `utils/auth0-<env>-client-id.secret` when absent) | `lib/token.sh` |
| `auth0-<env>-<identity>.secret` | two lines: `AUTH0_USERNAME=<lfid>` and `AUTH0_PASSWORD=<password>` of that identity on `<env>`; the headless login types them only on `AUTH0_DOMAIN` (plus `SSO_HOSTS`, if set) from `<env>.env.secret` | `lib/token.sh` (API tokens), `browser/lib/e2e.mjs` (headless login) |

`<env>` is `dev` or `prod`; `<identity>` is the LFID configured as `ID1` / `ID2` in `<env>.env.secret`
(`ID1` = the account that holds every role, `ID2` = the account without org-admin/CLA roles that the chains
grant and revoke). Four credential files are therefore expected:

```
auth0-dev-<ID1>.secret   auth0-dev-<ID2>.secret
auth0-prod-<ID1>.secret  auth0-prod-<ID2>.secret
```

## Files the suite generates

| File | What | Lifetime |
|---|---|---|
| `auth0-<env>-<identity>.token.secret` | api-gw access token minted by `lib/token.sh` (via `utils/get_auth0_token.sh` staged in a private temp dir) | reused while `exp` is more than 10 minutes away; dev tokens live 1 h, prod 3 h; `FORCE=1 lib/token.sh` re-mints |
| `pw-<env>-<identity>.state.secret` | Playwright storage state (Auth0/SS session cookies) saved after a successful headless login | reused on the next run (login reports `cached`); delete the file to force a fresh password login |

## Other credentials the suite reads but does not store here

* **AWS** (`scans/lambda_scan.sh`, ACS cross-checks in `api/interactions.py`): the profile named by
  `AWS_PROFILE_E2E` in `<env>.env.secret` (`lfproduct-dev` / `lfproduct-prod`) must have valid credentials
  (SSO / MFA session). Without it those checks are recorded as `SKIP`, never as failures.
* **GitHub** (`github/pr_flow.sh`, dev only): the `gh` CLI must be logged in as an account with push access to
  `GITHUB_REPO`; the throw-away commit is authored with `ID1_EMAIL` so GitHub attributes it to `ID1_GITHUB`.
