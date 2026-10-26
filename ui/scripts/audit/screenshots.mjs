#!/usr/bin/env node
// Screenshot audit for the UI overhaul. Expects the split dev pair to be running
// (`just run` + `cd ui && pnpm dev`); it never starts servers itself, so the
// evidence is always of the app a human can open. Every surface is also measured
// for rendered text contrast, so a light-on-light or dark-on-dark pair fails the
// audit instead of shipping.
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
  { name: 'toast', path: '/', seedToasts: true },
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

// Rendered ink check. Runs in the page: for every visible text node, resolve
// the effective background (compositing translucent ancestors) and measure the
// WCAG ratio. A screenshot cannot tell a light-on-light pair from a deliberate
// tint, so the pair is measured where it renders. Called through
// page.evaluate, so it must stay self-contained.
function sweepInk() {
  const canvas = document.createElement('canvas');
  canvas.width = 1;
  canvas.height = 1;
  const ctx = canvas.getContext('2d', { willReadFrequently: true });
  const cache = new Map();

  const toRgba = (value) => {
    if (cache.has(value)) return cache.get(value);
    ctx.clearRect(0, 0, 1, 1);
    ctx.fillStyle = '#000';
    ctx.fillStyle = value;
    ctx.fillRect(0, 0, 1, 1);
    const [r, g, b, a] = ctx.getImageData(0, 0, 1, 1).data;
    const out = { r, g, b, a: a / 255 };
    cache.set(value, out);
    return out;
  };
  const lin = (c) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4);
  const lum = ({ r, g, b }) => 0.2126 * lin(r / 255) + 0.7152 * lin(g / 255) + 0.0722 * lin(b / 255);
  const over = (fg, bg) => ({
    r: fg.r * fg.a + bg.r * (1 - fg.a),
    g: fg.g * fg.a + bg.g * (1 - fg.a),
    b: fg.b * fg.a + bg.b * (1 - fg.a),
    a: 1,
  });
  const ratio = (a, b) => {
    const [hi, lo] = [lum(a), lum(b)].sort((p, q) => q - p);
    return (hi + 0.05) / (lo + 0.05);
  };
  const hex = ({ r, g, b }) => '#' + [r, g, b].map((v) => Math.round(v).toString(16).padStart(2, '0')).join('');

  const visible = (el) => {
    const style = getComputedStyle(el);
    if (style.display === 'none' || style.visibility === 'hidden' || Number(style.opacity) === 0) return false;
    const rect = el.getBoundingClientRect();
    return rect.width > 1 && rect.height > 1;
  };

  // Gradients, images and backdrop filters have no single background colour to
  // measure against; those nodes are skipped rather than guessed.
  const background = (el) => {
    const stack = [];
    for (let node = el; node; node = node.parentElement) {
      const style = getComputedStyle(node);
      if (style.backgroundImage !== 'none') return null;
      const colour = toRgba(style.backgroundColor);
      if (colour.a > 0) {
        stack.push(colour);
        if (colour.a >= 1) break;
      }
    }
    let base = { r: 255, g: 255, b: 255, a: 1 };
    for (let i = stack.length - 1; i >= 0; i -= 1) base = over(stack[i], base);
    return base;
  };

  const findings = [];
  const seen = new Set();
  let scanned = 0;
  for (const el of document.querySelectorAll('*')) {
    const text = [...el.childNodes]
      .filter((node) => node.nodeType === Node.TEXT_NODE)
      .map((node) => node.textContent.trim())
      .join(' ')
      .trim();
    if (!text) continue;
    if (!visible(el)) continue;
    const bg = background(el);
    if (!bg) continue;
    scanned += 1;
    const style = getComputedStyle(el);
    const fg = over(toRgba(style.color), bg);
    const value = ratio(fg, bg);
    const size = Number.parseFloat(style.fontSize);
    const weight = Number(style.fontWeight) || 400;
    const minimum = size >= 24 || (size >= 18.66 && weight >= 700) ? 3 : 4.5;
    if (value + 0.005 >= minimum) continue;
    const key = `${hex(fg)}|${hex(bg)}|${style.fontSize}`;
    if (seen.has(key)) continue;
    seen.add(key);
    findings.push({
      text: text.slice(0, 48),
      ratio: Number(value.toFixed(2)),
      minimum,
      fg: hex(fg),
      bg: hex(bg),
      size: style.fontSize,
      className: String(el.getAttribute('class') || '').slice(0, 120),
    });
  }
  return { scanned, findings };
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
  const manifest = { baseUrl: args.baseUrl, startedAt: new Date().toISOString(), shots: [], failures: [], ink: [] };
  let scannedText = 0;

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
            if (surface.seedToasts) {
              // Dev-pair only: import the live toast store so the shot measures the
              // real component. A production build has no /src module graph, so a
              // failed import must not count as an audit failure.
              await page.evaluate(async () => {
                try {
                  const { notify } = await import('/src/lib/toasts.ts');
                  notify({
                    tone: 'danger',
                    title: '保存设置失败',
                    detail: '操作没有完成。请稍后重试；如果持续失败，展开详情复制诊断。',
                    technical: 'rpc error: code = Unavailable desc = connection closed\n    at /app/src/lib/rpc.ts:1:1',
                  });
                  notify({ tone: 'warn', title: '正在重新连接控制面', detail: '等待后端响应，界面仍可使用。' });
                  notify({ tone: 'ok', title: '已复制诊断' });
                } catch (error) {
                  console.warn('toast seeding skipped:', error);
                }
              });
              await page.waitForTimeout(300);
            }
            const ink = await page.evaluate(sweepInk);
            scannedText += ink.scanned;
            for (const finding of ink.findings) {
              manifest.ink.push({ theme: theme.id, width, surface: surface.name, ...finding });
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

  console.info(
    `audit: ${manifest.shots.length} screenshots, ${manifest.failures.length} failures, ` +
      `${manifest.ink.length} ink pairs below AA (${scannedText} text nodes measured) -> ${outRoot}`,
  );
  for (const failure of manifest.failures) console.error(`audit failure: ${failure.surface} @ ${failure.theme}/${failure.width}: ${failure.error}`);
  for (const pair of manifest.ink) {
    console.error(
      `ink: ${pair.theme}/${pair.width} ${pair.surface} ${pair.fg} on ${pair.bg} = ${pair.ratio} (min ${pair.minimum}) ${pair.size} "${pair.text}" ${pair.className}`,
    );
  }
  if (manifest.failures.length || manifest.ink.length) process.exitCode = 1;
}

await main();
