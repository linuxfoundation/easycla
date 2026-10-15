#!/usr/bin/env python3
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
"""Read ACL matrix (ops#382) for both identities and anonymous.

ID1 (CLA manager of the fixture companies): own-company reads 200, group-wide / other-company / sanctioned
reads 403. ID2: expectations are derived from its current user-service cla-manager scopes (what Self Serve
reads) - 403 everywhere unless a scope on the fixture organization exists (then 200). Anonymous: 401.
Also records the identity reads (my-clas, identities, permissions) and the fixture CCLA shape.
"""
import json
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "lib"))
import e2e  # noqa: E402

c = e2e.cfg
ID1, ID2 = c("ID1"), c("ID2")
PROJ, GRP, CO, SFID, CCLA = c("PROJECT_SFID"), c("CLA_GROUP"), c("COMPANY_ID"), c("COMPANY_SFID"), c("COMPANY_CCLA")
CO2, SFID2, CCLA2 = c("COMPANY2_ID"), c("COMPANY2_SFID"), c("COMPANY2_CCLA")
SANCT_ID, SANCT_SFID = c("SANCTIONED_ID"), c("SANCTIONED_SFID")
OTHER_ORG, OTHER_CO, OTHER_PROJ = c("OTHER_ORG_SFID"), c("OTHER_COMPANY_ID"), c("OTHER_PROJECT_SFID")
CONTRIB = f"/v4/company/external/{SFID}/cla-group/{GRP}/corporate-contributors?companyID={CO}&pageSize=50"

# ---- identity reads + scopes -------------------------------------------------
# authoritative source = ACS rolescopes (repo helper); the user-service feed only as fallback (it lags ACS by up to ~30 min)
PROJECT_SFIDS = [p for p in (PROJ, c("FOUNDATION_SFID")) if p]
st1 = e2e.identity_state(ID1, SFID, PROJECT_SFIDS)
st2 = e2e.identity_state(ID2, SFID, PROJECT_SFIDS)
_, id1_scopes = e2e.scopes(ID1)
_, id2_scopes = e2e.scopes(ID2)
e2e.info("SCOPES-ID1", f"{ID1} cla-manager scopes ({st1['source']}): {len(st1['pairs'])}; user-service feed: {len(id1_scopes)}", scopes=st1["pairs"][:40])
e2e.info("SCOPES-ID2", f"{ID2} cla-manager scopes ({st2['source']}): {len(st2['pairs'])}; user-service feed: {len(id2_scopes)}; contact orgs={st2['contact_orgs']}; roles={st2['roles']}", scopes=st2["pairs"][:40])
e2e.check("SCOPES-ID1-FIXTURE", st1["project_level"], f"{ID1} holds a cla-manager scope on the fixture organization {SFID} for {PROJECT_SFIDS} ({st1['source']})")
id2_org, id2_proj = st2["org_level"], st2["project_level"]
id2_other = OTHER_ORG in st2["manager_orgs"] or OTHER_ORG in st2["contact_orgs"]
id1_other = OTHER_ORG in st1["manager_orgs"] or OTHER_ORG in st1["contact_orgs"]
e2e.info("SCOPES-ID2-STATE", f"{ID2} on the fixture organization: org-level reads expected {'200' if id2_org else '403'} "
         f"(manager orgs={st2['manager_orgs']}, contact orgs={st2['contact_orgs']}), project-level reads expected {'200' if id2_proj else '403'} "
         f"(pairs={st2['pairs']}) [{st2['source']}]")
if st2["source"] != "acs":
    e2e.info("SCOPES-SOURCE-LAG", "ACS helper unavailable (AWS credentials?) - identity state derived from the user-service feed, which may lag ACS by ~30 min")

for who in (ID1, ID2):
    e2e.expect(f"I-{who}-MY-CLAS", who, 200, "GET", "/v4/my-clas?pageSize=100")
    e2e.expect(f"I-{who}-IDENTITIES", who, 200, "GET", "/v4/my-clas/identities")
e2e.expect("I-ANON-MY-CLAS", "anon", 401, "GET", "/v4/my-clas")
e2e.expect("I-ANON-COMPANY-EXTERNAL", "anon", 401, "GET", f"/v4/company/external/{SFID}")
e2e.expect("I-ANON-CCLA", "anon", 401, "GET", f"/v4/signatures/project/{PROJ}/company/{CO}")

# ---- ID1: own company reads 200 ------------------------------------------------
own = [
    ("R-ID1-COMPANY-EXT", f"/v4/company/external/{SFID}"),
    ("R-ID1-COMPANY-INT", f"/v4/company/{CO}"),
    ("R-ID1-COMPANY-EXT-GROUPS", f"/v4/company/external/{SFID}/cla-groups"),
    ("R-ID1-CLA-MANAGERS-PROJ", f"/v4/company/{CO}/project/{PROJ}/cla-managers"),
    ("R-ID1-CLA-MANAGERS-GROUP", f"/v4/company/{CO}/cla-group/{GRP}/cla-managers"),
    ("R-ID1-CCLA", f"/v4/signatures/project/{PROJ}/company/{CO}"),
    ("R-ID1-ECLA-LIST", f"/v4/signatures/project/{PROJ}/company/{CO}/employee?pageSize=5"),
    ("R-ID1-ECLA-CSV", f"/v4/signatures/project/{GRP}/company/{CO}/employee/csv"),
    ("R-ID1-SIGNED-DOCUMENT", f"/v4/signatures/{CCLA}/signed-document"),
    ("R-ID1-ACTIVE-CLA-LIST", f"/v4/company/{CO}/project/{PROJ}/active-cla-list"),
    ("R-ID1-COMPANY-PROJECT-CLA", f"/v4/company/{SFID}/project/{PROJ}/cla"),
    ("R-ID1-CONTRIBUTORS", f"/v4/company/{CO}/project/{PROJ}/contributors?pageSize=5"),
    ("R-ID1-CORPORATE-CONTRIBUTORS", CONTRIB),
    ("R-ID1-EVENTS-COMPANY-PROJECT", f"/v4/company/{CO}/project/{PROJ}/events?pageSize=5"),
    ("R-ID1-METRICS-COMPANY", f"/v4/metrics/company/{CO}"),
    ("R-ID1-METRICS-COMPANY-PROJECT", f"/v4/metrics/company/{CO}/project/{PROJ}"),
    ("R-ID1-IS-DESIGNEE", f"/v4/company/{SFID}/user/{ID1}/claGroupID/{GRP}/is-cla-manager-designee"),
    ("R-ID1-CLA-GROUP-SEARCH", f"/v4/cla-group/search?searchTerm={e2e.urllib.parse.quote(c('CLA_GROUP_NAME')[:12])}&limit=3"),
]
for cid, path in own:
    e2e.expect(cid, ID1, 200, "GET", path)
e2e.expect("R-ID1-MANAGER-REQUESTS", ID1, (200, 403), "GET", f"/v4/company/{CO}/project/{PROJ}/cla-manager/requests")
e2e.expect("R-ID1-EVENTS-PROJECT", ID1, (200, 403), "GET", f"/v4/events/project/{PROJ}?pageSize=3")
if c("FOUNDATION_SFID"):
    e2e.expect("R-ID1-EVENTS-FOUNDATION", ID1, (200, 403), "GET", f"/v4/events/foundation/{c('FOUNDATION_SFID')}?pageSize=3")
if CO2:
    e2e.expect("R-ID1-COMPANY2-EXT", ID1, 200, "GET", f"/v4/company/external/{SFID2}")
    e2e.expect("R-ID1-COMPANY2-CCLA", ID1, 200, "GET", f"/v4/signatures/project/{PROJ}/company/{CO2}")
    e2e.expect("R-ID1-COMPANY2-SIGNED-DOCUMENT", ID1, 200, "GET", f"/v4/signatures/{CCLA2}/signed-document")
    e2e.expect("R-ID1-COMPANY2-CSV", ID1, 200, "GET", f"/v4/signatures/project/{GRP}/company/{CO2}/employee/csv")

# ICLA pdf link: discover ID1's ICLA from my-clas (config ID1_ICLA wins when set)
code, body = e2e.call(ID1, "GET", e2e.v4("/v4/my-clas?pageSize=100"))
clas = (body or {}).get("clas") or [] if isinstance(body, dict) else []
icla = c("ID1_ICLA") or next((r.get("signatureID") for r in clas if r.get("claType") == "icla"), "")
if icla:
    e2e.expect("R-ID1-ICLA-PDF", ID1, 200, "GET", f"/v4/my-clas/{icla}/pdf")
else:
    e2e.skip("R-ID1-ICLA-PDF", f"{ID1} has no ICLA in my-clas on {e2e.ENV}")
e2e.info("R-ID1-MY-CLAS-SHAPE", f"{len(clas)} rows: " + ", ".join(f"{r.get('claType')}/{r.get('status')}" for r in clas[:12]))

# ---- ID1: refused reads 403 -----------------------------------------------------
e2e.expect("R-ID1-GROUP-WIDE-LIST-403", ID1, 403, "GET", f"/v4/signatures/project/{GRP}?pageSize=1")
e2e.expect("R-ID1-CCLA-BY-ID-PROJECT-SCOPE", ID1, (403, 200), "GET", f"/v4/signatures/id/{CCLA}", note="(project-scope route, 403 for company managers)")
if OTHER_CO:
    e2e.expect("R-ID1-OTHER-COMPANY-CCLA-403", ID1, 403, "GET", f"/v4/signatures/project/{PROJ}/company/{OTHER_CO}")
    e2e.expect("R-ID1-OTHER-COMPANY-ECLA-403", ID1, 403, "GET", f"/v4/signatures/project/{PROJ}/company/{OTHER_CO}/employee?pageSize=1")
if OTHER_PROJ:
    e2e.expect("R-ID1-OWN-COMPANY-OTHER-GROUP", ID1, (200, 403), "GET", f"/v4/signatures/project/{OTHER_PROJ}/company/{CO}")
if SANCT_SFID:
    e2e.expect("R-ID1-SANCTIONED-COMPANY-EXT-403", ID1, 403, "GET", f"/v4/company/external/{SANCT_SFID}")
    e2e.expect("R-ID1-SANCTIONED-CCLA-403", ID1, 403, "GET", f"/v4/signatures/project/{PROJ}/company/{SANCT_ID}")
else:
    e2e.skip("R-ID1-SANCTIONED-403", f"SANCTIONED_* not configured for {e2e.ENV}")
if OTHER_ORG:
    if id1_other:
        e2e.skip("R-ID1-OTHER-ORG-403", f"{ID1} holds a cla-manager scope on {OTHER_ORG} on {e2e.ENV} - cannot assert 403")
    else:
        e2e.expect("R-ID1-OTHER-ORG-403", ID1, 403, "GET", f"/v4/company/external/{OTHER_ORG}")
        e2e.expect("R-ID1-OTHER-ORG-GROUPS-403", ID1, 403, "GET", f"/v4/company/external/{OTHER_ORG}/cla-groups")

# ---- ID2: derived from its (authoritative) scopes, per route class ---------------------------------------------------
# org-level class: admitted by the organization half of any cla-manager scope or by the org `contact` role (gateway rule +
# backend IsUserAuthorizedForOrganization / isUserHaveAccessToCLAProjectOrganization's organization-only fallback)
want_org = 200 if id2_org else 403
for cid, path in [
    ("COMPANY-EXT", f"/v4/company/external/{SFID}"),
    ("COMPANY-EXT-GROUPS", f"/v4/company/external/{SFID}/cla-groups"),
    ("CORPORATE-CONTRIBUTORS", f"/v4/company/external/{SFID}/cla-group/{GRP}/corporate-contributors?pageSize=1"),
    ("CCLA", f"/v4/signatures/project/{PROJ}/company/{CO}"),
    ("EVENTS-COMPANY-PROJECT", f"/v4/company/{CO}/project/{PROJ}/events?pageSize=1"),
    ("CLA-MANAGERS", f"/v4/company/{CO}/project/{PROJ}/cla-managers"),
]:
    e2e.expect(f"R-ID2-{cid}-{want_org}", ID2, want_org, "GET", path)
# project-level class: the gateway requires a cla-manager scope pairing the project (or its foundation) with the organization
want_proj = 200 if id2_proj else 403
for cid, path in [
    ("ECLA-LIST", f"/v4/signatures/project/{PROJ}/company/{CO}/employee?pageSize=1"),
    ("ECLA-CSV", f"/v4/signatures/project/{GRP}/company/{CO}/employee/csv"),
    ("SIGNED-DOCUMENT", f"/v4/signatures/{CCLA}/signed-document"),
]:
    e2e.expect(f"R-ID2-{cid}-{want_proj}", ID2, want_proj, "GET", path)
# known (pre-existing): the company metrics route performs no organization check - 200 for any signed-in user today
e2e.expect("R-ID2-METRICS-COMPANY-KNOWN", ID2, None, "GET", f"/v4/metrics/company/{CO}", note="(known: no org check on metrics/company, 200 for any user today)")
id2_fixture = id2_org or id2_proj
e2e.expect("R-ID2-GROUP-WIDE-LIST-403", ID2, 403, "GET", f"/v4/signatures/project/{GRP}?pageSize=1")
e2e.expect("R-ID2-PROJECT-EVENTS-403", ID2, 403, "GET", f"/v4/events/project/{PROJ}?pageSize=1")
e2e.expect("R-ID2-COMPANY-BY-ID", ID2, None, "GET", f"/v4/company/{CO}")
if CO2 and SFID2 not in st2["manager_orgs"] and SFID2 not in st2["contact_orgs"]:
    e2e.expect("R-ID2-COMPANY2-EXT-403", ID2, 403, "GET", f"/v4/company/external/{SFID2}")
    e2e.expect("R-ID2-COMPANY2-SIGNED-DOCUMENT-403", ID2, 403, "GET", f"/v4/signatures/{CCLA2}/signed-document")
if OTHER_ORG and not id2_other:
    e2e.expect("R-ID2-OTHER-ORG-403", ID2, 403, "GET", f"/v4/company/external/{OTHER_ORG}")
# ---- fixture CCLA shape (read as ID1 - also the baseline of the negative writes below) ------------------------------
def fixture_ccla():
    code, body = e2e.call(ID1, "GET", e2e.v4(f"/v4/signatures/project/{PROJ}/company/{CO}"))
    sigs = (body or {}).get("signatures") or [] if isinstance(body, dict) else []
    return code, next((s for s in sigs if s.get("signatureID") == CCLA), sigs[0] if sigs else {})


def acl_of(row):
    return sorted(a.get("lfUsername") or a.get("username") or "" for a in row.get("signatureACL") or [])


def emails_of(row):
    # approval-list items are objects {approval_item: ..., date_added: ...} (swagger/common/corporate-signature.yaml)
    return [x.get("approval_item") if isinstance(x, dict) else x for x in row.get("emailApprovalList") or []]


code, row = fixture_ccla()
baseline_ok = code == 200 and row.get("signatureID") == CCLA
# negative writes with the no-role identity: nothing may change on the fixture. The approval-list and Auto-ECLA routes
# authorize against the CCLA's signatureACL (v2/signatures/handlers.go), so ID2 on that ACL would make them legitimate.
# Without a confirmed baseline the undo below could not tell an acceptance from a failed read, so the probes stay off.
if not baseline_ok:
    e2e.skip("W-ID2-NEGATIVE-WRITES", f"baseline read of the fixture CCLA failed (http {code}, signatureID={row.get('signatureID')}) - negative writes not sent")
elif id2_fixture or ID2 in acl_of(row):
    e2e.skip("W-ID2-NEGATIVE-WRITES", f"{ID2} currently holds the fixture scope or is on the CCLA's ACL - negative writes not applicable")
else:
    neg_email = f"e2e-neg-{ID2}-{e2e.TAG}@example.invalid"
    e2e.expect("W-ID2-APPROVAL-PUT-403", ID2, 403, "PUT", f"/v4/signatures/project/{PROJ}/company/{CO}/clagroup/{GRP}/approval-list",
               {"AddEmailApprovalList": [neg_email]})
    e2e.expect("W-ID2-CLA-MANAGER-POST-403", ID2, 403, "POST", f"/v4/company/{CO}/project/{PROJ}/cla-manager", {"userLFID": ID2})
    e2e.expect("W-ID2-AUTO-ECLA-PUT-403", ID2, 403, "PUT", f"/v4/signatures/company/{CO}/clagroup/{GRP}/ecla-auto-create", {"auto_create_ecla": True})
    # an unexpected acceptance is undone right away (as ID1) so the fixture keeps its baseline
    code_after, after = fixture_ccla()
    undo = []
    if code_after != 200 or after.get("signatureID") != CCLA:
        undo.append(("re-read", code_after))
    else:
        if neg_email in emails_of(after):
            undo.append(("approval-list", e2e.call(ID1, "PUT", e2e.v4(f"/v4/signatures/project/{PROJ}/company/{CO}/clagroup/{GRP}/approval-list"), {"RemoveEmailApprovalList": [neg_email]})[0]))
        if bool(after.get("autoCreateECLA")) != bool(row.get("autoCreateECLA")):
            undo.append(("auto-ecla", e2e.call(ID1, "PUT", e2e.v4(f"/v4/signatures/company/{CO}/clagroup/{GRP}/ecla-auto-create"), {"auto_create_ecla": bool(row.get("autoCreateECLA"))})[0]))
        if ID2 in acl_of(after):
            undo.append(("cla-manager", e2e.call(ID1, "DELETE", e2e.v4(f"/v4/company/{CO}/project/{PROJ}/cla-manager/{ID2}"))[0]))
    e2e.check("W-ID2-NEGATIVE-WRITES-NO-CHANGE", not undo, f"fixture CCLA unchanged after the refused writes={not undo}" + (f"; undone as {ID1}: {undo}" if undo else ""))

e2e.check("R-FIXTURE-CCLA-PRESENT", baseline_ok,
          f"fixture CCLA {CCLA[:8]} present: signed={row.get('signatureSigned')} approved={row.get('signatureApproved')} autoCreateECLA={row.get('autoCreateECLA')} "
          f"managers={[a.get('lfUsername') or a.get('username') for a in row.get('signatureACL') or []]} "
          f"lists={ {k: len(row.get(k) or []) for k in ('emailApprovalList', 'domainApprovalList', 'githubUsernameApprovalList', 'githubOrgApprovalList', 'gitlabUsernameApprovalList', 'gitlabOrgApprovalList')} }")
with open(os.path.join(e2e.EVIDENCE_DIR, "fixture-ccla.private.json"), "w") as fh:
    json.dump(row, fh, indent=1)
e2e.summary_line()
sys.exit(0 if e2e.COUNTS["FAIL"] == 0 else 1)
