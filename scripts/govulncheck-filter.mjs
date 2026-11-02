#!/usr/bin/env node
// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT
//
// Policy filter for `govulncheck -json` output - the Go counterpart of yarn-audit-filter.mjs.
//
// Only findings that govulncheck reports as "called" (the first trace frame names a function) can fail the
// check; import- and module-level findings are informational, exactly as in govulncheck's text mode. A called
// finding is skipped when .govulncheck-allowlist.json lists it, either as "GO-YYYY-NNNN" (any module) or as
// {"id": "GO-YYYY-NNNN", "module": "stdlib" | "<module path>"} (that module only). Anything else fails, and so does
// a results file without govulncheck's leading config message, with an unterminated trailing object or with text
// outside the JSON objects (an empty, truncated or corrupt scan must not pass).
//
// Usage: node scripts/govulncheck-filter.mjs <govulncheck-results.json> [.govulncheck-allowlist.json]
import fs from "node:fs";

const [,, resultsPath = "govulncheck-results.json", policyPath = ".govulncheck-allowlist.json"] = process.argv;

const policy = JSON.parse(fs.readFileSync(policyPath, "utf8"));
const allow = new Set();
for (const entry of policy.allowlist ?? []) {
  if (typeof entry === "string") allow.add(entry);
  else if (entry && typeof entry.id === "string") allow.add(entry.module ? `${entry.id}@${entry.module}` : entry.id);
}
const allowKeyFor = (id, module) => [id, `${id}@${module}`].find((key) => allow.has(key)) ?? null;

// govulncheck -json writes a stream of concatenated, pretty-printed JSON objects (config, progress, osv,
// finding, SBOM); split it by tracking brace depth outside of string literals.
function* jsonObjects(text) {
  let depth = 0;
  let start = -1;
  let inString = false;
  let escaped = false;
  for (let i = 0; i < text.length; i++) {
    const ch = text[i];
    if (inString) {
      if (escaped) escaped = false;
      else if (ch === "\\") escaped = true;
      else if (ch === '"') inString = false;
    } else if (depth === 0 && ch !== "{" && !/\s/.test(ch)) {
      throw new Error(`unexpected text outside JSON objects at offset ${i}`);
    } else if (ch === '"') {
      inString = true;
    } else if (ch === "{") {
      if (depth++ === 0) start = i;
    } else if (ch === "}" && depth > 0 && --depth === 0) {
      yield JSON.parse(text.slice(start, i + 1));
    }
  }
  if (depth !== 0 || inString) throw new Error(`unterminated JSON object at end of input (depth ${depth})`);
}

const symbol = (frame) => {
  const pkg = (frame.package ?? frame.module ?? "").split("/").pop();
  const receiver = frame.receiver ? `${frame.receiver.replace(/^\*/, "")}.` : "";
  return `${pkg}.${receiver}${frame.function}`;
};

const summaries = new Map(); // OSV id -> summary
const called = new Map(); // "<id>@<module>" -> { id, module, version, fixed, sites }
const notCalled = new Set(); // OSV ids seen only in import/module-level findings
let sawConfig = false; // govulncheck always opens the stream with a config message
let messages;
try {
  messages = [...jsonObjects(fs.readFileSync(resultsPath, "utf8"))];
} catch (err) {
  console.error(`govulncheck policy failure: cannot parse ${resultsPath}: ${err.message} (truncated or corrupt scan output)`);
  process.exit(2);
}
for (const msg of messages) {
  if (msg.config) sawConfig = true;
  if (msg.osv?.id) summaries.set(msg.osv.id, msg.osv.summary ?? "");
  const finding = msg.finding;
  if (!finding?.osv) continue;
  const vuln = finding.trace?.[0];
  if (!vuln?.function) {
    notCalled.add(finding.osv);
    continue;
  }
  const module = vuln.module ?? "unknown";
  const key = `${finding.osv}@${module}`;
  if (!called.has(key)) {
    called.set(key, { id: finding.osv, module, version: vuln.version ?? "?", fixed: finding.fixed_version ?? "none", sites: [] });
  }
  const entry = finding.trace[finding.trace.length - 1];
  const where = entry?.position ? `${entry.position.filename}:${entry.position.line}` : "?";
  called.get(key).sites.push(`${where}: ${symbol(entry)} -> ${symbol(vuln)}`);
}
for (const group of called.values()) notCalled.delete(group.id);
if (!sawConfig) {
  console.error(`govulncheck policy failure: no govulncheck config message in ${resultsPath} (empty or truncated scan output)`);
  process.exit(2);
}

const describe = (g) => `${g.id} ${g.module}@${g.version} (fixed in ${g.fixed}, ${g.sites.length} call site(s)): ${summaries.get(g.id) ?? ""}`.trim();

const byIDThenModule = (a, b) => a.id.localeCompare(b.id) || a.module.localeCompare(b.module);
const deferred = [];
const failures = [];
const matched = new Set();
for (const group of [...called.values()].sort(byIDThenModule)) {
  const key = allowKeyFor(group.id, group.module);
  if (key) {
    matched.add(key);
    deferred.push(group);
  } else {
    failures.push(group);
  }
}

for (const g of deferred) {
  const note = policy.notes?.[g.id];
  console.log(`allowlisted: ${describe(g)}${note ? `\n    note: ${note}` : ""}`);
}
for (const key of allow) {
  if (!matched.has(key)) console.log(`note: allowlist entry ${key} did not match any called finding - it can be removed`);
}
if (notCalled.size) console.log(`informational: ${[...notCalled].sort().join(", ")} affect imported packages only (not called)`);

if (failures.length) {
  console.error(`govulncheck policy failure: ${failures.length} called vulnerabilities (after allowlist)`);
  for (const g of failures) {
    console.error(`- ${describe(g)}`);
    for (const site of g.sites.slice(0, 5)) console.error(`    ${site}`);
    if (g.sites.length > 5) console.error(`    ... ${g.sites.length - 5} more call site(s)`);
  }
  process.exit(1);
}

console.log(`OK: no called vulnerabilities (after allowlist); ${deferred.length} allowlisted`);
