#!/usr/bin/env node
import fs from "node:fs";

const [,, auditPath = "audit.json", policyPath = ".yarn-audit-allowlist.json"] = process.argv;

const sevRank = { info: 0, low: 1, moderate: 2, high: 3, critical: 4 };

const policy = JSON.parse(fs.readFileSync(policyPath, "utf8"));
const threshold = sevRank[policy.minSeverity ?? "high"] ?? sevRank.high;

const allow = new Set((policy.allowlist ?? []).map(String));

const lines = fs.readFileSync(auditPath, "utf8").split(/\r?\n/).filter(Boolean);

const hits = [];
for (const line of lines) {
  let msg;
  try { msg = JSON.parse(line); } catch { continue; }
  if (msg.type !== "auditAdvisory") continue;

  // Yarn v1 emits { type:"auditAdvisory", data:{ advisory:{...}, ... } }
  const advisory = msg?.data?.advisory ?? msg?.data;
  const id = advisory?.id;
  const severity = advisory?.severity;
  const moduleName = advisory?.module_name;
  const title = advisory?.title;

  if (id == null || severity == null) continue;

  if (allow.has(String(id))) continue;
  if ((sevRank[severity] ?? -1) < threshold) continue;

  hits.push({ id, severity, moduleName, title });
}

if (hits.length) {
  console.error(`yarn-audit policy failure: ${hits.length} advisories >= ${policy.minSeverity} (after allowlist)`);
  for (const h of hits) {
    console.error(`- [${h.severity}] ${h.moduleName} (advisory ${h.id}) ${h.title ?? ""}`.trim());
  }
  process.exit(1);
}

console.log(`OK: no advisories >= ${policy.minSeverity} (after allowlist)`);
