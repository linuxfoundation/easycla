// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT
//
// Legacy EasyCLA consoles, read-only smoke for both identities: corporate console (v1 on prod, LFX corporate
// console on dev) company dashboard of the fixture company with its signed CLA group, contributor console
// landing, PCC landing (and the v1 PCC on dev). Every non-GET request outside the auth hosts is guard-blocked.
import * as e2e from './lib/e2e.mjs';

const { cfg, check, info, skip, sleep } = e2e;
const ID1 = cfg('ID1');
const ID2 = cfg('ID2');
const CORP = cfg('CORPORATE_CONSOLE').replace(/\/$/, '');
const KIND = cfg('CORPORATE_CONSOLE_KIND', 'v1');
const CONTRIB = cfg('CONTRIBUTOR_CONSOLE').replace(/\/$/, '');
const PCC = cfg('PCC_URL').replace(/\/$/, '');
const PCC_V1 = cfg('PCC_V1_URL').replace(/\/$/, '');
const companies = [
  { name: cfg('COMPANY2_NAME') || cfg('COMPANY_NAME'), sfid: cfg('COMPANY2_SFID') || cfg('COMPANY_SFID'), id: cfg('COMPANY2_ID') || cfg('COMPANY_ID') },
];
const GROUP_NAME = cfg('CLA_GROUP_NAME');
const esc = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

const settle = async (page, ms = 4000) => { await page.waitForLoadState('networkidle', { timeout: 30000 }).catch(() => {}); await sleep(ms); await e2e.hideOverlays(page); };
const has = async (page, re) => re.test(await e2e.bodyText(page));
async function go(page, url, settleMs = 4000) {
  await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 60000 }).catch(() => {});
  await page.waitForURL((u) => !e2e.isAuth0Host(u.hostname), { timeout: 60000 }).catch(() => {});
  await settle(page, settleMs);
}

for (const who of [ID1, ID2]) {
  const P = who === ID1 ? 'CON-ID1' : 'CON-ID2';
  const session = await e2e.launch({ identity: who });
  const { page, net } = session;
  try {
    // ---- corporate console ----
    if (!CORP) skip(`${P}-CORP`, 'CORPORATE_CONSOLE not configured');
    else {
      const how = await e2e.login(page, who, `${CORP}/`);
      await settle(page);
      check(`${P}-CORP-LOGIN`, !e2e.isAuth0Host(new URL(page.url()).hostname), `${who} corporate console (${KIND}) login ${how}; url=${page.url().slice(0, 100)}`);
      for (const co of companies) {
        if (!co.name) continue;
        if (KIND === 'v1') {
          // v1: company picker or last company dashboard; "Projects with Signed CLAs" loads asynchronously
          const link = page.getByText(co.name, { exact: false }).first();
          if (await e2e.visible(link, 15000)) { await link.click().catch(() => {}); await settle(page); }
          else {
            const picker = page.getByRole('link', { name: /companies|select company|company/i }).first();
            if (await picker.count()) { await picker.click().catch(() => {}); await settle(page); }
            const link2 = page.getByText(co.name, { exact: false }).first();
            if (await e2e.visible(link2, 15000)) { await link2.click().catch(() => {}); await settle(page); }
          }
          await page.getByText(/loading signed CLA/i).waitFor({ state: 'hidden', timeout: 90000 }).catch(() => {});
          await settle(page, 2000);
          await e2e.shot(page, `${who}-corp-company`);
          const text = await e2e.bodyText(page);
          const shown = new RegExp(esc(co.name), 'i').test(text);
          const groupShown = GROUP_NAME ? new RegExp(esc(GROUP_NAME), 'i').test(text) : true;
          const isManager = shown && groupShown;
          // the v1 console reads the company ACL directly: a CLA manager of the fixture company sees its dashboard
          if (who === ID1) check(`${P}-CORP-COMPANY-${co.sfid}`, isManager, `company "${co.name}" shown=${shown}; CLA group "${GROUP_NAME}" shown=${groupShown}; url=${new URL(page.url()).pathname}`);
          else info(`${P}-CORP-COMPANY-${co.sfid}`, `company shown=${shown} group shown=${groupShown} (depends on whether ${ID2} is a CLA manager right now); text=${text.slice(0, 120)}`);
          const groupLink = page.getByText(GROUP_NAME, { exact: false }).first();
          if (GROUP_NAME && (await e2e.visible(groupLink, 5000))) {
            await groupLink.click().catch(() => {});
            await settle(page);
            await e2e.shot(page, `${who}-corp-group`);
            info(`${P}-CORP-GROUP-DETAIL-${co.sfid}`, `managers=${await has(page, /cla manager/i)} approved list=${await has(page, /approved list|approval list|allow list|whitelist/i)} url=${new URL(page.url()).pathname}`);
          }
        } else {
          // LFX corporate console (dev): the foundation CLA page lists the CLA groups / signed CCLAs of the user's companies
          await go(page, `${CORP}/foundation/${cfg('CORPORATE_FOUNDATION_SFID')}/cla`, 6000);
          await e2e.shot(page, `${who}-corp-foundation-cla`);
          const text = await e2e.bodyText(page);
          const shown = new RegExp(esc(co.name), 'i').test(text) || (GROUP_NAME && new RegExp(esc(GROUP_NAME), 'i').test(text));
          const noCrash = !/something went wrong|unexpected error/i.test(text) && !e2e.isAuth0Host(new URL(page.url()).hostname);
          if (who === ID1) check(`${P}-CORP-FOUNDATION-CLA`, noCrash && text.length > 0, `foundation CLA page rendered; company/group named=${shown}; url=${new URL(page.url()).pathname}; text=${text.slice(0, 120)}`);
          else info(`${P}-CORP-FOUNDATION-CLA`, `rendered=${noCrash} company/group named=${shown}; text=${text.slice(0, 120)}`);
        }
      }
    }
    // ---- contributor console landing ----
    if (!CONTRIB) skip(`${P}-CONTRIB-LANDING`, 'CONTRIBUTOR_CONSOLE not configured');
    else {
      await go(page, `${CONTRIB}/`, 3000);
      await e2e.shot(page, `${who}-contrib-landing`);
      const text = await e2e.bodyText(page);
      check(`${P}-CONTRIB-LANDING`, text.length > 0 && !/something went wrong|unexpected error/i.test(text), `url=${page.url().slice(0, 100)} title="${await page.title().catch(() => '')}" text=${text.slice(0, 100)}`);
    }
    // ---- PCC ----
    if (!PCC) skip(`${P}-PCC-LANDING`, 'PCC_URL not configured');
    else {
      await go(page, `${PCC}/`, 4000);
      await e2e.shot(page, `${who}-pcc-landing`);
      const text = await e2e.bodyText(page);
      check(`${P}-PCC-LANDING`, !e2e.isAuth0Host(new URL(page.url()).hostname) && !/something went wrong|unexpected error/i.test(text), `url=${page.url().slice(0, 100)} text=${text.slice(0, 120)}`);
      if (cfg('PROJECT_SFID')) {
        await go(page, `${PCC}/project/${cfg('PROJECT_SFID')}/cla`, 6000);
        await e2e.shot(page, `${who}-pcc-project-cla`);
        const t2 = await e2e.bodyText(page);
        info(`${P}-PCC-PROJECT-CLA`, `url=${new URL(page.url()).pathname} group "${GROUP_NAME}" shown=${GROUP_NAME ? new RegExp(esc(GROUP_NAME), 'i').test(t2) : 'n/a'} text=${t2.slice(0, 120)}`);
      }
    }
    if (PCC_V1) {
      const proj = cfg('CORPORATE_FOUNDATION_SFID') || cfg('PROJECT_SFID');
      await go(page, `${PCC_V1}/project/${proj}/tools/easycla`, 6000);
      await e2e.shot(page, `${who}-pcc-v1-easycla`);
      const text = await e2e.bodyText(page);
      // the v1 PCC shows an empty shell (header only) to a user without any project role - expected for ID2, a crash for ID1
      const shell = text.length === 0 && who === ID2;
      check(`${P}-PCC-V1-EASYCLA`, !e2e.isAuth0Host(new URL(page.url()).hostname) && !/something went wrong|unexpected error/i.test(text) && (text.length > 0 || shell),
        `url=${page.url().slice(0, 100)} ${shell ? 'empty shell (no project role for this identity - expected)' : `text=${text.slice(0, 120)}`}`);
    }
    check(`${P}-NO-BLOCKED-WRITES`, net.blocked.length === 0, `guard blocked ${net.blocked.length} request(s) ${JSON.stringify(net.blocked.slice(0, 3))}`);
    const errs = net.api.filter((r) => r.status >= 500);
    check(`${P}-NO-5XX`, errs.length === 0, `own-host responses=${net.api.length}; 5xx=${JSON.stringify(errs.slice(0, 4))}`);
  } catch (err) {
    check(`${P}-HARNESS`, false, `error: ${err.message} at ${page.url().slice(0, 120)}`);
    await e2e.shot(page, `${who}-error`);
  } finally {
    await e2e.close(session);
  }
}
process.exit(e2e.finish());
