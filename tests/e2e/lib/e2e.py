#!/usr/bin/env python3
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
"""Shared helpers for the python modules of the E2E suite (stdlib only).

Reads secrets/<env>.env.secret (KEY=VALUE, never executed; template config/example.env) and secrets/*.secret, offers an HTTP helper that
records every call in the module's evidence dir and a `check()` that appends to the run's results.jsonl.
"""
import json
import os
import re
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
REPO_ROOT = os.path.dirname(os.path.dirname(ROOT))
ENV = os.environ.get("E2E_ENV", "")
if ENV not in ("dev", "prod"):
    sys.exit("error: E2E_ENV must be dev or prod")
SECRETS_DIR = os.environ.get("E2E_SECRETS_DIR", os.path.join(ROOT, "secrets"))
MODULE = os.environ.get("E2E_MODULE") or os.path.splitext(os.path.basename(sys.argv[0]))[0]
RUN_DIR = os.environ.get("E2E_RUN_DIR") or os.path.join(
    REPO_ROOT, "e2e-tests", "runs", "suite", f"{ENV}-{time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())}")
EVIDENCE_DIR = os.path.join(RUN_DIR, MODULE)
os.makedirs(EVIDENCE_DIR, exist_ok=True)
for d in (RUN_DIR, EVIDENCE_DIR):
    try:
        os.chmod(d, 0o700)
    except OSError:
        pass  # best effort: the evidence dir may be on a filesystem without chmod support
RESULTS = os.path.join(RUN_DIR, "results.jsonl")
LOG = os.path.join(RUN_DIR, "run.log")
NO_WRITES = os.environ.get("E2E_NO_WRITES", "0") == "1"
KEEP_STATE = os.environ.get("E2E_KEEP_STATE", "0") == "1"
TAG = os.environ.get("E2E_TAG") or time.strftime("%Y%m%d%H%M%S", time.gmtime())


def _load_config():
    cfg = {}
    path = os.path.join(ROOT, "secrets", f"{ENV}.env.secret")
    if not os.path.isfile(path):
        sys.exit(f"missing {path} - copy tests/e2e/config/example.env there and fill it in (see tests/e2e/README.md)")
    with open(path) as fh:
        for line in fh:
            m = re.match(r"^([A-Z][A-Z0-9_]*)=(.*)$", line.rstrip("\r\n"))
            if m:
                cfg[m.group(1)] = os.environ.get(m.group(1), m.group(2))
    return cfg


CFG = _load_config()


def cfg(key, default=""):
    return CFG.get(key) or default


def secret(name):
    path = os.path.join(SECRETS_DIR, f"{name}.secret")
    if not os.path.exists(path):
        return ""
    with open(path) as fh:
        return fh.read().strip()


def token(identity):
    return secret(f"auth0-{ENV}-{identity}.token")


def now():
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def redact(text, limit=300):
    text = re.sub(r"eyJ[A-Za-z0-9_.-]{20,}", "[TOKEN]", str(text))
    return text[:limit]


def log(msg):
    line = f"{now()} [{MODULE}] {msg}"
    print(line, flush=True)
    with open(LOG, "a") as fh:
        fh.write(line + "\n")


COUNTS = {"PASS": 0, "FAIL": 0, "SKIP": 0, "INFO": 0}


def check(cid, ok, note="", status=None, **info):
    """Record one check. `ok` may be True/False or ignored when `status` is given (SKIP/INFO)."""
    st = status or ("PASS" if ok else "FAIL")
    COUNTS[st] = COUNTS.get(st, 0) + 1
    row = {"id": cid, "status": st, "module": MODULE, "env": ENV, "at": now(), "note": redact(note, 400)}
    for k, v in info.items():
        try:
            json.dumps(v)
            row[k] = v
        except TypeError:
            row[k] = str(v)[:300]
    with open(RESULTS, "a") as fh:
        fh.write(json.dumps(row, default=str) + "\n")
    log(f"{st} {cid} - {note}")
    return st == "PASS"


def skip(cid, note):
    return check(cid, False, note, status="SKIP")


def info(cid, note, **extra):
    return check(cid, True, note, status="INFO", **extra)


def summary_line():
    log("module done: " + " ".join(f"{k}={v}" for k, v in COUNTS.items()) + f" evidence={EVIDENCE_DIR}")



def call(who, method, url, data=None, headers=None, timeout=90):
    """HTTP call as identity `who` ("anon" for none). Returns (status, parsed-json-or-text)."""
    if NO_WRITES and method not in ("GET", "HEAD", "OPTIONS"):
        return 0, {"skipped": "--no-writes"}
    body = json.dumps(data).encode() if data is not None else None
    req = urllib.request.Request(url, data=body, method=method)
    req.add_header("Accept", "application/json")
    req.add_header("Cache-Control", "no-cache")
    if who != "anon":
        tok = token(who)
        if not tok:
            raise SystemExit(f"no token for {who}: run lib/token.sh")
        req.add_header("Authorization", "Bearer " + tok)
    if body is not None:
        req.add_header("Content-Type", "application/json")
    for k, v in (headers or {}).items():
        req.add_header(k, v)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw, code = resp.read(), resp.status
    except urllib.error.HTTPError as err:
        raw, code = err.read(), err.code
    except (urllib.error.URLError, TimeoutError, OSError) as err:
        raw, code = str(err).encode(), 0
    try:
        parsed = json.loads(raw) if raw else None
    except ValueError:
        parsed = raw.decode(errors="replace")[:600]
    # one JSON line per call (append-only: linear and crash-safe, every receipt is on disk the moment the call returns)
    receipt = {"at": now(), "who": who, "method": method, "url": url, "http": code, "req": data,
               "resp": parsed if not isinstance(parsed, str) else parsed[:300]}
    receipts_path = os.path.join(EVIDENCE_DIR, "receipts.private.jsonl")
    with open(receipts_path, "a") as fh:
        fh.write(json.dumps(receipt, default=str) + "\n")
    try:
        os.chmod(receipts_path, 0o600)
    except OSError:
        pass  # best effort, same as above
    return code, parsed


def v4(path):
    return cfg("API_V4") + path


def expect(cid, who, codes, method, path, data=None, base=None, note=""):
    """Plain status-code expectation; `codes` is an int or a collection. `codes=None` records INFO."""
    url = (base or cfg("API_V4")) + path
    code, body = call(who, method, url, data)
    # only the writes call() suppressed under --no-writes become SKIP; a status 0 on a read is a real transport failure
    if NO_WRITES and code == 0 and method not in ("GET", "HEAD", "OPTIONS"):
        return skip(cid, f"--no-writes: {method} {path}"), None
    snippet = redact(json.dumps(body) if not isinstance(body, str) else body, 140) if code >= 400 or codes is None else ""
    if codes is None:
        info(cid, f"[{who}] {method} {path} -> {code} {snippet}", http=code)
        return True, body
    wanted = {codes} if isinstance(codes, int) else set(codes)
    ok = code in wanted
    check(cid, ok, f"[{who}] {method} {path} -> {code} (expected {sorted(wanted)}) {note} {snippet if not ok else ''}".strip(),
          http=code, expected=sorted(wanted), who=who)
    return ok, body


def scopes(who, role="cla-manager"):
    """user-service permission scope ids of `who` for `role` (what Self Serve reads), e.g.
    "a09P000000DsCE5IAN|0014100000Te0G7AAJ" (project|organization). Returns (http, sorted ids)."""
    code, body = call(who, "GET", f"{cfg('USER_SERVICE')}/me/permissions?role={urllib.parse.quote(role)}")
    ids = set()
    perms = (body or {}).get("Permissions") or (body or {}).get("permissions") or [] if isinstance(body, dict) else (body or [])
    for perm in perms:
        if not isinstance(perm, dict):
            continue
        for sc in perm.get("Scopes") or perm.get("scopes") or []:
            raw = sc.get("ID") if isinstance(sc, dict) else sc
            raw = raw.get("id") if isinstance(raw, dict) else raw
            for sid in (raw if isinstance(raw, list) else [raw]):
                if sid:
                    ids.add(str(sid))
    return code, sorted(ids)


def has_scope(ids, sfid):
    return any(sfid and sfid in s for s in ids)


_acs_unavailable = [False]


def acs_roles(who):
    """Authoritative ACS rolescopes of `who` through the repo helper `utils/<env>_acs_role_flip.sh roles <user>` (M2M token via
    SSM, needs AWS credentials for the env profile). Returns {role_name: set(scope ids)} - scope ids are "<project>|<org>" for
    project|organization scopes and "<org>" for organization ones - or None when the helper is unavailable."""
    if _acs_unavailable[0]:
        return None
    script = os.path.join(REPO_ROOT, "utils", f"{ENV}_acs_role_flip.sh")
    try:
        out = subprocess.run([script, "roles", who], capture_output=True, text=True, timeout=180).stdout
        data = json.loads(out[out.index("{"):]) if "{" in out else {}
    except (OSError, subprocess.SubprocessError, ValueError):
        data = {}
    if not isinstance(data, dict) or who not in data:
        _acs_unavailable[0] = True
        return None
    roles = {}
    for r in data.get(who) or []:
        roles.setdefault(r.get("role_name", ""), set()).update(str(sc.get("object_id") or "") for sc in r.get("scopes") or [])
    return roles


def identity_state(who, sfid, project_sfids=()):
    """What the gateway sees for `who` on organization `sfid`: org-level routes (company/external/{sfid}, cla-groups,
    corporate-contributors) are admitted by the org half of any cla-manager scope or by the org `contact` role; project-level
    routes (signatures/project/{proj}/company/..., employee list/csv, cla-managers, events) need a cla-manager scope whose
    project half is one of `project_sfids` (the project itself or, for CLA groups signed at foundation level, the foundation).
    Source is ACS when the helper works, else the user-service permissions feed (which lags ACS by up to ~30 min)."""
    roles = acs_roles(who)
    if roles is not None:
        pairs = {p for p in roles.get("cla-manager", set()) if "|" in p}
        contact_orgs = {o for o in roles.get("contact", set()) if "|" not in o}
        source = "acs"
    else:
        _, ids = scopes(who)
        pairs, contact_orgs, source = {p for p in ids if "|" in p}, set(), "user-service (lags ACS by up to ~30 min)"
    manager_orgs = {p.split("|")[-1] for p in pairs}
    org_level = sfid in manager_orgs or sfid in contact_orgs
    project_level = any(p.split("|")[-1] == sfid and p.split("|")[0] in set(project_sfids) for p in pairs)
    return {"source": source, "pairs": sorted(pairs), "manager_orgs": sorted(manager_orgs), "contact_orgs": sorted(contact_orgs),
            "roles": sorted(roles) if roles is not None else None, "org_level": org_level, "project_level": project_level}
