// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT
//
// M3 two-identity chain through the UI (port of the round-2 chain-lgpm/chain-morg scripts):
//   ID1 (CLA manager, Organization lens -> EasyCLA -> agreement detail) adds ID2's e-mail and GitHub
//   username to the approval list, switches Auto-ECLA on and watches for ID2's acknowledgment; when the
//   fixture only holds manager-invalidated acknowledgments for ID2 nothing may be resurrected and ID2
//   acknowledges again explicitly (Me lens -> Sign CLA -> contributor console -> corporate click-through,
//   POST /v2/request-employee-signature); ID1 then invalidates ID2's valid acknowledgment, ID2 sees it
//   invalidated in the Me lens, ID1 removes the criteria (no resurrection), makes ID2 a CLA manager, ID2
//   opens the organization's EasyCLA page (bridge), ID1 removes the role again.
// State-aware like api/interactions.py: suite-owned criteria left by an earlier run are removed first,
// the manager add/remove order follows whether ID2 is on the roster at baseline, every chain step that
// cannot run in the current state is reported as INFO/SKIP, the end state must equal the baseline (ID2's
// acknowledgments are kept as proof - each run leaves one more invalidated acknowledgment behind).
// Writes: the M3 fixture signature only (approval list / auto-ECLA / acknowledgments / managers) plus
// ID2's own prepare-sign and the legacy employee acknowledgment; everything else is guard-blocked.
import * as e2e from './lib/e2e.mjs';
import { m3, fixtureWrites } from './lib/m3.mjs';
import * as signing from './lib/signing.mjs';

const { cfg, check, info, skip, log, sleep } = e2e;
const ID1 = cfg('ID1');
const ID2 = cfg('ID2');
const ID2_EMAIL = cfg('ID2_EMAIL');
const ID2_GH = cfg('ID2_GITHUB');
const fx = { sfid: cfg('M3_ORG_SFID'), name: cfg('M3_ORG_NAME'), group: cfg('M3_CLA_GROUP'), sig: cfg('M3_CCLA') };
const GROUP_NAME = cfg('M3_CLA_GROUP_NAME', cfg('CLA_GROUP_NAME'));
const WRITES = e2e.gate('ALLOW_FIXTURE_WRITES') && !e2e.NO_WRITES;
const id2Ack = (a) => a.lfLogin === ID2 || a.githubUsername === ID2_GH || a.email === ID2_EMAIL || JSON.stringify(a).includes(`"${ID2}"`);

if (!fx.sfid || !fx.sig || !fx.group) { skip('C-FIXTURE', 'M3_ORG_SFID / M3_CLA_GROUP / M3_CCLA not configured'); process.exit(e2e.finish()); }
if (!WRITES) { skip('C-ALL', `ALLOW_FIXTURE_WRITES=${cfg('ALLOW_FIXTURE_WRITES')} E2E_NO_WRITES=${e2e.NO_WRITES ? 1 : 0}: the chain only makes sense with writes`); process.exit(e2e.finish()); }

// ---- ID1 session (manager side) --------------------------------------------------------------------
const s1 = await e2e.launch({ identity: ID1, writes: fixtureWrites(fx.sfid, fx.sig) });
const h = m3(s1, fx);
// ---- ID2 session (contributor side) ----------------------------------------------------------------
const s2 = await e2e.launch({ identity: ID2, writes: signing.signingWrites('ecla') });
const ld2 = e2e.attachLd(s2.page);

let base; let addedManager = false; let targetAck = null; let expectedEntries = null;
const id2 = { lfUsername: ID2, firstName: cfg('ID2_FIRST'), lastName: cfg('ID2_LAST'), email: ID2_EMAIL };
try {
  const how1 = await e2e.login(s1.page, ID1, `${e2e.SS}/org/easycla`);
  check('C-ID1-LOGIN', new URL(s1.page.url()).hostname === e2e.SS_HOST, `${ID1} login ${how1}`);
  const how2 = await e2e.login(s2.page, ID2, `${e2e.SS}/profile/clas`);
  check('C-ID2-LOGIN', new URL(s2.page.url()).hostname === e2e.SS_HOST, `${ID2} login ${how2}`);

  // ---- C0 baseline -------------------------------------------------------------------------------
  base = await h.state();
  e2e.evidence('before.private.json', base);
  check('C0-BASELINE', !!base.row?.signed && Object.values(base.http).every((s) => s === 200), `managers=${JSON.stringify(base.managers)} entries=${base.entries.length} acks=${base.acks.length} auto=${base.row?.autoCreateEcla}`);
  const id2OnRoster = base.managers.includes(ID2);
  const id2Acks0 = base.acks.filter(id2Ack);
  const validBefore = id2Acks0.filter(h.ackValid);
  const me0 = await signing.myAgreements(s2.context, fx.group);
  e2e.evidence('id2-me-clas-before.private.json', me0);
  expectedEntries = base.entries.filter((x) => !(x.value === ID2_EMAIL || x.value === ID2_GH)).map((x) => `${x.kind}:${x.value}`).sort();
  info('C0-STATE', `ID2 on roster=${id2OnRoster}; ID2 acks on the fixture=${id2Acks0.length} (valid=${validBefore.length}); ID2 me/clas rows for the group=${me0.fixture.length} ${JSON.stringify(me0.fixture.map((a) => `${a.kind}:${a.status}:${a.company}`))}`);

  await h.openDetail('C0-OPEN-DETAIL');

  // ---- P0 pre-flight: suite-owned criteria from an earlier run ----------------------------------------
  await h.tab('approval');
  const stale = base.entries.filter((x) => x.value === ID2_EMAIL || x.value === ID2_GH);
  for (const x of stale) await h.removeEntry(x.value, `P0-STALE-${x.kind === 'email' ? 'EMAIL' : 'GITHUB'}-REMOVED`);
  if (stale.length) info('P0-STALE-CRITERIA', `${stale.length} ID2 criteria left by an earlier run removed first`);

  // ---- C1 manager adds ID2's criteria ------------------------------------------------------------------
  await h.addEntry(/^email$/i, ID2_EMAIL, 'C1-ADD-ID2-EMAIL');
  await h.addEntry(/github user/i, ID2_GH, 'C1-ADD-ID2-GITHUB-USERNAME');

  // ---- C2 Auto-ECLA on -> acknowledgment for ID2 ------------------------------------------------------
  await h.toggleAuto(true, 'C2-AUTO-ON');
  const preIds = new Set(base.acks.map(h.ackId));
  const freshValid = (list) => list.filter((a) => id2Ack(a) && h.ackValid(a) && !preIds.has(h.ackId(a)));
  if (validBefore.length) {
    targetAck = validBefore[0];
    info('C2-ACK-PREEXISTING', `${ID2} already holds a valid acknowledgment ${h.ackId(targetAck)} - Auto-ECLA adds no second one; reusing it`);
    const acks = await h.pollAcks(() => false, 3);
    check('C2-AUTO-NO-DUPLICATE', freshValid(acks).length === 0, `acks=${acks.length}, fresh ID2 acks=${freshValid(acks).length}`);
  } else {
    let acks = await h.pollAcks((l) => freshValid(l).length > 0, 20);
    if (!freshValid(acks).length) {
      // creation is triggered by the criterion write while auto is on: re-write the (idempotent) e-mail criterion
      await h.tab('approval');
      await h.removeEntry(ID2_EMAIL, 'C2-REWRITE-EMAIL-REMOVE');
      await h.addEntry(/^email$/i, ID2_EMAIL, 'C2-REWRITE-EMAIL-ADD');
      acks = await h.pollAcks((l) => freshValid(l).length > 0, 20);
    }
    e2e.evidence('acks-after-auto-on.private.json', acks);
    if (freshValid(acks).length) {
      targetAck = freshValid(acks)[0];
      check('C2-AUTO-CREATES-ID2-ACK', true, `new valid acknowledgment ${h.ackId(targetAck)} for ${ID2}`);
    } else if (id2Acks0.length) {
      // design rule: a manager-invalidated acknowledgment is never auto-recreated; ID2 has to acknowledge again
      check('C2-AUTO-NO-RESURRECTION', true, `Auto-ECLA created nothing: ${id2Acks0.length} invalidated acknowledgment(s) of ${ID2} stay invalidated (expected)`);
    } else {
      check('C2-AUTO-CREATES-ID2-ACK', false, `no acknowledgment for ${ID2} appeared although none existed before (acks=${acks.length})`);
    }
  }
  await h.toggleAuto(!!base.row?.autoCreateEcla, 'C2-AUTO-RESTORE'); // back to the baseline value, not blindly off

  // ---- C3 ID2 acknowledges explicitly through the console when needed ------------------------------------
  if (!targetAck) {
    try {
      const hand = await signing.handOffToConsole(s2, ID2, { groupId: fx.group, groupName: GROUP_NAME, githubId: cfg('ID2_GITHUB_ID') });
      check('C3-ID2-CONSOLE-HANDOFF', new URL(hand.url).hostname === new URL(cfg('CONTRIBUTOR_CONSOLE')).hostname, `picker="${hand.picker.slice(0, 80)}" identity="${hand.idText.slice(0, 80)}" -> ${new URL(hand.url).origin} console auth=${hand.auth}`);
      const ack = await signing.corporateAcknowledge(s2, fx.name);
      e2e.evidence('id2-ack-receipt.private.json', ack);
      check('C3-ID2-ECLA-CLICKTHROUGH', ack.status === 200 && !ack.receipt.errors && ack.done, `POST /v2/request-employee-signature -> ${ack.status} signature=${ack.signatureId} errors=${JSON.stringify(ack.receipt.errors || null)} done=${ack.done}`);
      await signing.exitConsole(s2);
      const me1 = await signing.myAgreements(s2.context, fx.group);
      const valid = me1.fixture.filter((a) => a.kind === 'ECLA' && a.status === 'valid');
      check('C3-ID2-MECLAS-VALID', valid.length === 1 && (!ack.signatureId || valid[0].id === ack.signatureId), `valid ECLA rows for the group=${valid.length} ${JSON.stringify(valid)}`);
      const acks = await h.pollAcks((l) => freshValid(l).length > 0, 20);
      targetAck = freshValid(acks)[0] || null;
      check('C3-MANAGER-SEES-ID2-ACK', !!targetAck, `acknowledgments tab data: fresh valid ID2 ack=${targetAck ? h.ackId(targetAck) : 'none'}`);
    } catch (err) {
      check('C3-ID2-EXPLICIT-ACK', false, `error: ${err.message} at ${s2.page.url().slice(0, 120)}`);
      await e2e.shot(s2.page, 'c3-error');
    }
  } else {
    info('C3-ID2-EXPLICIT-ACK', 'not needed: a valid acknowledgment exists');
  }

  // ---- C4 ID2 Me lens shows the acknowledgment as Valid -------------------------------------------------
  if (targetAck) {
    const row = await signing.meLensRow(s2, h.ackId(targetAck));
    await e2e.shot(s2.page, 'id2-me-lens-valid');
    check('C4-ID2-ME-LENS-VALID', /\bvalid\b/i.test(row) && !/invalidated/i.test(row), `agreement-row-${h.ackId(targetAck)}: "${row.slice(0, 120)}"`);
    const mgrs = await e2e.bff(s2.context, 'GET', `/api/me/clas/${h.ackId(targetAck)}/cla-managers`);
    check('C4-ID2-READS-OWN-ACK-MANAGERS', mgrs.status === 200, `GET /api/me/clas/<ack>/cla-managers -> ${mgrs.status}; managers=${(mgrs.json?.managers || mgrs.json?.list || []).length}`);
  } else {
    skip('C4-ID2-ME-LENS-VALID', 'no valid acknowledgment for ID2');
  }

  // ---- C5 manager invalidates, C6 ID2 sees it, C7 criteria removed (no resurrection) ------------------------
  // The invalidate dialog lists the approval entries added specifically for the contributor and (default ON) removes
  // them in a follow-up PUT; whatever it removed is verified in C5B, the rest of ID2's criteria is removed in C7.
  if (targetAck) {
    const inv = await h.invalidateAck(h.ackId(targetAck), 'C5-MANAGER-INVALIDATES');
    const after = inv?.after;
    const matched = inv?.matches || [];
    if (matched.length) {
      check('C5B-DIALOG-LISTS-ID2-CRITERIA', matched.some((m) => m.includes(ID2_EMAIL) || m.includes(ID2_GH)), `dialog matches=${JSON.stringify(matched)} alsoRemove(default)=${inv.alsoRemove}`);
      if (inv.alsoRemove) check('C5B-DIALOG-REMOVED-MATCHES', !!inv.removal && inv.removal.status === 200, `follow-up PUT approval-list -> ${inv.removal ? `${inv.removal.status}; entries=${inv.removal.entries}` : 'not observed'}`);
    } else info('C5B-DIALOG-NO-INDIVIDUAL-MATCH', `dialog found no individual approval entry for ${ID2} (alsoRemove offered=${inv?.alsoRemoveOffered})`);
    const row = await signing.meLensRow(s2, h.ackId(targetAck));
    await e2e.shot(s2.page, 'id2-me-lens-invalidated');
    check('C6-ID2-ME-LENS-INVALIDATED', /invalidated/i.test(row), `agreement-row-${h.ackId(targetAck)}: "${row.slice(0, 120)}"`);
    const me2 = await signing.myAgreements(s2.context, fx.group);
    check('C6-ID2-BFF-INVALIDATED', me2.fixture.find((a) => a.id === h.ackId(targetAck))?.status === 'invalidated', `me/clas status=${me2.fixture.find((a) => a.id === h.ackId(targetAck))?.status}`);
    log(`invalidated ack state: ${h.ackState(after)}`);
  }
  await h.tab('approval');
  const left = ((await h.get(h.paths.approval)).json?.entries || []).map((x) => `${x.kind}:${x.value}`);
  for (const [kind, value, cid] of [['email', ID2_EMAIL, 'C7-REMOVE-ID2-EMAIL'], ['github-username', ID2_GH, 'C7-REMOVE-ID2-GITHUB-USERNAME']]) {
    if (left.includes(`${kind}:${value}`)) await h.removeEntry(value, cid);
    else info(cid, `${kind}:${value} already removed by the invalidate dialog`);
  }
  const rest = ((await h.get(h.paths.approval)).json?.entries || []).filter((x) => x.value === ID2_EMAIL || x.value === ID2_GH);
  check('C7-ID2-CRITERIA-ABSENT', rest.length === 0, `ID2 criteria left on the approval list: ${JSON.stringify(rest)}`);
  if (targetAck) {
    const acks = await h.pollAcks(() => false, 2);
    const still = acks.find((a) => h.ackId(a) === h.ackId(targetAck));
    check('C7-ACK-STAYS-INVALIDATED', !!still && !h.ackValid(still), `ack ${h.ackId(targetAck)} state=${h.ackState(still)} after the criteria removal`);
  }

  // ---- C8 manager role for ID2, C9 bridge as ID2, C10 role removed ----------------------------------------
  await h.tab('managers');
  await s1.page.getByTestId('org-easycla-managers-row').first().waitFor({ state: 'visible', timeout: 60000 });
  await sleep(1500);
  async function addId2(cid) {
    await e2e.closeOverlay(s1.page);
    await s1.page.getByTestId('org-easycla-managers-add').getByRole('button').click();
    const dialog = s1.page.getByTestId('org-easycla-add-manager-dialog');
    await dialog.waitFor({ state: 'visible', timeout: 20000 });
    const fields = dialog.locator('input');
    await fields.nth(0).fill(cfg('ID2_FIRST')); await fields.nth(1).fill(cfg('ID2_LAST')); await fields.nth(2).fill(ID2_EMAIL);
    const [added] = await Promise.all([h.waitWrite((p) => p === h.paths.managers, 'POST'), s1.page.getByTestId('org-easycla-add-manager-submit').getByRole('button').click()]);
    const body = await added.json().catch(() => ({}));
    await sleep(3000);
    const roster = (await h.state()).managers;
    await e2e.shot(s1.page, cid.toLowerCase());
    check(cid, [200, 201].includes(added.status()) && roster.includes(ID2), `POST managers (${ID2_EMAIL}) -> ${added.status()} lfUsername=${body.lfUsername}; roster=${JSON.stringify(roster)}`);
    return roster.includes(ID2);
  }
  async function removeId2(cid) {
    const row = s1.page.getByTestId('org-easycla-managers-row').filter({ hasText: ID2_EMAIL });
    await row.waitFor({ state: 'visible', timeout: 60000 });
    await row.getByTestId('org-easycla-managers-remove').getByRole('button').click();
    const [removed] = await Promise.all([h.waitWrite((p) => p.startsWith(h.paths.managers + '/'), 'DELETE'), s1.page.getByRole('button', { name: 'Remove', exact: true }).click()]);
    await sleep(2000);
    const roster = (await h.state()).managers;
    await e2e.shot(s1.page, cid.toLowerCase());
    check(cid, [200, 204].includes(removed.status()) && !roster.includes(ID2), `DELETE managers/${ID2} -> ${removed.status()}; roster=${JSON.stringify(roster)}`);
  }
  if (id2OnRoster) info('C8-ID2-MANAGER', `${ID2} is on the roster at baseline (kept proof of an earlier run) - bridge checks run against that role`);
  else addedManager = await addId2('C8-MANAGER-ADDS-ID2');

  // ID2 side: role-grants + Organization lens EasyCLA page of the fixture organization. Self Serve reads the ACS authorize
  // cache (~30 min); on dev the cache is flushed right after the grant, on prod we wait at most SS_PROPAGATION_SECS and
  // report INFO/SKIP (not FAIL) when the grant has not reached SS yet - the ACS side is asserted by api/interactions.py W1.
  const budget = addedManager ? Number(cfg('SS_PROPAGATION_SECS', '120')) : 60;
  if (addedManager) info('C8-ACS-CACHE', e2e.flushAcsCache());
  let grants = null; let waitedGrant = 0;
  for (;;) {
    grants = await e2e.bff(s2.context, 'GET', '/api/orgs/me/role-grants');
    if (grants.status === 200 && (grants.json?.claManagerOrgs || []).includes(fx.sfid)) break;
    if (waitedGrant >= budget) break;
    await sleep(10000); waitedGrant += 10;
  }
  e2e.evidence('id2-role-grants.private.json', grants?.json);
  const granted = grants?.status === 200 && (grants.json?.claManagerOrgs || []).includes(fx.sfid);
  if (granted) check('C9-ID2-ROLE-GRANTS-CLA-MANAGER', true, `claManagerOrgs=${JSON.stringify(grants?.json?.claManagerOrgs)} writers=${(grants?.json?.writers || []).length} after ${waitedGrant}s`);
  else if (addedManager && grants?.status === 200) info('C9-ID2-ROLE-GRANTS-CLA-MANAGER', `grant not visible to Self Serve ${waitedGrant}s after the roster add (ACS authorize cache ~30 min; raise SS_PROPAGATION_SECS to wait longer); claManagerOrgs=${JSON.stringify(grants.json?.claManagerOrgs)}`);
  else check('C9-ID2-ROLE-GRANTS-CLA-MANAGER', false, `role-grants -> ${grants?.status}; claManagerOrgs=${JSON.stringify(grants?.json?.claManagerOrgs)} after ${waitedGrant}s`);
  const list2 = await e2e.bff(s2.context, 'GET', h.paths.list);
  const row2 = (list2.json?.claGroups || []).find((r) => r.id === fx.sig);
  if (granted) check('C9-ID2-BFF-LENS-LIST', list2.status === 200 && !!row2 && row2.viewerIsClaManager === true, `GET lens/cla-groups as ${ID2} -> ${list2.status}; fixture row viewerIsClaManager=${row2?.viewerIsClaManager}`);
  else skip('C9-ID2-BFF-LENS-LIST', `grant not yet visible to Self Serve (GET lens/cla-groups -> ${list2.status}, viewerIsClaManager=${row2?.viewerIsClaManager})`);
  await e2e.gotoOrgEasyCla(s2.page, fx.sfid, null);
  const flags2 = ld2.flags();
  const m3Flag = flags2[cfg('LD_FLAG_M3')];
  const p2 = new URL(s2.page.url()).pathname;
  const cards2 = await s2.page.locator('[data-testid="org-easycla-card-link"]').count();
  const nav2 = await s2.page.$$eval('nav a[href], aside a[href]', (as) => [...new Set(as.filter((a) => /^\/org\//.test(a.getAttribute('href') || '')).map((a) => (a.textContent || '').trim().replace(/\s+/g, ' ')))]).catch(() => []);
  const text2 = await e2e.bodyText(s2.page);
  await e2e.shot(s2.page, 'id2-bridge-easycla');
  if (m3Flag === false) skip('C9-ID2-BRIDGE-PAGE', `${cfg('LD_FLAG_M3')}=false for ${ID2}: the CLA-only Organization lens page is not released to this identity; path=${p2}`);
  else if (!granted) skip('C9-ID2-BRIDGE-PAGE', `grant not yet visible to Self Serve; path=${p2} cards=${cards2}`);
  else check('C9-ID2-BRIDGE-PAGE', /\/easycla/.test(p2) && !/not-found/.test(p2) && cards2 > 0, `path=${p2} cards=${cards2} nav=${JSON.stringify(nav2)} flag=${m3Flag} text=${text2.slice(0, 100)}`);
  const writer2 = (grants?.json?.writers || []).some((o) => (typeof o === 'string' ? o : o.uid || o.sfid || o.id) === fx.sfid);
  if (m3Flag !== false && granted && !writer2) check('C9-ID2-EASYCLA-ONLY-NAV', nav2.length >= 1 && nav2.every((t) => /easycla/i.test(t)) && !/Org Admin|Dashboard|Memberships/.test(text2), `CLA manager without org admin role sees only EasyCLA: nav=${JSON.stringify(nav2)}`);
  else if (writer2) info('C9-ID2-EASYCLA-ONLY-NAV', `${ID2} is also a writer of ${fx.sfid}: full sidebar expected; nav=${JSON.stringify(nav2)}`);

  // ID2 leaves the roster when it was not there at baseline - decided from the baseline and the current roster, not
  // from the add's bookkeeping (a C8 add that failed its own check but still landed must be undone too)
  if (!id2OnRoster && (await h.state()).managers.includes(ID2)) {
    await removeId2('C10-MANAGER-REMOVES-ID2');
    info('C10-ACS-CACHE', e2e.flushAcsCache());
    let after = null; let waited = 0;
    for (;;) {
      after = await e2e.bff(s2.context, 'GET', h.paths.list);
      if (after.status !== 200 || !(after.json?.claGroups || []).some((r) => r.id === fx.sig && r.viewerIsClaManager)) break;
      if (waited >= budget) break;
      await sleep(10000); waited += 10;
    }
    const rowA = (after?.json?.claGroups || []).find((r) => r.id === fx.sig);
    if (after?.status !== 200 || !rowA?.viewerIsClaManager) check('C10-ID2-ACCESS-REVOKED', true, `GET lens/cla-groups as ${ID2} -> ${after?.status} viewerIsClaManager=${rowA?.viewerIsClaManager} after ${waited}s`);
    else info('C10-ID2-ACCESS-REVOKED', `${ID2} still reads the fixture as manager ${waited}s after the removal (Self Serve reads the ACS authorize cache, ~30 min on prod; the ACS revoke itself is asserted by api/interactions.py W3C)`);
  }

  // ---- F end state -------------------------------------------------------------------------------------
  const end = await h.state();
  e2e.evidence('end.private.json', end);
  const endEntries = end.entries.map((x) => `${x.kind}:${x.value}`).sort();
  check('F-ENTRIES-BASELINE', JSON.stringify(endEntries) === JSON.stringify(expectedEntries), `entries=${JSON.stringify(endEntries)} expected=${JSON.stringify(expectedEntries)}`);
  check('F-MANAGERS-BASELINE', JSON.stringify(end.managers) === JSON.stringify(base.managers), `managers=${JSON.stringify(end.managers)} baseline=${JSON.stringify(base.managers)}`);
  check('F-AUTO-BASELINE', end.row?.autoCreateEcla === base.row?.autoCreateEcla, `auto=${end.row?.autoCreateEcla} baseline=${base.row?.autoCreateEcla}`);
  if (targetAck) {
    const kept = end.acks.find((a) => h.ackId(a) === h.ackId(targetAck));
    check('F-ID2-ACK-RETAINED-INVALIDATED', !!kept && !h.ackValid(kept), `ack ${h.ackId(targetAck)} kept as proof, state=${h.ackState(kept)}`);
  }
  check('C-NO-BLOCKED-WRITES', s1.net.blocked.length === 0 && s2.net.blocked.filter((b) => !/launchdarkly|analytics|segment|sentry|datadog/i.test(b.url)).length === 0, `blocked ID1=${JSON.stringify(s1.net.blocked.slice(0, 3))} ID2=${JSON.stringify(s2.net.blocked.slice(0, 3))}`);
  check('C-NO-FORBIDDEN-WRITES', ![...s1.net.writes, ...s2.net.writes].some((w) => signing.SIGNING_FORBIDDEN.test(w.url)), `writes ID1=${s1.net.writes.length} ID2=${s2.net.writes.length}`);
  info('C-WRITES-MADE', JSON.stringify([...new Set([...s1.net.writes, ...s2.net.writes].map((w) => `${w.method} ${w.url.replace(/^https?:\/\//, '').replace(h.prefix, '…')}`))]).slice(0, 600));
} catch (err) {
  check('C-HARNESS', false, `error: ${err.message} (ID1 at ${s1.page.url().slice(0, 100)}, ID2 at ${s2.page.url().slice(0, 100)})`);
  await e2e.shot(s1.page, 'id1-error');
  await e2e.shot(s2.page, 'id2-error');
} finally {
  // whatever happened above: ID2's suite-owned entries, the roster (baseline + current roster decide) and Auto-ECLA back
  // to the baseline through the BFF, so the next run starts from the baseline
  if (base && expectedEntries) {
    try { await h.reconcile({ entries: expectedEntries, managers: base.managers, manager: id2, auto: !!base.row?.autoCreateEcla }, 'F-RECONCILE'); }
    catch (e2) { log(`restore failed: ${e2.message}`); }
  }
  await e2e.close(s2);
  await e2e.close(s1);
}
process.exit(e2e.finish());
