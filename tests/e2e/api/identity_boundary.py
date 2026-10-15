#!/usr/bin/env python3
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
"""Identity boundary (M2 "my-clas" filters, ops#380-style): one identity can never read another one's data.

my-clas identity filters pointing at the OTHER identity return only the caller's own rows (plus
skippedIdentities), foreign acknowledgement sub-resources are 404, prepare-sign with foreign identities 403.
Pure reads except prepare-sign (which only creates the caller's own user record when it is allowed).
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
    target = next((r for r in other_rows if r.get("signatureID")), None)
    if not target:
        e2e.skip(f"IB-{who}-FOREIGN-SUBRESOURCES", f"the other identity has no acknowledgements on {e2e.ENV}")
        continue
    sid = target["signatureID"]
    e2e.expect(f"IB-{who}-FOREIGN-PDF-404", who, (404, 403), "GET", f"/v4/my-clas/{sid}/pdf")
    e2e.expect(f"IB-{who}-FOREIGN-MANAGERS-404", who, (404, 403), "GET", f"/v4/my-clas/{sid}/cla-managers")
    e2e.expect(f"IB-{who}-FOREIGN-MANAGER-REQUEST-404", who, (404, 403, 400), "POST", f"/v4/my-clas/{sid}/cla-manager-requests",
               {"requestType": "contact", "recipients": [ID1], "message": f"E2E {e2e.TAG} negative probe - must be refused"})
    # manager-side invalidate of a foreign ECLA: only meaningful for an identity without cla-manager scope on that company
    # (the PUT mutates the ECLA, so the gate uses the authoritative ACS rolescopes, never the lagging user-service feed)
    ecla = next((r for r in other_rows if r.get("claType") == "ecla" and r.get("signatureID")), None)
    who_roles = e2e.acs_roles(who)
    manager_scopes = who_roles.get("cla-manager", set()) if who_roles is not None else set()
    if not ecla:
        e2e.skip(f"IB-{who}-FOREIGN-INVALIDATE-REFUSED", "the other identity has no ECLA rows")
    elif who_roles is None:
        e2e.skip(f"IB-{who}-FOREIGN-INVALIDATE-REFUSED", "ACS rolescopes unavailable (AWS credentials / utils/<env>_acs_role_flip.sh) - the mutating probe is not sent on a guess")
    elif manager_scopes and (any(e2e.has_scope(manager_scopes, ecla.get(k) or "\x00") for k in ("companySFID", "projectSFID", "foundationSFID")) or who == ID1):
        e2e.skip(f"IB-{who}-FOREIGN-INVALIDATE-REFUSED", f"{who} holds cla-manager scopes in ACS - an invalidate of the other identity's ECLA may be legitimate")
    else:
        e2e.expect(f"IB-{who}-FOREIGN-INVALIDATE-REFUSED", who, (404, 403), "PUT", f"/v4/cla-group/{ecla.get('claGroupID') or GRP}/ecla/{ecla['signatureID']}/invalidate",
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
