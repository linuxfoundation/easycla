// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT
//
// Organization lens <-> EasyCLA bridge cases (Ahmed's list B1-B13, plus lfx-self-serve#3631) for both identities,
// read-only except B2 (one synthetic approval-list domain added and removed by ID2 on the CLA-only fixture).
// ID1 = org admin + CLA manager (B9), ID2 = CLA manager without organization roles (B1-B8, B12) when it holds the
// role, otherwise the "no roles" cases. Every expectation is derived from the live /api/orgs/me/role-grants so the
// module is correct whatever the current role state is; UI cases are SKIPPED for an identity whose LD flag is off.
//   B1  CLA manager without org admin opens the company EasyCLA tab and sees nothing else of the organization
//   B2  ... can add and remove approval-list entries
//   B3  CLA manager of several companies switches between them and sees only the selected company's agreements
//   B4  ... cannot open any other page of the company organization view (overview/people/memberships/projects)
//   B5  ... cannot open another company's organization view
//   B6  (removal propagation <= 30 min) covered by api/interactions.py W3/W4 and the m3_chain C10 step
//   B7  company on an old / odd id gets a limited page, not an error
//   B8  non-member company gets an EasyCLA-only page
//   B9  org admin who is also a CLA manager sees the organization once with full access
//   B10 (#3631) org-lens writer without an EasyCLA ACS scope opens Organization -> EasyCLA: no crash, clear state
//   B11 CLA signatory / designee only -> no Organization lens entry (API: identity_boundary; here: ID2 when applicable)
//   B12 user with no organization or CLA role has no Organization lens
//   B13 org admin of some organizations and only a CLA manager of another one: the selector lists the admin
//       organizations (full sidebar), the CLA-managed company is not a CLA-only entry (lfx-self-serve
//       org-cla-only-orgs.service.ts claOnlyEntryApplies); its deep link must not crash
//   B-LD LD kill switch: when the M3 flag is off the EasyCLA tab is not served (observed, not forced)
import * as e2e from './lib/e2e.mjs';
import { m3, fixtureWrites } from './lib/m3.mjs';

const { cfg, check, info, skip, sleep } = e2e;
const ID1 = cfg('ID1');
const ID2 = cfg('ID2');
const WRITES = e2e.gate('ALLOW_FIXTURE_WRITES') && !e2e.NO_WRITES;
const claOnly = { sfid: cfg('CLA_ONLY_COMPANY_SFID'), name: cfg('COMPANY_NAME'), sig: cfg('CLA_ONLY_COMPANY_CCLA'), group: cfg('CLA_GROUP') };
const m3Org = { sfid: cfg('M3_ORG_SFID'), name: cfg('M3_ORG_NAME'), sig: cfg('M3_CCLA'), group: cfg('M3_CLA_GROUP') };
const other = { sfid: cfg('OTHER_ORG_SFID'), name: cfg('OTHER_ORG_NAME') };
const adminOrgs = cfg('ID1_ADMIN_ORG_SFIDS').split(',').map((s) => s.trim()).filter(Boolean);
const uid = (o) => (typeof o === 'string' ? o : o?.uid || o?.sfid || o?.id);
const navLinks = (page) => page.$$eval('nav a[href], aside a[href]', (as) => [...new Set(as.filter((a) => /^\/org\//.test(a.getAttribute('href') || '')).map((a) => (a.getAttribute('href') || '') + ' :: ' + (a.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 40)))]).catch(() => []);
const adminWords = /Org Admin|Dashboard|Memberships|People/;
const deniedPath = (p) => /not-found|access-denied/.test(p) || p === '/' || p === '/org/overview';
// Self Serve sometimes keeps a no-role identity on /org/<sfid>/overview and renders the unbound overview shell
// ("Select Organization", zero data) instead of redirecting: denied as long as that company's name is not rendered
const deniedOverview = (w, name) => deniedPath(w.path) || (/^\/org\/[^/]+\/overview$/.test(w.path) && !!name && !w.text.toLowerCase().includes(name.toLowerCase()));

async function visit(page, p, shotName, settle = 7000) {
  await page.goto(`${e2e.SS}${p}`, { waitUntil: 'domcontentloaded', timeout: 60000 }).catch(() => {});
  await sleep(settle);
  await e2e.hideOverlays(page);
  await e2e.closeOverlay(page);
  const out = {
    requested: p,
    path: new URL(page.url()).pathname,
    cards: await page.locator('[data-testid="org-easycla-card-link"]').count(),
    nav: await navLinks(page),
    text: await e2e.bodyText(page),
    alerts: e2e.squash(await page.locator('[role="alert"]:visible').allInnerTexts().then((a) => a.join(' | ')).catch(() => '')),
  };
  await e2e.shot(page, shotName);
  return out;
}
const summary = (v) => `${v.requested} -> ${v.path}; cards=${v.cards}; nav=${JSON.stringify(v.nav.map((n) => n.split(' :: ')[1]))}; text=${v.text.slice(0, 90)}`;
const crashed = (v) => /something went wrong|unexpected error|internal server error|cannot read propert|TypeError/i.test(v.text + ' ' + v.alerts);

for (const who of [ID1, ID2]) {
  const P = who === ID1 ? 'B-ID1' : 'B-ID2';
  const session = await e2e.launch({ identity: who, writes: who === ID2 && WRITES && claOnly.sfid ? fixtureWrites(claOnly.sfid, claOnly.sig) : [] });
  const { page, context, net } = session;
  const ld = e2e.attachLd(page);
  try {
    const how = await e2e.login(page, who, `${e2e.SS}/`);
    check(`${P}-LOGIN`, new URL(page.url()).hostname === e2e.SS_HOST, `${who} login ${how}`);
    await sleep(3000);
    const flag = ld.flags()[cfg('LD_FLAG_M3')];
    const grants = await e2e.bff(context, 'GET', '/api/orgs/me/role-grants');
    const g = grants.json || {};
    const writers = (g.writers || []).map(uid);
    const claOrgs = (g.claManagerOrgs || []).map(uid);
    e2e.evidence(`${who}-role-grants.private.json`, g);
    check(`${P}-ROLE-GRANTS`, grants.status === 200, `writers=${JSON.stringify(writers)} claManagerOrgs=${JSON.stringify(claOrgs)} ${cfg('LD_FLAG_M3')}=${flag}`);
    const uiOk = flag !== false;
    if (!uiOk) info(`${P}-LD-FLAG-OFF`, `${cfg('LD_FLAG_M3')}=false for ${who}: Organization lens EasyCLA UI cases are SKIPPED (B13 kill switch observed below)`);

    // organizations where this identity is a CLA manager but NOT a writer -> CLA-only bridge entries
    const claOnlyOrgs = claOrgs.filter((s) => !writers.includes(s));
    const adminAndCla = claOrgs.filter((s) => writers.includes(s));
    const nav = await e2e.bff(context, 'GET', '/api/nav/org-items');
    const items = Array.isArray(nav.json) ? nav.json : (nav.json?.items || nav.json?.orgItems || []);
    const itemIds = items.map((i) => uid(i) || i?.sfid || i?.id).filter(Boolean);
    const claOnlyItems = items.filter((i) => i && (i.claOnly || i.kind === 'cla-only' || i.type === 'claOnly'));
    e2e.evidence(`${who}-nav-org-items.private.json`, nav.json ?? nav.text);
    check(`${P}-NAV-ITEMS-MATCH-GRANTS`, nav.status === 200 && claOnlyOrgs.every((s) => itemIds.includes(s)) && writers.every((s) => itemIds.includes(s)),
      `nav/org-items=${items.length} (claOnly=${claOnlyItems.length}); every writer org and every CLA-only org is listed: missing=${JSON.stringify([...claOnlyOrgs, ...writers].filter((s) => !itemIds.includes(s)))}`);

    if (who === ID2) {
      check(`${P}-NO-ADMIN-WRITER-GRANTS`, !writers.some((s) => adminOrgs.includes(s) || s === other.sfid), `ID2 writers=${JSON.stringify(writers)} must not include ID1's admin orgs / OTHER_ORG`);
      if (!claOrgs.length && !writers.length) {
        // B12: nothing at all. Self Serve offers the Organization lens tab to everybody (lens.utils deriveAllowedLenses
        // always appends 'org'); a user without grants lands on the empty state (/org/not-found) with no organization
        // selector (the selector slot stays data-visible=false) and no EasyCLA cards anywhere.
        await visit(page, '/', 'id2-home');
        const lens = await visit(page, '/org', 'id2-org-lens');
        const slot = page.getByTestId('org-selector-slot').first();
        const slotVisible = (await slot.count()) ? await slot.getAttribute('data-visible') : 'absent';
        const trigger = await page.getByTestId('org-selector').first().isVisible().catch(() => false);
        check('B12-NO-ORG-LENS', deniedPath(lens.path) && lens.cards === 0 && !trigger && slotVisible !== 'true' && !crashed(lens), `no roles -> /org -> ${lens.path}; org selector visible=${trigger} slot data-visible=${slotVisible}; cards=${lens.cards}; text=${lens.text.slice(0, 80)}`);
        const deep = await visit(page, `/org/${claOnly.sfid}/easycla`, 'id2-deep-cla-only-denied');
        check('B12-DEEP-LINK-DENIED', deniedPath(deep.path) && deep.cards === 0 && !crashed(deep), summary(deep));
        info('B1-B8', `${ID2} is not a CLA manager of any organization right now: B1-B8 need the role (api/interactions.py W1 or m3_chain C8 grants it) - covered by the ID1 run only`);
      }
    }

    // ---- B1 / B8 / B3 / B4 / B5 / B7 for every CLA-only organization of this identity ----------------------
    for (const sfid of claOnlyOrgs.slice(0, 3)) {
      const tag = `${P}-${sfid}`;
      if (!uiOk) { skip(`B1-${tag}`, 'LD flag off'); continue; }
      const v = await visit(page, `/org/${sfid}/easycla`, `${who}-b1-${sfid}`);
      const onlyEasyCla = v.nav.length >= 1 && v.nav.every((n) => /easycla/i.test(n));
      check(`B1-EASYCLA-ONLY-PAGE-${tag}`, /\/easycla/.test(v.path) && !deniedPath(v.path) && v.cards > 0 && onlyEasyCla && !adminWords.test(v.text) && !crashed(v), summary(v));
      // B8: a non-member company has no memberships/projects - the page is still an EasyCLA-only page (same assertion, recorded separately when the org is not an LF member)
      const orgInfo = await e2e.bff(context, 'GET', `/api/orgs/${sfid}`);
      const member = orgInfo.status === 200 && /member/i.test(JSON.stringify(orgInfo.json || {}).slice(0, 4000));
      info(`B8-NON-MEMBER-${tag}`, `GET /api/orgs/${sfid} -> ${orgInfo.status}; membership words present=${member}; EasyCLA-only page rendered=${v.cards > 0 && onlyEasyCla}`);
      for (const sub of ['overview', 'people', 'memberships', 'projects']) {
        const w = await visit(page, `/org/${sfid}/${sub}`, `${who}-b4-${sfid}-${sub}`, 6000);
        check(`B4-NO-${sub.toUpperCase()}-${tag}`, (deniedPath(w.path) || /\/easycla/.test(w.path)) && !adminWords.test(w.text.replace(/EasyCLA/g, '')) && !crashed(w), summary(w));
      }
    }
    if (uiOk && claOnlyOrgs.length >= 2) {
      // B3: switch between two CLA-only companies through the selector; each shows its own agreements only
      const [a, b] = claOnlyOrgs;
      const va = await visit(page, `/org/${a}/easycla`, `${who}-b3-a`);
      const listA = await e2e.bff(context, 'GET', `/api/orgs/${a}/lens/cla-groups`);
      await page.getByTestId('org-selector').click().catch(() => {});
      await sleep(1500);
      const items = (await page.locator('[data-testid^="org-item-"]').evaluateAll((els) => els.map((e) => (e.getAttribute('data-testid') || '').replace('org-item-', ''))).catch(() => []));
      await e2e.shot(page, `${who}-b3-selector`);
      await page.getByTestId(`org-item-${b}`).click({ timeout: 10000 }).catch(() => {});
      await sleep(7000);
      await e2e.closeOverlay(page);
      const pb = new URL(page.url()).pathname;
      const listB = await e2e.bff(context, 'GET', `/api/orgs/${b}/lens/cla-groups`);
      const cardsB = await page.locator('[data-testid="org-easycla-card-link"]').count();
      const hrefsB = await page.locator('[data-testid="org-easycla-card-link"]').evaluateAll((as) => as.map((x) => x.getAttribute('href') || '')).catch(() => []);
      await e2e.shot(page, `${who}-b3-b`);
      const idsA = new Set((listA.json?.claGroups || []).map((r) => r.id));
      const idsB = new Set((listB.json?.claGroups || []).map((r) => r.id));
      const onlyB = hrefsB.every((h) => [...idsB].some((id) => h.includes(id)) || !/\/easycla\//.test(h));
      check(`B3-SWITCH-COMPANIES-${P}`, items.includes(a) && items.includes(b) && pb.includes(b) && cardsB > 0 && onlyB,
        `selector items=${JSON.stringify(items)}; ${a} cards=${va.cards} (${idsA.size} agreements); after switch path=${pb} cards=${cardsB} (${idsB.size} agreements) all cards belong to ${b}=${onlyB}`);
    } else if (uiOk && who === ID2) info(`B3-SWITCH-COMPANIES-${P}`, `${ID2} is CLA-only manager of ${claOnlyOrgs.length} organization(s): the switch needs two`);

    if (uiOk && other.sfid) {
      // B5: another company's organization view
      const v = await visit(page, `/org/${other.sfid}/easycla`, `${who}-b5-other`);
      const w = await visit(page, `/org/${other.sfid}/overview`, `${who}-b5-other-overview`, 6000);
      const noRole = !writers.includes(other.sfid) && !claOrgs.includes(other.sfid);
      if (noRole) check(`B5-OTHER-COMPANY-DENIED-${P}`, deniedPath(v.path) && v.cards === 0 && deniedOverview(w, other.name) && !crashed(v) && !crashed(w), `${summary(v)} || overview -> ${w.path}`);
      else info(`B5-OTHER-COMPANY-${P}`, `${who} holds a role on OTHER_ORG ${other.sfid}: ${summary(v)}`);
    }
    if (uiOk) {
      // B7: odd ids -> limited page or not-found, never a crash
      for (const [label, odd] of [['15CHAR', (claOnly.sfid || m3Org.sfid || '0014100000Te0G7AAJ').slice(0, 15)], ['LFID', 'lf-oldcompany-0001']]) {
        const v = await visit(page, `/org/${odd}/easycla`, `${who}-b7-${label.toLowerCase()}`, 6000);
        check(`B7-ODD-ID-${label}-${P}`, !crashed(v) && (deniedPath(v.path) || /\/easycla/.test(v.path)), summary(v));
      }
    }

    if (who === ID1) {
      // B9: org admin + CLA manager -> organization listed once, full sidebar, EasyCLA tab with the agreements
      const target = adminAndCla[0] || (writers.includes(m3Org.sfid) ? m3Org.sfid : null);
      if (!uiOk) skip('B9-ADMIN-AND-CLA-MANAGER', 'LD flag off');
      else if (!target) info('B9-ADMIN-AND-CLA-MANAGER', `${ID1} is not both writer and CLA manager of any organization right now (writers=${writers.length}, claManagerOrgs=${JSON.stringify(claOrgs)})`);
      else {
        const v = await visit(page, `/org/${target}/easycla`, 'id1-b9-easycla');
        await page.getByTestId('org-selector').click().catch(() => {});
        await sleep(1500);
        const occurrences = await page.locator(`[data-testid="org-item-${target}"]`).count();
        await e2e.shot(page, 'id1-b9-selector');
        await page.keyboard.press('Escape').catch(() => {});
        const full = v.nav.length > 1 && v.nav.some((n) => /easycla/i.test(n)) && v.nav.some((n) => !/easycla/i.test(n));
        check('B9-ADMIN-AND-CLA-MANAGER', occurrences === 1 && full && v.cards > 0 && !crashed(v), `${target}: selector occurrences=${occurrences}; ${summary(v)}`);
      }
      // B10 (#3631): writer organizations WITHOUT an EasyCLA ACS scope (role-grants: writer but not claManagerOrg)
      const plainAdmin = writers.filter((s) => !claOrgs.includes(s)).slice(0, 2);
      for (const sfid of plainAdmin) {
        if (!uiOk) { skip(`B10-WRITER-NO-CLA-SCOPE-${sfid}`, 'LD flag off'); continue; }
        net.capture(new RegExp(`/api/orgs/${sfid}/lens/cla-groups`));
        const v = await visit(page, `/org/${sfid}/easycla`, `id1-b10-${sfid}`, 9000);
        const lens = net.api.filter((r) => r.url.includes(`/api/orgs/${sfid}/lens/cla-groups`)).map((r) => r.status);
        const sign = page.getByRole('button', { name: /sign cla/i }).first();
        const signState = { visible: await sign.isVisible().catch(() => false), disabled: await sign.isDisabled().catch(() => null) };
        const widgets = await page.locator('[data-testid*="easycla"]').evaluateAll((els) => [...new Set(els.map((e) => e.getAttribute('data-testid')))].slice(0, 12)).catch(() => []);
        check(`B10-WRITER-NO-CLA-SCOPE-${sfid}`, !crashed(v) && /\/easycla/.test(v.path) && (v.cards > 0 || signState.visible || /no (signed )?agreements|sign (a )?cla|not signed|no cla/i.test(v.text) || lens.some((s) => s >= 400)),
          `${summary(v)}; lens/cla-groups statuses=${JSON.stringify(lens)}; Sign CLA=${JSON.stringify(signState)}; widgets=${JSON.stringify(widgets)}; alerts=${v.alerts.slice(0, 100)}`);
      }
      if (!plainAdmin.length) info('B10-WRITER-NO-CLA-SCOPE', `every writer organization of ${ID1} also carries a CLA manager scope - #3631 state not reproducible right now`);
      // B11: signatory / designee only -> handled by api/identity_boundary.py (role-grants excludes them); recorded here from the grants shape
      info('B11-SIGNATORY-ONLY', `role-grants only lists writers/readers/claManagerOrgs - CLA signatory/designee roles never create an Organization lens entry (API module identity_boundary asserts the ACS side)`);
    }

    // ---- B2: ID2 adds + removes one synthetic approval-list domain on the CLA-only fixture ------------------------
    if (who === ID2 && claOnly.sfid && claOnly.sig) {
      if (!claOrgs.includes(claOnly.sfid)) info('B2-ADD-REMOVE-ENTRY', `${ID2} is not a CLA manager of ${claOnly.name} right now - B2 needs the role (api/interactions.py W2C covers the API side)`);
      else if (!WRITES) skip('B2-ADD-REMOVE-ENTRY', 'writes disabled');
      else if (!uiOk) skip('B2-ADD-REMOVE-ENTRY', 'LD flag off');
      else {
        const h = m3(session, claOnly);
        const before = await h.state();
        const synthetic = `qa-e2e-bridge-${e2e.TAG}.example.com`;
        const bridgeOwned = (x) => x.kind === 'domain' && /^qa-e2e-bridge-.*\.example\.com$/.test(x.value);
        const stale = before.entries.filter(bridgeOwned);
        try {
          await h.openDetail('B2-OPEN-DETAIL');
          await h.tab('approval');
          for (const x of stale) await h.removeEntry(x.value, `B2-P0-STALE-REMOVED-${x.value.replace(/[^a-z0-9]/gi, '').slice(0, 20).toUpperCase()}`, x.kind);
          await h.addEntry(/^email domain$/i, synthetic, 'B2-ADD-ENTRY');
          await h.removeEntry(synthetic, 'B2-REMOVE-ENTRY', 'domain');
        } finally {
          // the synthetic domain must not outlive the run even when the UI removal timed out: BFF cleanup of any leftover
          const left = (await h.state()).entries.filter(bridgeOwned);
          if (left.length) info('B2-BFF-CLEANUP', `UI left ${JSON.stringify(left.map((x) => x.value))} on the list -> PUT approval-list remove -> ${(await e2e.bff(context, 'PUT', h.paths.approval, { add: [], remove: left })).status}`);
        }
        const after = await h.state();
        const expected = before.entries.filter((x) => !stale.includes(x)).map((x) => `${x.kind}:${x.value}`).sort();
        check('B2-END-STATE-RESTORED', JSON.stringify(after.entries.map((x) => `${x.kind}:${x.value}`).sort()) === JSON.stringify(expected), `entries=${after.entries.length} (baseline ${before.entries.length}, stale removed ${stale.length})`);
      }
    }

    if (who === ID1) {
      // ---- B13: org admin of some organizations AND only a CLA manager of another one -------------------------------
      // Self Serve lists the admin organizations with the full sidebar; CLA-only entries apply only to callers without
      // any Org Lens grant (org-cla-only-orgs.service.ts claOnlyEntryApplies), so the CLA-managed company is not in the
      // selector / role-grants.claManagerOrgs. Whether ID1 manages such a company is read from the lens itself.
      const mixedOrg = [claOnly.sfid, m3Org.sfid].find((s) => s && !writers.includes(s));
      const lens = mixedOrg ? await e2e.bff(context, 'GET', `/api/orgs/${mixedOrg}/lens/cla-groups`) : null;
      const managerThere = lens?.status === 200 && (lens.json?.claGroups || []).some((r) => r.viewerIsClaManager === true);
      if (!writers.length || !mixedOrg || !managerThere) info('B13-ADMIN-PLUS-CLA-ONLY', `mixed case not reproducible for ${ID1} right now: writer orgs=${writers.length}, CLA-managed non-writer company=${mixedOrg || 'none'} (lens -> ${lens?.status ?? '-'}, viewerIsClaManager=${managerThere})`);
      else {
        check('B13-ADMIN-PLUS-CLA-ONLY-SELECTOR', writers.every((s) => itemIds.includes(s)) && !itemIds.includes(mixedOrg) && !claOrgs.includes(mixedOrg),
          `nav/org-items=${JSON.stringify(itemIds)} lists the ${writers.length} admin organization(s) and not the CLA-managed ${mixedOrg}; role-grants.claManagerOrgs=${JSON.stringify(claOrgs)}`);
        if (!uiOk) skip('B13-ADMIN-PLUS-CLA-ONLY-DEEP-LINK', 'LD flag off');
        else {
          const v = await visit(page, `/org/${mixedOrg}/easycla`, 'id1-b13-cla-only-deep-link');
          check('B13-ADMIN-PLUS-CLA-ONLY-DEEP-LINK', !crashed(v) && (deniedPath(v.path) || /\/easycla/.test(v.path)), `${summary(v)} (ID1 manages ${mixedOrg} in EasyCLA while the selector lists only the admin organizations)`);
        }
      }
    }

    // ---- B-LD: LD kill switch observation ------------------------------------------------------------------------
    const flags = ld.flags();
    e2e.evidence(`${who}-ld-flags.private.json`, flags);
    if (flag === false) {
      const v = await visit(page, `/org/${claOrgs[0] || writers[0] || m3Org.sfid}/easycla`, `${who}-ld-kill-switch`, 6000);
      check(`B-LD-KILL-SWITCH-${P}`, !/\/easycla$/.test(v.path) || v.cards === 0, `flag off -> ${summary(v)}`);
    } else info(`B-LD-KILL-SWITCH-${P}`, `${cfg('LD_FLAG_M3')}=${flag} for ${who} (flags seen: ${JSON.stringify(flags).slice(0, 200)}) - the off-state redirect was verified on 2026-10-14 (dev) before the rule targeted this identity`);

    check(`${P}-NO-BLOCKED-WRITES`, net.blocked.length === 0, `guard blocked ${net.blocked.length} request(s) ${JSON.stringify(net.blocked.slice(0, 3))}`);
    const claErrs = net.api.filter((r) => r.status >= 500 && /\/lens\/cla-groups|\/role-grants|\/nav\/org-items|\/me\/clas/.test(r.url));
    check(`${P}-NO-CLA-5XX`, claErrs.length === 0, `CLA-related 5xx=${JSON.stringify(claErrs.slice(0, 4))}`);
  } catch (err) {
    check(`${P}-HARNESS`, false, `error: ${err.message} at ${page.url().slice(0, 120)}`);
    await e2e.shot(page, `${who}-error`);
  } finally {
    await e2e.close(session);
  }
}
process.exit(e2e.finish());
