# EasyCLA v4 role matrix

Copyright The Linux Foundation and each contributor to CommunityBridge.

SPDX-License-Identifier: CC-BY-4.0

`role_matrix.py` calls the deployed EasyCLA v4 API (through the LFX API gateway) as several
personas and compares the HTTP status of every Milestone 3 endpoint with the expected
authorization outcome. It is the API-level authorization check for the Organization lens
CCLA work: the UI only hides controls, v4 is the enforcement point.

Python 3 standard library only. Runs in about a minute.

## Personas

| Role | Meaning at the fixture company (Infosys Limited / SUN on dev) |
| --- | --- |
| `manager` | In the CCLA `signatureACL` (ACS `cla-manager`, project\|organization scope) |
| `orgadmin` | Organization-scoped role at the company, not a CLA manager |
| `member` | Valid LFID, no role at the company |
| `outsider` | CLA manager at another company |
| `anon` | No token (always included) |

Personas must not be LF admins (`system-admin`/`lf-staff`): ACS sends admins no
project|organization scopes, so v4 manager writes, which refuse the admin shortcut, answer
403 even for a real CLA manager. On dev, `utils/dev_acs_role_flip.sh admin off|on <user>`
toggles this.

The run prints each persona's LFID and whether it is in the CCLA ACL, so a mis-assigned
persona is visible before the table.

## Running

```bash
cp personas.example.env .env   # fill in; .env is git-ignored
python3 role_matrix.py                 # safe mode: no data changes
python3 role_matrix.py --mutate        # dev only: manager add/remove approval-list entry, toggle/restore auto-ECLA
python3 role_matrix.py --json out.json # also dump every call
```

MFA-enrolled accounts cannot use the password grant (the dev client does not allow the
`mfa-otp` grant either). Set `RM_<PERSONA>_TOKEN` to an Auth0 access token whose audience is
the API gateway (`https://api-gw.dev.platform.linuxfoundation.org/`) instead of
username/password; the LFX console's own session token is not accepted by the gateway.
Tokens live one hour.

Exit code is 1 when any cell differs from its expectation. Cells marked `rec` are recorded
only; they are endpoints where the intended M3 behaviour for that role is still a decision,
not a bug.

Status labels: `403(gw)` is the API gateway's ACS check denying the path before v4 is
reached; `403(svc)` is v4's own check; `403(sanctioned)` is the sanctioned-company guard.
The gateway also answers `403(gw)` for paths it does not route, so a gateway denial on a
new endpoint can also mean "route not deployed". A `manager` persona disambiguates (200).

## Safe mode: how writes are probed without side effects

| Endpoint | Probe | Why it is safe |
| --- | --- | --- |
| approval-list PUT | empty body `{}` | allowed callers get 400 "missing approval list items" |
| ecla-auto-create PUT | current flag value | no change, but still a write: dev only |
| cla-manager POST | unknown email | denied callers get 403; allowed callers fail at user lookup |
| cla-manager DELETE | LFID that is not a manager | denied callers get 403; allowed callers get 4xx |
| cla-manager-designee POST | unknown email | fixture CCLA is signed, service refuses before any change; open to any logged-in user by design |
| cla-manager/requests POST | deny-side personas only | an allowed caller would create a request and email managers |
| ecla invalidate PUT | deny-side personas only; needs a real ECLA id | lookup happens before authorization |
| self-serve/request-corporate-signature POST | deny-side personas only | an allowed caller would create a DocuSign envelope |

`self-serve/prepare-sign` is not in the matrix: it is gated by an Auth0 client allow-list
(`azp`), not by role, and answers 401 to every test client.

## Fixtures

Defaults match `tests/functional/cypress/appConfig/config.dev.ts`. Override with
`RM_COMPANY_ID`, `RM_COMPANY_SFID`, `RM_PROJECT_SFID`, `RM_CLA_GROUP_ID`. The CCLA signature,
its ACL, the auto-ECLA flag and an ECLA signature id are discovered at run time; the ECLA id
needs a persona that can read the ECLA list (a manager).

For prod, put `APP_URL`/`AUTH0_TOKEN_API`/`AUTH0_CLIENT_ID` and the fixtures (a company you are
a CLA manager of) in a separate file and pass `--env prod.env`, so the dev `.env` personas are
not loaded. `--mutate` refuses any target that is not dev.
