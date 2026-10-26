#!/usr/bin/env node
// Contrast audit for the authored token layer. Reads ui/src/styles.css, resolves
// each family at its declared appearance, converts oklch/hex to sRGB and checks
// the text and non-text pairs the UI actually renders (WCAG 1.4.3 / 1.4.11).
// Exit code 1 when a pair misses its minimum.
//
// Usage: pnpm run audit:contrast   (or node scripts/audit/contrast.mjs)
//
// The family -> appearance map is read from hooks/use-theme.ts so the registry
// stays the only author; a family present in CSS but missing there fails.

import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const UI_ROOT = path.resolve(HERE, '..', '..');
const CSS_PATH = path.join(UI_ROOT, 'src', 'styles.css');
const REGISTRY_PATH = path.join(UI_ROOT, 'src', 'hooks', 'use-theme.ts');

// WCAG 1.4.3 for text; 1.4.11 for non-text marks (fills, icons, focus rings).
const AA_BODY = 4.5;
const AA_GRAPHIC = 3;

const PAIRS = [
  ['fg on canvas', 'fg', 'bg-app', AA_BODY],
  ['muted on canvas', 'fg-muted', 'bg-app', AA_BODY],
  ['subtle on canvas', 'fg-subtle', 'bg-app', AA_BODY],
  ['fg on raised', 'fg', 'bg-raised', AA_BODY],
  ['muted on surface', 'fg-muted', 'bg-surface', AA_BODY],
  ['brand on canvas', 'brand', 'bg-app', AA_GRAPHIC],
  ['brand on raised', 'brand', 'bg-raised', AA_GRAPHIC],
  ['brand on brand-soft', 'brand', 'brand-soft', AA_GRAPHIC],
  ['brand-fill on canvas', 'brand-fill', 'bg-app', AA_GRAPHIC],
  ['brand-fill on raised', 'brand-fill', 'bg-raised', AA_GRAPHIC],
  ['brand-fg on brand-fill', 'brand-fg', 'brand-fill', AA_BODY],
  ['brand-ink on canvas', 'brand-ink', 'bg-app', AA_BODY],
  ['brand-ink on raised', 'brand-ink', 'bg-raised', AA_BODY],
  ['brand-ink on brand-soft', 'brand-ink', 'brand-soft', AA_BODY],
  // Ink on a soft tint must be the tone itself, never `-fg` (which is the ink
  // for the solid tone). The source ratchet in theme/tokens.test.ts enforces
  // that pairing; these pairs prove the tone-on-soft value is legible.
  ['ok on ok-soft', 'ok', 'ok-soft', AA_BODY],
  ['warn on warn-soft', 'warn', 'warn-soft', AA_BODY],
  ['danger on danger-soft', 'danger', 'danger-soft', AA_BODY],
  ['info on info-soft', 'info', 'info-soft', AA_BODY],
  ['ok on canvas', 'ok', 'bg-app', AA_BODY],
  ['warn on canvas', 'warn', 'bg-app', AA_BODY],
  ['danger on canvas', 'danger', 'bg-app', AA_BODY],
  ['info on canvas', 'info', 'bg-app', AA_BODY],
  ['ok-fg on ok', 'ok-fg', 'ok', AA_BODY],
  ['warn-fg on warn', 'warn-fg', 'warn', AA_BODY],
  ['danger-fg on danger', 'danger-fg', 'danger', AA_BODY],
  ['info-fg on info', 'info-fg', 'info', AA_BODY],
];

// ── CSS reading ────────────────────────────────────────────────────────────

function topLevelBlocks(css) {
  const source = css.replace(/\/\*[\s\S]*?\*\//g, '');
  const blocks = [];
  let segmentStart = 0;
  for (let i = 0; i < source.length; i += 1) {
    if (source[i] !== '{') continue;
    const selector = source.slice(segmentStart, i).trim();
    let depth = 1;
    let j = i + 1;
    for (; j < source.length && depth > 0; j += 1) {
      if (source[j] === '{') depth += 1;
      else if (source[j] === '}') depth -= 1;
    }
    const body = source.slice(i + 1, j - 1);
    if (selector && !selector.startsWith('@')) blocks.push({ selector, body, index: blocks.length });
    i = j - 1;
    segmentStart = j;
  }
  return blocks;
}

function declarations(body) {
  const out = new Map();
  for (const chunk of body.split(';')) {
    const at = chunk.indexOf(':');
    if (at < 0) continue;
    const name = chunk.slice(0, at).trim();
    if (!name.startsWith('--')) continue;
    out.set(name, chunk.slice(at + 1).trim());
  }
  return out;
}

function selectorMatches(selector, element) {
  const tokens = selector.match(/\[[^\]]+\]|:[a-z-]+/g);
  if (!tokens) return null;
  let specificity = 0;
  for (const token of tokens) {
    if (token === ':root') {
      specificity += 1;
      continue;
    }
    const attr = token.match(/^\[([a-z-]+)(?:="([^"]*)")?\]$/i);
    if (!attr) return null;
    specificity += 1;
    const value = element[attr[1]];
    if (value === undefined) return null;
    if (attr[2] !== undefined && value !== attr[2]) return null;
  }
  return specificity;
}

function collect(element) {
  const blocks = topLevelBlocks(readFileSync(CSS_PATH, 'utf8'));
  const matched = [];
  for (const block of blocks) {
    let best = null;
    for (const selector of block.selector.split(',')) {
      const specificity = selectorMatches(selector.trim(), element);
      if (specificity !== null && (best === null || specificity > best)) best = specificity;
    }
    if (best !== null) matched.push({ ...block, specificity: best });
  }
  matched.sort((a, b) => a.specificity - b.specificity || a.index - b.index);
  const tokens = new Map();
  for (const block of matched) for (const [name, value] of declarations(block.body)) tokens.set(name, value);
  return tokens;
}

// ── Value resolution ───────────────────────────────────────────────────────

function evaluateCalc(value) {
  let current = value;
  for (let guard = 0; guard < 20 && current.includes('calc('); guard += 1) {
    current = current.replace(/calc\(([^()]*)\)/g, (_, inner) => {
      const safe = inner.replace(/[^0-9.+\-*/\s]/g, '');
      // eslint-disable-next-line no-new-func
      const result = Function(`"use strict";return (${safe});`)();
      if (!Number.isFinite(result)) throw new Error(`calc() did not evaluate: ${inner}`);
      return String(Number(result.toFixed(6)));
    });
  }
  return current;
}

function resolve(name, tokens, seen = new Set()) {
  const key = name.replace(/^--/, '');
  if (seen.has(key)) throw new Error(`circular token reference at --${key}`);
  seen.add(key);
  const raw = tokens.get(`--${key}`);
  if (raw === undefined) throw new Error(`--${key} is not declared`);
  let value = raw;
  for (let guard = 0; guard < 10 && value.includes('var('); guard += 1) {
    value = value.replace(/var\((--[a-z0-9-]+)\)/gi, (_, ref) => resolve(ref.slice(2), tokens, new Set(seen)));
  }
  return evaluateCalc(value);
}

// ── Colour maths ───────────────────────────────────────────────────────────

function srgbToLinear(channel) {
  return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
}

function linearToSrgb(channel) {
  return channel <= 0.0031308 ? channel * 12.92 : 1.055 * channel ** (1 / 2.4) - 0.055;
}

function parseColor(text) {
  const oklch = text.match(/^oklch\(\s*([\d.]+)\s+([\d.]+)\s+([\d.]+)\s*(?:\/\s*([\d.]+%?))?\s*\)$/i);
  if (oklch) {
    const [, lText, cText, hText, alphaText] = oklch;
    const l = Number(lText);
    const c = Number(cText);
    const h = (Number(hText) * Math.PI) / 180;
    const a = c * Math.cos(h);
    const b = c * Math.sin(h);
    const lPrime = (l + 0.3963377774 * a + 0.2158037573 * b) ** 3;
    const mPrime = (l - 0.1055613458 * a - 0.0638541728 * b) ** 3;
    const sPrime = (l - 0.0894841775 * a - 1.291485548 * b) ** 3;
    const linear = [
      4.0767416621 * lPrime - 3.3077115913 * mPrime + 0.2309699292 * sPrime,
      -1.2684380046 * lPrime + 2.6097574011 * mPrime - 0.3413193965 * sPrime,
      -0.0041960863 * lPrime - 0.7034186147 * mPrime + 1.707614701 * sPrime,
    ];
    const inGamut = linear.every((channel) => channel >= -0.001 && channel <= 1.001);
    const clamped = linear.map((channel) => Math.min(1, Math.max(0, channel)));
    const alpha = alphaText === undefined ? 1 : alphaText.endsWith('%') ? Number(alphaText.slice(0, -1)) / 100 : Number(alphaText);
    return { linear: clamped, alpha, inGamut, text };
  }
  const hex = text.match(/^#([0-9a-f]{3,8})$/i);
  if (hex) {
    const digits = hex[1];
    const expand = digits.length === 3 || digits.length === 4
      ? digits.split('').map((d) => d + d).join('')
      : digits;
    const [r, g, b] = [0, 2, 4].map((offset) => Number.parseInt(expand.slice(offset, offset + 2), 16) / 255);
    const alpha = expand.length === 8 ? Number.parseInt(expand.slice(6, 8), 16) / 255 : 1;
    return { linear: [r, g, b].map(srgbToLinear), alpha, inGamut: true, text };
  }
  throw new Error(`cannot parse colour: ${text}`);
}

function composite(color, backdrop) {
  if (color.alpha >= 1) return color;
  return {
    ...color,
    alpha: 1,
    linear: color.linear.map((channel, i) => channel * color.alpha + backdrop.linear[i] * (1 - color.alpha)),
  };
}

const luminance = (color) => 0.2126 * color.linear[0] + 0.7152 * color.linear[1] + 0.0722 * color.linear[2];

function contrast(foreground, background) {
  const bg = parseColor(background);
  const fg = composite(parseColor(foreground), bg);
  const [a, b] = [luminance(fg), luminance(bg)].sort((x, y) => y - x);
  return (a + 0.05) / (b + 0.05);
}

function toHex(color) {
  const channels = color.linear.map((channel) => Math.round(Math.min(1, Math.max(0, linearToSrgb(channel))) * 255));
  return `#${channels.map((channel) => channel.toString(16).padStart(2, '0')).join('')}`;
}

// ── Registry ───────────────────────────────────────────────────────────────

function loadRegistry() {
  const source = readFileSync(REGISTRY_PATH, 'utf8');
  const entries = [...source.matchAll(/\{\s*id:\s*'([a-z0-9-]+)',\s*appearance:\s*'(light|dark)'/gi)];
  if (entries.length === 0) throw new Error('no themes found in use-theme.ts');
  return entries.map(([, id, appearance]) => ({ id, appearance }));
}

function cssFamilies() {
  const ids = new Set();
  for (const block of topLevelBlocks(readFileSync(CSS_PATH, 'utf8'))) {
    for (const match of block.selector.matchAll(/\[data-theme="([a-z0-9-]+)"\]/gi)) ids.add(match[1]);
  }
  return ids;
}

// ── Run ────────────────────────────────────────────────────────────────────

const registry = loadRegistry();
const declared = cssFamilies();
const missingInCss = registry.filter((family) => !declared.has(family.id));
const missingInRegistry = [...declared].filter((id) => !registry.some((family) => family.id === id));
if (missingInCss.length || missingInRegistry.length) {
  for (const family of missingInCss) console.error(`family "${family.id}" is in use-theme.ts but has no [data-theme] block`);
  for (const id of missingInRegistry) console.error(`family "${id}" has a [data-theme] block but no registry entry`);
  process.exit(1);
}

const failures = [];
const outOfGamut = new Set();
const rows = [];

for (const family of registry) {
  const tokens = collect({ 'data-theme': family.id, 'data-appearance': family.appearance });
  const values = new Map();
  for (const [name] of tokens) {
    try {
      values.set(name, resolve(name, tokens));
    } catch {
      /* tokens referencing other families are resolved on demand below */
    }
  }
  const read = (name) => {
    if (!values.has(name)) values.set(name, resolve(name, tokens));
    return values.get(name);
  };
  const row = { family: family.id, appearance: family.appearance, pairs: [] };
  for (const [label, fg, bg, minimum] of PAIRS) {
    let ratio;
    try {
      ratio = contrast(read(fg), read(bg));
    } catch (cause) {
      failures.push(`${family.id}: ${label} — ${cause.message}`);
      continue;
    }
    const fgColor = parseColor(read(fg));
    const bgColor = parseColor(read(bg));
    if (!fgColor.inGamut) outOfGamut.add(`${family.id}: ${fg} ${read(fg)}`);
    if (!bgColor.inGamut) outOfGamut.add(`${family.id}: ${bg} ${read(bg)}`);
    const rounded = Number(ratio.toFixed(2));
    row.pairs.push({ label, ratio: rounded, minimum });
    if (rounded < minimum) failures.push(`${family.id}: ${label} = ${rounded.toFixed(2)} (needs ${minimum})`);
  }
  row.swatches = ['bg-app', 'bg-raised', 'fg', 'fg-muted', 'brand', 'brand-fill', 'brand-ink', 'brand-fg', 'ok', 'warn', 'danger']
    .map((name) => `${name} ${toHex(parseColor(read(name)))}`).join('  ');
  rows.push(row);
}

for (const row of rows) {
  console.log(`\n${row.family} (${row.appearance})`);
  console.log(`  ${row.swatches}`);
  const cells = row.pairs
    .map((pair) => `${pair.label}=${pair.ratio.toFixed(2)}${pair.ratio < pair.minimum ? '!!' : ''}`);
  for (let i = 0; i < cells.length; i += 4) console.log(`  ${cells.slice(i, i + 4).join('  ')}`);
}

if (outOfGamut.size) {
  console.error(`\n${outOfGamut.size} token(s) outside sRGB; browsers clamp them, so the rendered colour differs:`);
  for (const line of outOfGamut) console.error(`  ${line}`);
}

if (failures.length || outOfGamut.size) {
  if (failures.length) {
    console.error(`\n${failures.length} contrast failure(s):`);
    for (const line of failures) console.error(`  ${line}`);
  }
  process.exit(1);
}
console.log(`\ncontrast: ${rows.length} families, ${rows.reduce((sum, row) => sum + row.pairs.length, 0)} pairs, all >= AA`);
