import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  APPEARANCE_STORAGE_KEY,
  DEFAULT_THEME_ID,
  THEMES,
  THEME_IDS,
  applyThemeToDOM,
  isThemeId,
  readStoredTheme,
  resetThemeForTests,
  setTheme,
  themeAppearance,
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
      expect(['light', 'dark']).toContain(theme.appearance);
      // Colour values live in styles.css only; the registry stays structural.
      expect(Object.keys(theme).sort()).toEqual(['appearance', 'id']);
    }
  });

  it('keeps the legacy storage ids so existing installs do not migrate', () => {
    for (const id of ['default', 'dark', 'pink', 'miku']) expect(isThemeId(id)).toBe(true);
    expect(themeAppearance('dark')).toBe('dark');
    expect(themeAppearance('miku')).toBe('dark');
    expect(themeAppearance('default')).toBe('light');
    expect(themeAppearance('rice')).toBe('light');
  });

  it('provides a display label and description for every theme through i18n', async () => {
    const { t } = await import('../i18n');
    for (const id of THEME_IDS) {
      expect(t(`themes.${id}.label`).length).toBeGreaterThan(0);
      expect(t(`themes.${id}.description`).length).toBeGreaterThan(0);
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

  it('sets data-theme and data-appearance and toggles the dark class', () => {
    applyThemeToDOM('pink');
    expect(doc.attributes.get('data-theme')).toBe('pink');
    expect(doc.attributes.get('data-appearance')).toBe('light');
    expect(doc.classes.has('dark')).toBe(false);

    applyThemeToDOM('miku');
    expect(doc.attributes.get('data-theme')).toBe('miku');
    expect(doc.attributes.get('data-appearance')).toBe('dark');
    expect(doc.classes.has('dark')).toBe(true);

    applyThemeToDOM('default');
    expect(doc.attributes.get('data-theme')).toBe('default');
    expect(doc.attributes.get('data-appearance')).toBe('light');
    expect(doc.classes.has('dark')).toBe(false);
  });

  it('never writes inline style properties on the root element', () => {
    const root = doc.documentElement as unknown as { style?: { setProperty?: () => void } };
    expect(root.style).toBeUndefined();
  });

  it('setTheme applies to the DOM and persists theme plus appearance', () => {
    const values = new Map<string, string>();
    stubWindowWithStorage(values);

    setTheme('miku');
    expect(doc.attributes.get('data-theme')).toBe('miku');
    expect(doc.classes.has('dark')).toBe(true);
    expect(values.get('vivy.theme')).toBe('miku');
    expect(values.get(APPEARANCE_STORAGE_KEY)).toBe('dark');

    setTheme('pink');
    expect(doc.attributes.get('data-theme')).toBe('pink');
    expect(doc.classes.has('dark')).toBe(false);
    expect(values.get('vivy.theme')).toBe('pink');
    expect(values.get(APPEARANCE_STORAGE_KEY)).toBe('light');
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
