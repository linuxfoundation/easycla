// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT
//
// Shared helpers for the headless-browser modules (tests/e2e/browser/*.mjs).
// Mirrors tests/e2e/lib/e2e.py: config from secrets/<env>.env.secret (template config/example.env),
// credentials from secrets/*.secret,
// one JSON line per check in <run dir>/results.jsonl, screenshots and captured bodies under
// <run dir>/<module>/. Every module gets a network guard: production DocuSign is never reachable,
// sandbox DocuSign only with ALLOW_DOCUSIGN=1, and non-GET calls to our own backends are blocked
// unless the module declared them (or E2E_NO_WRITES=1, which blocks all of them).
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { chromium } from 'playwright';

process.umask(0o077);

export const ROOT = path.resolve(path.dirname(new URL(import.meta.url).pathname), '..', '..');
export const REPO_ROOT = path.resolve(ROOT, '..', '..');
export const ENV = process.env.E2E_ENV || '';
if (!['dev', 'prod'].includes(ENV)) {
  console.error('error: E2E_ENV must be dev or prod');
  process.exit(2);
}
export const MODULE = process.env.E2E_MODULE || path.basename(process.argv[1] || 'browser', '.mjs');
const SECRETS_DIR = process.env.E2E_SECRETS_DIR || path.join(ROOT, 'secrets');
const stamp = () => new Date().toISOString().replace(/[-:]/g, '').replace(/\.\d+Z$/, 'Z');
export const RUN_DIR = process.env.E2E_RUN_DIR
  || path.join(REPO_ROOT, 'e2e-tests', 'runs', 'suite', `${ENV}-${stamp()}`);
export const EVIDENCE_DIR = path.join(RUN_DIR, MODULE);
fs.mkdirSync(EVIDENCE_DIR, { recursive: true, mode: 0o700 });
const RESULTS = path.join(RUN_DIR, 'results.jsonl');
const LOG = path.join(RUN_DIR, 'run.log');
export const NO_WRITES = process.env.E2E_NO_WRITES === '1';
export const KEEP_STATE = process.env.E2E_KEEP_STATE === '1';
export const TAG = process.env.E2E_TAG || stamp().replace(/[TZ]/g, '');

// ---- config / secrets ------------------------------------------------------------------------
function loadConfig() {
  const out = {};
  const file = path.join(ROOT, 'secrets', `${ENV}.env.secret`);
  if (!fs.existsSync(file)) {
    console.error(`missing ${file} - copy tests/e2e/config/example.env there and fill it in (see tests/e2e/README.md)`);
    process.exit(2);
  }
  const text = fs.readFileSync(file, 'utf8');
  for (const line of text.split(/\r?\n/)) {
    const m = line.match(/^([A-Z][A-Z0-9_]*)=(.*)$/);
    if (m) out[m[1]] = process.env[m[1]] ?? m[2];
  }
  return out;
}
export const CFG = loadConfig();
export const cfg = (key, dflt = '') => CFG[key] || dflt;
export const gate = (key) => cfg(key) === '1';
export const SS = cfg('SS_URL').replace(/\/$/, '');
export const SS_HOST = new URL(SS).hostname;

export function secret(name) {
  const p = path.join(SECRETS_DIR, `${name}.secret`);
  return fs.existsSync(p) ? fs.readFileSync(p, 'utf8').trim() : '';
}
export const token = (identity) => secret(`auth0-${ENV}-${identity}.token`);

// secrets/auth0-<env>-<identity>.secret holds AUTH0_USERNAME=... / AUTH0_PASSWORD=... (same file
// utils/get_auth0_token.sh reads); the user name falls back to the identity itself.
export function credentials(identity) {
  const out = { username: identity, password: '' };
  for (const line of secret(`auth0-${ENV}-${identity}`).split(/\r?\n/)) {
    const m = line.match(/^(?:export\s+)?(AUTH0_USERNAME|AUTH0_PASSWORD)=(.*)$/);
    if (!m) continue;
    const v = m[2].trim().replace(/^(['"])(.*)\1$/, '$2');
    if (m[1] === 'AUTH0_USERNAME' && v) out.username = v;
    if (m[1] === 'AUTH0_PASSWORD') out.password = v;
  }
  return out;
}

// ---- results --------------------------------------------------------------------------------
export const COUNTS = { PASS: 0, FAIL: 0, SKIP: 0, INFO: 0 };
const SECRET_VALUES = [];
export const now = () => new Date().toISOString().replace(/\.\d+Z$/, 'Z');

export function redact(text, limit = 400) {
  let s = String(text ?? '');
  for (const v of SECRET_VALUES) if (v) s = s.split(v).join('[REDACTED]');
  s = s.replace(/eyJ[A-Za-z0-9_.-]{20,}/g, '[TOKEN]');
  return s.length > limit ? s.slice(0, limit) : s;
}

export function log(msg) {
  const line = `${now()} [${MODULE}] ${redact(msg, 2000)}`;
  console.log(line);
  fs.appendFileSync(LOG, line + '\n');
}

export function check(id, ok, note = '', info = {}) {
  const st = info.status || (ok ? 'PASS' : 'FAIL');
  COUNTS[st] = (COUNTS[st] || 0) + 1;
  const row = { id, status: st, module: MODULE, env: ENV, at: now(), note: redact(note) };
  for (const [k, v] of Object.entries(info)) {
    if (k === 'status') continue;
    try { JSON.stringify(v); row[k] = v; } catch { row[k] = String(v).slice(0, 300); }
  }
  fs.appendFileSync(RESULTS, JSON.stringify(row) + '\n');
  log(`${st} ${id} - ${note}`);
  return st === 'PASS';
}
export const skip = (id, note, info = {}) => check(id, false, note, { ...info, status: 'SKIP' });
export const info = (id, note, extra = {}) => check(id, true, note, { ...extra, status: 'INFO' });

export function finish(extra = '') {
  log(`module done: ${Object.entries(COUNTS).map(([k, v]) => `${k}=${v}`).join(' ')} evidence=${EVIDENCE_DIR} ${extra}`.trim());
  return COUNTS.FAIL === 0 ? 0 : 1;
}

export function evidence(name, obj) {
  const p = path.join(EVIDENCE_DIR, name);
  fs.writeFileSync(p, typeof obj === 'string' ? obj : JSON.stringify(obj, null, 1) + '\n', { mode: 0o600 });
  return p;
}

export async function shot(page, name) {
  const p = path.join(EVIDENCE_DIR, `${name}.png`);
  await page.screenshot({ path: p, fullPage: false }).catch(() => {});
  return p;
}

export const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
export const squash = (s) => String(s ?? '').replace(/\s+/g, ' ').trim();
export const bodyText = async (page) => squash(await page.locator('body').innerText().catch(() => ''));
export const hideOverlays = (page) => page.addStyleTag({
  content: '#transcend-consent-manager,.osano-cm-window,.osano-cm-dialog,#onetrust-consent-sdk{display:none!important;pointer-events:none!important}',
}).catch(() => {});

// ---- browser / network guard ---------------------------------------------------------------------
const OWN_HOSTS = new Set([SS_HOST, 'API_GW', 'API_V4', 'LEGACY_API', 'LEGACY_API_ALIAS', 'CORPORATE_CONSOLE',
  'CONTRIBUTOR_CONSOLE', 'PCC_URL', 'PCC_V1_URL'].map((k) => {
  if (k === SS_HOST) return k;
  try { return cfg(k) ? new URL(cfg(k)).hostname : ''; } catch { return ''; }
}).filter(Boolean));
const SANDBOX_DOCUSIGN = new Set(['demo.docusign.net', 'apps-d.docusign.com', 'account-d.docusign.com']);
// Hosts that may receive the identity's password: AUTH0_DOMAIN plus the optional comma-separated SSO_HOSTS.
// isAuth0Host() is the broader route-guard predicate (any auth hop may pass); fillLogin() uses the allow-list only.
const CREDENTIAL_HOSTS = new Set([cfg('AUTH0_DOMAIN'), ...cfg('SSO_HOSTS').split(',')].map((s) => s.trim()).filter(Boolean));
export const isCredentialHost = (h) => CREDENTIAL_HOSTS.has(h);
export const isAuth0Host = (h) => isCredentialHost(h) || /(^|\.)auth0\.com$/.test(h) || /^sso\./.test(h);
const ALWAYS_WRITE_OK = [/^\/api\/auth\//, /^\/callback$/, /\/permissions\/checks$/];

// launch({ identity, writes: [regex on "METHOD host/path"], allowDocusign: bool, state: bool })
// Returns { browser, context, page, net } where net.api lists our-host responses, net.blocked the
// requests the guard refused, net.bodies the captured bodies for net.capture(regex).
// context -> { writes, net } of the session that owns it, so bff() can apply the same guard as the route handler
const GUARDS = new WeakMap();

export async function launch(opts = {}) {
  const identity = opts.identity || cfg('ID1');
  const statePath = path.join(SECRETS_DIR, `pw-${ENV}-${identity}.state.secret`);
  const useState = opts.state !== false && fs.existsSync(statePath);
  const browser = await chromium.launch({ headless: true, args: ['--disable-dev-shm-usage', '--no-sandbox'] });
  const context = await browser.newContext({
    viewport: { width: 1440, height: 1000 },
    ...(useState ? { storageState: statePath } : {}),
  });
  const page = await context.newPage();
  const net = { api: [], writes: [], blocked: [], bodies: [], captures: [], identity, statePath, usedState: useState };
  net.capture = (re) => net.captures.push(re);
  const allowDocusign = !!opts.allowDocusign && gate('ALLOW_DOCUSIGN') && ENV === 'dev' && !NO_WRITES;
  const writes = NO_WRITES ? [] : (opts.writes || []);
  const creds = credentials(identity);
  if (creds.password) SECRET_VALUES.push(creds.password);
  GUARDS.set(context, { writes, net });

  await context.route('**/*', async (route) => {
    const req = route.request();
    let u;
    try { u = new URL(req.url()); } catch { await route.continue(); return; }
    const method = req.method();
    const isRead = ['GET', 'HEAD', 'OPTIONS'].includes(method);
    const docusign = /(^|\.)docusign\.(net|com)$/.test(u.hostname);
    // sandbox sessions: writes only to the demo hosts; reads to any DocuSign host may pass
    // (the demo signing page beacons telemetry to a.docusign.com, a GET cannot sign anything)
    if (docusign && !(allowDocusign && (SANDBOX_DOCUSIGN.has(u.hostname) || isRead))) {
      net.blocked.push({ method, url: u.origin + u.pathname, reason: 'docusign' });
      await route.abort('blockedbyclient');
      return;
    }
    if (isRead || isAuth0Host(u.hostname) || docusign || !OWN_HOSTS.has(u.hostname)) { await route.continue(); return; }
    const key = `${method} ${u.hostname}${u.pathname}`;
    if (ALWAYS_WRITE_OK.some((re) => re.test(u.pathname)) || writes.some((re) => re.test(key))) {
      if (!ALWAYS_WRITE_OK.some((re) => re.test(u.pathname))) {
        let body = null;
        try { body = req.postDataJSON(); } catch { body = req.postData()?.slice(0, 300) ?? null; }
        net.writes.push({ at: now(), method, url: u.origin + u.pathname, body });
      }
      await route.continue();
      return;
    }
    net.blocked.push({ method, url: u.origin + u.pathname, reason: NO_WRITES ? 'no-writes' : 'undeclared-write' });
    await route.abort('blockedbyclient');
  });
  page.on('response', async (res) => {
    let u;
    try { u = new URL(res.url()); } catch { return; }
    if (!OWN_HOSTS.has(u.hostname)) return;
    const method = res.request().method();
    const entry = { at: now(), status: res.status(), method, url: (u.origin + u.pathname + u.search).slice(0, 300) };
    net.api.push(entry);
    if (net.captures.some((re) => re.test(u.pathname))) {
      const text = await res.text().catch(() => '');
      net.bodies.push({ ...entry, body: redact(text, 20000) });
    }
  });
  return { browser, context, page, net };
}

export async function close({ browser, context, page, net }, saveState = true) {
  try {
    if (saveState && context && net?.statePath && page && !page.isClosed()) {
      await context.storageState({ path: net.statePath });
      fs.chmodSync(net.statePath, 0o600);
    }
  } catch { /* best effort */ }
  // one file per identity: modules that drive two sessions must not overwrite the first one's capture
  const sfx = net?.identity && net.identity !== cfg('ID1') ? `-${net.identity}` : '';
  if (net) evidence(`network${sfx}.private.json`, { api: net.api, writes: net.writes, blocked: net.blocked });
  if (net?.bodies?.length) evidence(`bodies${sfx}.private.json`, net.bodies);
  await browser?.close().catch(() => {});
}

// Auth0 / LF SSO login (prod sso.linuxfoundation.org, dev linuxfoundation-dev.auth0.com). Returns
// 'cached' when the stored session is still valid, 'password' after a fresh login.
export async function login(page, identity, landing = `${SS}/`) {
  await page.goto(landing, { waitUntil: 'domcontentloaded', timeout: 60000 });
  await page.waitForURL((u) => isAuth0Host(u.hostname) || u.hostname === new URL(landing).hostname, { timeout: 30000 }).catch(() => {});
  await sleep(2500);
  if (!isAuth0Host(new URL(page.url()).hostname)) return 'cached';
  await fillLogin(page, identity);
  await page.waitForURL((u) => !isAuth0Host(u.hostname), { timeout: 60000 });
  await sleep(2000);
  return 'password';
}

// Fill and submit the Auth0 / LF SSO form the page currently shows (also used for the console's second SSO hop).
// Credentials are only ever typed on an allow-listed host (AUTH0_DOMAIN / SSO_HOSTS).
export async function fillLogin(page, identity) {
  const host = new URL(page.url()).hostname;
  if (!isCredentialHost(host)) throw new Error(`refusing to enter credentials on ${host}: not in AUTH0_DOMAIN/SSO_HOSTS`);
  const creds = credentials(identity);
  if (!creds.password) throw new Error(`no password for ${identity} (secrets/auth0-${ENV}-${identity}.secret)`);
  await hideOverlays(page);
  const user = page.locator('#username, input[name="username"], input[name="email"]').first();
  if (await user.count()) await user.fill(creds.username);
  else await page.getByRole('textbox', { name: /username or email|username|email/i }).first().fill(creds.username);
  const pass = page.locator('#password, input[name="password"], input[type="password"]').first();
  if (await pass.count()) await pass.fill(creds.password);
  else await page.getByRole('textbox', { name: /password/i }).first().fill(creds.password);
  await hideOverlays(page);
  await page.getByRole('button', { name: /sign in|log in|continue/i }).first().click();
  try {
    await page.waitForURL(/\/u\/consent/, { timeout: 5000 });
    await page.getByRole('button', { name: /accept|authorize|allow/i }).first().click();
  } catch { /* no consent step */ }
}

// ---- Self Serve helpers -------------------------------------------------------------------------
export async function selectOrgCookie(context, sfid) {
  await context.addCookies([{ name: 'lfx-selected-account', value: JSON.stringify({ uid: sfid }), domain: SS_HOST, path: '/' }]);
}

// Select an organization through the in-app switcher when the trigger shows no/other org (the FE only
// restores the lfx-selected-account cookie when persona detection seeded organizations).
export async function ensureOrgSelected(page, uid, name) {
  const trigger = () => page.getByTestId('org-selector-trigger-name').innerText({ timeout: 3000 }).then(squash).catch(() => '');
  const matches = (t) => !!name && !!t && t.startsWith(name.slice(0, 12));
  for (let i = 0; i < 20; i++) {
    const t = await trigger();
    if (t && t !== 'Select Organization') break;
    await sleep(1000);
  }
  if (matches(await trigger())) return trigger();
  for (let attempt = 0; attempt < 3; attempt++) {
    const expanded = await page.getByTestId('org-selector').getAttribute('aria-expanded').catch(() => null);
    if (expanded !== 'true') {
      await page.getByTestId('org-selector').click().catch(() => {});
      await sleep(1500);
    }
    const item = page.getByTestId(`org-item-${uid}`);
    const seen = await item.waitFor({ state: 'visible', timeout: 15000 }).then(() => true).catch(() => false);
    if (!seen) { await page.keyboard.press('Escape').catch(() => {}); await sleep(800); continue; }
    await item.click();
    for (let i = 0; i < 10; i++) {
      await sleep(1000);
      if (matches(await trigger())) return trigger();
    }
  }
  return trigger();
}

export async function closeOverlay(page) {
  for (let i = 0; i < 3; i++) {
    const panel = page.locator('.org-selector-panel, [class*="selector-backdrop"]').first();
    if (!(await panel.isVisible().catch(() => false))) return;
    await page.keyboard.press('Escape').catch(() => {});
    await sleep(500);
  }
}

// BFF call with the browser session (cookies). Returns { status, json, text }.
// Direct BFF call with the session cookies. context.request.fetch does NOT pass through context.route, so non-GET calls
// are guarded here exactly like browser-issued writes: refused under --no-writes, allowed only when a declared write
// regex of the owning session matches, and recorded in net.writes. Refused calls return { status: 0, skipped }.
export async function bff(context, method, p, data) {
  if (!['GET', 'HEAD', 'OPTIONS'].includes(method)) {
    const g = GUARDS.get(context);
    const u = new URL(SS + p);
    const key = `${method} ${u.hostname}${u.pathname}`;
    const reason = NO_WRITES ? 'no-writes' : (!g || !g.writes.some((re) => re.test(key))) ? 'undeclared-write' : null;
    if (reason) {
      g?.net.blocked.push({ method, url: u.origin + u.pathname, reason, via: 'bff' });
      return { status: 0, json: null, text: '', skipped: reason };
    }
    g.net.writes.push({ at: now(), method, url: u.origin + u.pathname, body: data ?? null, via: 'bff' });
  }
  const res = await context.request.fetch(SS + p, { method, ...(data !== undefined ? { data } : {}) });
  const text = await res.text().catch(() => '');
  let json = null;
  try { json = JSON.parse(text); } catch { /* not json */ }
  return { status: res.status(), json, text };
}

// dev only: drop the whole ACS answer cache through utils/dev_acs_role_flip.sh flush-cache (M2M token from SSM, needs the
// lfproduct-dev AWS profile) so a grant/revoke becomes visible to Self Serve right away instead of after ~30 minutes.
// Returns a short note; never throws (prod has no flush - the chains then wait SS_PROPAGATION_SECS and report INFO).
export function flushAcsCache() {
  if (ENV !== 'dev') return 'no cache flush on prod (ACS authorize cache ~30 min)';
  const script = path.join(REPO_ROOT, 'utils', 'dev_acs_role_flip.sh');
  if (!fs.existsSync(script)) return 'utils/dev_acs_role_flip.sh not found';
  try {
    const out = execFileSync(script, ['flush-cache'], { encoding: 'utf8', timeout: 120000, stdio: ['ignore', 'pipe', 'pipe'] });
    return (out.split('\n').find((l) => /flushed/i.test(l)) || out.trim().split('\n')[0] || 'flushed').slice(0, 120);
  } catch (err) {
    return `flush-cache unavailable (${(err.stderr || err.message || '').toString().trim().split('\n').pop().slice(0, 120)})`;
  }
}

// Open the Organization lens EasyCLA page of <sfid> and return the path we ended on.
export async function gotoOrgEasyCla(page, sfid, name, suffix = '/easycla') {
  await selectOrgCookie(page.context(), sfid);
  await page.goto(`${SS}/org/${sfid}${suffix}`, { waitUntil: 'domcontentloaded', timeout: 60000 });
  await sleep(5000);
  await hideOverlays(page);
  if (name) await ensureOrgSelected(page, sfid, name).catch(() => {});
  await closeOverlay(page);
  return new URL(page.url()).pathname;
}

// Wait for a locator to be visible; returns true/false instead of throwing.
export const visible = (loc, timeout = 15000) => loc.waitFor({ state: 'visible', timeout }).then(() => true).catch(() => false);

// Collect the LaunchDarkly flag evaluations the SS front end receives for the logged-in user.
// Returns { calls, flags(): {name: value} } - only flag names matching LD_FLAG_FILTER are kept.
export function attachLd(page) {
  const calls = [];
  const filter = new RegExp(cfg('LD_FLAG_FILTER', 'cla|org-lens|easycla|m1|m2|m3'), 'i');
  page.on('response', async (r) => {
    let u;
    try { u = new URL(r.url()); } catch { return; }
    if (!/launchdarkly/i.test(u.hostname)) return;
    const text = await r.text().catch(() => '');
    calls.push({ host: u.hostname, path: u.pathname.slice(0, 90), status: r.status(), text });
  });
  return {
    calls,
    flags() {
      const out = {};
      for (const c of calls) {
        if (!c.text) continue;
        let j;
        try { j = JSON.parse(c.text); } catch { continue; }
        if (!j || typeof j !== 'object') continue;
        for (const [k, v] of Object.entries(j)) {
          if (filter.test(k)) out[k] = (v && typeof v === 'object' && 'value' in v) ? v.value : v;
        }
      }
      return out;
    },
  };
}
