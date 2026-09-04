# Org lens API — EasyCLA backend for LFX Self Serve M3

Copyright The Linux Foundation and each contributor to CommunityBridge.

SPDX-License-Identifier: CC-BY-4.0

Backend endpoints for the **M3 "Org lens"** milestone of the EasyCLA → LFX Self Serve
program. Design and delivery rules: `~/M3-BE-plan.md` appendices embedded in the
lfx-self-serve issues below.

## `GET /v4/company/external/{companySFID}/cla-groups` ([lfx-self-serve#2149](https://github.com/linuxfoundation/lfx-self-serve/issues/2149))

Read-only organization CLA landing list: one entry per **(signing entity × CLA group)**,
derived from the signed+approved CCLA signatures of every signing entity (company record)
carrying the given external `companySFID`. Each entry carries all its ids
(`companyID`, `companySFID`, `claGroupID`, `foundationSFID`, `signatureID`, project
`projectSFID`, manager `userID`) plus `companyName`,
`signingEntityName`, `claGroupName`, `foundationName`, the covered `projects`,
`signed`/`signedOn`, the stored `sanctioned` flag, `approvedContributorsCount` (employee
acknowledgements under the CCLA, same count as the corporate-console contributors list),
`claManagers` (from the signature ACL) with `claManagersCount`,
`needsClaManager` (= signed with zero managers) and `autoCreateECLA`.

An unknown company or a company with no CCLAs returns HTTP 200 with an empty `list` —
the endpoint never auto-creates the company record. Auth: LF admin, `organization`
scope for the `companySFID`, or any `project|organization` scope whose organization half
matches (ACS resource `company_cla_groups`, action `view_all`). Probe:
`utils/company_cla_groups.sh`.

## Managers & acknowledgments write ops ([lfx-self-serve#2151](https://github.com/linuxfoundation/lfx-self-serve/issues/2151))

CLA-manager request lifecycle under
`/v4/company/{companyID}/project/{projectSFID}/cla-manager/requests`: `GET` (list — HTTP
200 with an empty `requests` array when none), `GET .../{requestID}`,
`PUT .../{requestID}/approve` and `PUT .../{requestID}/deny`. The v4 surface wraps the v1
request service verbatim: approve flips the request to `approved`, adds the requester to
the CCLA signature ACL and emails the CLA managers + requester; deny flips it to `denied`
and emails without touching the ACL. Responses reuse the id-complete
`cla-manager-request` shape (`requestID`, `companyID`/`companyExternalID`,
`projectID`/`projectExternalID`, `userID`/`userExternalID`, names, emails, `status`,
`created`/`updated`). A request belonging to another company or CLA group returns 404.

`PUT /v4/cla-group/{claGroupID}/ecla/{signatureID}/invalidate` invalidates one employee
acknowledgement, mirroring the ICLA invalidate internals: sets
`signature_approved=false` with invalidation metadata from the optional `reason`/`note`
body, logs the event and emails the employee. 400 when the signature is not an ECLA or
belongs to another CLA group, 409 when already invalidated; the response echoes
`signature_id`, `cla_group_id`, `company_id`, `user_id`.

Auth for all five: `project|organization` tree scope for the project/company pair, LF
admin disallowed (ACS resources `cla_manager_request_admin`,
`cla_manager_request_approve`, `cla_manager_request_deny`, `ecla_invalidate`). The
existing last-CLA-manager guard is unchanged: a signed CCLA always keeps at least one
manager. Probe: `utils/cla_manager_requests_ops.sh` — approve/deny/invalidate mutate;
list/get and the 400/403/404/409 probes are side-effect free.
