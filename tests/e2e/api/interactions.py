#!/usr/bin/env python3
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
"""Two-identity interaction chain on the QA company x CLA group (ID1 = CLA manager, ID2 = contributor).

B0 baseline (ACL, approval lists, auto-ECLA flag, ID2's ACS cla-manager scopes, ID2's acks on the group)
P0 pre-flight: the fixture is brought to the chain's start state - ID2 not in the ACL (removed + ACS revoke awaited when a
   kept proof / earlier --keep-state run left it there; put back in F), ID2's suite-owned criteria absent
I1 manager adds ID2's e-mail + GitHub criteria                 I2 ID2 prepare-sign (user record / identities)
I3 auto-ECLA on -> ID2's acknowledgment auto-created, visible to the manager; flag restored
I4 ID2 Me-lens reads: own ack managers 200, contact-manager request sent, foreign ack 404, org-scoped route 403
I5 manager invalidates -> ID2 sees invalidated, double invalidate 409, event names the manager, row unapproved
I6 re-add does NOT resurrect a manager-invalidated ack; criteria removal keeps it invalid
I7 auto-ECLA on + re-add after a manager invalidation does NOT create a fresh ack (Michal F-2)
W1 manager adds ID2 as CLA manager -> signatureACL + ACS cla-manager scope -> ID2 reads/writes 200, other org 403
W3 manager removes ID2 -> ACL + ACS revoked -> ID2 writes 403 (org-level reads may stay 200: contact role / warden cache)
F  everything back to the captured baseline: ID2's manager role is always restored when the baseline had it; the suite's
   own criteria / auto-ECLA changes are undone unless --keep-state

ACS state is read through utils/<env>_acs_role_flip.sh status (M2M token from SSM, read-only); when that
helper cannot run (no AWS credentials) the ACS checks are recorded as SKIP and the HTTP evidence stands.
"""
import json
import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "lib"))
import e2e  # noqa: E402

c = e2e.cfg
ID1, ID2, ID2_EMAIL, ID2_GH = c("ID1"), c("ID2"), c("ID2_EMAIL"), c("ID2_GITHUB")
PROJ, GRP, CO, SFID, CCLA = (c("WRITE_PROJECT_SFID") or c("PROJECT_SFID"), c("WRITE_CLA_GROUP") or c("CLA_GROUP"), c("WRITE_COMPANY_ID") or c("COMPANY_ID"),
                             c("WRITE_COMPANY_SFID") or c("COMPANY_SFID"), c("WRITE_COMPANY_CCLA") or c("COMPANY_CCLA"))
APPROVAL = f"/v4/signatures/project/{PROJ}/company/{CO}/clagroup/{GRP}/approval-list"
AUTO = f"/v4/signatures/company/{CO}/clagroup/{GRP}/ecla-auto-create"
CONTRIB = f"/v4/company/external/{SFID}/cla-group/{GRP}/corporate-contributors?companyID={CO}&pageSize=200"
LISTS = ("emailApprovalList", "domainApprovalList", "githubUsernameApprovalList", "githubOrgApprovalList", "gitlabUsernameApprovalList", "gitlabOrgApprovalList")
ACS_SCRIPT = os.path.join(e2e.REPO_ROOT, "utils", f"{e2e.ENV}_acs_role_flip.sh")
ACS_POLL = int(os.environ.get("E2E_ACS_POLL_SECS", "150"))

if c("ALLOW_FIXTURE_WRITES", "0") != "1" or e2e.NO_WRITES:
    e2e.skip("IX-ALL", "interaction chain disabled (ALLOW_FIXTURE_WRITES / --no-writes)")
    e2e.summary_line()
    sys.exit(0)


def ccla():
    code, body = e2e.call(ID1, "GET", e2e.v4(f"/v4/signatures/project/{PROJ}/company/{CO}"))
    rows = (body or {}).get("signatures") or [] if isinstance(body, dict) else []
    return code, next((r for r in rows if r.get("signatureID") == CCLA), None)


def items(row, key):
    return sorted(x.get("approval_item") for x in (row or {}).get(key) or [])


def acl(row):
    return sorted(a.get("lfUsername") or a.get("username") or "" for a in (row or {}).get("signatureACL") or [])


def my_clas(who):
    code, body = e2e.call(who, "GET", e2e.v4("/v4/my-clas?pageSize=100"))
    return code, ((body or {}).get("clas") or []) if isinstance(body, dict) else []


def id2_acks():
    code, rows = my_clas(ID2)
    return code, [r for r in rows if r.get("claGroupID") == GRP and r.get("claType") == "ecla"]


def id2_ack(sid=None):
    code, acks = id2_acks()
    if sid:
        return code, next((r for r in acks if r.get("signatureID") == sid), None)
    valid = [r for r in acks if r.get("status") == "valid"]
    return code, (valid[0] if valid else None)


_acs_ok = [None]


def acs(user):
    """(scopes, roles) of `user` from the repo ACS helper; ([], None) and a one-time SKIP when unavailable."""
    if _acs_ok[0] is False:
        return [], None
    try:
        out = subprocess.run([ACS_SCRIPT, "status", user], capture_output=True, text=True, timeout=180).stdout
    except (OSError, subprocess.SubprocessError) as err:
        out = ""
        e2e.log(f"acs helper failed: {err}")
    roles = [l.strip() for l in out.splitlines() if l.startswith("roles:")]
    if not roles:
        if _acs_ok[0] is None:
            e2e.skip("ACS-HELPER", f"{os.path.basename(ACS_SCRIPT)} status unavailable (AWS credentials?) - ACS checks skipped")
        _acs_ok[0] = False
        return [], None
    _acs_ok[0] = True
    scopes = sorted(l.strip() for l in out.splitlines() if l.strip().startswith(("cla-manager:", "cla-manager-designee:", "cla-signatory:")))
    return scopes, roles[0]


def foundation_sfid():
    """foundationSFID of the write CLA group (from the company cla-groups row): groups signed at foundation level get
    their ACS cla-manager scope on the foundation, not on the project (dynamo_events updateCLAManagerPermissions)."""
    code, body = e2e.call(ID1, "GET", e2e.v4(f"/v4/company/external/{SFID}/cla-groups"))
    rows = (body or {}).get("list") or [] if isinstance(body, dict) else []
    row = next((r for r in rows if r.get("claGroupID") == GRP), None)
    return (row or {}).get("foundationSFID") or ""


FOUNDATION = foundation_sfid()
MANAGER_SCOPES = {f"{PROJ}|{SFID}"} | ({f"{FOUNDATION}|{SFID}"} if FOUNDATION else set())


def manager_scopes(scopes):
    """the fixture-org cla-manager scopes (project- or foundation-level) present in an ACS scope listing"""
    return sorted(s for s in scopes if s.startswith("cla-manager:") and any(m in s for m in MANAGER_SCOPES))


def acs_has_manager(scopes):
    return bool(manager_scopes(scopes))


def events(match, budget=30):
    """newest fixture event whose Pascal/camel-case data matches `match(text)`; both company id forms of the route are
    tried (the UUID form lists project-level groups, the SFID form foundation-level ones) and the feed is polled."""
    def fetch():
        for co in (CO, SFID):
            code, body = e2e.call(ID1, "GET", e2e.v4(f"/v4/company/{co}/project/{PROJ}/events?pageSize=40"))
            evs = ((body or {}).get("Events") or (body or {}).get("events") or []) if isinstance(body, dict) else []
            for x in evs:
                text = " ".join(str(x.get(k) or "") for k in ("EventData", "eventData", "EventSummary", "eventSummary", "EventType", "eventType"))
                if match(text):
                    return code, x
        return code, None
    (code, ev), _ = poll(fetch, lambda v: v[1] is not None, budget)
    return code, ev


def poll(fn, want, budget, step=5):
    t0 = time.time()
    val = fn()
    while not want(val) and time.time() - t0 < budget:
        time.sleep(step)
        val = fn()
    return val, int(time.time() - t0)


def contrib_row():
    code, body = e2e.call(ID1, "GET", e2e.v4(CONTRIB))
    rows = (body or {}).get("list") or [] if isinstance(body, dict) else []
    mine = [r for r in rows if ID2 in (r.get("github_id"), r.get("linux_foundation_id"), r.get("lf_username")) or r.get("email") == ID2_EMAIL]
    return code, mine, len(rows)


def put_approval(who, data):
    return e2e.call(who, "PUT", e2e.v4(APPROVAL), data)


# ---- B0 baseline --------------------------------------------------------------------------------------
code, row0 = ccla()
if code != 200 or row0 is None:
    e2e.check("B0-BASELINE", False, f"cannot read fixture CCLA {CCLA[:8]} (http {code})")
    e2e.summary_line()
    sys.exit(1)
base = {"acl": acl(row0), "lists": {k: items(row0, k) for k in LISTS}, "auto": bool(row0.get("autoCreateECLA"))}
acs0, _roles0 = acs(ID2)
_, acks0 = id2_acks()
_, rows1 = my_clas(ID1)
id1_ecla0 = next((r for r in rows1 if r.get("signatureID") == c("ID1_ECLA")), None) if c("ID1_ECLA") else None
id2_in_acl0 = ID2 in base["acl"]
e2e.info("B0-BASELINE", f"acl={base['acl']} auto={base['auto']} lists={ {k: len(v) for k, v in base['lists'].items()} } "
         f"id2-acs={acs0} id2-acks={[(a.get('signatureID', '')[:8], a.get('status')) for a in acks0]} id1-ecla={(id1_ecla0 or {}).get('status')}")
pre_ids = {a.get("signatureID") for a in acks0}
pre_valid = next((a for a in acks0 if a.get("status") == "valid"), None)

# ---- P0 pre-flight: reach the chain's start state (the baseline is restored in F) ----------------------------------
if id2_in_acl0:
    e2e.info("P0-ID2-MANAGER-AT-BASELINE", f"{ID2} is in the fixture ACL at baseline (kept proof / --keep-state of an earlier run): "
             f"removed now so W1/W3 run from the chain's start state; F puts it back")
    code, body = e2e.call(ID1, "DELETE", e2e.v4(f"/v4/company/{CO}/project/{PROJ}/cla-manager/{ID2}"))
    _, row = ccla()
    e2e.check("P0-ID2-REMOVED-FROM-ACL", code in (200, 204) and ID2 not in acl(row), f"DELETE cla-manager/{ID2} -> {code}; acl={acl(row)}", http=code)
if _acs_ok[0] is not False and (id2_in_acl0 or acs_has_manager(acs0)):
    # the revoke runs in the dynamo-signatures lambda: wait for it so W1's grant cannot race an older revoke
    (sc, _r), waited = poll(lambda: acs(ID2), lambda v: not acs_has_manager(v[0]), ACS_POLL)
    if acs_has_manager(sc):
        e2e.info("P0-ID2-STALE-ACS-SCOPE", f"{ID2} still holds ACS {manager_scopes(sc)} after {waited}s without being in the ACL (a revoke failed, "
                 f"e.g. org-service 502 in the dynamo-signatures lambda) - W1/W3 re-grant and revoke it; F reports the outcome")
    else:
        e2e.check("P0-ID2-ACS-CLEAN", True, f"no fixture-org cla-manager scope on {ID2} (after {waited}s)")
    acs0 = sc
else:
    e2e.check("P0-ID2-NOT-MANAGER", True, f"{ID2} starts outside the fixture ACL (acs-manager scopes={manager_scopes(acs0) or 'none'})")
# suite-owned criteria: ID2's e-mail and GitHub username are added by I1 and must be absent before and after the chain
stale = {"RemoveEmailApprovalList": [x for x in [ID2_EMAIL] if x in base["lists"]["emailApprovalList"]],
         "RemoveGithubUsernameApprovalList": [x for x in [ID2_GH] if x in base["lists"]["githubUsernameApprovalList"]]}
stale = {k: v for k, v in stale.items() if v}
expected_lists = {k: sorted(set(v) - {ID2_EMAIL, ID2_GH}) for k, v in base["lists"].items()}
if stale:
    code, body = put_approval(ID1, stale)
    _, row = ccla()
    e2e.info("P0-STALE-ID2-CRITERIA-REMOVED", f"{stale} were on the approval lists at baseline (earlier run ended early / --keep-state) -> removed ({code}); "
             f"lists now={ {k: len(items(row, k)) for k in LISTS} }", http=code)


def org_scoped_probe(cid):
    """ID2 holds no fixture-org role: the org-scoped manager-request route must be refused by the gateway (warden, ACS scopes).
    Within ~10 min of a previous allow the warden may still serve its cached decision - then the handler answers 400
    "project already signed" for the signed fixture; that window is reported as INFO, anything else than 403/that 400 FAILs."""
    code, body = e2e.call(ID2, "POST", e2e.v4(f"/v4/company/{CO}/project/{PROJ}/cla-manager/requests"),
                          {"contactAdmin": False, "fullName": f"{c('ID2_FIRST')} {c('ID2_LAST')}", "userEmail": ID2_EMAIL})
    msg = (body or {}).get("Message", "") if isinstance(body, dict) else ""
    if code == 400 and "already signed" in msg:
        e2e.info(cid + "-CACHED", f"POST cla-manager/requests as {ID2} -> 400 '{msg}': warden served a cached allow (<=10 min after the last grant)", http=code)
    else:
        e2e.check(cid, code == 403, f"POST cla-manager/requests as {ID2} -> {code} {e2e.redact(json.dumps(body), 100) if code != 403 else ''}", http=code)


def reconcile():
    """Runs in `finally`: whatever happened in the chain (a FAIL, a timeout, an exception), ID2's manager role, the
    suite-owned criteria and the auto-ECLA flag go back to their baseline state - a kept-proof manager role is re-added
    whatever the flags, everything else is left as is with --keep-state."""
    code, row = ccla()
    if row is None:
        e2e.info("F-RECONCILE", f"cannot re-read the fixture CCLA (http {code}) - nothing reconciled")
        return
    if id2_in_acl0 and ID2 not in acl(row):
        # the baseline had ID2 as a manager (kept proof): put it back so the end state equals the baseline
        code, body = e2e.call(ID1, "POST", e2e.v4(f"/v4/company/{CO}/project/{PROJ}/cla-manager"), {"firstName": c("ID2_FIRST"), "lastName": c("ID2_LAST"), "userEmail": ID2_EMAIL})
        _, row = ccla()
        (sc, _r), waited = poll(lambda: acs(ID2), lambda v: acs_has_manager(v[0]), ACS_POLL) if _acs_ok[0] is not False else (([], None), 0)
        e2e.check("F-ID2-MANAGER-RESTORED", code in (200, 201) and ID2 in acl(row) and (acs_has_manager(sc) or _acs_ok[0] is False),
                  f"re-add -> {code}; acl={acl(row)}; ACS manager scope back after {waited}s: {manager_scopes(sc) or 'n/a'}", http=code)
    elif not id2_in_acl0 and ID2 in acl(row) and not e2e.KEEP_STATE:
        # W3 did not run or failed: the role granted by W1 must not outlive the run
        code, body = e2e.call(ID1, "DELETE", e2e.v4(f"/v4/company/{CO}/project/{PROJ}/cla-manager/{ID2}"))
        _, row = ccla()
        (sc, _r), waited = poll(lambda: acs(ID2), lambda v: not acs_has_manager(v[0]), ACS_POLL) if _acs_ok[0] is not False else (([], None), 0)
        e2e.check("F-ID2-MANAGER-REMOVED", code in (200, 204) and ID2 not in acl(row) and (not acs_has_manager(sc) or _acs_ok[0] is False),
                  f"the chain left {ID2} in the ACL: DELETE cla-manager/{ID2} -> {code}; acl={acl(row)}; ACS scope gone after {waited}s: {manager_scopes(sc) or 'none'}", http=code)
    if not e2e.KEEP_STATE:
        # I1's (or a stale earlier run's) ID2 criteria are removed here too, in case the I5-I7 chain was skipped or ended early
        extra = {"RemoveEmailApprovalList": [x for x in [ID2_EMAIL] if x in items(row, "emailApprovalList")],
                 "RemoveGithubUsernameApprovalList": [x for x in [ID2_GH] if x in items(row, "githubUsernameApprovalList")]}
        extra = {k: v for k, v in extra.items() if v}
        if extra:
            code, body = put_approval(ID1, extra)
            e2e.info("F-RESTORE-CRITERIA", f"removed {extra} -> {code}", http=code)
        if bool(row.get("autoCreateECLA")) != base["auto"]:
            code, body = e2e.call(ID1, "PUT", e2e.v4(AUTO), {"auto_create_ecla": base["auto"]})
            e2e.info("F-RESTORE-AUTO", f"auto_create_ecla back to {base['auto']} -> {code}", http=code)


ACK = None
try:
    if not acs_has_manager(acs0):
        org_scoped_probe("I0-ID2-ORG-SCOPED-REQUEST-ROUTE-403")

    # ---- I1 manager adds ID2's criteria -----------------------------------------------------------------------
    code, body = put_approval(ID1, {"AddEmailApprovalList": [ID2_EMAIL], "AddGithubUsernameApprovalList": [ID2_GH]})
    _, row = ccla()
    e2e.check("I1-MANAGER-ADDS-ID2-CRITERIA", code == 200 and ID2_EMAIL in items(row, "emailApprovalList") and ID2_GH in items(row, "githubUsernameApprovalList"),
              f"PUT add {ID2_EMAIL} + github {ID2_GH} -> {code}", http=code)

    # ---- I2 contributor prepare-sign ------------------------------------------------------------------------------
    code, body = e2e.call(ID2, "POST", e2e.v4("/v4/self-serve/prepare-sign"),
                          {"claGroupId": GRP, "returnUrl": c("SS_URL") + "/profile/clas", "githubUsername": ID2_GH, "email": ID2_EMAIL})
    b = body if isinstance(body, dict) else {}
    ID2_USER_ID = b.get("userId")
    e2e.check("I2-ID2-PREPARE-SIGN", code == 200 and bool(ID2_USER_ID),
              f"POST prepare-sign -> {code} userCreated={b.get('userCreated')} identity={b.get('identity')} skipped={b.get('skippedIdentities')}", http=code)

    # ---- I3 auto-ECLA on -> acknowledgment auto-created ----------------------------------------------------------------
    code, body = e2e.call(ID1, "PUT", e2e.v4(AUTO), {"auto_create_ecla": True})
    e2e.check("I3A-AUTO-ON", code == 200, f"PUT auto_create_ecla=true -> {code}", http=code)
    pre_manager_invalidated = [a for a in acks0 if a.get("status") == "invalidated"]
    if pre_valid:
        ack = pre_valid
        e2e.info("I3-ACK-PREEXISTING", f"{ID2} already holds a valid ack {ack['signatureID'][:8]} on the group - auto-ECLA will not add another; reusing it")
    else:
        (code2, ack), waited = poll(id2_ack, lambda v: v[1] is not None, 60)
        if ack is None:  # creation is triggered on the criterion write when auto is on: re-put the idempotent criterion
            put_approval(ID1, {"AddEmailApprovalList": [ID2_EMAIL]})
            (code2, ack), waited2 = poll(id2_ack, lambda v: v[1] is not None, 60)
            waited += waited2
        if ack is None and pre_manager_invalidated:
            # design rule (Michal F-2): an acknowledgment a manager invalidated is never auto-recreated; the contributor has to
            # acknowledge again explicitly - the console flow behind prepare-sign's signUrl, driven here through the legacy API
            e2e.check("I3-AUTO-ECLA-NO-RESURRECTION", True,
                      f"auto-ECLA created nothing for {ID2} within {waited}s: {len(pre_manager_invalidated)} manager-invalidated ack(s) "
                      f"{[a.get('signatureID', '')[:8] for a in pre_manager_invalidated]} stay invalidated (expected)")
            if ID2_USER_ID:
                legacy = c("LEGACY_API")
                code3, _body3 = e2e.call("anon", "POST", f"{legacy}/v2/check-prepare-employee-signature", {"project_id": GRP, "company_id": CO, "user_id": ID2_USER_ID})
                code4, body4 = e2e.call("anon", "POST", f"{legacy}/v2/request-employee-signature",
                                        {"project_id": GRP, "company_id": CO, "user_id": ID2_USER_ID, "return_url_type": "github", "return_url": c("SS_URL") + "/profile/clas"})
                sig4 = (body4 or {}).get("signature_id") if isinstance(body4, dict) else None
                (code2, ack), waited3 = poll(id2_ack, lambda v: v[1] is not None, 60)
                e2e.check("I3-EXPLICIT-REACK-CREATES-ACK", code3 == 200 and code4 == 200 and sig4 is not None and ack is not None and ack.get("signatureID") == sig4,
                          f"legacy check-prepare -> {code3}, request-employee-signature -> {code4} signature={str(sig4)[:8]}; "
                          f"my-clas shows valid ack {(ack or {}).get('signatureID', '-')[:8]} after {waited3}s "
                          f"{e2e.redact(json.dumps(body4), 160) if code4 != 200 or sig4 is None else ''}", ecla=sig4)
            else:
                e2e.skip("I3-EXPLICIT-REACK-CREATES-ACK", "prepare-sign returned no userId - explicit re-acknowledgment impossible")
        else:
            # a pre-existing ack that was only removal-voided (the previous run's cleanup took ID2 off the list) comes back
            # on the criterion re-add instead of a new one being created - the approval-list rule, not a defect
            resurrected = ack is not None and ack.get("signatureID") in pre_ids
            e2e.check("I3-AUTO-ECLA-CREATED-FOR-ID2", ack is not None and ack.get("status") == "valid",
                      f"{'removal-voided ack resurrected by the criterion re-add' if resurrected else 'new'} valid ack for {ID2}: "
                      f"{(ack or {}).get('signatureID', '-')[:8]} company={(ack or {}).get('companyName')} after {waited}s",
                      ecla=(ack or {}).get("signatureID"))
    ACK = (ack or {}).get("signatureID")
    code, mine, n = contrib_row()
    e2e.check("I3B-MANAGER-SEES-ID2-ACK", code == 200 and any(r.get("signatureApproved", True) is not False for r in mine),
              f"corporate-contributors -> {code}: {len(mine)} row(s) for {ID2} of {n}; approved flags={[r.get('signatureApproved') for r in mine]}", http=code)
    code, body = e2e.call(ID1, "PUT", e2e.v4(AUTO), {"auto_create_ecla": base["auto"]})
    _, row = ccla()
    e2e.check("I3C-AUTO-RESTORED", code == 200 and bool(row.get("autoCreateECLA")) == base["auto"], f"PUT auto_create_ecla={base['auto']} -> {code}", http=code)

    # ---- I4 contributor Me-lens reads --------------------------------------------------------------------------------
    if ACK:
        code, body = e2e.call(ID2, "GET", e2e.v4(f"/v4/my-clas/{ACK}/cla-managers"))
        mgrs = ((body or {}).get("list") or (body or {}).get("claManagers") or (body or {}).get("managers") or []) if isinstance(body, dict) else []
        e2e.check("I4-ID2-READS-OWN-ACK-MANAGERS", code == 200, f"GET my-clas/{ACK[:8]}/cla-managers -> {code} ({len(mgrs)} managers listed)", http=code)
        if c("ALLOW_EMAIL", "1") == "1":  # sends one real e-mail to ID1's mailbox per run
            code, body = e2e.call(ID2, "POST", e2e.v4(f"/v4/my-clas/{ACK}/cla-manager-requests"),
                                  {"requestType": "contact", "recipients": [ID1], "message": f"EasyCLA {e2e.ENV} E2E suite {e2e.TAG}: contact-manager request from {ID2} (test, ignore)"})
            e2e.check("I4B-ID2-CONTACT-MANAGER-REQUEST", code == 200 and isinstance(body, dict) and body.get("status") == "sent",
                      f"POST cla-manager-requests(contact) -> {code} status={(body or {}).get('status') if isinstance(body, dict) else body}", http=code)
        else:
            e2e.skip("I4B-ID2-CONTACT-MANAGER-REQUEST", "ALLOW_EMAIL=0 (the contact request e-mails ID1)")
    else:
        e2e.skip("I4-ID2-OWN-ACK-READS", "no acknowledgment for ID2 - I4 skipped")
    if c("ID1_ECLA"):
        e2e.expect("I4C-ID2-FOREIGN-ACK-MANAGERS-404", ID2, 404, "GET", f"/v4/my-clas/{c('ID1_ECLA')}/cla-managers")
    else:
        e2e.skip("I4C-ID2-FOREIGN-ACK-MANAGERS-404", "ID1_ECLA not configured")

    # ---- I5 manager invalidates ---------------------------------------------------------------------------------------
    if ACK and ack.get("status") == "valid":
        code, body = e2e.call(ID1, "PUT", e2e.v4(f"/v4/cla-group/{GRP}/ecla/{ACK}/invalidate"),
                              {"reason": "other", "note": f"EasyCLA {e2e.ENV} E2E suite {e2e.TAG}: manager invalidation of {ID2}'s acknowledgment (test)"})
        e2e.check("I5-MANAGER-INVALIDATES", code == 200, f"PUT ecla/{ACK[:8]}/invalidate -> {code}", http=code)
        time.sleep(3)
        code, a2 = id2_ack(ACK)
        e2e.check("I5B-ID2-SEES-INVALIDATED", code == 200 and (a2 or {}).get("status") == "invalidated", f"my-clas status={(a2 or {}).get('status')}")
        e2e.expect("I5C-DOUBLE-INVALIDATE-409", ID1, 409, "PUT", f"/v4/cla-group/{GRP}/ecla/{ACK}/invalidate", {"reason": "other", "note": "repeat"})
        code, ev = events(lambda t: "signature.invalidated" in t or ("nvalidated" in t and ID2 in t))
        by = ((ev or {}).get("UserName") or (ev or {}).get("userName"), (ev or {}).get("LfUsername") or (ev or {}).get("lfUsername"))
        text = (ev or {}).get("EventData") or (ev or {}).get("eventData") or ""
        e2e.check("I5D-EVENT-NAMES-MANAGER", ev is not None and ID2 in text and (ID1 in by or c("ID1_FULLNAME").split()[-1] in text),
                  f"events -> {code}; invalidation event={e2e.redact(text, 160)} by={by}", http=code)
        code, mine, n = contrib_row()
        me = next((r for r in mine if r.get("signatureID") == ACK), None) or (mine[0] if mine else None)
        e2e.check("I5E-CONTRIB-ROW-UNAPPROVED", code == 200 and me is not None and me.get("signatureApproved") is False,
                  f"corporate-contributors row approved={(me or {}).get('signatureApproved')} ({len(mine)} rows for {ID2})", http=code)
        # ---- I6 re-add does not resurrect; removal keeps invalid ----------------------------------------------------------
        code, body = put_approval(ID1, {"AddEmailApprovalList": [ID2_EMAIL]})
        time.sleep(3)
        _, a3 = id2_ack(ACK)
        e2e.check("I6-READD-DOES-NOT-RESURRECT", code in (200, 400, 409) and (a3 or {}).get("status") != "valid", f"re-add -> {code}; ack status={(a3 or {}).get('status')}", http=code)
        code, body = put_approval(ID1, {"RemoveEmailApprovalList": [ID2_EMAIL], "RemoveGithubUsernameApprovalList": [ID2_GH]})
        _, row = ccla()
        time.sleep(3)
        _, a4 = id2_ack(ACK)
        e2e.check("I6B-CRITERIA-REMOVED-STILL-INVALID", code == 200 and ID2_EMAIL not in items(row, "emailApprovalList") and ID2_GH not in items(row, "githubUsernameApprovalList") and (a4 or {}).get("status") != "valid",
                  f"remove -> {code}; ack status={(a4 or {}).get('status')}", http=code)
        # ---- I7 auto-ECLA + re-add after a manager invalidation: no fresh ack (Michal F-2) ----------------------------------
        _, before = id2_acks()
        before_ids = {a.get("signatureID") for a in before}
        code, body = e2e.call(ID1, "PUT", e2e.v4(AUTO), {"auto_create_ecla": True})
        code2, _body2 = put_approval(ID1, {"AddEmailApprovalList": [ID2_EMAIL], "AddGithubUsernameApprovalList": [ID2_GH]})
        (_, after), waited = poll(id2_acks, lambda v: any(a.get("signatureID") not in before_ids for a in v[1]), 60)
        fresh = [a for a in after if a.get("signatureID") not in before_ids]
        e2e.check("I7-NO-FRESH-ACK-AFTER-MANAGER-INVALIDATION", code == 200 and code2 == 200 and not fresh,
                  f"auto on ({code}) + re-add ({code2}): fresh acks within {waited}s = {[(a.get('signatureID', '')[:8], a.get('status')) for a in fresh]} (expected none)")
        e2e.call(ID1, "PUT", e2e.v4(AUTO), {"auto_create_ecla": base["auto"]})
        put_approval(ID1, {"RemoveEmailApprovalList": [ID2_EMAIL], "RemoveGithubUsernameApprovalList": [ID2_GH]})
    else:
        e2e.skip("I5-I7-INVALIDATION-CHAIN", "no valid acknowledgment for ID2 - invalidation chain skipped")

    # ---- W1 manager adds ID2 as CLA manager --------------------------------------------------------------------------------
    code, body = e2e.call(ID1, "POST", e2e.v4(f"/v4/company/{CO}/project/{PROJ}/cla-manager"), {"firstName": c("ID2_FIRST"), "lastName": c("ID2_LAST"), "userEmail": ID2_EMAIL})
    e2e.check("W1-MANAGER-ADDS-ID2-AS-CLA-MANAGER", code in (200, 201), f"POST cla-manager -> {code} {e2e.redact(json.dumps(body), 120) if code >= 400 else ''}", http=code)
    _, row = ccla()
    e2e.check("W1B-ID2-IN-SIGNATURE-ACL", ID2 in acl(row), f"signatureACL={acl(row)}")
    if _acs_ok[0] is not False:
        (sc, _r), waited = poll(lambda: acs(ID2), lambda v: acs_has_manager(v[0]), ACS_POLL)
        e2e.check("W1C-ID2-ACS-CLA-MANAGER-GRANTED", acs_has_manager(sc),
                  f"ACS cla-manager scope on project {PROJ} or foundation {FOUNDATION or '-'} |{SFID} after {waited}s: {manager_scopes(sc)}")
    (code, body), waited = poll(lambda: e2e.call(ID2, "GET", e2e.v4(f"/v4/company/external/{SFID}/cla-groups"), headers={"Cache-Control": "no-cache"}), lambda v: v[0] == 200, 120)
    e2e.check("W2-ID2-READS-COMPANY-GROUPS-200", code == 200, f"GET company/external/{SFID}/cla-groups as {ID2} -> {code} after {waited}s", http=code)
    e2e.expect("W2B-ID2-READS-CCLA-200", ID2, 200, "GET", f"/v4/signatures/project/{PROJ}/company/{CO}")
    dom = f"qa-id2-{e2e.TAG}.example.org"
    code, body = put_approval(ID2, {"AddDomainApprovalList": [dom]})
    _, row = ccla()
    added = dom in items(row, "domainApprovalList")
    code2, _body2 = put_approval(ID2, {"RemoveDomainApprovalList": [dom]})
    _, row = ccla()
    e2e.check("W2C-ID2-APPROVAL-WRITE-200-AND-RESTORED", code == 200 and added and code2 == 200 and dom not in items(row, "domainApprovalList"), f"add -> {code} (present={added}), remove -> {code2}")
    e2e.expect("W2D-ID2-OTHER-ORG-STILL-403", ID2, 403, "GET", f"/v4/company/external/{c('OTHER_ORG_SFID')}/cla-groups")
    code, sc_id2 = e2e.scopes(ID2)
    e2e.info("W2E-ID2-USER-SERVICE-SCOPES", f"user-service cla-manager scopes now: {len(sc_id2)} (fixture org present={e2e.has_scope(sc_id2, SFID)}; this feed lags ACS by ~25 min)")

    # ---- W3 manager removes ID2 ---------------------------------------------------------------------------------------------
    code, body = e2e.call(ID1, "DELETE", e2e.v4(f"/v4/company/{CO}/project/{PROJ}/cla-manager/{ID2}"))
    e2e.check("W3-MANAGER-REMOVES-ID2", code in (200, 204), f"DELETE cla-manager/{ID2} -> {code}", http=code)
    _, row = ccla()
    e2e.check("W3B-ID2-NOT-IN-ACL", ID2 not in acl(row), f"signatureACL={acl(row)}")
    if _acs_ok[0] is not False:
        (sc, _r), waited = poll(lambda: acs(ID2), lambda v: not acs_has_manager(v[0]), ACS_POLL)
        e2e.check("W3C-ID2-ACS-REVOKED", not acs_has_manager(sc),
                  f"ACS cla-manager scope gone after {waited}s: {manager_scopes(sc) or 'none'}" if not acs_has_manager(sc) else
                  f"ACS cla-manager scope still present after {waited}s: {manager_scopes(sc)} (dynamo-events revoke failed? check the "
                  f"dynamo-signatures lambda log for a DeleteOrgUserRoleOrgScopeProjectOrg error, e.g. an org-service 502)")
    time.sleep(5)
    e2e.expect("W4B-ID2-WRITE-403-AGAIN", ID2, 403, "PUT", APPROVAL, {"RemoveDomainApprovalList": [f"absent-{e2e.TAG}.nomatch.invalid"]})
    code, body = e2e.call(ID2, "GET", e2e.v4(f"/v4/company/external/{SFID}/cla-groups"), headers={"Cache-Control": "no-cache"})
    _sc, roles_now = acs(ID2)
    contact = "contact" in (roles_now or "")
    if code == 403:
        e2e.check("W4-ID2-READ-403-AGAIN", True, f"GET company/external/{SFID}/cla-groups as {ID2} -> 403", http=code)
    else:
        # known behaviour, recorded so the day it flips to 403 the summary shows it: the org-level `contact` role granted with
        # the first manager add survives the removal (org-only reads stay 200); right after a revoke the warden may also still
        # serve its cached allow (<=10 min)
        e2e.info("W4-ID2-READ-AFTER-REMOVAL", f"GET company/external/{SFID}/cla-groups as {ID2} -> {code} after the revoke "
                 f"({'org contact role survives' if contact else 'warden answer cache <=10 min'}; the write above is already 403)", http=code)
    e2e.info("W4E-ID2-CONTACT-ROLE-SURVIVES", f"ACS {roles_now or 'roles: ?'} -> contact role {'present' if contact else 'absent'} after the manager removal")
    org_scoped_probe("W4C-ID2-ORG-SCOPED-REQUEST-ROUTE-403")

except Exception as exc:  # reconcile() below must run even when a probe blows up
    e2e.check("IX-UNEXPECTED-ERROR", False, f"{type(exc).__name__}: {e2e.redact(str(exc), 200)}")
finally:
    reconcile()

# ---- F final checks ------------------------------------------------------------------------------------------------------
_, rowF = ccla()
diff = {k: sorted(set(items(rowF, k)) ^ set(v)) for k, v in expected_lists.items() if set(items(rowF, k)) != set(v)}
e2e.check("F-ACL-BASELINE", sorted(acl(rowF)) == sorted(base["acl"]), f"acl={acl(rowF)} baseline={base['acl']}")
if e2e.KEEP_STATE:
    e2e.info("F-STATE", f"--keep-state: lists diff vs baseline={diff} auto={rowF.get('autoCreateECLA')} (baseline {base['auto']})")
else:
    e2e.check("F-LISTS-BASELINE", not diff, f"approval lists diff vs baseline (minus {ID2}'s suite-owned criteria)={diff}")
    e2e.check("F-AUTO-BASELINE", bool(rowF.get("autoCreateECLA")) == base["auto"], f"autoCreateECLA={rowF.get('autoCreateECLA')} baseline={base['auto']}")
if _acs_ok[0]:
    scF, _r = acs(ID2)
    if id2_in_acl0:
        e2e.check("F-ID2-ACS-RESTORED", acs_has_manager(scF), f"{ID2} holds the fixture-org cla-manager scope again as at baseline: {manager_scopes(scF)}")
    else:
        e2e.check("F-ID2-ACS-CLEAN", not acs_has_manager(scF),
                  f"no fixture-org cla-manager scope left on {ID2} (baseline had {manager_scopes(acs0) or 'none'}); other scopes={scF}")
if c("ID1_ECLA") and id1_ecla0:
    _, rows1 = my_clas(ID1)
    a = next((r for r in rows1 if r.get("signatureID") == c("ID1_ECLA")), None)
    e2e.check("F-ID1-ECLA-UNCHANGED", (a or {}).get("status") == id1_ecla0.get("status"), f"{ID1} ecla status={(a or {}).get('status')} (baseline {id1_ecla0.get('status')})")
if ACK:
    _, aF = id2_ack(ACK)
    e2e.check("F-ID2-ACK-RETAINED-INVALIDATED", (aF or {}).get("status") == "invalidated", f"{ID2} ack {ACK[:8]} status={(aF or {}).get('status')} (kept as proof)")
e2e.summary_line()
sys.exit(0 if e2e.COUNTS["FAIL"] == 0 else 1)
