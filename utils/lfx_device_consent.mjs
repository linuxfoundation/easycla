// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT
//
// Local-only helper used by utils/get_lfx_token.sh: completes the Auth0 Device Code consent
// (hosted LF login + device confirmation) in a headless browser with the credentials stored in
// utils/auth0-<stage>.secret (AUTH0_USERNAME / AUTH0_PASSWORD). Nothing confidential is embedded
// here and nothing confidential is printed. Stops (exit 2) on an MFA/captcha/unknown page.
//
// Usage: LFX_VERIFY_URL=<verification_uri_complete> LFX_USER_CODE=<user_code> node utils/lfx_device_consent.mjs <dev|prod>
// (the URL and code come from the environment so that they never appear in process arguments)
import fs from 'node:fs/promises';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const [stage] = process.argv.slice(2);
const verificationUrl = process.env.LFX_VERIFY_URL || '';
const userCode = process.env.LFX_USER_CODE || '';
if (!['dev', 'prod'].includes(stage) || !verificationUrl || !userCode) {
  console.error('usage: LFX_VERIFY_URL=<verification_uri_complete> LFX_USER_CODE=<user_code> node utils/lfx_device_consent.mjs <dev|prod>');
  process.exit(1);
}
const utilsDir = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(utilsDir, '..');
const expectedHost = stage === 'prod' ? 'sso.linuxfoundation.org' : 'linuxfoundation-dev.auth0.com';
if (new URL(verificationUrl).hostname !== expectedHost) {
  console.error(`error: verification URL host is not ${expectedHost}`);
  process.exit(1);
}
const secretText = await fs.readFile(path.join(utilsDir, `auth0-${stage}.secret`), 'utf8');
const credentials = Object.fromEntries(secretText.split('\n').filter(l => l.includes('=')).map(l => {
  const at = l.indexOf('=');
  return [l.slice(0, at).trim(), l.slice(at + 1).trim()];
}));
if (!credentials.AUTH0_USERNAME || !credentials.AUTH0_PASSWORD) {
  console.error(`error: AUTH0_USERNAME/AUTH0_PASSWORD missing in utils/auth0-${stage}.secret`);
  process.exit(1);
}
const { chromium } = createRequire(path.join(repoRoot, 'tests/e2e/package.json'))('playwright');
const deadline = Date.now() + 150000;
const browser = await chromium.launch({ headless: true });
let result = 2;
try {
  const page = await (await browser.newContext()).newPage();
  await page.goto(verificationUrl, { waitUntil: 'domcontentloaded' });
  const firstVisible = async (locator) => {
    for (let i = 0; i < await locator.count(); i++) {
      const item = locator.nth(i);
      if (await item.isVisible().catch(() => false)) return item;
    }
    return null;
  };
  const clickOne = async (names) => {
    for (const name of names) {
      const button = await firstVisible(page.getByRole('button', { name, exact: false }));
      if (button) { await button.click(); console.error(`consent: clicked "${name}"`); return true; }
    }
    return false;
  };
  const diagFile = process.env.LFX_CONSENT_DIAG || '';
  let step = 0;
  while (Date.now() < deadline) {
    await page.waitForTimeout(1500);
    const body = (await page.locator('body').innerText().catch(() => '')) || '';
    const flat = body.toLowerCase();
    if (diagFile) {
      const inputs = await page.locator('input').evaluateAll(els => els.map(el => ({ id: el.id, name: el.name, type: el.type, visible: !!(el.offsetWidth || el.offsetHeight) })));
      const buttons = await page.getByRole('button').allTextContents().catch(() => []);
      const snapshot = { step: step++, url: page.url().split('?')[0], text: body.slice(0, 400), inputs, buttons };
      await fs.appendFile(diagFile, JSON.stringify(snapshot) + '\n');
    }
    if (/(device|you).*(confirmed|logged in|successfully|you can close|all set)/s.test(flat) || /congratulations/.test(flat)) {
      console.error('consent: confirmation page reached');
      result = 0;
      break;
    }
    if (/(one-time|verification code|authenticator|captcha|recaptcha|security code)/.test(flat)) {
      console.error('consent: MFA or captcha challenge shown - stop and complete it manually');
      result = 2;
      break;
    }
    if (flat.includes('device confirmation') || page.url().includes('/device/confirmation')) {
      const codeInput = await firstVisible(page.locator('input[name="user_code"], input[type="text"]'));
      if (codeInput && !(await codeInput.inputValue().catch(() => ''))) await codeInput.fill(userCode);
      if (!await clickOne(['Confirm', 'Continue', 'Submit', 'Next'])) { if (codeInput) await codeInput.press('Enter'); }
      continue;
    }
    const codeInput = await firstVisible(page.locator('input[name="user_code"]'));
    if (codeInput) {
      await codeInput.fill(userCode);
      if (!await clickOne(['Continue', 'Confirm', 'Submit', 'Next'])) await codeInput.press('Enter');
      continue;
    }
    const username = await firstVisible(page.locator('input#username, input[name="username"], input[name="email"], input[autocomplete="username"], input[type="email"]'));
    const password = await firstVisible(page.locator('input#password, input[name="password"], input[type="password"]'));
    if (username || password) {
      if (username) await username.fill(credentials.AUTH0_USERNAME);
      if (password) await password.fill(credentials.AUTH0_PASSWORD);
      if (!await clickOne(['Sign In', 'Sign in', 'SIGN IN', 'Log In', 'Log in', 'Login', 'Continue', 'Next'])) {
        await (password || username).press('Enter');
      }
      continue;
    }
    if (body.replace(/[^A-Z0-9]/gi, '').toUpperCase().includes(userCode.replace(/[^A-Z0-9]/gi, '').toUpperCase())) {
      if (await clickOne(['Confirm', 'Allow', 'Authorize', 'Accept', 'Continue', 'Yes'])) continue;
    }
  }
  if (result !== 0 && Date.now() >= deadline) console.error('consent: timed out without a confirmation page');
} catch {
  // Playwright errors can quote the values typed into the page, so the error itself is never printed.
  console.error('consent: browser interaction failed - approve the printed URL manually or retry');
  result = 2;
} finally {
  await browser.close();
}
process.exit(result);
