#!/usr/bin/env python3
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
"""Identity boundary (M2 "my-clas" filters, ops#380-style): one identity can never read another one's data.

my-clas identity filters pointing at the OTHER identity return only the caller's own rows (plus
skippedIdentities), foreign acknowledgement sub-resources are 404, prepare-sign with foreign identities 403.
Pure reads except prepare-sign (which only creates the caller's own user record when it is allowed) and the two
negative mutating probes (manager request / invalidate naming a foreign ECLA): those only ever name an ECLA of the
other identity on a fixture CLA group x fixture company and run under ALLOW_FIXTURE_WRITES (the manager request
also under ALLOW_EMAIL), so a regressed refusal could only ever touch suite-owned data.
"""
import json
import os
import sys
import urllib.parse

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "lib"))
import e2e  # noqa: E402

c = e2e.cfg
ID1, ID2 = c("ID1"), c("ID2")
GRP = c("CLA_GROUP")
# the suite's fixture CLA groups and companies (dev keeps the write/M3 fixture on a second group; prod uses one group for all)
FIXTURE_GROUPS = {g for g in (c("CLA_GROUP"), c("WRITE_CLA_GROUP"), c("M3_CLA_GROUP")) if g}
FIXTURE_COMPANIES = {x for x in (c("COMPANY_ID"), c("WRITE_COMPANY_ID"), c("COMPANY2_ID")) if x}
FIXTURE_WRITES = c("ALLOW_FIXTURE_WRITES", "0") == "1"
EMAIL = c("ALLOW_EMAIL", "0") == "1"


def own_rows(who):
    code, body = e2e.call(who, "GET", e2e.v4("/v4/my-clas?pageSize=100"))
    return code, (body or {}).get("clas") or [] if isinstance(body, dict) else []


code1, rows1 = own_rows(ID1)
code2, rows2 = own_rows(ID2)
ids1 = {r.get("signatureID") for r in rows1}
ids2 = {r.get("signatureID") for r in rows2}
e2e.check("IB-BASELINE", code1 == 200 and code2 == 200, f"my-clas: {ID1}={len(rows1)} rows, {ID2}={len(rows2)} rows")

# filters naming the other identity must not leak rows
pairs = [(ID1, ID2, c("ID2_EMAIL"), c("ID2_GITHUB"), ids1), (ID2, ID1, c("ID1_EMAIL"), c("ID1_GITHUB"), ids2)]
for who, other, other_email, other_gh, own in pairs:
    for label, q in (("GITHUB", f"githubUsername={urllib.parse.quote(other_gh)}"), ("EMAIL", f"email={urllib.parse.quote(other_email)}"), ("LFUSERNAME", f"lfUsername={urllib.parse.quote(other)}")):
        code, body = e2e.call(who, "GET", e2e.v4(f"/v4/my-clas?pageSize=100&{q}"))
        rows = (body or {}).get("clas") or [] if isinstance(body, dict) else []
        got = {r.get("signatureID") for r in rows}
        leaked = got - own
        skipped = (body or {}).get("skippedIdentities") if isinstance(body, dict) else None
        e2e.check(f"IB-{who}-FILTER-{label}-NO-LEAK", code == 200 and not leaked,
                  f"[{who}] my-clas?{label.lower()}=<{other}> -> {code}, rows={len(rows)}, foreign rows={len(leaked)}, skippedIdentities={json.dumps(skipped)[:120] if skipped is not None else 'absent'}")

# foreign acknowledgement sub-resources: 404 (not 403, to avoid existence oracles)
for who, other_rows in ((ID2, rows1), (ID1, rows2)):
    # the backend also answers 404 for a signature of the wrong TYPE (/pdf serves signed ICLAs only, the manager routes
    # ECLAs only), so every probe uses a foreign signature of the type its route serves - with an arbitrary row an
    # ownership regression would stay hidden behind the type refusal
    icla = next((r for r in other_rows if r.get("claType") == "icla" and r.get("signatureID")), None)
    ecla = next((r for r in other_rows if r.get("claType") == "ecla" and r.get("signatureID")), None)
    # the two MUTATING probes (manager request, invalidate) only ever name a foreign ECLA on a fixture group x fixture company:
    # should the refusal regress, the write lands on suite-owned data, never on a real company of the other identity
    fx_ecla = next((r for r in other_rows if r.get("claType") == "ecla" and r.get("signatureID")
                    and r.get("claGroupID") in FIXTURE_GROUPS and r.get("companyID") in FIXTURE_COMPANIES), None)
    if not icla and not ecla:
        e2e.skip(f"IB-{who}-FOREIGN-SUBRESOURCES", f"the other identity has no acknowledgements on {e2e.ENV}")
        continue
    if icla:
        e2e.expect(f"IB-{who}-FOREIGN-PDF-404", who, (404, 403), "GET", f"/v4/my-clas/{icla['signatureID']}/pdf")
    else:
        e2e.skip(f"IB-{who}-FOREIGN-PDF-404", "the other identity has no ICLA rows")
    if ecla:
        e2e.expect(f"IB-{who}-FOREIGN-MANAGERS-404", who, (404, 403), "GET", f"/v4/my-clas/{ecla['signatureID']}/cla-managers")
    else:
        e2e.skip(f"IB-{who}-FOREIGN-MANAGERS-404", "the other identity has no ECLA rows")
    if not fx_ecla:
        e2e.skip(f"IB-{who}-FOREIGN-MANAGER-REQUEST-404", "no ECLA of the other identity on the fixture group/company")
    elif not (FIXTURE_WRITES and EMAIL):
        e2e.skip(f"IB-{who}-FOREIGN-MANAGER-REQUEST-404", f"ALLOW_FIXTURE_WRITES={c('ALLOW_FIXTURE_WRITES', '0')} ALLOW_EMAIL={c('ALLOW_EMAIL', '0')}: "
                 "a regressed refusal would create a manager request and e-mail ID1")
    else:
        # ownership is checked before the recipients (findOwnedEcla first): a 400 here would mean the foreign ECLA passed it
        e2e.expect(f"IB-{who}-FOREIGN-MANAGER-REQUEST-404", who, (404, 403), "POST", f"/v4/my-clas/{fx_ecla['signatureID']}/cla-manager-requests",
                   {"requestType": "contact", "recipients": [ID1], "message": f"E2E {e2e.TAG} negative probe - must be refused"})
    # manager-side invalidate of a foreign ECLA: only meaningful for an identity without cla-manager scope on that company
    # (the PUT mutates the ECLA, so the gate uses the authoritative ACS rolescopes, never the lagging user-service feed)
    if not fx_ecla:
        e2e.skip(f"IB-{who}-FOREIGN-INVALIDATE-REFUSED", "no ECLA of the other identity on the fixture group/company")
        continue
    if not FIXTURE_WRITES:
        e2e.skip(f"IB-{who}-FOREIGN-INVALIDATE-REFUSED", f"ALLOW_FIXTURE_WRITES={c('ALLOW_FIXTURE_WRITES', '0')}: a regressed refusal would invalidate the other identity's fixture ECLA")
        continue
    who_roles = e2e.acs_roles(who)
    manager_scopes = who_roles.get("cla-manager", set()) if who_roles is not None else set()
    # my-clas rows carry no companySFID and leave projectSFID/foundationSFID empty for ambiguous multi-project mappings:
    # a manager whose scope cannot be matched against the target must not send the mutating probe on a guess
    mapping_known = any(fx_ecla.get(k) for k in ("companySFID", "projectSFID", "foundationSFID"))
    if who_roles is None:
        e2e.skip(f"IB-{who}-FOREIGN-INVALIDATE-REFUSED", "ACS rolescopes unavailable (AWS credentials / utils/<env>_acs_role_flip.sh) - the mutating probe is not sent on a guess")
    elif manager_scopes and (not mapping_known or any(e2e.has_scope(manager_scopes, fx_ecla.get(k) or "\x00") for k in ("companySFID", "projectSFID", "foundationSFID")) or who == ID1):
        e2e.skip(f"IB-{who}-FOREIGN-INVALIDATE-REFUSED", f"{who} holds cla-manager scopes in ACS - an invalidate of the other identity's ECLA may be legitimate"
                 + ("" if mapping_known else " (the target's company/project mapping is unknown)"))
    else:
        e2e.expect(f"IB-{who}-FOREIGN-INVALIDATE-REFUSED", who, (404, 403), "PUT", f"/v4/cla-group/{fx_ecla['claGroupID']}/ecla/{fx_ecla['signatureID']}/invalidate",
                   {"reason": "other", "note": f"E2E {e2e.TAG} negative probe - must be refused"})

# prepare-sign with foreign identities: the claimed github/e-mail is NOT attached (lands in skippedIdentities) or the call is refused
for label, payload, foreign in (
        ("GITHUB", {"githubUsername": c("ID1_GITHUB"), "email": c("ID2_EMAIL")}, f"github-username:{c('ID1_GITHUB')}"),
        ("EMAIL", {"githubUsername": c("ID2_GITHUB"), "email": c("ID1_EMAIL")}, f"email:{c('ID1_EMAIL')}")):
    code, body = e2e.call(ID2, "POST", e2e.v4("/v4/self-serve/prepare-sign"), {"claGroupId": GRP, "returnUrl": c("SS_URL") + "/profile/clas", **payload})
    if e2e.NO_WRITES and code == 0:
        e2e.skip(f"IB-ID2-PREPARE-SIGN-FOREIGN-{label}", "--no-writes")
        continue
    b = body if isinstance(body, dict) else {}
    identity = [str(x).lower() for x in b.get("identity") or []]
    skipped = [str(x).lower() for x in b.get("skippedIdentities") or []]
    attached = any(foreign.lower() == x or foreign.split(":", 1)[1].lower() == x.split(":", 1)[-1] for x in identity)
    ok = code in (400, 403, 422) or (code == 200 and not attached and (not skipped or any(foreign.split(":", 1)[1].lower() in x for x in skipped)))
    e2e.check(f"IB-ID2-PREPARE-SIGN-FOREIGN-{label}", ok,
              f"[{ID2}] prepare-sign claiming {foreign} -> {code}; identity={identity} skipped={skipped}", http=code)
# anonymous
e2e.expect("IB-ANON-IDENTITIES-401", "anon", 401, "GET", "/v4/my-clas/identities")
e2e.expect("IB-ANON-PREPARE-SIGN-401", "anon", 401, "POST", "/v4/self-serve/prepare-sign", {"claGroupId": GRP})
e2e.summary_line()
sys.exit(0 if e2e.COUNTS["FAIL"] == 0 else 1)
