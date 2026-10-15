#!/usr/bin/env python3
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
"""Fixture writes as ID1 (CLA manager) on the QA company x CLA group, always restoring the baseline.

A  approval list: add ID1's own criteria, add six synthetic criteria types, invalid e-mail refused,
   duplicate add idempotent, remove the synthetic ones (ID1's own entries are restored to their baseline state)
AU auto-ECLA toggle on/off (restored to the starting value)
DS designee read, duplicate manager add, manager requests list
SA sanctioned company: every write refused with 403 (when SANCTIONED_* is configured)
RM/RA approval-list removal voids ID1's own valid ECLA on the group; re-adding the criterion recovers it
   (removal-only invalidation rule) - when ID1's ecla is missing or manager-invalidated a fresh one is acquired first
   through the legacy API (the contributor console's request-employee-signature), when ID1_UID is configured
F  final baseline comparison
"""
import json
import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "lib"))
import e2e  # noqa: E402

c = e2e.cfg
ID1 = c("ID1")
PROJ, GRP, CO, SFID, CCLA = (c("WRITE_PROJECT_SFID") or c("PROJECT_SFID"), c("WRITE_CLA_GROUP") or c("CLA_GROUP"), c("WRITE_COMPANY_ID") or c("COMPANY_ID"),
                             c("WRITE_COMPANY_SFID") or c("COMPANY_SFID"), c("WRITE_COMPANY_CCLA") or c("COMPANY_CCLA"))
APPROVAL = f"/v4/signatures/project/{PROJ}/company/{CO}/clagroup/{GRP}/approval-list"
AUTO = f"/v4/signatures/company/{CO}/clagroup/{GRP}/ecla-auto-create"
LISTS = ("emailApprovalList", "domainApprovalList", "githubUsernameApprovalList", "githubOrgApprovalList", "gitlabUsernameApprovalList", "gitlabOrgApprovalList")
# username entries make the backend look the account up and create/re-parent its EasyCLA user record (createOrGetEmployeeModels),
# so only suite-owned accounts may be used: ID2's GitHub login by default, a GitLab login only when configured (no third-party
# defaults). Organization/group entries touch no user record, public names are fine there.
SYN = {"email": f"{ID1.split('@')[0]}+qa-{e2e.TAG}@example.org", "domain": f"qa-{e2e.TAG}.example.org",
       "ghuser": c("QA_GITHUB_USER") or c("ID2_GITHUB"), "ghorg": c("QA_GITHUB_ORG", "github"),
       "gluser": c("QA_GITLAB_USER"), "glorg": c("QA_GITLAB_ORG", "gitlab-org")}

if not e2e.cfg("ALLOW_FIXTURE_WRITES", "0") == "1" or e2e.NO_WRITES:
    e2e.skip("FW-ALL", "fixture writes disabled (ALLOW_FIXTURE_WRITES / --no-writes)")
    e2e.summary_line()
    sys.exit(0)


def ccla():
    code, body = e2e.call(ID1, "GET", e2e.v4(f"/v4/signatures/project/{PROJ}/company/{CO}"))
    rows = (body or {}).get("signatures") or [] if isinstance(body, dict) else []
    return code, next((r for r in rows if r.get("signatureID") == CCLA), None)


def items(row, key):
    return sorted(x.get("approval_item") for x in (row or {}).get(key) or [])


def put_approval(data):
    return e2e.call(ID1, "PUT", e2e.v4(APPROVAL), data)


def my_ecla():
    # ID1 can hold ECLAs for several employers on the same group: only the write fixture's company (CO) counts
    code, body = e2e.call(ID1, "GET", e2e.v4("/v4/my-clas?pageSize=100"))
    rows = (body or {}).get("clas") or [] if isinstance(body, dict) else []
    rows = [r for r in rows if r.get("claGroupID") == GRP and r.get("companyID") == CO]
    wanted = next((r for r in rows if r.get("signatureID") == c("ID1_ECLA")), None) if c("ID1_ECLA") else None
    if wanted:
        return wanted
    valid = [r for r in rows if r.get("claType") == "ecla" and r.get("status") == "valid"]
    return valid[0] if valid else next((r for r in rows if r.get("claType") == "ecla"), None)


# ---- baseline ----------------------------------------------------------------------
code, row0 = ccla()
if code != 200 or row0 is None:
    e2e.check("A0-BASELINE", False, f"cannot read fixture CCLA {CCLA[:8]} (http {code})")
    e2e.summary_line()
    sys.exit(1)
baseline = {k: items(row0, k) for k in LISTS}
auto0 = row0.get("autoCreateECLA")
own_email0 = c("ID1_EMAIL") in baseline["emailApprovalList"]
own_gh0 = c("ID1_GITHUB") in baseline["githubUsernameApprovalList"]
e2e.info("A0-BASELINE", f"auto={auto0} lists={ {k: len(v) for k, v in baseline.items()} } own-email={own_email0} own-github={own_gh0} acl={[a.get('lfUsername') or a.get('username') for a in row0.get('signatureACL') or []]}")


def restore_baseline():
    """Safety net that runs even when a probe raises: synthetic entries introduced by this run removed, ID1's own
    entries and the auto-ECLA flag back to their baseline state (skipped with --keep-state, except for the synthetic ones)."""
    code, row = ccla()
    if row is None:
        e2e.info("F0-RESTORE", f"cannot re-read the fixture CCLA (http {code}) - nothing restored")
        return
    pending = {}
    for key, (value, remove_field) in introduced.items():
        if value in items(row, key):
            pending.setdefault(remove_field, []).append(value)
    if not e2e.KEEP_STATE:
        for present0, value, add_field, remove_field, key in (
                (own_email0, c("ID1_EMAIL"), "AddEmailApprovalList", "RemoveEmailApprovalList", "emailApprovalList"),
                (own_gh0, c("ID1_GITHUB"), "AddGithubUsernameApprovalList", "RemoveGithubUsernameApprovalList", "githubUsernameApprovalList")):
            now_present = value in items(row, key)
            if present0 and not now_present:
                pending.setdefault(add_field, []).append(value)
            elif now_present and not present0:
                pending.setdefault(remove_field, []).append(value)
    codes = [put_approval({field: values})[0] for field, values in pending.items()]
    if not e2e.KEEP_STATE and row.get("autoCreateECLA") != auto0:
        codes.append(e2e.call(ID1, "PUT", e2e.v4(AUTO), {"auto_create_ecla": bool(auto0)})[0])
    e2e.info("F0-RESTORE", f"baseline restore: {pending or 'no approval-list changes needed'}, auto={'reset' if len(codes) > len(pending) else 'unchanged'} -> http {codes}")


introduced = {}  # approval list key -> (synthetic value, Remove* field) for values that were absent at baseline and added by this run
try:
    # ---- A: approval list (everything from here runs under try/finally -> restore_baseline) ----
    code, body = put_approval({"AddEmailApprovalList": [c("ID1_EMAIL")], "AddGithubUsernameApprovalList": [c("ID1_GITHUB")]})
    _, row1 = ccla()
    e2e.check("A1-ADD-OWN-EMAIL-GITHUB", code == 200 and c("ID1_EMAIL") in items(row1, "emailApprovalList") and c("ID1_GITHUB") in items(row1, "githubUsernameApprovalList"),
              f"PUT add own e-mail + github -> {code}")
    # one PUT per criterion type: the approval-list PUT validates GitHub/GitLab names remotely and is NOT atomic
    # (a 400 on one item still persists the valid ones), so types are exercised and cleaned up independently
    added = {}
    for key, syn, add_field, remove_field, strict in (
            ("emailApprovalList", "email", "AddEmailApprovalList", "RemoveEmailApprovalList", True),
            ("domainApprovalList", "domain", "AddDomainApprovalList", "RemoveDomainApprovalList", True),
            ("githubUsernameApprovalList", "ghuser", "AddGithubUsernameApprovalList", "RemoveGithubUsernameApprovalList", False),
            ("githubOrgApprovalList", "ghorg", "AddGithubOrgApprovalList", "RemoveGithubOrgApprovalList", False),
            ("gitlabUsernameApprovalList", "gluser", "AddGitlabUsernameApprovalList", "RemoveGitlabUsernameApprovalList", False),
            ("gitlabOrgApprovalList", "glorg", "AddGitlabOrgApprovalList", "RemoveGitlabOrgApprovalList", False)):
        if not SYN[syn]:
            e2e.skip(f"A2-ADD-{key}", f"no suite-owned account configured for {key} (QA_GITHUB_USER/ID2_GITHUB or QA_GITLAB_USER)")
            continue
        pre = SYN[syn] in baseline[key]
        code, body = put_approval({add_field: [SYN[syn]]})
        _, r = ccla()
        present = SYN[syn] in items(r, key)
        added[key] = (syn, remove_field, present and not pre)
        if present and not pre:
            introduced[key] = (SYN[syn], remove_field)
        if strict:
            e2e.check(f"A2-ADD-{key}", code == 200 and present, f"PUT {add_field}=[{SYN[syn]}] -> {code}, present={present}", http=code)
        elif code == 200 and present:
            e2e.check(f"A2-ADD-{key}", True, f"PUT {add_field}=[{SYN[syn]}] -> 200, present=True", http=code)
        elif code == 400 and not present:
            e2e.info(f"A2-ADD-{key}", f"PUT {add_field}=[{SYN[syn]}] -> 400 (remote name validation refused the QA value: {e2e.redact(json.dumps(body), 120)})", http=code)
        else:
            e2e.check(f"A2-ADD-{key}", False, f"PUT {add_field}=[{SYN[syn]}] -> {code}, present={present}", http=code)
    code, body = put_approval({"AddEmailApprovalList": ["not-an-email"]})
    e2e.check("A3-INVALID-EMAIL-REJECTED", code in (400, 422), f"PUT invalid e-mail -> {code}")
    code, body = put_approval({"AddDomainApprovalList": [SYN["domain"]]})
    _, row3 = ccla()
    e2e.check("A4-DUPLICATE-ADD-IDEMPOTENT", code in (200, 400, 409) and items(row3, "domainApprovalList").count(SYN["domain"]) == 1,
              f"PUT duplicate domain -> {code}, occurrences={items(row3, 'domainApprovalList').count(SYN['domain'])}")
    # approval-list adds create user rows per criterion: visible through corporate-contributors (INFO)
    code, body = e2e.call(ID1, "GET", e2e.v4(f"/v4/company/external/{SFID}/cla-group/{GRP}/corporate-contributors?companyID={CO}&pageSize=100"))
    rows = (body or {}).get("list") or [] if isinstance(body, dict) else []
    e2e.info("A5-CORPORATE-CONTRIBUTORS", f"{len(rows)} contributor rows after the adds (http {code})")
    # cleanup the synthetic entries this run introduced (pre-existing QA_* values are left untouched), one type at a time
    left = {}
    for key, (syn, remove_field, introduced_now) in added.items():
        if not introduced_now:
            continue
        code, body = put_approval({remove_field: [SYN[syn]]})
        _, r = ccla()
        if SYN[syn] in items(r, key):
            left[key] = (code, SYN[syn])
        else:
            introduced.pop(key, None)
    e2e.check("A9-REMOVE-SYNTHETIC", not left, f"all synthetic criteria introduced by this run removed; left={left}")

    # ---- AU: auto-ECLA toggle ------------------------------------------------------------
    code, body = e2e.call(ID1, "PUT", e2e.v4(AUTO), {"auto_create_ecla": not bool(auto0)})
    _, r = ccla()
    e2e.check("AU1-AUTO-TOGGLE", code == 200 and r.get("autoCreateECLA") == (not bool(auto0)), f"PUT auto_create_ecla={not bool(auto0)} -> {code}, now {r.get('autoCreateECLA')}")
    code, body = e2e.call(ID1, "PUT", e2e.v4(AUTO), {"auto_create_ecla": bool(auto0)})
    _, r = ccla()
    e2e.check("AU2-AUTO-RESTORED", code == 200 and r.get("autoCreateECLA") == bool(auto0), f"PUT auto_create_ecla={bool(auto0)} -> {code}, now {r.get('autoCreateECLA')}")

    # ---- DS: designee / manager requests -----------------------------------------------------
    e2e.expect("DS1-IS-DESIGNEE-READ", ID1, 200, "GET", f"/v4/company/{SFID}/user/{ID1}/claGroupID/{GRP}/is-cla-manager-designee")
    first, last = (c("ID1_FULLNAME") + " ").split(" ", 1)
    e2e.expect("DS2-DUPLICATE-MANAGER-ADD", ID1, (200, 400, 409), "POST", f"/v4/company/{CO}/project/{PROJ}/cla-manager", {"firstName": first, "lastName": last.strip() or "QA", "userEmail": c("ID1_EMAIL")})
    e2e.expect("DS3-MANAGER-REQUESTS-LIST", ID1, (200, 403), "GET", f"/v4/company/{CO}/project/{PROJ}/cla-manager/requests")

    # ---- SA: sanctioned company --------------------------------------------------------------------
    if c("SANCTIONED_SFID"):
        sf, sid = c("SANCTIONED_SFID"), c("SANCTIONED_ID")
        for cid, codes, method, path, data in [
            ("SA-DESIGNEE-PROJECT", 403, "POST", f"/v4/company/{sf}/project/{PROJ}/cla-manager-designee", {"userEmail": c("ID1_EMAIL")}),
            ("SA-DESIGNEE-GROUP", 403, "POST", f"/v4/company/{sf}/claGroup/{GRP}/cla-manager-designee", {"userEmail": c("ID1_EMAIL")}),
            ("SA-REQUEST-CREATE", 403, "POST", f"/v4/company/{sf}/project/{PROJ}/cla-manager/requests", {"contactAdmin": False, "fullName": "QA Probe", "userEmail": c("ID1_EMAIL")}),
            ("SA-APPROVAL-LIST", 403, "PUT", f"/v4/signatures/project/{PROJ}/company/{sid}/clagroup/{GRP}/approval-list", {"RemoveEmailApprovalList": [f"absent-{e2e.TAG}@nomatch.invalid"]}),
            # ecla-auto-create answers 404 before loading the company when the sanctioned company has no signed CCLA on the
            # group (v2/signatures/handlers.go EclaAutoCreateHandler, TestEclaAutoCreateWithoutSignedCCLAReturnsNotFound); 403 otherwise
            ("SA-AUTO-CREATE", (403, 404), "PUT", f"/v4/signatures/company/{sid}/clagroup/{GRP}/ecla-auto-create", {"auto_create_ecla": False}),
            ("SA-MANAGER-CREATE", 403, "POST", f"/v4/company/{sid}/project/{PROJ}/cla-manager", {"firstName": "QA", "lastName": "Probe", "userEmail": c("ID1_EMAIL")}),
        ]:
            e2e.expect(cid, ID1, codes, method, path, data)
    else:
        e2e.skip("SA-SANCTIONED", f"SANCTIONED_* not configured for {e2e.ENV}")

    # ---- RM/RA: removal-only invalidation + recovery on ID1's own ECLA -------------------------------
    ack = my_ecla()
    if (not ack or ack.get("status") != "valid") and c("ID1_UID") and c("LEGACY_API"):
        # the browser modules manager-invalidate ID1's ack on every run (m3_org_lens) and such an ack is never resurrected:
        # acknowledge again explicitly through the legacy API, the way the contributor console does (same as interactions I3)
        put_approval({"AddEmailApprovalList": [c("ID1_EMAIL")], "AddGithubUsernameApprovalList": [c("ID1_GITHUB")]})
        time.sleep(4)
        ack = my_ecla()  # a removal-voided ack comes back on the re-add alone
        if not ack or ack.get("status") != "valid":
            was = "missing" if not ack else ack.get("status")
            legacy = c("LEGACY_API")
            code3, _ = e2e.call("anon", "POST", f"{legacy}/v2/check-prepare-employee-signature", {"project_id": GRP, "company_id": CO, "user_id": c("ID1_UID")})
            code4, body4 = e2e.call("anon", "POST", f"{legacy}/v2/request-employee-signature",
                                    {"project_id": GRP, "company_id": CO, "user_id": c("ID1_UID"), "return_url_type": "github", "return_url": c("SS_URL") + "/profile/clas"})
            sig4 = (body4 or {}).get("signature_id") if isinstance(body4, dict) else None
            time.sleep(4)
            ack = my_ecla()
            # a 500 "TypeError: 'NoneType' object is not subscriptable" here is the legacy (Python-parity) answer when the
            # user has no active pull-request metadata: the acknowledgment is already stored by then, my-clas is the truth
            e2e.info("RM-RA-REACK", f"ID1's ECLA was {was}: legacy check-prepare -> {code3}, request-employee-signature -> {code4} "
                                    f"signature={str(sig4)[:8]}; my-clas now {(ack or {}).get('signatureID', '-')[:8]}/{(ack or {}).get('status')}")
    if ack and ack.get("status") == "valid":
        sid = ack["signatureID"]
        code, body = put_approval({"RemoveEmailApprovalList": [c("ID1_EMAIL")], "RemoveGithubUsernameApprovalList": [c("ID1_GITHUB")]})
        time.sleep(4)
        after = my_ecla() or {}
        e2e.check("RM1-REMOVAL-VOIDS-OWN-ECLA", code == 200 and after.get("signatureID") == sid and after.get("status") != "valid",
                  f"remove own criteria -> {code}; ecla {sid[:8]} status={after.get('status')}")
        code, body = put_approval({"AddEmailApprovalList": [c("ID1_EMAIL")], "AddGithubUsernameApprovalList": [c("ID1_GITHUB")]})
        time.sleep(4)
        after2 = my_ecla() or {}
        e2e.check("RA1-READD-RECOVERS-OWN-ECLA", code == 200 and after2.get("signatureID") == sid and after2.get("status") == "valid",
                  f"re-add own criteria -> {code}; ecla {sid[:8]} status={after2.get('status')} (removal-voided acks are resurrected by a re-add)")
    elif ack:
        e2e.skip("RM-RA-OWN-ECLA", f"{ID1}'s ECLA on the fixture group is {ack.get('status')} - removal/re-add rule needs a valid one")
    else:
        e2e.skip("RM-RA-OWN-ECLA", f"{ID1} has no ECLA on the fixture group {GRP[:8]} on {e2e.ENV}")

except Exception as exc:  # the restore below must run even when a probe blows up
    e2e.check("FW-UNEXPECTED-ERROR", False, f"{type(exc).__name__}: {e2e.redact(str(exc), 200)}")
finally:
    restore_baseline()
# ---- final -------------------------------------------------------------------------------------------
_, rowF = ccla()
rowF = rowF or {}
final = {k: items(rowF, k) for k in LISTS}
diff = {k: sorted(set(final[k]) ^ set(baseline[k])) for k in LISTS if set(final[k]) != set(baseline[k])}
if e2e.KEEP_STATE:
    e2e.info("F1-LISTS-STATE", f"--keep-state: diff vs baseline {diff}")
else:
    e2e.check("F1-LISTS-BASELINE", not diff, f"approval lists back to baseline; diff={diff}")
e2e.check("F2-AUTO-BASELINE", rowF.get("autoCreateECLA") == auto0, f"autoCreateECLA={rowF.get('autoCreateECLA')} (baseline {auto0})")
e2e.summary_line()
sys.exit(0 if e2e.COUNTS["FAIL"] == 0 else 1)
