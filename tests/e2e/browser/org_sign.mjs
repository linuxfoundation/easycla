// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT
//
// M3 Organization lens CCLA signing as ID1 (org admin of M3_ORG), dev only (ALLOW_DOCUSIGN=1, DocuSign sandbox):
//   Organization -> EasyCLA -> Sign CLA -> ORGSIGN_GROUP picker -> (CLA-manager question -> designee) -> attestation
//   -> BFF POST lens/cla-groups/sign -> DocuSign sandbox -> /v2/return-url -> back on the SS group page
//   -> lens row "signed", ID1 on the CLA managers roster (stream lambda grants cla-manager).
// A CCLA is signed once per organization x CLA group: when the fixture row is already signed the module verifies that
// end state and reports INFO (point ORGSIGN_GROUP at an unsigned group of M3_ORG to exercise DocuSign again; the group's
// project must still exist in the project service, otherwise the designee step is refused - ORGSIGN-DESIGNEE).
import * as e2e from './lib/e2e.mjs';
import * as signing from './lib/signing.mjs';

const { cfg, check, info, skip, log, sleep } = e2e;
const ID1 = cfg('ID1');
const SFID = cfg('ORGSIGN_ORG_SFID', cfg('M3_ORG_SFID'));
const NAME = cfg('ORGSIGN_ORG_NAME', cfg('M3_ORG_NAME'));
const GROUP = cfg('ORGSIGN_GROUP');
const SEARCH = cfg('ORGSIGN_GROUP_SEARCH');
const LIST = `/api/orgs/${SFID}/lens/cla-groups`;
const DOCUSIGN = e2e.ENV === 'dev' && e2e.gate('ALLOW_DOCUSIGN') && !e2e.NO_WRITES;

if (e2e.ENV !== 'dev') { skip('ORGSIGN', `organization CCLA signing runs on dev only (env=${e2e.ENV})`); process.exit(e2e.finish()); }
if (!GROUP || !SFID) { skip('ORGSIGN', 'ORGSIGN_GROUP / M3_ORG_SFID not configured'); process.exit(e2e.finish()); }

const session = await e2e.launch({
  identity: ID1,
  allowDocusign: DOCUSIGN,
  writes: [new RegExp(`^POST ${e2e.SS_HOST}${LIST}/(sign|designee)$`)],
});
const { page, context, net } = session;
const bodies = [];
page.on('response', async (r) => {
  if (/lens\/cla-groups\/(sign|sign-options|designee|ccla-preview)|request-corporate-signature|\/v2\/return-url\//.test(r.url())) {
    const b = r.request().method() === 'GET' && /return-url/.test(r.url()) ? '' : await r.text().catch(() => '');
    bodies.push({ status: r.status(), method: r.request().method(), url: r.url().replace(e2e.SS, ''), body: e2e.redact(b, 500) });
  }
});
const rowOf = (list) => (list.json?.claGroups || []).find((r) => r.claGroupId === GROUP) || null;
const hide = () => e2e.hideOverlays(page);
class Stop extends Error {} // thrown after a recorded FAIL when the flow cannot continue (no extra HARNESS failure)
try {
  const how = await e2e.login(page, ID1, `${e2e.SS}/org/easycla`);
  check('ORGSIGN-LOGIN', new URL(page.url()).hostname === e2e.SS_HOST, `${ID1} login ${how}`);
  await e2e.selectOrgCookie(context, SFID);
  await page.goto(`${e2e.SS}/org/easycla`, { waitUntil: 'domcontentloaded', timeout: 60000 });
  const selected = await e2e.ensureOrgSelected(page, SFID, NAME);
  await page.keyboard.press('Escape').catch(() => {});
  await hide();
  check('ORGSIGN-ORG-SELECTED', !!selected, `organization=${selected || NAME} (${SFID})`);

  const before = await e2e.bff(context, 'GET', LIST);
  const row0 = rowOf(before);
  e2e.evidence('list-before.private.json', before.json);
  check('ORGSIGN-LENS-LIST', before.status === 200, `GET ${LIST} -> ${before.status}; rows=${(before.json?.claGroups || []).length}; fixture row=${row0 ? `${row0.id} status=${row0.status}` : 'none'}`);

  if (row0 && (row0.signed || /signed/i.test(row0.status || ''))) {
    info('ORGSIGN-ALREADY-SIGNED', `${NAME} x ${GROUP} already signed (signature ${row0.id}); a CCLA is signed once - set ORGSIGN_GROUP to an unsigned group to exercise the DocuSign flow`);
    // the list is paginated (8 cards per page): filter through the search box first, then page with "Next"
    const card = page.locator(`[data-testid="org-easycla-card-link"][href*="${GROUP}"][href*="sig=${row0.id}"], [data-testid="org-easycla-card-link"][href*="${GROUP}"]:not([href*="sig="])`).first();
    await page.locator('[data-testid="org-easycla-card-link"]').first().waitFor({ state: 'visible', timeout: 60000 }).catch(() => {});
    let seen = await e2e.visible(card, 10000);
    const search = page.locator('input#org-easycla-search-input, input[placeholder*="Search CLA"]').first();
    const term = SEARCH || row0.claGroupName || '';
    if (!seen && term && (await search.count())) { await search.fill(term); await sleep(3000); seen = await e2e.visible(card, 15000); }
    for (let i = 0; i < 10 && !seen; i++) {
      const next = page.getByTestId('org-easycla-next-page').getByRole('button');
      if (!(await next.count()) || !(await next.isEnabled().catch(() => false))) break;
      await next.click(); await sleep(2500);
      seen = await e2e.visible(card, 10000);
    }
    if (seen) {
      await card.click();
      await page.getByTestId('org-easycla-detail-status').waitFor({ timeout: 60000 });
      const status = e2e.squash(await page.getByTestId('org-easycla-detail-status').innerText());
      await e2e.shot(page, 'signed-detail');
      check('ORGSIGN-DETAIL-SIGNED', /signed/i.test(status), `detail status="${status}" url=${new URL(page.url()).pathname}`);
    } else { await e2e.shot(page, 'signed-card-missing'); check('ORGSIGN-DETAIL-SIGNED', false, `signed group card not rendered on the EasyCLA page (search "${term}", ${(before.json?.claGroups || []).length} rows)`); }
    const managers = await e2e.bff(context, 'GET', `${LIST}/${row0.id}/managers`);
    const roster = (managers.json?.managers || []).map((m) => m.lfUsername);
    check('ORGSIGN-ACL-HAS-ID1', managers.status === 200 && roster.includes(ID1), `GET managers -> ${managers.status}; roster=${JSON.stringify(roster)}`);
  } else if (!DOCUSIGN) {
    skip('ORGSIGN-FLOW', 'ALLOW_DOCUSIGN=0 / --no-writes: the unsigned fixture is left untouched');
  } else {
    // ---- picker ----
    await page.getByTestId('org-easycla-sign-cla').locator('button:not([disabled])').first().waitFor({ timeout: 60000 });
    await page.getByTestId('org-easycla-sign-cla').locator('button').first().click();
    await page.getByTestId('org-easycla-group-select-dialog').waitFor({ timeout: 30000 });
    await page.locator('#org-easycla-group-select-search-input').fill(SEARCH || GROUP);
    await sleep(4000);
    const options = [];
    let pick = null;
    for (const o of await page.locator('[data-testid^="org-easycla-group-select-"][role="option"]').all()) {
      const id = ((await o.getAttribute('data-testid')) || '').replace('org-easycla-group-select-', '');
      const disabled = (await o.getAttribute('aria-disabled')) === 'true';
      options.push({ id, disabled, text: e2e.squash(await o.innerText()).slice(0, 90) });
      if (id === GROUP && !disabled) pick = o;
    }
    await e2e.shot(page, 'picker');
    check('ORGSIGN-PICKER-LISTS-GROUP', !!pick, `search "${SEARCH || GROUP}" options=${JSON.stringify(options)}`);
    if (!pick) throw new Error('ORGSIGN_GROUP is not a selectable picker option');
    await pick.click();
    await page.getByTestId('org-easycla-group-continue').click();
    await sleep(6000);
    await hide();
    const notStarted = await page.getByTestId('org-easycla-detail-not-started').count();
    check('ORGSIGN-NOT-STARTED-VIEW', new URL(page.url()).pathname.endsWith(`/easycla/${GROUP}`) && notStarted === 1, `url=${new URL(page.url()).pathname} not-started view=${notStarted}`);
    await e2e.shot(page, 'unsigned-overview');

    // ---- start -> (manager question) -> attestation -> hand-off ----
    await page.getByTestId('org-easycla-detail-start-cla').waitFor({ timeout: 20000 });
    await page.getByTestId('org-easycla-detail-start-cla').click();
    await Promise.race([
      page.getByTestId('org-easycla-attestation-dialog').waitFor({ timeout: 20000 }),
      page.getByTestId('org-easycla-manager-question-dialog').waitFor({ timeout: 20000 }),
    ]);
    if (await page.getByTestId('org-easycla-manager-question-dialog').count()) {
      await e2e.shot(page, 'manager-question');
      const [designee] = await Promise.all([
        page.waitForResponse((r) => /lens\/cla-groups\/designee$/.test(r.url()) && r.request().method() === 'POST', { timeout: 45000 }).catch(() => null),
        page.getByTestId('org-easycla-manager-question-yes').locator('button').first().click().catch(() => page.getByTestId('org-easycla-manager-question-yes').click()),
      ]);
      info('ORGSIGN-MANAGER-QUESTION', `"Are you authorized to be a CLA Manager?" answered Yes -> POST designee ${designee ? designee.status() : 'not observed'}`);
      if (!designee || designee.status() >= 400) {
        // the BFF relabels CLA-service refusals (409 already-signed, 400 no-lf-login, 403 sanctioned/not-authorized) and
        // answers 502 "unknown" for anything else - e.g. a CLA group whose project SFID the project service no longer knows
        const body = designee ? e2e.squash(await designee.text().catch(() => '')) : '';
        check('ORGSIGN-DESIGNEE', false, `POST ${LIST}/designee -> ${designee ? designee.status() : 'not observed'} ${body.slice(0, 160)}; ${ID1} cannot become the initial CLA manager for ${GROUP} - probe POST /v4/company/<companyId>/project/<projectSfid>/cla-manager-designee for the reason or point ORGSIGN_GROUP at another unsigned group`);
        throw new Stop('designee refused');
      }
      await page.getByTestId('org-easycla-attestation-dialog').waitFor({ timeout: 45000 });
    } else info('ORGSIGN-MANAGER-QUESTION', 'not shown (viewer already holds a CLA role on the project)');
    for (const id of ['org-easycla-attestation-authority', 'org-easycla-attestation-embargo']) {
      const box = page.getByTestId(id);
      await box.locator('input[type="checkbox"]').first().check({ force: true }).catch(async () => box.click());
    }
    await sleep(500);
    await e2e.shot(page, 'attestation');
    const [signRes] = await Promise.all([
      page.waitForResponse((r) => /lens\/cla-groups\/sign$/.test(r.url()) && r.request().method() === 'POST', { timeout: 90000 }).catch(() => null),
      page.getByTestId('org-easycla-attestation-continue').click(),
    ]);
    await Promise.race([
      page.getByTestId('org-easycla-sign-ready').waitFor({ timeout: 60000 }),
      page.getByTestId('org-easycla-sign-failed').waitFor({ timeout: 60000 }),
    ]).catch(() => {});
    await sleep(1500);
    await e2e.shot(page, 'handoff');
    const failed = await page.getByTestId('org-easycla-sign-failed').count();
    const ready = await page.getByTestId('org-easycla-sign-ready').count();
    const failMsg = failed ? e2e.squash(await page.getByTestId('org-easycla-sign-failure-message').innerText().catch(() => '')) : '';
    const signBody = signRes ? await signRes.json().catch(() => ({})) : {};
    let signHost = '';
    try { signHost = new URL(signBody.signUrl || '').hostname; } catch { /* no url */ }
    check('ORGSIGN-BFF-SIGN', !!signRes && signRes.status() === 200 && !!signBody.signUrl, `POST ${LIST}/sign -> ${signRes ? signRes.status() : 'not observed'}; signature=${signBody.signatureId || signBody.signatureID || '?'} signHost=${signHost}`);
    check('ORGSIGN-HANDOFF-READY', ready === 1 && failed === 0, `ready=${ready} failed=${failed} msg="${failMsg.slice(0, 300)}"`);
    if (ready !== 1) throw new Error(`signing hand-off never became ready: ${failMsg}`);

    // ---- DocuSign sandbox -> return-url -> SS ----
    await page.getByTestId('org-easycla-sign-review').click();
    const ds = await signing.docusignSign(page, ['Director', NAME, '123 Test Street, Test City', 'TS 00000, United States']);
    check('ORGSIGN-DOCUSIGN-SANDBOX', ds.signed && ds.finished, `sandbox envelope: fields filled=${ds.filled} signed=${ds.signed} finished=${ds.finished} -> ${ds.finalUrl.slice(0, 90)}`);
    await page.waitForURL((u) => u.hostname === e2e.SS_HOST, { timeout: 90000 }).catch(() => {});
    await sleep(8000);
    await hide();
    await e2e.shot(page, 'returned');
    const returnHop = bodies.find((b) => /\/v2\/return-url\//.test(b.url));
    check('ORGSIGN-RETURNED-TO-SS', new URL(page.url()).hostname === e2e.SS_HOST && /\/easycla/.test(page.url()), `return-url hop=${returnHop ? returnHop.status : 'not observed'}; landed on ${page.url().slice(0, 120)}`);

    // ---- end state: lens row signed, ID1 on the roster (stream lambda) ----
    let after = null; let rowA = null; let waited = 0;
    for (; waited < 180; waited += 10) {
      after = await e2e.bff(context, 'GET', LIST);
      rowA = rowOf(after);
      if (rowA && (rowA.signed || /signed/i.test(rowA.status || ''))) break;
      await sleep(10000);
    }
    e2e.evidence('list-after.private.json', after?.json);
    check('ORGSIGN-ROW-SIGNED', !!rowA && (rowA.signed || /signed/i.test(rowA.status || '')), `fixture row after ${waited}s: ${rowA ? `${rowA.id} status=${rowA.status}` : 'missing'}`);
    if (rowA) {
      let roster = []; let mWaited = 0; let mStatus = 0;
      for (; mWaited < 120; mWaited += 10) {
        const managers = await e2e.bff(context, 'GET', `${LIST}/${rowA.id}/managers`);
        mStatus = managers.status;
        roster = (managers.json?.managers || []).map((m) => m.lfUsername);
        if (roster.includes(ID1)) break;
        await sleep(10000);
      }
      check('ORGSIGN-ACL-HAS-ID1', mStatus === 200 && roster.includes(ID1), `GET managers -> ${mStatus} after ${mWaited}s; roster=${JSON.stringify(roster)}`);
      const detail = await e2e.bff(context, 'GET', `${LIST}/${rowA.id}/managers`);
      log(`signed signature ${rowA.id}; managers http=${detail.status}`);
    }
  }
  e2e.evidence('sign-traffic.private.json', bodies);
  check('ORGSIGN-NO-FORBIDDEN-WRITES', !net.writes.some((w) => signing.SIGNING_FORBIDDEN.test(w.url)), `writes=${JSON.stringify([...new Set(net.writes.map((w) => `${w.method} ${w.url.replace(/^https?:\/\//, '')}`))]).slice(0, 400)}`);
  const blocked = net.blocked.filter((b) => !/telemetry|analytics|segment|sentry|datadog|launchdarkly/i.test(b.url));
  check('ORGSIGN-NO-BLOCKED-WRITES', blocked.length === 0, `guard blocked ${blocked.length} request(s) ${JSON.stringify(blocked.slice(0, 3))}`);
} catch (err) {
  if (!(err instanceof Stop)) check('ORGSIGN-HARNESS', false, `error: ${err.message} at ${page.url().slice(0, 120)}`);
  await e2e.shot(page, 'error');
} finally {
  await e2e.close(session);
}
process.exit(e2e.finish());
