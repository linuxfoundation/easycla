#!/usr/bin/env python3
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
"""Render SUMMARY.md (and print a short console summary) from a run's results.jsonl.

usage: summary.py <run-dir>
exit code: 1 when any check FAILed, else 0.
"""
import collections
import json
import os
import sys

run_dir = sys.argv[1]
path = os.path.join(run_dir, "results.jsonl")
rows = []
if os.path.exists(path):
    with open(path) as fh:
        for line in fh:
            line = line.strip()
            if line:
                try:
                    rows.append(json.loads(line))
                except json.JSONDecodeError:
                    rows.append({"id": "unparseable", "status": "FAIL", "module": "summary", "note": line[:200]})

by_module = collections.OrderedDict()
for row in rows:
    by_module.setdefault(row.get("module", "?"), []).append(row)
totals = collections.Counter(r.get("status", "?") for r in rows)
env = next((r.get("env") for r in rows if r.get("env")), "?")

out = [f"# EasyCLA E2E suite - {env} - {os.path.basename(run_dir.rstrip('/'))}", ""]
out.append("| module | PASS | FAIL | SKIP | INFO |")
out.append("|---|---|---|---|---|")
for module, items in by_module.items():
    c = collections.Counter(r.get("status") for r in items)
    out.append(f"| {module} | {c['PASS']} | {c['FAIL']} | {c['SKIP']} | {c['INFO']} |")
out.append(f"| **total** | **{totals['PASS']}** | **{totals['FAIL']}** | **{totals['SKIP']}** | **{totals['INFO']}** |")
out.append("")
fails = [r for r in rows if r.get("status") == "FAIL"]
if fails:
    out.append("## Failures")
    out.append("")
    for r in fails:
        out.append(f"- `{r.get('module')}` **{r.get('id')}** - {r.get('note', '')}")
    out.append("")
skips = [r for r in rows if r.get("status") == "SKIP"]
if skips:
    out.append("## Skipped")
    out.append("")
    for r in skips:
        out.append(f"- `{r.get('module')}` {r.get('id')} - {r.get('note', '')}")
    out.append("")
out.append("## All checks")
out.append("")
out.append("| module | id | status | note |")
out.append("|---|---|---|---|")
for r in rows:
    note = str(r.get("note", "")).replace("|", "\\|").replace("\n", " ")
    out.append(f"| {r.get('module')} | {r.get('id')} | {r.get('status')} | {note} |")
out.append("")
with open(os.path.join(run_dir, "SUMMARY.md"), "w") as fh:
    fh.write("\n".join(out))
print(f"{env}: PASS={totals['PASS']} FAIL={totals['FAIL']} SKIP={totals['SKIP']} INFO={totals['INFO']} -> {os.path.join(run_dir, 'SUMMARY.md')}")
for r in fails:
    print(f"  FAIL {r.get('module')}/{r.get('id')}: {r.get('note', '')}")
sys.exit(1 if fails else 0)
