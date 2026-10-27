import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

import { THEME_IDS } from '../hooks/use-theme';

// The theme contract has two authored files and one derived one:
//   styles.css        — every colour value (oklch only, no hex)
//   index.html        — exactly two neutral pre-paint pairs, appearance only
//   hooks/use-theme.ts— ids, appearances and persistence, no colour values
// These assertions keep the FOUC path from growing a second copy of the theme.

const THIS_DIR = fileURLToPath(new URL('.', import.meta.url));
const HTML = readFileSync(resolve(THIS_DIR, '..', '..', 'index.html'), 'utf8');
const CSS = readFileSync(resolve(THIS_DIR, '..', 'styles.css'), 'utf8');
const REGISTRY = readFileSync(resolve(THIS_DIR, '..', 'hooks', 'use-theme.ts'), 'utf8');

describe('pre-paint theme in index.html', () => {
  it('carries exactly two neutral colour pairs', () => {
    const hexes = HTML.match(/#[0-9a-fA-F]{3,8}\b/g) ?? [];
    expect(hexes.sort()).toEqual(['#071519', '#1e3137', '#e9f4f8', '#ebfaff']);
  });

  it('knows appearances but no theme ids', () => {
    expect(HTML).not.toContain('data-theme');
    for (const id of THEME_IDS) {
      // `dark` is also an appearance value, so this file may name it; every
      // other id must stay in the registry.
      if (id === 'dark') continue;
      expect(HTML, `index.html must not name the "${id}" family`).not.toMatch(new RegExp(`["']${id}["']`));
    }
    expect(HTML).toContain("localStorage.getItem('vivy.appearance')");
    expect(HTML).toContain("setAttribute('data-appearance', appearance)");
    expect(HTML).toContain("classList.toggle('dark', appearance === 'dark')");
  });

  it('declares no tokens and no colour functions', () => {
    expect(HTML).not.toMatch(/oklch\(/);
    expect(HTML).not.toMatch(/--[a-z-]+\s*:/);
  });
});

describe('authored token source', () => {
  it('styles.css uses no hex literals', () => {
    expect(CSS.match(/#[0-9a-fA-F]{3,8}\b/g) ?? []).toEqual([]);
  });

  it('declares every family for both the live and the preview selector', () => {
    for (const id of THEME_IDS) {
      expect(CSS, `missing [data-theme="${id}"]`).toContain(`[data-theme="${id}"]`);
      expect(CSS, `missing [data-theme-preview="${id}"]`).toContain(`[data-theme-preview="${id}"]`);
    }
  });

  it('gives each family its own brand ramp and tint', () => {
    expect(CSS.match(/--brand:/g)?.length).toBe(THEME_IDS.length);
    expect(CSS.match(/--tint-h:/g)?.length).toBeGreaterThanOrEqual(THEME_IDS.length);
    expect(CSS.match(/--tint-c:/g)?.length).toBeGreaterThanOrEqual(THEME_IDS.length);
  });

  it('keeps the registry free of colour values', () => {
    expect(REGISTRY).not.toMatch(/#[0-9a-fA-F]{3,8}\b/);
    expect(REGISTRY).not.toMatch(/oklch\(|rgba?\(|linear-gradient/);
  });
});
