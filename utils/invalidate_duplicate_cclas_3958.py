#!/usr/bin/env python3
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
"""lfx-self-serve#3958 - duplicate CCLAs of member companies, scenarios 2 and 3.

Input: the `3958 scenarios 2+3` tab of the Member duplicates sheet exported as CSV (one row per CCLA).
Dry run (default): every row is re-read from DynamoDB and checked for drift against the sheet; the
intended changes, a backup of every row that would be touched and a restore file are written to
--out-dir. Nothing is written without --apply, and --apply refuses rows without a reviewer yes
(--blanket-14-8 accepts the historical 14 + 8 "proposed tranche" rows; optional rows always need
their own yes). --restore replays the backup (conditional on the rows still carrying our note). backup.json keeps the
rows in DynamoDB's typed form, so numbers and sets restore as they were.
"""
import argparse
import csv
import datetime
import json
import os
import re
import sys
import uuid

import boto3
from boto3.dynamodb.conditions import Attr, Key
from boto3.dynamodb.types import TypeDeserializer, TypeSerializer

PROPOSED = "proposed tranche"
GITHUB_CRITERIA = "githubUsername"
EMAIL_CRITERIA = "email"


def now_iso():
    return datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%fZ")


def norm(value):
    return (value or "").strip()


def to_wire(item):
    """DynamoDB item as the resource API returns it (Decimal, sets, ...) -> the typed wire form backup.json keeps."""
    return TypeSerializer().serialize(item)["M"]


def from_wire(wire):
    return TypeDeserializer().deserialize({"M": wire})


def parse_additions(text):
    """'GitHub mgielda; e-mail a@b.com (note)' -> [(githubUsername, mgielda), (email, a@b.com)]"""
    out = []
    for part in re.split(r"[;\n]", text or ""):
        part = re.sub(r"\(.*?\)", "", part).strip().rstrip(".")
        if not part or re.match(r"(?i)^(not added|already covered)", part):
            continue
        m = re.match(r"(?i)^(github|e-?mail)\s+(\S+)$", part)
        if not m:
            raise ValueError("cannot parse approval-list addition %r" % part)
        kind = GITHUB_CRITERIA if m.group(1).lower() == "github" else EMAIL_CRITERIA
        out.append((kind, m.group(2).strip().rstrip(",")))
    return out


class Store:
    def __init__(self, stage):
        ddb = boto3.resource("dynamodb")
        self.stage = stage
        self.signatures = ddb.Table("cla-%s-signatures" % stage)
        self.approvals = ddb.Table("cla-%s-approvals" % stage)

    def signature(self, signature_id):
        return self.signatures.get_item(Key={"signature_id": signature_id}, ConsistentRead=True).get("Item")

    def active_eclas(self, company_id, cla_group_id):
        kwargs = {
            "IndexName": "signature-user-ccla-company-index",
            "KeyConditionExpression": Key("signature_user_ccla_company_id").eq(company_id) & Key("signature_project_id").eq(cla_group_id),
            "FilterExpression": Attr("signature_approved").eq(True) & Attr("signature_signed").eq(True),
            "ProjectionExpression": "signature_id, signature_reference_id",
        }
        rows = []
        while True:
            page = self.signatures.query(**kwargs)
            rows.extend(page.get("Items", []))
            if "LastEvaluatedKey" not in page:
                return rows
            kwargs["ExclusiveStartKey"] = page["LastEvaluatedKey"]


def check_row(store, row):
    """Returns (plan dict, drift list) for one sheet row."""
    scenario = norm(row["Scenario"])
    dup_sig_id = norm(row["Duplicate CCLA signature_id (to invalidate)"])
    dup_company = norm(row["Duplicate company_id"])
    group_id = norm(row["CLA group ID"])
    member_sig_ids = [s.strip() for s in norm(row["Member CCLA signature_id (same CLA group)"]).split(",") if s.strip()]
    drift = []
    dup = store.signature(dup_sig_id)
    if not dup:
        return None, ["duplicate CCLA %s not found" % dup_sig_id]
    if dup.get("signature_type") != "ccla" or dup.get("signature_reference_id") != dup_company or dup.get("signature_project_id") != group_id:
        drift.append("duplicate CCLA %s is not the ccla of company %s on group %s" % (dup_sig_id, dup_company, group_id))
    if not (dup.get("signature_approved") is True and dup.get("signature_signed") is True):
        drift.append("duplicate CCLA %s is no longer signed+approved" % dup_sig_id)
    eclas = store.active_eclas(dup_company, group_id)
    expected = int(norm(row["Active ECLAs under the duplicate (scenario 3: all by the manager)"]) or 0)
    if len(eclas) != expected:
        drift.append("duplicate %s now has %d active ECLAs on the group, sheet says %d" % (dup_sig_id, len(eclas), expected))
    plan = {
        "scenario": scenario, "company": norm(row["EasyCLA company (duplicate)"]), "duplicate_signature_id": dup_sig_id,
        "duplicate_company_id": dup_company, "cla_group_id": group_id, "member_account": norm(row["Member Salesforce Account ID"]),
        "member_signature_ids": member_sig_ids, "status": norm(row["Status in the proposal"]),
        "decision": norm(row["Decision (yes/no)"]).lower(), "reviewer": norm(row["Reviewer"]), "additions": [], "backup": {dup_sig_id: dup},
    }
    members = []
    for sig_id in member_sig_ids:
        member = store.signature(sig_id)
        if not member:
            drift.append("member CCLA %s not found" % sig_id)
            continue
        if not (member.get("signature_approved") is True and member.get("signature_signed") is True and member.get("signature_project_id") == group_id):
            drift.append("member CCLA %s is not a signed+approved ccla on group %s" % (sig_id, group_id))
        members.append(member)
        plan["backup"][sig_id] = member
    if scenario == "3":
        if not members:
            drift.append("scenario 3 row without a member CCLA")
        else:
            member = members[0]
            for kind, entry in parse_additions(row["Approval-list additions on the member CCLA (scenario 3)"]):
                column = "github_whitelist" if kind == GITHUB_CRITERIA else "email_whitelist"
                present = [v.lower() for v in member.get(column, []) or []]
                plan["additions"].append({"signature_id": member["signature_id"], "company_id": member.get("signature_reference_id"),
                                          "company_name": member.get("signature_reference_name", ""), "column": column,
                                          "criteria": kind, "entry": entry, "already_present": entry.lower() in present})
    return plan, drift


def approved(plan, blanket):
    if plan["decision"] == "yes":
        return True
    return blanket and plan["status"].startswith(PROPOSED)


def apply_plan(store, plan, approved_by, journal):
    stamp = now_iso()
    # approval-list additions first (scenario 3), one entry at a time, never duplicating an entry
    for add in plan["additions"]:
        if add["already_present"]:
            continue
        store.signatures.update_item(
            Key={"signature_id": add["signature_id"]},
            UpdateExpression="SET #col = list_append(if_not_exists(#col, :empty), :entry), date_modified = :now",
            ConditionExpression="attribute_not_exists(#col) OR NOT contains(#col, :one)",
            ExpressionAttributeNames={"#col": add["column"]},
            ExpressionAttributeValues={":empty": [], ":entry": [add["entry"]], ":one": add["entry"], ":now": stamp},
        )
        approval = {
            "approval_id": str(uuid.uuid4()), "signature_id": add["signature_id"], "project_id": plan["cla_group_id"],
            "company_id": add["company_id"], "approval_company_name": add["company_name"], "approval_criteria": add["criteria"],
            "approval_name": add["entry"], "active": True, "date_added": stamp, "date_created": stamp, "date_modified": stamp,
            "note": "lfx-self-serve#3958 scenario 3, approved by %s" % approved_by,
        }
        store.approvals.put_item(Item=approval, ConditionExpression="attribute_not_exists(approval_id)")
        journal.write(json.dumps({"op": "approval_added", "approval": approval, "at": stamp}) + "\n")
    note = "duplicate of member %s CCLA %s (lfx-self-serve#3958, approved by %s on %s)" % (
        plan["member_account"], ",".join(plan["member_signature_ids"]), approved_by, stamp[:10])
    existing_note = norm(plan["backup"][plan["duplicate_signature_id"]].get("note"))
    store.signatures.update_item(
        Key={"signature_id": plan["duplicate_signature_id"]},
        UpdateExpression="SET signature_approved = :f, signature_signed = :f, sigtype_signed_approved_id = :key, date_modified = :now, note = :note",
        ConditionExpression="signature_approved = :t AND signature_signed = :t",
        ExpressionAttributeValues={":f": False, ":t": True, ":key": "ccla#false#false#%s" % plan["duplicate_company_id"], ":now": stamp,
                                   ":note": (existing_note + " | " if existing_note else "") + note},
    )
    journal.write(json.dumps({"op": "ccla_invalidated", "signature_id": plan["duplicate_signature_id"], "at": stamp}) + "\n")


def verify(store, plan):
    problems = []
    dup = store.signature(plan["duplicate_signature_id"]) or {}
    if dup.get("signature_approved") is not False or dup.get("signature_signed") is not False or "lfx-self-serve#3958" not in norm(dup.get("note")):
        problems.append("duplicate CCLA %s not invalidated as intended" % plan["duplicate_signature_id"])
    for add in plan["additions"]:
        member = store.signature(add["signature_id"]) or {}
        if add["entry"].lower() not in [v.lower() for v in member.get(add["column"], []) or []]:
            problems.append("entry %s missing on %s" % (add["entry"], add["signature_id"]))
    return problems


def restore(store, backup_path, journal_path):
    with open(backup_path, encoding="utf-8") as fh:
        backup = json.load(fh)
    restored = 0
    for sig_id, wire in backup.items():
        item = from_wire(wire)
        current = store.signature(sig_id) or {}
        if "lfx-self-serve#3958" not in norm(current.get("note")) and current == item:
            continue
        store.signatures.put_item(Item=item)
        restored += 1
    removed = 0
    if journal_path and os.path.exists(journal_path):
        with open(journal_path, encoding="utf-8") as fh:
            for line in fh:
                rec = json.loads(line)
                if rec.get("op") == "approval_added":
                    store.approvals.delete_item(Key={"approval_id": rec["approval"]["approval_id"]})
                    removed += 1
    print("restored %d signature rows, removed %d approval rows" % (restored, removed))


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--stage", default=os.environ.get("STAGE", "prod"))
    ap.add_argument("--rows", help="sheet tab CSV export")
    ap.add_argument("--out-dir", default="./3958-out")
    ap.add_argument("--apply", action="store_true")
    ap.add_argument("--approved-by", default="")
    ap.add_argument("--blanket-14-8", action="store_true", help="a single green light covers the 'proposed tranche' rows")
    ap.add_argument("--only", default="", help="comma-separated duplicate CCLA signature ids to limit the run")
    ap.add_argument("--restore", help="backup.json written by a previous run; replays it (with the journal next to it)")
    args = ap.parse_args()

    store = Store(args.stage)
    if args.restore:
        restore(store, args.restore, os.path.join(os.path.dirname(args.restore), "execution.jsonl"))
        return 0
    if not args.rows:
        ap.error("--rows is required")
    os.makedirs(args.out_dir, exist_ok=True)
    only = {s.strip() for s in args.only.split(",") if s.strip()}
    plans, blocked = [], []
    with open(args.rows, encoding="utf-8") as fh:
        for row in csv.DictReader(fh):
            if norm(row.get("Scenario")) not in ("2", "3"):
                continue
            sig_id = norm(row.get("Duplicate CCLA signature_id (to invalidate)"))
            if only and sig_id not in only:
                continue
            plan, drift = check_row(store, row)
            if plan is None or drift:
                blocked.append({"signature_id": sig_id, "drift": drift})
                print("DRIFT %s: %s" % (sig_id, "; ".join(drift)))
                continue
            plan["approved"] = approved(plan, args.blanket_14_8)
            plans.append(plan)
            print("%s scenario %s %s -> invalidate %s%s%s" % (
                "OK " if plan["approved"] else "NO-YES", plan["scenario"], plan["company"], sig_id,
                ", add %d approval entries first" % len([a for a in plan["additions"] if not a["already_present"]]) if plan["additions"] else "",
                "" if plan["approved"] else " (no reviewer yes)"))
    backup = {}
    for plan in plans:
        backup.update(plan["backup"])
    with open(os.path.join(args.out_dir, "plan.json"), "w", encoding="utf-8") as fh:
        json.dump([{k: v for k, v in p.items() if k != "backup"} for p in plans], fh, indent=2, default=str)
    with open(os.path.join(args.out_dir, "backup.json"), "w", encoding="utf-8") as fh:
        json.dump({sig_id: to_wire(item) for sig_id, item in backup.items()}, fh, indent=2)
    with open(os.path.join(args.out_dir, "drift.json"), "w", encoding="utf-8") as fh:
        json.dump(blocked, fh, indent=2)
    print("rows ok=%d drift=%d approved=%d" % (len(plans), len(blocked), len([p for p in plans if p["approved"]])))
    if not args.apply:
        print("dry run - nothing written; plan.json, backup.json, drift.json in %s" % args.out_dir)
        return 1 if blocked else 0
    if not args.approved_by:
        print("--apply needs --approved-by", file=sys.stderr)
        return 2
    todo = [p for p in plans if p["approved"]]
    if not todo:
        print("nothing approved - nothing written", file=sys.stderr)
        return 2
    print("APPLY on %s: %d CCLAs, type the stage name to continue: " % (args.stage, len(todo)), end="", flush=True)
    if sys.stdin.readline().strip() != args.stage:
        print("aborted")
        return 2
    problems = []
    with open(os.path.join(args.out_dir, "execution.jsonl"), "a", encoding="utf-8") as journal:
        for plan in todo:
            apply_plan(store, plan, args.approved_by, journal)
            problems.extend(verify(store, plan))
    print("applied %d rows, verification problems: %d" % (len(todo), len(problems)))
    for p in problems:
        print("PROBLEM " + p)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
