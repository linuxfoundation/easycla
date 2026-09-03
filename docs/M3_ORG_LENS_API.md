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
`projectSFID`/`projectExternalID`, manager `userID`) plus `companyName`,
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
