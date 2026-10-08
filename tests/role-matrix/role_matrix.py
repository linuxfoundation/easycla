#!/usr/bin/env python3
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
"""EasyCLA v4 role matrix: persona x endpoint, expected vs actual HTTP status.

Runs against a deployed API gateway (dev by default) with real user tokens.
Default mode never mutates data: writes are probed only with bodies that are
rejected before any side effect (no-op approval-list update, unknown emails,
unknown LFIDs) or only for personas that must be denied. See README.md.
"""
import argparse
import base64
import json
import os
import re
import sys
import urllib.error
import urllib.request

GATEWAY_DENY = "does not have access to resource or path"
ROLES = ("manager", "orgadmin", "member", "outsider", "anon")
SENTINEL_EMAIL = "role-matrix-no-such-user@example.invalid"

# Expectation vocabulary: an int = exact status; "ok" = 2xx; "not403" = any status but
# 403 (the probe body is rejected later for a safe reason); "rec" = record only.
#   name, method, path template, body, {role: expectation}, needs (fixture keys)
ENDPOINTS = [
    ("org.cla-groups", "GET", "company/external/{sfid}/cla-groups", None,
     dict(manager="ok", orgadmin="ok", member=403, outsider=403, anon=401), ()),
    ("org.project-managers", "GET", "company/{co}/project/{proj}/cla-managers", None,
     dict(manager="ok", orgadmin="ok", member=403, outsider=403, anon=401), ()),
    ("org.group-managers", "GET", "company/{co}/cla-group/{grp}/cla-managers", None,  # public route by design
     dict(manager="ok", orgadmin="ok", member="ok", outsider="ok", anon="ok"), ()),
    ("org.events", "GET", "company/{co}/project/{proj}/events?pageSize=1", None,
     dict(manager="ok", orgadmin="ok", member=403, outsider=403, anon=401), ()),
    ("ccla.list", "GET", "signatures/project/{proj}/company/{co}", None,
     dict(manager="ok", orgadmin="ok", member=403, outsider=403, anon=401), ()),
    ("ecla.list", "GET", "signatures/project/{proj}/company/{co}/employee?pageSize=1", None,
     dict(manager="ok", orgadmin="rec", member=403, outsider=403, anon=401), ()),
    ("ccla.signed-document", "GET", "signatures/{ccla_sig}/signed-document", None,
     dict(manager="ok", orgadmin="rec", member=403, outsider=403, anon=401), ("ccla_sig",)),
    ("corp.contributors", "GET", "cla-group/{grp}/corporate-contributors?companyID={co}&pageSize=1", None,
     dict(manager="rec", orgadmin="rec", member=403, outsider=403, anon=401), ()),  # project roles only: a pure manager gets 403
    ("org.contributors", "GET", "company/external/{sfid}/cla-group/{grp}/corporate-contributors?pageSize=1", None,
     dict(manager="ok", orgadmin="rec", member=403, outsider=403, anon=401), ()),  # path the Org Lens uses
    ("template.preview", "GET", "template/{grp}/preview?claType=ccla", None,  # public route by design
     dict(manager="ok", orgadmin="ok", member="ok", outsider="ok", anon="ok"), ()),
    ("cla-group.search", "GET", "cla-group/search?searchTerm=sun", None,
     dict(manager="ok", orgadmin="ok", member="ok", outsider="ok", anon=401), ()),
    ("mgr.requests.list", "GET", "company/{co}/project/{proj}/cla-manager/requests", None,
     dict(manager="ok", orgadmin=403, member=403, outsider=403, anon=401), ()),
    # Writes. Bodies are no-ops or fail safely when the caller is authorized.
    ("approval-list.put", "PUT", "signatures/project/{proj}/company/{co}/clagroup/{grp}/approval-list", {},
     dict(manager="not403", orgadmin=403, member=403, outsider=403, anon=401), ()),
    ("ecla-auto-create.put", "PUT", "signatures/company/{co}/clagroup/{grp}/ecla-auto-create", "AUTO_ECLA_CURRENT",
     dict(manager="ok", orgadmin=403, member=403, outsider=403, anon=401), ("auto_ecla", "dev")),  # still a write
    ("cla-manager.post", "POST", "company/{co}/project/{proj}/cla-manager",
     {"firstName": "Role", "lastName": "Matrix", "userEmail": SENTINEL_EMAIL},
     dict(manager="not403", orgadmin=403, member=403, outsider=403, anon=401), ()),
    ("cla-manager.delete", "DELETE", "company/{co}/project/{proj}/cla-manager/{non_manager_lfid}", None,
     dict(manager="not403", orgadmin=403, member=403, outsider=403, anon=401), ()),
    # Open to any logged-in user by design: CCLA signing starts here, before the company has a manager.
    ("designee.post", "POST", "company/{co}/project/{proj}/cla-manager-designee", {"userEmail": SENTINEL_EMAIL},
     dict(manager="not403", orgadmin="not403", member="not403", outsider="not403", anon=401), ()),
    ("mgr.requests.post", "POST", "company/{co}/project/{proj}/cla-manager/requests",
     {"fullName": "Role Matrix", "userEmail": SENTINEL_EMAIL},
     dict(member=403, outsider=403, anon=401), ()),  # allowed callers would create a request: deny side only
    ("ecla.invalidate", "PUT", "cla-group/{grp}/ecla/{ecla_sig}/invalidate", None,
     dict(orgadmin=403, member=403, outsider=403, anon=401), ("ecla_sig",)),  # deny side only
    ("self-serve.request-ccla", "POST", "self-serve/request-corporate-signature",
     {"company_sfid": "{sfid}", "project_sfid": "{proj}"},
     dict(orgadmin=403, member=403, outsider=403, anon=401), ()),  # deny side only
]

# --mutate: manager allow-side probes with cleanup, in order.
MUTATIONS = [
    ("approval-list.add", "PUT", "signatures/project/{proj}/company/{co}/clagroup/{grp}/approval-list",
     {"AddEmailApprovalList": [SENTINEL_EMAIL]}),
    ("approval-list.remove", "PUT", "signatures/project/{proj}/company/{co}/clagroup/{grp}/approval-list",
     {"RemoveEmailApprovalList": [SENTINEL_EMAIL]}),
    ("ecla-auto-create.toggle", "PUT", "signatures/company/{co}/clagroup/{grp}/ecla-auto-create", "AUTO_ECLA_FLIPPED"),
    ("ecla-auto-create.restore", "PUT", "signatures/company/{co}/clagroup/{grp}/ecla-auto-create", "AUTO_ECLA_CURRENT"),
]


def load_env(path):
    if not os.path.exists(path):
        return
    with open(path) as f:
        for line in f:
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                k, v = line.split("=", 1)
                os.environ.setdefault(k.strip(), v.strip())


def jwt_claims(token):
    payload = token.split(".")[1]
    payload += "=" * (-len(payload) % 4)
    return json.loads(base64.urlsafe_b64decode(payload))


def http(method, url, token=None, body=None, timeout=60):
    headers = {"Accept": "application/json", "Cache-Control": "no-cache"}  # skip the gateway's cached X-ACL
    data = None
    if token:
        headers["Authorization"] = "Bearer " + token
    if body is not None:
        headers["Content-Type"] = "application/json"
        data = json.dumps(body).encode()
    req = urllib.request.Request(url, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status, parse_body(resp.read())
    except urllib.error.HTTPError as e:
        return e.code, parse_body(e.read())


def parse_body(raw):
    try:
        return json.loads(raw)
    except Exception:
        return {"_raw": raw[:200].decode(errors="replace"), "_bytes": len(raw)}


def password_grant(username, password):
    body = {
        "grant_type": "http://auth0.com/oauth/grant-type/password-realm",
        "realm": "Username-Password-Authentication",
        "username": username,
        "password": password,
        "client_id": os.environ["AUTH0_CLIENT_ID"],
        "audience": os.environ.get("AUTH0_AUDIENCE") or os.environ["APP_URL"],
        "scope": "access:api openid profile email",
    }
    status, resp = http("POST", os.environ["AUTH0_TOKEN_API"], body=body)
    if status != 200:
        raise SystemExit("token grant for %s failed: %s %s (MFA-enrolled accounts must use RM_<PERSONA>_TOKEN)"
                         % (username, status, resp.get("error_description", resp) if isinstance(resp, dict) else resp))
    return resp["access_token"]


def load_personas():
    """RM_<NAME>_TOKEN or RM_<NAME>_USERNAME/_PASSWORD; NAME maps to a role by stripping digits."""
    personas = []
    names = set()
    for key in os.environ:
        m = re.match(r"^RM_([A-Z0-9_]+?)_(TOKEN|USERNAME)$", key)
        if m:
            names.add(m.group(1))
    for name in sorted(names):
        role = re.sub(r"[0-9_]+$", "", name).lower()
        if role not in ROLES:
            raise SystemExit("persona %s: role %r must be one of %s" % (name, role, ROLES))
        token = os.environ.get("RM_%s_TOKEN" % name)
        if not token:
            token = password_grant(os.environ["RM_%s_USERNAME" % name], os.environ["RM_%s_PASSWORD" % name])
        claims = jwt_claims(token)
        personas.append({"name": name.lower(), "role": role, "token": token,
                         "username": claims.get("http://lfx.dev/claims/username", "?")})
    personas.append({"name": "anon", "role": "anon", "token": None, "username": "-"})
    return personas


def discover(base, personas, fx):
    """Fill in the CCLA signature id, its ACL, the auto-ECLA flag and (if readable) an ECLA id."""
    found = {}
    for p in personas:
        if not p["token"]:
            continue
        status, body = http("GET", base + "signatures/project/{proj}/company/{co}".format(**fx), p["token"])
        if status == 200 and body.get("signatures"):
            sig = body["signatures"][0]
            found["ccla_sig"] = sig.get("signatureID")
            found["auto_ecla"] = bool(sig.get("autoCreateECLA"))
            found["ccla_acl"] = sorted(a.get("lfUsername") or a.get("username") or "?" for a in sig.get("signatureACL") or [])
            break
    for p in personas:
        if not p["token"] or "ecla_sig" in found:
            continue
        status, body = http("GET", base + "signatures/project/{proj}/company/{co}/employee?pageSize=1".format(**fx), p["token"])
        if status == 200 and body.get("signatures"):
            found["ecla_sig"] = body["signatures"][0].get("signatureID")
    return found


def classify(status, body):
    if status == 403 and isinstance(body, dict) and GATEWAY_DENY in str(body.get("Message", body.get("message", ""))):
        return "403(gw)"
    if status == 403 and isinstance(body, dict) and body.get("code") == "company_sanctioned":
        return "403(sanctioned)"
    if status == 403:
        return "403(svc)"
    return str(status)


def matches(expect, status):
    if expect == "rec":
        return True
    if expect == "ok":
        return 200 <= status < 300
    if expect == "not403":
        return status != 403
    return status == expect


def render_body(body, fx):
    if body == "AUTO_ECLA_CURRENT":
        return {"auto_create_ecla": fx["auto_ecla"]}
    if body == "AUTO_ECLA_FLIPPED":
        return {"auto_create_ecla": not fx["auto_ecla"]}
    if isinstance(body, dict):
        return {k: (v.format(**fx) if isinstance(v, str) else v) for k, v in body.items()}
    return body


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--env", default=os.path.join(os.path.dirname(os.path.abspath(__file__)), ".env"))
    ap.add_argument("--mutate", action="store_true",
                    help="also run manager allow-side writes with cleanup (dev only; never with prod fixtures)")
    ap.add_argument("--json", help="write full results to this file")
    args = ap.parse_args()
    load_env(args.env)
    os.environ.setdefault("APP_URL", "https://api-gw.dev.platform.linuxfoundation.org/")
    os.environ.setdefault("AUTH0_TOKEN_API", "https://linuxfoundation-dev.auth0.com/oauth/token")
    base = os.environ["APP_URL"].rstrip("/") + "/cla-service/v4/"
    is_dev = ".dev." in base
    if args.mutate and not is_dev:
        raise SystemExit("--mutate is dev only")
    fx = {
        "co": os.environ.get("RM_COMPANY_ID", "f7c7ac9c-4dbf-4104-ab3f-6b38a26d82dc"),           # Infosys Limited (dev)
        "sfid": os.environ.get("RM_COMPANY_SFID", "0014100000Te0G7AAJ"),
        "proj": os.environ.get("RM_PROJECT_SFID", "a09P000000DsCE5IAN"),                       # SUN (dev)
        "grp": os.environ.get("RM_CLA_GROUP_ID", "01af041c-fa69-4052-a23c-fb8c1d3bef24"),
        "non_manager_lfid": os.environ.get("RM_NON_MANAGER_LFID", "role-matrix-no-such-lfid"),
        "dev": True if is_dev else None,
    }
    personas = load_personas()
    fx.update(discover(base, personas, fx))

    print("target   %s" % base)
    print("fixtures company=%s project=%s cla_group=%s" % (fx["co"], fx["proj"], fx["grp"]))
    print("ccla     signature=%s auto_ecla=%s acl=%s" % (fx.get("ccla_sig"), fx.get("auto_ecla"), fx.get("ccla_acl")))
    print("ecla     signature=%s" % fx.get("ecla_sig", "(not discoverable with these personas)"))
    for p in personas:
        in_acl = p["username"] in (fx.get("ccla_acl") or [])
        print("persona  %-10s role=%-8s user=%-20s in_ccla_acl=%s" % (p["name"], p["role"], p["username"], in_acl))
        if p["role"] == "manager" and not in_acl:
            print("         WARNING: manager persona is not in the CCLA ACL; expectations will not hold")
        if p["role"] in ("orgadmin", "member", "outsider") and in_acl:
            print("         WARNING: %s persona is a CLA manager on this CCLA; expectations will not hold" % p["role"])
    print()

    results, failures, skipped = [], 0, []
    width = max(len(e[0]) for e in ENDPOINTS)
    print("%-*s %-7s %s" % (width, "endpoint", "method", "  ".join("%-24s" % p["name"] for p in personas)))
    for name, method, path, body, expect, needs in ENDPOINTS:
        missing = [n for n in needs if fx.get(n) is None]
        if missing:
            skipped.append((name, "needs %s" % ",".join(missing)))
            continue
        cells = []
        for p in personas:
            exp = expect.get(p["role"])
            if exp is None:
                cells.append("%-24s" % "-")
                continue
            status, resp = http(method, base + path.format(**fx), p["token"], render_body(body, fx))
            label = classify(status, resp)
            ok = matches(exp, status)
            failures += 0 if ok else 1
            cells.append("%-24s" % ("%s %s%s" % (label, "" if ok else "!=", "" if ok else exp)))
            results.append({"endpoint": name, "method": method, "path": path.format(**fx), "persona": p["name"],
                            "role": p["role"], "expected": exp, "status": status, "label": label, "ok": ok,
                            "message": (resp.get("Message") or resp.get("message") or resp.get("_raw")) if isinstance(resp, dict) else None})
        print("%-*s %-7s %s" % (width, name, method, "  ".join(cells)))

    if args.mutate:
        managers = [p for p in personas if p["role"] == "manager"]
        if not managers:
            print("\n--mutate: no manager persona; skipping allow-side writes")
        for p in managers[:1]:
            print("\nmutations as %s (%s):" % (p["name"], p["username"]))
            for name, method, path, body in MUTATIONS:
                status, resp = http(method, base + path.format(**fx), p["token"], render_body(body, fx))
                ok = 200 <= status < 300
                failures += 0 if ok else 1
                print("  %-28s %s%s" % (name, status, "" if ok else "  !=2xx " + json.dumps(resp)[:160]))
                results.append({"endpoint": name, "method": method, "persona": p["name"], "role": p["role"],
                                "expected": "ok", "status": status, "label": str(status), "ok": ok,
                                "message": json.dumps(resp)[:160] if not ok else None})

    for name, why in skipped:
        print("skipped  %-26s %s" % (name, why))
    for r in results:
        if not r["ok"]:
            print("MISMATCH %s %s as %s: got %s, expected %s: %s"
                  % (r["method"], r["endpoint"], r["persona"], r["label"], r["expected"], (r.get("message") or "")[:160]))
    print("\n%d checks, %d mismatches" % (len(results), failures))
    if args.json:
        with open(args.json, "w") as f:
            json.dump({"target": base, "fixtures": fx, "results": results}, f, indent=2)
    sys.exit(1 if failures else 0)


if __name__ == "__main__":
    main()
