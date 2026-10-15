// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT
//
// M2 signing as ID1 through the Me lens, dev only (ALLOW_DOCUSIGN=1, DocuSign sandbox):
//   ICLA: Sign CLA -> ICLA_GROUP -> contributor console "individual" -> DocuSign sandbox -> back -> me/clas row Valid
//   ECLA: Sign CLA -> M3_CLA_GROUP -> console "corporate" -> M3_ORG_NAME -> certification -> POST /v2/request-employee-signature
// State-aware: when ID1 already holds a valid agreement of that kind for the group the flow cannot be repeated
// (an ICLA is never invalidated by the user) - the module then verifies the existing row and reports INFO.
import * as e2e from './lib/e2e.mjs';
import * as signing from './lib/signing.mjs';
import { m3, fixtureWrites } from './lib/m3.mjs';

const { cfg, check, info, skip, sleep } = e2e;
const ID1 = cfg('ID1');
const ICLA_GROUP = cfg('ICLA_GROUP');
const ICLA_SEARCH = cfg('ICLA_GROUP_SEARCH') || cfg('CLA_GROUP_NAME');
const ECLA_GROUP = cfg('M3_CLA_GROUP');
const ECLA_GROUP_NAME = cfg('M3_CLA_GROUP_NAME', cfg('CLA_GROUP_NAME'));
const ECLA_COMPANY = cfg('M3_ORG_NAME');
const fx = { sfid: cfg('M3_ORG_SFID'), name: ECLA_COMPANY, group: ECLA_GROUP, sig: cfg('M3_CCLA') };
const DOCUSIGN = e2e.ENV === 'dev' && e2e.gate('ALLOW_DOCUSIGN') && !e2e.NO_WRITES;
const WRITES = e2e.ENV === 'dev' && e2e.gate('ALLOW_FIXTURE_WRITES') && !e2e.NO_WRITES;

if (e2e.ENV !== 'dev') { skip('M2-SIGNING', `signing flows run on dev only (env=${e2e.ENV})`); process.exit(e2e.finish()); }
if (!WRITES && !DOCUSIGN) { skip('M2-SIGNING', 'ALLOW_DOCUSIGN=0 / ALLOW_FIXTURE_WRITES=0 / --no-writes'); process.exit(e2e.finish()); }

const session = await e2e.launch({ identity: ID1, allowDocusign: DOCUSIGN, writes: [...signing.signingWrites('icla'), ...signing.signingWrites('ecla'), ...(fx.sfid && fx.sig ? fixtureWrites(fx.sfid, fx.sig) : [])] });
const { page, context, net } = session;
net.capture(/\/api\/me\/clas\/(prepare-sign|sign-options)|request-individual-signature|request-employee-signature/);
try {
  const how = await e2e.login(page, ID1, `${e2e.SS}/profile/clas`);
  check('M2-LOGIN', new URL(page.url()).hostname === e2e.SS_HOST, `${ID1} login ${how}`);
  const before = await signing.myAgreements(context);
  e2e.evidence('me-clas-before.private.json', before);
  check('M2-ME-CLAS-BEFORE', before.http === 200, `GET /api/me/clas -> ${before.http}; agreements=${before.rows.length}`);

  // ---- ICLA (DocuSign) ----
  if (!ICLA_GROUP) skip('M2-ICLA', 'ICLA_GROUP not configured');
  else if (!DOCUSIGN) skip('M2-ICLA', 'ALLOW_DOCUSIGN=0');
  else {
    const existing = before.rows.filter((a) => a.claGroupId === ICLA_GROUP && a.kind === 'ICLA' && a.status === 'valid');
    if (existing.length) {
      const row = await signing.meLensRow(session, existing[0].id);
      info('M2-ICLA-ALREADY-SIGNED', `${ID1} already holds a valid ICLA ${existing[0].id} for ${ICLA_GROUP} (an ICLA cannot be re-signed; set ICLA_GROUP to an unsigned group to exercise DocuSign); Me lens row: "${row.slice(0, 100)}"`);
      check('M2-ICLA-ROW-VALID', /\bvalid\b/i.test(row), `agreement-row-${existing[0].id} shows Valid`);
    } else {
      try {
        const hand = await signing.handOffToConsole(session, ID1, { groupId: ICLA_GROUP, groupName: ICLA_SEARCH, githubId: cfg('ID1_GITHUB_ID'), contract: /individual|icla/i });
        check('M2-ICLA-CONSOLE-HANDOFF', new URL(hand.url).hostname === new URL(cfg('CONTRIBUTOR_CONSOLE')).hostname, `picker="${hand.picker.slice(0, 80)}" -> ${new URL(hand.url).origin} console auth=${hand.auth}`);
        const indiv = page.getByRole('button', { name: /proceed as an individual contributor/i }).first();
        await indiv.waitFor({ state: 'visible', timeout: 30000 });
        // the console's individual dashboard POSTs request-individual-signature while it loads ("Preparing CLA...");
        // SIGN CLA only opens the sign_url it got back and stays disabled when that call failed
        const reqP = page.waitForResponse((r) => /request-individual-signature/.test(r.url()) && r.request().method() === 'POST', { timeout: 90000 }).catch(() => null);
        await indiv.click();
        const req = await reqP;
        const reqBody = req ? await req.json().catch(() => null) : null;
        await sleep(3000);
        await e2e.hideOverlays(page);
        await e2e.shot(page, 'icla-console-ready');
        const reqMsg = e2e.squash(reqBody?.Message || reqBody?.message || reqBody?.errors?.[0] || '');
        // a recognised eligibility refusal (e.g. "an individual CLA for this CLA Group was invalidated by an administrator -
        // signing a new individual CLA ... is not permitted") means this ICLA_GROUP cannot be signed by ID1: a configuration
        // problem, not a product defect. Any other 4xx (bad ids, auth, routing) fails the request check below.
        const ineligible = !!req && req.status() >= 400 && req.status() < 500 && /invalidated by an administrator|not permitted|already (?:signed|has)|not eligible/i.test(reqMsg);
        if (ineligible) {
          skip('M2-ICLA', `ICLA_GROUP ${ICLA_GROUP} is not signable by ${ID1}: POST request-individual-signature -> ${req.status()} ${reqMsg.slice(0, 160)} - point ICLA_GROUP at another unsigned group`);
        } else {
          check('M2-ICLA-REQUEST-SIGNATURE', !!req && req.status() < 300 && !!reqBody?.sign_url, `POST request-individual-signature -> ${req ? req.status() : 'not observed'} sign_url=${!!reqBody?.sign_url} ${reqMsg.slice(0, 120)}`);
          const compliance = page.locator('input[type="checkbox"]').first();
          if (await compliance.isVisible().catch(() => false)) await compliance.check({ force: true }).catch(() => compliance.click({ force: true }).catch(() => {}));
          else { const lbl = page.getByText(/i hereby certify/i).first(); if (await lbl.isVisible().catch(() => false)) await lbl.click().catch(() => {}); }
          await sleep(1500);
          const signBtn = page.getByRole('button', { name: /open cla|sign cla|sign icla|agree/i }).first();
          await signBtn.click();
          const ds = await signing.docusignSign(page);
          check('M2-ICLA-DOCUSIGN-SANDBOX', ds.signed && ds.finished, `sandbox envelope: fields filled=${ds.filled} signed=${ds.signed} finished=${ds.finished} -> ${ds.finalUrl.slice(0, 80)}`);
          let valid = [];
          for (let i = 0; i < 24 && !valid.length; i++) {
            valid = (await signing.myAgreements(context)).rows.filter((a) => a.claGroupId === ICLA_GROUP && a.kind === 'ICLA' && a.status === 'valid');
            if (!valid.length) await sleep(5000);
          }
          check('M2-ICLA-MECLAS-VALID', valid.length === 1, `valid ICLA rows for ${ICLA_GROUP}=${valid.length} ${JSON.stringify(valid)}`);
          if (valid.length) {
            const row = await signing.meLensRow(session, valid[0].id);
            await e2e.shot(page, 'icla-me-lens');
            check('M2-ICLA-ROW-VALID', /\bvalid\b/i.test(row), `agreement-row-${valid[0].id}: "${row.slice(0, 100)}"`);
          }
        }
      } catch (err) {
        check('M2-ICLA', false, `error: ${err.message} at ${page.url().slice(0, 120)}`);
        await e2e.shot(page, 'icla-error');
      }
    }
  }

  // ---- ECLA (click-through, no DocuSign) ----
  if (!ECLA_GROUP || !ECLA_COMPANY) skip('M2-ECLA', 'M3_CLA_GROUP / M3_ORG_NAME not configured');
  else if (!WRITES) skip('M2-ECLA', 'ALLOW_FIXTURE_WRITES=0');
  else {
    const now = await signing.myAgreements(context, ECLA_GROUP);
    const existing = now.fixture.filter((a) => a.kind === 'ECLA' && a.status === 'valid');
    if (existing.length) {
      const row = await signing.meLensRow(session, existing[0].id);
      info('M2-ECLA-ALREADY-VALID', `${ID1} already holds a valid ECLA ${existing[0].id} for ${ECLA_GROUP} (${existing[0].company}); the click-through is exercised by api/interactions.py and m3_chain for ID2; row: "${row.slice(0, 100)}"`);
      check('M2-ECLA-ROW-VALID', /\bvalid\b/i.test(row), `agreement-row-${existing[0].id} shows Valid`);
    } else {
      // Pre-flight: the console only offers the click-through when the contributor is on the company's approval list;
      // otherwise it lands on "Request Authorization to Contribute". ID1 is the CLA manager of the fixture, so add a
      // GitHub-username entry for ID1 through the BFF when nothing covers it, and remove it again afterwards.
      let added = null;
      const h = fx.sfid && fx.sig ? m3(session, fx) : null;
      try {
        if (h) {
          const st = await h.state();
          const domain = (cfg('ID1_EMAIL') || '').split('@')[1] || '';
          const covers = (x) => (x.kind === 'github-username' && x.value === cfg('ID1_GITHUB')) || (x.kind === 'email' && x.value === cfg('ID1_EMAIL')) || (x.kind === 'domain' && domain && x.value === domain);
          if (Object.values(st.http).some((c) => c !== 200)) info('M2-ECLA-PREFLIGHT', `fixture reads ${JSON.stringify(st.http)} - approval-list coverage not checked`);
          else if (st.entries.some(covers)) info('M2-ECLA-PREFLIGHT', `${ID1} already covered by the approval list: ${JSON.stringify(st.entries.filter(covers))}`);
          else if (!cfg('ID1_GITHUB')) info('M2-ECLA-PREFLIGHT', `${ID1} not on the approval list and ID1_GITHUB is not configured - the console will ask for authorization`);
          else {
            const entry = { kind: 'github-username', value: cfg('ID1_GITHUB') };
            const r = await e2e.bff(context, 'PUT', h.paths.approval, { add: [entry], remove: [] });
            check('M2-ECLA-PREFLIGHT', r.status === 200, `PUT approval-list add ${entry.kind}:${entry.value} -> ${r.status} (removed again after the flow)`);
            if (r.status === 200) added = entry;
          }
        }
        const hand = await signing.handOffToConsole(session, ID1, { groupId: ECLA_GROUP, groupName: ECLA_GROUP_NAME, githubId: cfg('ID1_GITHUB_ID') });
        check('M2-ECLA-CONSOLE-HANDOFF', new URL(hand.url).hostname === new URL(cfg('CONTRIBUTOR_CONSOLE')).hostname, `picker="${hand.picker.slice(0, 80)}" -> ${new URL(hand.url).origin} console auth=${hand.auth}`);
        const ack = await signing.corporateAcknowledge(session, ECLA_COMPANY);
        e2e.evidence('ecla-receipt.private.json', ack);
        check('M2-ECLA-CLICKTHROUGH', ack.status === 200 && !ack.receipt.errors && ack.done, `POST /v2/request-employee-signature -> ${ack.status} signature=${ack.signatureId} done=${ack.done}`);
        await signing.exitConsole(session);
        const after = await signing.myAgreements(context, ECLA_GROUP);
        const valid = after.fixture.filter((a) => a.kind === 'ECLA' && a.status === 'valid');
        check('M2-ECLA-MECLAS-VALID', valid.length === 1 && (!ack.signatureId || valid[0].id === ack.signatureId), `valid ECLA rows=${valid.length} ${JSON.stringify(valid)}`);
        if (valid.length) {
          const row = await signing.meLensRow(session, valid[0].id);
          await e2e.shot(page, 'ecla-me-lens');
          check('M2-ECLA-ROW-VALID', /\bvalid\b/i.test(row), `agreement-row-${valid[0].id}: "${row.slice(0, 100)}"`);
        }
      } catch (err) {
        const asked = /request-authorization/.test(page.url());
        check('M2-ECLA', false, `error: ${err.message} at ${page.url().slice(0, 120)}${asked ? ' (the console asked for authorization: the contributor is not on the approval list)' : ''}`);
        await e2e.shot(page, 'ecla-error');
      } finally {
        if (added) {
          const r = await e2e.bff(context, 'PUT', h.paths.approval, { add: [], remove: [added] }).catch((e) => ({ status: `error ${e.message}` }));
          info('M2-ECLA-PREFLIGHT-RESTORE', `PUT approval-list remove ${added.kind}:${added.value} -> ${r.status}`);
        }
      }
    }
  }
  check('M2-NO-FORBIDDEN-WRITES', !net.writes.some((w) => signing.SIGNING_FORBIDDEN.test(w.url)), `writes=${JSON.stringify([...new Set(net.writes.map((w) => `${w.method} ${w.url.replace(/^https?:\/\//, '')}`))]).slice(0, 400)}`);
  const blocked = net.blocked.filter((b) => !/telemetry|analytics|segment|sentry|datadog|launchdarkly/i.test(b.url));
  check('M2-NO-BLOCKED-WRITES', blocked.length === 0, `guard blocked ${blocked.length} request(s) ${JSON.stringify(blocked.slice(0, 3))}`);
} catch (err) {
  check('M2-HARNESS', false, `error: ${err.message} at ${page.url().slice(0, 120)}`);
  await e2e.shot(page, 'error');
} finally {
  await e2e.close(session);
}
process.exit(e2e.finish());
