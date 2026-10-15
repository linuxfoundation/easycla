// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT
//
// Self Serve identity probe (read-only) for ID1 (roles everywhere) and ID2 (no organization roles):
// login, BFF reads (role-grants, me/clas, personas, nav/org-items), Me lens, organization selector,
// /org/easycla landing and the deep links of the QA organizations (own, other, CLA-only).
import * as e2e from './lib/e2e.mjs';

const { cfg, check, info, skip } = e2e;
const ID1 = cfg('ID1');
const ID2 = cfg('ID2');
const adminOrgs = cfg('ID1_ADMIN_ORG_SFIDS').split(',').map((s) => s.trim()).filter(Boolean);
const orgs = [
  ['M3', cfg('M3_ORG_SFID'), cfg('M3_ORG_NAME')],
  ['CLA-ONLY', cfg('CLA_ONLY_COMPANY_SFID'), cfg('COMPANY_NAME')],
  ['OTHER', cfg('OTHER_ORG_SFID'), cfg('OTHER_ORG_NAME')],
];
const denied = (p, t) => /not-found|access-denied|\/overview$|\/$/.test(p) || /do not have access|not found|no access|no organization/i.test(t);

for (const who of [ID1, ID2]) {
  const session = await e2e.launch({ identity: who });
  const { page, context, net } = session;
  const P = `SS-${who === ID1 ? 'ID1' : 'ID2'}`;
  try {
    const how = await e2e.login(page, who, `${e2e.SS}/profile/clas`);
    check(`${P}-LOGIN`, new URL(page.url()).hostname === e2e.SS_HOST, `${who} login ${how}; path=${new URL(page.url()).pathname}`);

    const grants = await e2e.bff(context, 'GET', '/api/orgs/me/role-grants');
    check(`${P}-BFF-ROLE-GRANTS`, grants.status === 200, `GET /api/orgs/me/role-grants -> ${grants.status}`);
    const g = grants.json || {};
    const writers = (g.writers || []).map((o) => (typeof o === 'string' ? o : o.uid || o.sfid || o.id));
    const claOrgs = g.claManagerOrgs || [];
    e2e.evidence(`${who}-role-grants.private.json`, g);
    info(`${P}-ROLE-GRANTS-SHAPE`, `isStaff=${g.isStaff} writers=${writers.length} readers=${(g.readers || []).length} claManagerOrgs=${JSON.stringify(claOrgs).slice(0, 200)}`);
    if (who === ID1) {
      const missing = adminOrgs.filter((s) => !writers.includes(s));
      check(`${P}-ADMIN-ORGS-ARE-WRITERS`, missing.length === 0, `ID1_ADMIN_ORG_SFIDS in writers: missing=${JSON.stringify(missing)}`);
    } else {
      const leaked = writers.filter((s) => adminOrgs.includes(s) || s === cfg('OTHER_ORG_SFID'));
      check(`${P}-NO-ADMIN-WRITER-GRANTS`, leaked.length === 0, `writers=${JSON.stringify(writers)} (must not contain ID1's admin orgs or OTHER_ORG)`);
      info(`${P}-CLA-MANAGER-ORGS`, `claManagerOrgs=${JSON.stringify(claOrgs).slice(0, 300)} (CLA-only bridge entries; empty when ID2 is not a CLA manager right now)`);
    }

    const clas = await e2e.bff(context, 'GET', '/api/me/clas');
    const agreements = clas.json?.agreements || [];
    check(`${P}-BFF-ME-CLAS`, clas.status === 200, `GET /api/me/clas -> ${clas.status}; agreements=${agreements.length}`);
    e2e.evidence(`${who}-me-clas.private.json`, clas.json ?? clas.text);
    const personas = await e2e.bff(context, 'GET', '/api/user/personas');
    info(`${P}-BFF-PERSONAS`, `GET /api/user/personas -> ${personas.status} ${e2e.squash(personas.text).slice(0, 160)}`);
    const nav = await e2e.bff(context, 'GET', '/api/nav/org-items');
    check(`${P}-BFF-NAV-ORG-ITEMS`, nav.status === 200, `GET /api/nav/org-items -> ${nav.status}`);
    const items = Array.isArray(nav.json) ? nav.json : (nav.json?.items || nav.json?.orgItems || []);
    const claOnly = items.filter((i) => i && (i.claOnly || i.kind === 'cla-only' || i.type === 'claOnly'));
    info(`${P}-NAV-ORG-ITEMS-SHAPE`, `items=${items.length} claOnly=${claOnly.length} names=${JSON.stringify(items.slice(0, 8).map((i) => i?.name || i?.label || i?.uid)).slice(0, 240)}`);
    e2e.evidence(`${who}-nav-org-items.private.json`, nav.json ?? nav.text);

    // Me lens
    await page.goto(`${e2e.SS}/profile/clas`, { waitUntil: 'domcontentloaded', timeout: 60000 });
    const rowSel = '[data-testid^="agreement-row-"]:not([data-testid^="agreement-row-actions-"])';
    await page.locator(`${rowSel}, [data-testid="my-clas-error"], [data-testid^="my-clas-empty"]`).first().waitFor({ timeout: 90000 }).catch(() => {});
    await e2e.hideOverlays(page);
    const rows = await page.locator(rowSel).count();
    const empty = await page.locator('[data-testid^="my-clas-empty"]').count();
    const errorState = await page.locator('[data-testid="my-clas-error"]').count();
    const signBtn = await page.getByTestId('sign-cla-action').count();
    await e2e.shot(page, `${who}-me-clas`);
    check(`${P}-ME-LENS`, errorState === 0 && (rows > 0 || empty > 0), `rows=${rows} empty=${empty > 0} error=${errorState > 0} sign-cla-action=${signBtn}`);
    check(`${P}-ME-LENS-MATCHES-BFF`, rows === agreements.length || (rows === 0 && agreements.length === 0), `UI rows=${rows} vs BFF agreements=${agreements.length}`);

    // home / selector
    await page.goto(`${e2e.SS}/`, { waitUntil: 'domcontentloaded', timeout: 60000 });
    await e2e.sleep(4000);
    await e2e.hideOverlays(page);
    const trigger = page.getByTestId('org-selector');
    let selectorItems = [];
    if (await trigger.count()) {
      await trigger.click().catch(() => {});
      await e2e.sleep(1500);
      selectorItems = (await page.locator('[data-testid^="org-item-"]').allInnerTexts().catch(() => [])).map(e2e.squash).filter(Boolean);
      await e2e.shot(page, `${who}-selector`);
      await page.keyboard.press('Escape').catch(() => {});
    }
    const homeText = await e2e.bodyText(page);
    info(`${P}-HOME-SELECTOR`, `path=${new URL(page.url()).pathname} selector=${(await trigger.count()) > 0} items=${JSON.stringify(selectorItems.slice(0, 10)).slice(0, 240)}`);
    if (who === ID2 && !claOrgs.length) {
      check(`${P}-NO-ORG-LENS-ENTRY`, selectorItems.length === 0 && !/Organization lens/i.test(homeText), `no organization roles -> selector items=${selectorItems.length}`);
    }

    await page.goto(`${e2e.SS}/org/easycla`, { waitUntil: 'domcontentloaded', timeout: 60000 });
    await e2e.sleep(5000);
    await e2e.shot(page, `${who}-org-easycla-landing`);
    info(`${P}-ORG-EASYCLA-LANDING`, `path=${new URL(page.url()).pathname} text=${(await e2e.bodyText(page)).slice(0, 160)}`);

    // deep links
    for (const [key, sfid, name] of orgs) {
      if (!sfid) { skip(`${P}-DEEP-${key}`, 'organization not configured'); continue; }
      const p = await e2e.gotoOrgEasyCla(page, sfid, null);
      const t = await e2e.bodyText(page);
      const cards = await page.locator('[data-testid^="org-easycla-card"], [data-testid="org-easycla-detail-status"], [data-testid="org-easycla-sign-cla"]').count();
      await e2e.shot(page, `${who}-deep-${key.toLowerCase()}`);
      const isDenied = denied(p, t) && cards === 0;
      const note = `${name} /org/${sfid}/easycla -> ${p}; easycla widgets=${cards}; denied=${isDenied}; text=${t.slice(0, 120)}`;
      // ID2 sees exactly the organizations it is a CLA manager of (role-grants.claManagerOrgs), nothing else
      const id2Expected = claOrgs.includes(sfid);
      if (key === 'OTHER') check(`${P}-DEEP-OTHER-DENIED`, isDenied, note);
      else if (who === ID1) check(`${P}-DEEP-${key}-RENDERS`, !isDenied, note);
      else if (id2Expected) check(`${P}-DEEP-${key}-CLA-ONLY-RENDERS`, !isDenied && /EasyCLA/.test(t) && !/Org Admin|Dashboard|Memberships/.test(t), `${note} (CLA manager -> EasyCLA-only view, no admin sidebar)`);
      else check(`${P}-DEEP-${key}-DENIED`, isDenied, `${note} (not a CLA manager of ${name})`);
    }
    const claPath = /\/lens\/cla-groups|\/api\/me\/clas|\/role-grants|\/nav\/org-items|cla-service/;
    const errs = net.api.filter((r) => r.status >= 500);
    const claErrs = errs.filter((r) => claPath.test(r.url));
    check(`${P}-NO-CLA-5XX`, claErrs.length === 0, `${net.api.length} own-host responses, CLA-related 5xx=${JSON.stringify(claErrs.slice(0, 5))}`);
    if (errs.length > claErrs.length) info(`${P}-OTHER-5XX`, `non-CLA 5xx responses (platform noise): ${JSON.stringify([...new Set(errs.filter((r) => !claPath.test(r.url)).map((r) => `${r.status} ${r.url}`))].slice(0, 5))}`);
    check(`${P}-NO-BLOCKED-WRITES`, net.blocked.length === 0, `guard blocked ${net.blocked.length} request(s) ${JSON.stringify(net.blocked.slice(0, 3))}`);
  } catch (err) {
    check(`${P}-HARNESS`, false, `error: ${err.message}`);
    await e2e.shot(page, `${who}-error`);
  } finally {
    await e2e.close(session);
  }
}
process.exit(e2e.finish());
