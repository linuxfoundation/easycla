// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT
//
// M3: Organization lens -> EasyCLA -> signed agreement detail as ID1 (CLA manager) on the M3 fixture.
// Approval list (validation, add, duplicate, delete-cancel, remove), Auto-ECLA on/off, acknowledgment
// invalidation, CLA manager roster (validation, add/remove ID2, remove-cancel), activity log, end state.
// State-aware: synthetic approval entries left by an earlier run are removed first, ID1's own e-mail/GitHub
// entries are added only when absent at baseline (and then removed again), the manager add/remove order follows
// whether ID2 is already on the roster, and the end state must equal the baseline. Whatever happens, a BFF
// reconcile in `finally` puts entries, roster and Auto-ECLA back to the baseline.
// Browser writes only under /api/orgs/<M3_ORG_SFID>/lens/cla-groups/<M3_CCLA>/ (guard-enforced). The one exception is
// the acknowledgment probe: it manager-invalidates ID1's own acknowledgment (EasyCLA e-mails ID1 about it) and then
// re-acknowledges ID1 through the legacy API (needs ID1_UID + LEGACY_API + ID1_GITHUB and the fixture CCLA confirmed as the
// resolved company's signature on M3_PROJECT_SFID before anything is invalidated, otherwise the probe is skipped;
// a GitHub-username entry covers ID1 for the duration of the re-acknowledgment when the baseline list does not), leaving
// one more invalidated acknowledgment record on the fixture - the same residue m3_chain/interactions leave for ID2.
import * as e2e from './lib/e2e.mjs';
import { m3, fixtureWrites } from './lib/m3.mjs';

const { cfg, check, info, skip } = e2e;
const ID1 = cfg('ID1');
const ID2 = cfg('ID2');
const fx = { sfid: cfg('M3_ORG_SFID'), name: cfg('M3_ORG_NAME'), group: cfg('M3_CLA_GROUP'), sig: cfg('M3_CCLA') };
const WRITES = e2e.gate('ALLOW_FIXTURE_WRITES') && !e2e.NO_WRITES;
const synthetic = `qa-e2e-suite-${e2e.TAG}.example.com`;
// only the synthetic domains are suite-owned: ID1's own e-mail / GitHub entries may legitimately be part of the fixture
const suiteOwned = (v) => /^qa-e2e-suite-.*\.example\.com$/.test(v);
const OWN = [{ kind: 'github-username', value: cfg('ID1_GITHUB'), tag: 'GITHUB-USERNAME', re: /github user/i }, { kind: 'email', value: cfg('ID1_EMAIL'), tag: 'EMAIL', re: /^email$/i }];

if (!fx.sfid || !fx.sig) { skip('M3-FIXTURE', 'M3_ORG_SFID / M3_CCLA not configured'); process.exit(e2e.finish()); }

const session = await e2e.launch({ identity: ID1, writes: WRITES ? fixtureWrites(fx.sfid, fx.sig) : [] });
const { page, net } = session;
const h = m3(session, fx);
let base = null; let expectedEntries = null;
const id2 = { lfUsername: ID2, firstName: cfg('ID2_FIRST'), lastName: cfg('ID2_LAST'), email: cfg('ID2_EMAIL') };
const LEGACY = cfg('LEGACY_API').replace(/\/$/, '');
const jsonCall = async (method, url, body, bearer) => {
  const headers = { Accept: 'application/json', ...(body ? { 'Content-Type': 'application/json' } : {}), ...(bearer ? { Authorization: `Bearer ${bearer}` } : {}) };
  try {
    const r = await fetch(url, { method, headers, body: body ? JSON.stringify(body) : undefined });
    const text = await r.text().catch(() => '');
    let json = null;
    try { json = JSON.parse(text); } catch { /* not json */ }
    return { status: r.status, json, text: text.slice(0, 200) };
  } catch (err) { return { status: 0, json: null, text: String(err).slice(0, 200) }; }
};
// A manager-invalidated acknowledgment is never resurrected by the backend (not by Auto-ECLA, not by a criteria re-add), so
// the invalidation probe below would leave ID1 unapproved on the fixture after every run. Acknowledge again through the
// legacy API the way the contributor console does (the recipe of fixture_writes RM/RA and interactions I3) and wait until
// the lens lists a valid ID1 acknowledgment again. The legacy pre-check refuses a contributor nobody on the approval list
// covers ("user not authorized for this ccla"), and after the dialog's "also remove" the baseline may not cover ID1 at
// all (dev: a gitlab-username entry ID1's record cannot match) - so, like m2_signing's ECLA pre-flight, cover ID1 with a
// GitHub-username entry for the duration of the re-acknowledgment and take it off again. Where the baseline covers ID1
// (prod) the new acknowledgment stays valid; where it does not, the removal voids it the resurrectable way (see below),
// which is the state the module found. The invalidation e-mail EasyCLA sends to ID1 is expected residue.
// The company the re-acknowledgment targets: several signing entities may share one Salesforce organization, so the
// parent entity's internal id is only accepted when the fixture CCLA is that company's signature on the fixture project.
// Resolved BEFORE the invalidation - an unconfirmed mapping skips the probe instead of leaving ID1 invalidated.
async function resolveAckCompany(projectSfid) {
  const co = await jsonCall('GET', `${cfg('API_V4')}/v4/company/external/${fx.sfid}`, null, e2e.token(ID1));
  const companyId = co.json?.companyID;
  if (!companyId) return { note: `GET /v4/company/external/${fx.sfid} -> ${co.status} (no companyID)` };
  const sigs = await jsonCall('GET', `${cfg('API_V4')}/v4/signatures/project/${projectSfid}/company/${companyId}`, null, e2e.token(ID1));
  const ids = (sigs.json?.signatures || []).map((s) => s.signatureID);
  if (!ids.includes(fx.sig)) return { note: `CCLA ${fx.sig} is not a signature of company ${companyId} on project ${projectSfid} (GET -> ${sigs.status}, ${JSON.stringify(ids)}) - another signing entity of ${fx.sfid}?` };
  return { companyId, note: `company ${companyId} holds CCLA ${fx.sig} on project ${projectSfid}` };
}
async function reacknowledgeId1(companyId) {
  const steps = [];
  const domain = (cfg('ID1_EMAIL') || '').split('@')[1] || '';
  const covers = (x) => (x.kind === 'github-username' && x.value === cfg('ID1_GITHUB')) || (x.kind === 'email' && x.value === cfg('ID1_EMAIL')) || (x.kind === 'domain' && domain && x.value === domain);
  const validId1 = (a) => h.ackOf(a, ID1, cfg('ID1_GITHUB')) && h.ackValid(a);
  let temp = null;
  if (!(await h.state()).entries.some(covers)) {
    temp = { kind: 'github-username', value: cfg('ID1_GITHUB') };
    const r = await e2e.bff(session.context, 'PUT', h.paths.approval, { add: [temp], remove: [] });
    steps.push(`temporary ${temp.kind}:${temp.value} add -> ${r.status}`);
    if (r.status !== 200) temp = null;
  }
  const body = { project_id: fx.group, company_id: companyId, user_id: cfg('ID1_UID') };
  const prep = await jsonCall('POST', `${LEGACY}/v2/check-prepare-employee-signature`, body);
  // a 500 "TypeError: 'NoneType' object is not subscriptable" here is the legacy answer when the user has no active
  // pull-request metadata - the acknowledgment is already stored by then, the lens listing is the truth
  const req = await jsonCall('POST', `${LEGACY}/v2/request-employee-signature`, { ...body, return_url_type: 'github', return_url: `${e2e.SS}/profile/clas` });
  const errors = req.json?.errors || prep.json?.errors;
  steps.push(`legacy check-prepare -> ${prep.status}, request-employee-signature -> ${req.status}${errors ? ` errors=${JSON.stringify(errors).slice(0, 160)}` : ''}`);
  let ok = false;
  for (let i = 0; i < 12 && !ok; i++) {
    ok = (await h.state()).acks.some(validId1);
    if (!ok) await e2e.sleep(5000);
  }
  if (temp) {
    const r = await e2e.bff(session.context, 'PUT', h.paths.approval, { add: [], remove: [temp] });
    // backend rule (signatures/repository.go verifyUserApprovals): taking the only criterion covering ID1 away voids the
    // acknowledgment with removedFromApprovalList, and the next criterion add for ID1 resurrects it - the state every
    // m2_signing ECLA run leaves behind and what this module finds (and resurrects with its own GitHub-username add) at
    // the start of its next run. A manager-invalidated acknowledgment, by contrast, never comes back.
    const mine = (await h.state()).acks.filter((a) => a.lfLogin === ID1 || a.githubUsername === cfg('ID1_GITHUB')).sort((a, b) => String(b.signedOn || '').localeCompare(String(a.signedOn || '')));
    const newest = mine[0];
    const state = !newest ? 'missing' : h.ackValid(newest) ? 'valid' : newest.removedFromApprovalList === true ? 'removal-voided (resurrects on the next criterion add)' : `invalidated (${newest.invalidationReason || 'other'})`;
    steps.push(`temporary entry removed -> ${r.status}; newest ID1 acknowledgment ${newest ? h.ackId(newest) : '-'} ${state}`);
    ok = ok && (state === 'valid' || state.startsWith('removal-voided'));
  }
  return { ok, note: steps.join('; ') };
}
try {
  const how = await e2e.login(page, ID1, `${e2e.SS}/org/easycla`);
  check('M3-LOGIN', new URL(page.url()).hostname === e2e.SS_HOST, `${ID1} login ${how}`);

  base = await h.state();
  e2e.evidence('before.private.json', base);
  check('M3-BEFORE-STATE', !!base.row?.signed && Object.values(base.http).every((s) => s === 200),
    `row=${JSON.stringify(base.row && { status: base.row.status, claManagersCount: base.row.claManagersCount, approvalCriteriaCount: base.row.approvalCriteriaCount, autoCreateEcla: base.row.autoCreateEcla, viewerIsClaManager: base.row.viewerIsClaManager })} managers=${JSON.stringify(base.managers)} entries=${base.entries.length} acks=${base.acks.length}`);
  check('M3-VIEWER-IS-CLA-MANAGER', base.row?.viewerIsClaManager === true, `viewerIsClaManager=${base.row?.viewerIsClaManager}`);
  expectedEntries = base.entries.filter((x) => !suiteOwned(x.value)).map((x) => `${x.kind}:${x.value}`).sort();
  const id2OnRoster = base.managers.includes(ID2);
  const ownAtBaseline = (o) => base.entries.some((x) => x.kind === o.kind && x.value === o.value);
  info('M3-BASELINE-EXPECTATION', `end state must be: entries=${JSON.stringify(expectedEntries)} managers=${JSON.stringify(base.managers)} auto=${base.row?.autoCreateEcla}; ID2 on roster at start=${id2OnRoster}; own entries at baseline=${JSON.stringify(OWN.filter(ownAtBaseline).map((o) => `${o.kind}:${o.value}`))}`);

  await h.openDetail();
  for (const t of ['overview', 'approval', 'acknowledgments', 'managers', 'activity']) {
    await h.tab(t);
    const alert = e2e.squash(await page.locator('[role="alert"]:visible').allInnerTexts().then((a) => a.join(' ')).catch(() => ''));
    const text = await e2e.bodyText(page);
    check(`M3-TAB-${t.toUpperCase()}`, text.length > 0 && !/something went wrong|failed to load|error loading|unexpected error/i.test(alert + ' ' + text), `tab ${t} renders; alert="${alert.slice(0, 80)}"`);
  }

  if (!WRITES) {
    skip('M3-WRITES', `ALLOW_FIXTURE_WRITES=${cfg('ALLOW_FIXTURE_WRITES')} E2E_NO_WRITES=${e2e.NO_WRITES ? 1 : 0}: approval/auto-ECLA/ack/manager writes skipped`);
  } else {
    // ---- approval list ----
    await h.tab('approval');
    const stale = base.entries.filter((x) => suiteOwned(x.value));
    for (const x of stale) await h.removeEntry(x.value, `M3-P0-STALE-ENTRY-REMOVED-${x.value.replace(/[^a-z0-9]/gi, '').slice(0, 24).toUpperCase()}`, x.kind);
    if (stale.length) info('M3-P0-STALE-ENTRIES', `${stale.length} synthetic entr(ies) from an earlier run removed first: ${JSON.stringify(stale.map((x) => x.value))}`);
    await e2e.shot(page, 'approval-start');
    await h.rejectEntry(/^email domain$/i, 'not a domain', 'M3-APPROVAL-INVALID-DOMAIN');
    await h.addEntry(/^email domain$/i, synthetic, 'M3-APPROVAL-ADD-DOMAIN');
    await h.rejectEntry(/^email domain$/i, synthetic, 'M3-APPROVAL-DUPLICATE', /already/i);
    let added = 1;
    for (const o of OWN) {
      // ID1's own entries are part of the fixture on some environments: only add (and later remove) the ones absent at baseline
      if (ownAtBaseline(o)) info(`M3-APPROVAL-ADD-${o.tag}`, `${o.kind}:${o.value} is on the approval list at baseline - add skipped, kept as is`);
      else { await h.addEntry(o.re, o.value, `M3-APPROVAL-ADD-${o.tag}`); added += 1; }
    }
    const w = h.writesSoFar();
    const domainRow = h.approvalRow(synthetic, 'domain');
    await domainRow.getByTestId('org-easycla-approval-delete').getByRole('button').click();
    await page.getByRole('button', { name: 'Cancel', exact: true }).click();
    await e2e.sleep(1000);
    check('M3-APPROVAL-DELETE-CANCEL', h.writesSoFar() === w && (await domainRow.count()) === 1, `Cancel keeps the row (rows=${await domainRow.count()}) and makes no write (writes=${h.writesSoFar() - w})`);
    const afterAdds = await h.state();
    check('M3-APPROVAL-STATE-NEW-ENTRIES', afterAdds.entries.length === expectedEntries.length + added, `entries=${afterAdds.entries.length} (baseline ${expectedEntries.length} + ${added} added): ${JSON.stringify(afterAdds.entries.map((x) => `${x.kind}:${x.value}`))}`);

    // ---- Auto-ECLA ----
    const acks0 = afterAdds.acks;
    const mine0 = acks0.filter((a) => h.ackOf(a, ID1, cfg('ID1_GITHUB')));
    const validBefore = mine0.some(h.ackValid);
    const invalidatedBefore = mine0.length > 0 && !validBefore;
    await h.toggleAuto(true, 'M3-AUTO-ECLA-ON');
    const acks1 = await h.pollAcks((list) => list.length > acks0.length, invalidatedBefore || validBefore ? 8 : 30);
    e2e.evidence('acks-after-auto-on.private.json', acks1);
    const mine1 = acks1.filter((a) => h.ackOf(a, ID1, cfg('ID1_GITHUB')));
    if (invalidatedBefore) {
      // signatures/service.go: an explicitly invalidated ECLA is left alone, it needs an explicit re-approval
      check('M3-AUTO-ECLA-LEAVES-INVALIDATED-ACK-ALONE', acks1.length === acks0.length && mine1.every((a) => !h.ackValid(a)), `acks before=${acks0.length} after=${acks1.length}; ID1's acks stay invalidated`);
    } else if (validBefore) {
      check('M3-AUTO-ECLA-NO-DUPLICATE-ACK', acks1.length === acks0.length, `ID1 already holds a valid ack; acks before=${acks0.length} after=${acks1.length}`);
    } else {
      check('M3-AUTO-ECLA-CREATES-ACK', mine1.some(h.ackValid), `acks before=${acks0.length} after=${acks1.length}; ID1 ack created=${mine1.some(h.ackValid)}`);
    }
    // back to the baseline value (the fixture may legitimately run with Auto-ECLA on)
    await h.toggleAuto(!!base.row?.autoCreateEcla, 'M3-AUTO-ECLA-RESTORE');

    // ---- acknowledgment invalidation (ID1's own valid ack when there is one, re-acknowledged afterwards) ----
    const acksNow = (await h.state()).acks;
    const valid = acksNow.filter((a) => h.ackOf(a, ID1, cfg('ID1_GITHUB')) && h.ackValid(a));
    const configured = !!(cfg('ID1_UID') && LEGACY && cfg('ID1_GITHUB'));
    const target = valid.length && configured ? await resolveAckCompany(cfg('M3_PROJECT_SFID') || base.row?.pairProjectSfid) : null;
    if (valid.length && !target?.companyId) {
      skip('M3-ACK-INVALIDATE', `ID1 holds a valid acknowledgment ${h.ackId(valid[0])} but ${configured ? `the re-acknowledgment target is not confirmed (${target.note})` : 'ID1_UID / LEGACY_API / ID1_GITHUB are not configured'}, so the suite could not re-acknowledge ID1 afterwards (a manager-invalidated acknowledgment is never resurrected) - probe skipped`);
    } else if (valid.length) {
      try {
        const probe = await h.invalidateAck(h.ackId(valid[0]), 'M3-ACK-INVALIDATE');
        // the dialog's default-on "also remove the matching criteria" may take ID1's BASELINE entries with it (a correct
        // backend behaviour, not fixture drift): put them back through the BFF right away so the end state still matches
        if (probe?.alsoRemove) {
          const left = (await h.state()).entries.map((x) => `${x.kind}:${x.value}`);
          const lost = OWN.filter((o) => ownAtBaseline(o) && !left.includes(`${o.kind}:${o.value}`)).map((o) => ({ kind: o.kind, value: o.value }));
          if (lost.length) {
            const r = await e2e.bff(session.context, 'PUT', h.paths.approval, { add: lost, remove: [] });
            info('M3-ACK-INVALIDATE-RESTORE-BASELINE', `the dialog removed baseline entr(ies) ${JSON.stringify(lost.map((x) => `${x.kind}:${x.value}`))} -> PUT approval-list add -> ${r.status}`);
          }
        }
      } finally {
        // the recovery runs whatever happened after the invalidation attempt (a read in between may throw): nothing in
        // the module's end-state restore brings a manager-invalidated acknowledgment back
        const re = await reacknowledgeId1(target.companyId).catch((err) => ({ ok: false, note: `recovery threw: ${err.message}` }));
        check('M3-ACK-REACK', re.ok, `ID1 re-acknowledged after the probe (${target.note}): ${re.note}`);
      }
    } else {
      info('M3-ACK-INVALIDATE', `no valid ID1 acknowledgment on the fixture (${acksNow.length} acks, ID1's=${acksNow.filter((a) => h.ackOf(a, ID1, cfg('ID1_GITHUB'))).length} all invalidated) - invalidation exercised by m3_chain/interactions`);
      await h.tab('acknowledgments');
      await e2e.shot(page, 'acks');
    }

    // ---- CLA managers ----
    await h.tab('managers');
    await page.getByTestId('org-easycla-managers-row').first().waitFor({ state: 'visible', timeout: 60000 });
    await e2e.sleep(1500);
    await e2e.shot(page, 'managers-start');
    const selfRow = page.getByTestId('org-easycla-managers-row').filter({ hasText: cfg('ID1_EMAIL') });
    const blockedBtn = selfRow.getByTestId('org-easycla-managers-remove-blocked');
    const rosterNow = (await h.state()).managers;
    if (rosterNow.length === 1) check('M3-MANAGERS-SELF-REMOVE-BLOCKED', (await blockedBtn.count()) >= 1, `sole manager: self-remove control blocked=${await blockedBtn.count()}`);
    else info('M3-MANAGERS-SELF-REMOVE-BLOCKED', `roster has ${rosterNow.length} managers (blocked control only shown for the sole manager); blocked=${await blockedBtn.count()} remove=${await selfRow.getByTestId('org-easycla-managers-remove').count()}`);

    const id2Row = () => page.getByTestId('org-easycla-managers-row').filter({ hasText: cfg('ID2_EMAIL') });
    async function removeId2(cid) {
      await id2Row().waitFor({ state: 'visible', timeout: 60000 });
      const removeBtn = id2Row().getByTestId('org-easycla-managers-remove').getByRole('button');
      await removeBtn.waitFor({ state: 'visible', timeout: 60000 });
      const w1 = h.writesSoFar();
      await removeBtn.click();
      await page.getByRole('button', { name: 'Cancel', exact: true }).click();
      await e2e.sleep(800);
      check('M3-MANAGER-REMOVE-CANCEL', h.writesSoFar() === w1, `Cancel in the remove dialog makes no write (writes=${h.writesSoFar() - w1})`);
      await removeBtn.click();
      const [removed] = await Promise.all([h.waitWrite((p) => p.startsWith(h.paths.managers + '/'), 'DELETE'), page.getByRole('button', { name: 'Remove', exact: true }).click()]);
      await e2e.sleep(2000);
      const roster = (await h.state()).managers;
      await e2e.shot(page, cid.toLowerCase());
      check(cid, [200, 204].includes(removed.status()) && !roster.includes(ID2), `DELETE managers/${ID2} -> ${removed.status()}; roster=${JSON.stringify(roster)}`);
    }
    async function addId2(cid) {
      await e2e.closeOverlay(page);
      await page.getByTestId('org-easycla-managers-add').getByRole('button').click();
      const dialog = page.getByTestId('org-easycla-add-manager-dialog');
      const submit = page.getByTestId('org-easycla-add-manager-submit').getByRole('button');
      await dialog.waitFor({ state: 'visible', timeout: 20000 });
      const fields = dialog.locator('input');
      const w0 = h.writesSoFar();
      await submit.click();
      const errs = await Promise.all(['first-name', 'last-name', 'email'].map((f) => e2e.visible(page.getByTestId(`org-easycla-add-manager-${f}-error`), 10000)));
      await fields.nth(0).fill(cfg('ID2_FIRST')); await fields.nth(1).fill(cfg('ID2_LAST')); await fields.nth(2).fill('not-an-email');
      await submit.click();
      const emailErr = await e2e.visible(page.getByTestId('org-easycla-add-manager-email-error'), 10000);
      await e2e.shot(page, 'manager-validation');
      check('M3-MANAGER-VALIDATION', errs.every(Boolean) && emailErr && h.writesSoFar() === w0, `required errors=${JSON.stringify(errs)} email-format error=${emailErr} writes=${h.writesSoFar() - w0}`);
      await fields.nth(2).fill(cfg('ID2_EMAIL'));
      const [added] = await Promise.all([h.waitWrite((p) => p === h.paths.managers, 'POST'), submit.click()]);
      const body = await added.json().catch(() => ({}));
      await e2e.sleep(3000);
      const roster = (await h.state()).managers;
      await e2e.shot(page, cid.toLowerCase());
      check(cid, [200, 201].includes(added.status()) && roster.includes(ID2), `POST managers (${cfg('ID2_EMAIL')}) -> ${added.status()} lfUsername=${body.lfUsername}; roster=${JSON.stringify(roster)}`);
    }
    if (id2OnRoster) { await removeId2('M3-MANAGER-REMOVE'); await addId2('M3-MANAGER-ADD-RESTORED'); }
    else { await addId2('M3-MANAGER-ADD'); await removeId2('M3-MANAGER-REMOVE'); }

    // ---- cleanup of the entries added above (own entries that were at baseline stay) ----
    await h.tab('approval');
    for (const o of [{ kind: 'domain', value: synthetic, tag: 'DOMAIN' }, ...OWN.filter((x) => !ownAtBaseline(x))]) {
      if ((await h.state()).entries.some((x) => x.kind === o.kind && x.value === o.value)) await h.removeEntry(o.value, `M3-APPROVAL-REMOVE-${o.tag}`, o.kind);
    }
  }

  // ---- activity + end state ----
  await h.tab('activity');
  await page.locator('[data-testid^="org-easycla-activity-log"]').first().waitFor({ timeout: 60000 }).catch(() => {});
  await e2e.shot(page, 'activity-end');
  const activity = await h.get(h.paths.activity);
  check('M3-ACTIVITY-LOG', activity.status === 200 && (activity.json?.list || []).length > 0, `GET activity -> ${activity.status}; entries=${(activity.json?.list || []).length}; newest=${JSON.stringify((activity.json?.list || []).slice(0, 3).map((a) => (a.summary || '').slice(0, 80)))}`);
  const end = await h.state();
  e2e.evidence('end.private.json', end);
  const endEntries = end.entries.map((x) => `${x.kind}:${x.value}`).sort();
  if (WRITES) {
    check('M3-END-STATE', JSON.stringify(endEntries) === JSON.stringify(expectedEntries) && JSON.stringify(end.managers) === JSON.stringify(base.managers) && end.row?.autoCreateEcla === base.row?.autoCreateEcla,
      `entries=${JSON.stringify(endEntries)} (expected ${JSON.stringify(expectedEntries)}) managers=${JSON.stringify(end.managers)} (baseline ${JSON.stringify(base.managers)}) auto=${end.row?.autoCreateEcla} (baseline ${base.row?.autoCreateEcla})`);
  }
  check('M3-NO-BLOCKED-WRITES', net.blocked.length === 0, `guard blocked ${net.blocked.length} request(s) ${JSON.stringify(net.blocked.slice(0, 3))}`);
  info('M3-WRITES-MADE', `${net.writes.length} business write(s): ${JSON.stringify([...new Set(net.writes.map((x) => `${x.method} ${x.url.replace(e2e.SS, '').replace(h.prefix, '…')}`))])}`);
} catch (err) {
  check('M3-HARNESS', false, `error: ${err.message} at ${new URL(page.url()).pathname}`);
  await e2e.shot(page, 'error');
} finally {
  // whatever happened above (FAIL, timeout, exception): entries, roster and Auto-ECLA back to the baseline through the BFF
  if (WRITES && base && expectedEntries) {
    // suite-owned here: the synthetic domain and ID1's own entries that were NOT on the list at baseline
    const owned = (x) => suiteOwned(x.value) || OWN.some((o) => o.kind === x.kind && o.value === x.value && !base.entries.some((b) => b.kind === o.kind && b.value === o.value));
    try { await h.reconcile({ entries: expectedEntries, managers: base.managers, manager: id2, auto: !!base.row?.autoCreateEcla, owned }, 'M3-FINAL-RECONCILE'); }
    catch (err) { info('M3-FINAL-RECONCILE', `reconcile failed: ${err.message}`); }
  }
  await e2e.close(session);
}
process.exit(e2e.finish());
