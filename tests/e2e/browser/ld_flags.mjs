// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT
//
// LaunchDarkly flags as evaluated by the Self Serve front end for ID1 and ID2 (read-only).
// The M3 kill switch (LD_FLAG_M3) decides whether the Organization lens renders the EasyCLA pages;
// the bridge/m3 modules re-read it themselves, this module records the baseline for the report.
import * as e2e from './lib/e2e.mjs';

const { cfg, check, info, skip, log } = e2e;
const FLAG = cfg('LD_FLAG_M3');
const identities = [[cfg('ID1'), true], [cfg('ID2'), e2e.gate('ID2_LD_TARGETED')]];
const all = {};

for (const [who, expectOn] of identities) {
  const session = await e2e.launch({ identity: who });
  const { page } = session;
  const ld = e2e.attachLd(page);
  try {
    const how = await e2e.login(page, who, `${e2e.SS}/org/${cfg('CLA_ONLY_COMPANY_SFID')}/easycla`);
    log(`${who}: login ${how}`);
    await page.goto(`${e2e.SS}/org/${cfg('CLA_ONLY_COMPANY_SFID')}/easycla`, { waitUntil: 'domcontentloaded', timeout: 60000 });
    await e2e.sleep(15000);
    const flags = ld.flags();
    all[who] = { calls: ld.calls.length, flags, finalPath: new URL(page.url()).pathname };
    info(`LD-${who}-FLAGS`, `${ld.calls.length} LD call(s); ${Object.keys(flags).length} CLA-related flag(s): ${JSON.stringify(flags)}`, { flags });
    if (!ld.calls.length) {
      skip(`LD-${who}-${FLAG}`, 'no LaunchDarkly traffic observed (SDK not loaded or blocked)');
    } else if (!(FLAG in flags)) {
      info(`LD-${who}-${FLAG}`, `flag ${FLAG} not present in the evaluation payload (${Object.keys(flags).length} flags seen)`);
    } else if (expectOn) {
      check(`LD-${who}-${FLAG}`, flags[FLAG] === true, `expected ON for ${who}; got ${JSON.stringify(flags[FLAG])}`);
    } else {
      info(`LD-${who}-${FLAG}`, `${JSON.stringify(flags[FLAG])} (identity not expected to be targeted; set ID2_LD_TARGETED=1 once added to the LD rule)`);
    }
    await e2e.shot(page, `ld-${who}`);
  } catch (err) {
    check(`LD-${who}-HARNESS`, false, `error: ${err.message}`);
    await e2e.shot(page, `ld-${who}-error`);
  } finally {
    await e2e.close(session);
  }
}
e2e.evidence('ld-flags.private.json', all);
process.exit(e2e.finish());
