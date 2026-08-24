import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  DEFAULT_THEME_ID,
  THEMES,
  THEME_IDS,
  applyThemeToDOM,
  isThemeId,
  readStoredTheme,
  resetThemeForTests,
  setTheme,
} from './use-theme';

type DocumentStub = {
  documentElement: {
    classList: { toggle: (name: string, force?: boolean) => void; contains: (name: string) => boolean };
    setAttribute: (name: string, value: string) => void;
    getAttribute: (name: string) => string | null;
  };
  classes: Set<string>;
  attributes: Map<string, string>;
};

function makeDocumentStub(): DocumentStub {
  const classes = new Set<string>();
  const attributes = new Map<string, string>();
  return {
    classes,
    attributes,
    documentElement: {
      classList: {
        toggle: (name: string, force?: boolean) => {
          const next = force ?? !classes.has(name);
          if (next) classes.add(name);
          else classes.delete(name);
        },
        contains: (name: string) => classes.has(name),
      },
      setAttribute: (name: string, value: string) => attributes.set(name, value),
      getAttribute: (name: string) => attributes.get(name) ?? null,
    },
  };
}

function stubWindowWithStorage(values: Map<string, string>) {
  vi.stubGlobal('window', {
    localStorage: {
      getItem: (key: string) => values.get(key) ?? null,
      setItem: (key: string, value: string) => values.set(key, value),
      removeItem: (key: string) => values.delete(key),
    },
  });
}

describe('theme registry', () => {
  it('exposes one complete entry per theme id with default as the fallback', () => {
    expect(THEMES.map((theme) => theme.id)).toEqual([...THEME_IDS]);
    expect(new Set(THEME_IDS).size).toBe(THEME_IDS.length);
    expect(DEFAULT_THEME_ID).toBe('default');
    for (const theme of THEMES) {
      expect(theme.label.length).toBeGreaterThan(0);
      expect(theme.description.length).toBeGreaterThan(0);
      expect(['light', 'dark']).toContain(theme.appearance);
      expect(theme.preview).toMatch(/linear-gradient/);
      expect(theme.accent).toMatch(/^#/);
    }
  });

  it('rejects ids outside the registry', () => {
    for (const id of THEME_IDS) expect(isThemeId(id)).toBe(true);
    expect(isThemeId('light')).toBe(false);
    expect(isThemeId('agent-diva')).toBe(false);
    expect(isThemeId(null)).toBe(false);
    expect(isThemeId(42)).toBe(false);
  });
});

describe('stored theme', () => {
  beforeEach(() => { resetThemeForTests(); });
  afterEach(() => { vi.unstubAllGlobals(); });

  it('restores a persisted valid theme', () => {
    stubWindowWithStorage(new Map([['vivy.theme', 'miku']]));
    expect(readStoredTheme()).toBe('miku');
  });

  it('falls back to the default theme for missing or invalid values', () => {
    stubWindowWithStorage(new Map([['vivy.theme', 'love']]));
    expect(readStoredTheme()).toBe(DEFAULT_THEME_ID);
    stubWindowWithStorage(new Map());
    expect(readStoredTheme()).toBe(DEFAULT_THEME_ID);
  });

  it('falls back when storage is unavailable', () => {
    expect(readStoredTheme()).toBe(DEFAULT_THEME_ID);
  });
});

describe('theme application', () => {
  let doc: DocumentStub;

  beforeEach(() => {
    resetThemeForTests();
    doc = makeDocumentStub();
    vi.stubGlobal('document', doc);
  });
  afterEach(() => { vi.unstubAllGlobals(); });

  it('sets data-theme and toggles the dark class by appearance', () => {
    applyThemeToDOM('pink');
    expect(doc.attributes.get('data-theme')).toBe('pink');
    expect(doc.classes.has('dark')).toBe(false);

    applyThemeToDOM('miku');
    expect(doc.attributes.get('data-theme')).toBe('miku');
    expect(doc.classes.has('dark')).toBe(true);

    applyThemeToDOM('default');
    expect(doc.attributes.get('data-theme')).toBe('default');
    expect(doc.classes.has('dark')).toBe(false);
  });

  it('setTheme applies to the DOM and persists', () => {
    const values = new Map<string, string>();
    stubWindowWithStorage(values);

    setTheme('miku');
    expect(doc.attributes.get('data-theme')).toBe('miku');
    expect(doc.classes.has('dark')).toBe(true);
    expect(values.get('vivy.theme')).toBe('miku');

    setTheme('pink');
    expect(doc.attributes.get('data-theme')).toBe('pink');
    expect(doc.classes.has('dark')).toBe(false);
    expect(values.get('vivy.theme')).toBe('pink');
  });

  it('setTheme ignores invalid values and repeated values', () => {
    const values = new Map<string, string>();
    stubWindowWithStorage(values);

    setTheme('pink');
    expect(doc.attributes.get('data-theme')).toBe('pink');
    expect(values.get('vivy.theme')).toBe('pink');

    setTheme('neon');
    expect(doc.attributes.get('data-theme')).toBe('pink');
    expect(values.get('vivy.theme')).toBe('pink');

    doc.attributes.delete('data-theme');
    setTheme('pink');
    expect(doc.attributes.has('data-theme')).toBe(false);
  });
});
