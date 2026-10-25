#!/usr/bin/env node
// Screenshot audit for the UI overhaul. Expects the split dev pair to be running
// (`just run` + `cd ui && pnpm dev`); it never starts servers itself, so the
// evidence is always of the app a human can open.
//
// Usage:
//   pnpm run audit
//   pnpm run audit -- --base-url http://127.0.0.1:3015 --themes default,dark
//   pnpm run audit -- --widths 390,1280 --only chat-empty,settings
//
// Output: ui/.audit/<timestamp>/<theme>-<width>/<surface>.png plus manifest.json.

import { readFileSync } from 'node:fs';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from '@playwright/test';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const UI_ROOT = path.resolve(HERE, '..', '..');

// Families come from the registry, so the audit never photographs a stale list.
function readThemes() {
  const source = readFileSync(path.join(UI_ROOT, 'src', 'hooks', 'use-theme.ts'), 'utf8');
  const entries = [...source.matchAll(/\{\s*id:\s*'([a-z0-9-]+)',\s*appearance:\s*'(light|dark)'/gi)];
  if (entries.length === 0) throw new Error('no themes found in src/hooks/use-theme.ts');
  return entries.map(([, id, appearance]) => ({ id, appearance }));
}

const THEMES = readThemes();

const WELCOME_KEY = 'vivy.ui.welcome.completed';

const SURFACES = [
  { name: 'chat-empty', path: '/' },
  { name: 'settings', path: '/settings' },
  { name: 'theme-picker', path: '/settings', scrollTo: '[data-testid="theme-picker"]' },
  { name: 'dashboard', path: '/dashboard' },
  { name: 'approvals', path: '/approvals' },
  { name: 'mcp', path: '/mcp' },
  { name: 'skills', path: '/skills' },
  { name: 'cron-tasks', path: '/cron-tasks' },
  { name: 'lifecycle', path: '/lifecycle' },
  { name: 'welcome', path: '/', welcome: 'open' },
  { name: 'nav-toggle', path: '/', clickHeader: true },
];

function parseArgs(argv) {
  const args = { baseUrl: 'http://127.0.0.1:3015', out: '', widths: '390,768,1280', themes: '', only: '' };
  for (let i = 0; i < argv.length; i += 1) {
    const token = argv[i];
    if (token === '--') continue;
    if (!token.startsWith('--')) continue;
    const [flag, inline] = token.slice(2).split('=');
    const value = inline ?? argv[i + 1];
    if (inline === undefined) i += 1;
    if (value === undefined) throw new Error(`--${flag} needs a value`);
    if (flag === 'base-url') args.baseUrl = value;
    else if (flag === 'out') args.out = value;
    else if (flag === 'widths') args.widths = value;
    else if (flag === 'themes') args.themes = value;
    else if (flag === 'only') args.only = value;
  }
  return args;
}

function stamp() {
  return new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
}

async function waitForApp(page) {
  await page.waitForFunction(() => (document.querySelector('#root')?.childElementCount ?? 0) > 0, null, {
    timeout: 20_000,
  });
  // Let fonts, theme application and the first data fetch settle.
  await page.waitForTimeout(600);
}

async function launchBrowser() {
  try {
    return await chromium.launch();
  } catch (bundledError) {
    try {
      // No bundled Chromium on this host; use the installed Chrome so the audit
      // does not depend on a browser download.
      return await chromium.launch({ channel: 'chrome' });
    } catch {
      throw new Error(
        `no usable browser: run \`pnpm exec playwright install chromium\` (bundled launch failed: ${bundledError.message})`,
      );
    }
  }
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const wantedThemes = args.themes ? new Set(args.themes.split(',')) : null;
  const wantedSurfaces = args.only ? new Set(args.only.split(',')) : null;
  const widths = args.widths
    .split(',')
    .map((value) => Number(value.trim()))
    .filter((value) => Number.isFinite(value) && value > 0);
  const outRoot = path.isAbsolute(args.out || '')
    ? args.out
    : path.resolve(UI_ROOT, args.out || path.join('.audit', stamp()));

  const themes = THEMES.filter((theme) => !wantedThemes || wantedThemes.has(theme.id));
  const surfaces = SURFACES.filter((surface) => !wantedSurfaces || wantedSurfaces.has(surface.name));
  if (!themes.length) throw new Error(`no theme matched: ${args.themes}`);
  if (!surfaces.length) throw new Error(`no surface matched: ${args.only}`);

  const response = await fetch(args.baseUrl, { redirect: 'manual' }).catch((error) => {
    throw new Error(`cannot reach ${args.baseUrl}: ${error.message}`);
  });
  if (response.status >= 500) throw new Error(`${args.baseUrl} answered ${response.status}`);

  const browser = await launchBrowser();
  const manifest = { baseUrl: args.baseUrl, startedAt: new Date().toISOString(), shots: [], failures: [] };

  try {
    for (const theme of themes) {
      for (const width of widths) {
        const context = await browser.newContext({
          viewport: { width, height: 900 },
          deviceScaleFactor: 1,
          locale: 'zh-CN',
          reducedMotion: 'reduce',
        });
        await context.addInitScript(
          ({ themeId, appearance, welcomeKey, welcome }) => {
            try {
              // Both keys mirror a returning user, so the pre-paint script in index.html
              // has everything it needs and the shot cannot hide a theme flash.
              localStorage.setItem('vivy.theme', themeId);
              localStorage.setItem('vivy.appearance', appearance);
              if (welcome === 'open') localStorage.removeItem(welcomeKey);
              else localStorage.setItem(welcomeKey, '1');
            } catch {
              /* storage unavailable: audit still captures the default surface */
            }
          },
          { themeId: theme.id, appearance: theme.appearance, welcomeKey: WELCOME_KEY, welcome: 'closed' },
        );
        const page = await context.newPage();
        for (const surface of surfaces) {
          const shot = path.join(outRoot, `${theme.id}-${width}`, `${surface.name}.png`);
          const entry = { theme: theme.id, appearance: theme.appearance, width, surface: surface.name, path: shot };
          try {
            await page.goto(`${args.baseUrl}${surface.path}`, { waitUntil: 'domcontentloaded' });
            if (surface.welcome === 'open') {
              await page.evaluate((key) => localStorage.removeItem(key), WELCOME_KEY);
              await page.reload({ waitUntil: 'domcontentloaded' });
            }
            await waitForApp(page);
            if (surface.clickHeader) {
              await page.locator('header button').first().click();
              await page.waitForTimeout(400);
            }
            if (surface.scrollTo) {
              await page.locator(surface.scrollTo).first().scrollIntoViewIfNeeded();
              await page.waitForTimeout(250);
            }
            await mkdir(path.dirname(shot), { recursive: true });
            await page.screenshot({ path: shot, animations: 'disabled' });
            manifest.shots.push(entry);
          } catch (error) {
            manifest.failures.push({ ...entry, error: error instanceof Error ? error.message : String(error) });
          }
        }
        await context.close();
      }
    }
  } finally {
    await browser.close();
    await mkdir(outRoot, { recursive: true });
    await writeFile(path.join(outRoot, 'manifest.json'), `${JSON.stringify(manifest, null, 2)}\n`, 'utf8');
  }

  console.info(`audit: ${manifest.shots.length} screenshots, ${manifest.failures.length} failures -> ${outRoot}`);
  for (const failure of manifest.failures) console.error(`audit failure: ${failure.surface} @ ${failure.theme}/${failure.width}: ${failure.error}`);
  if (manifest.failures.length) process.exitCode = 1;
}

await main();
