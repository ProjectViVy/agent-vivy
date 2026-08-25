import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { getLocale, localeOptions, resetLocaleForTests, setLocale, t } from './index';
import { zh } from './zh';
import { en } from './en';

/** 收集词典的全部叶子路径；数组以 <array:长度> 标注，用于校验 en 与 zh 结构/长度一致。 */
function leafPaths(value: unknown, prefix = ''): string[] {
  if (Array.isArray(value)) {
    return [`${prefix}<array:${value.length}>`];
  }
  if (value !== null && typeof value === 'object') {
    return Object.keys(value).flatMap((key) =>
      leafPaths((value as Record<string, unknown>)[key], prefix ? `${prefix}.${key}` : key),
    );
  }
  return [prefix];
}

afterEach(() => {
  resetLocaleForTests();
  vi.unstubAllGlobals();
});

describe('i18n dictionary parity', () => {
  it('zh and en share the same leaf keys and array lengths', () => {
    const zhPaths = leafPaths(zh).sort();
    const enPaths = leafPaths(en).sort();
    expect(enPaths).toEqual(zhPaths);
    expect(zhPaths.length).toBeGreaterThan(100);
  });

  it('every zh leaf is a non-empty string', () => {
    for (const key of leafPaths(zh)) {
      if (key.endsWith('>')) continue;
      const value = t(key);
      expect(value, key).not.toBe('');
    }
  });
});

describe('t() lookup', () => {
  it('defaults to zh', () => {
    expect(getLocale()).toBe('zh');
    expect(t('notebook.reports')).toBe('报告');
  });

  it('falls back to zh when the current locale lacks a key', () => {
    setLocale('en');
    expect(t('notebook.reports')).toBe('Reports');
    resetLocaleForTests();
  });

  it('returns the key itself when no dictionary has it', () => {
    expect(t('does.not.exist')).toBe('does.not.exist');
  });

  it('interpolates {{params}}', () => {
    expect(t('masks.current', { name: '程序员' })).toBe('当前：程序员');
    expect(t('masks.useMask', { name: '研究员' })).toBe('使用「研究员」');
  });

  it('indexes array leaves by dotted path', () => {
    expect(t('demo.tokens.timeline.months.0')).toBe('1月');
    expect(t('demo.tokens.timeline.months.11')).toBe('12月');
  });
});

describe('locale switching', () => {
  let storage: Record<string, string>;
  let lang: string;

  beforeEach(() => {
    storage = {};
    lang = '';
    vi.stubGlobal('window', {
      localStorage: {
        getItem: (key: string) => storage[key] ?? null,
        setItem: (key: string, value: string) => { storage[key] = value; },
      },
      dispatchEvent: () => true,
    });
    vi.stubGlobal('document', {
      documentElement: { lang: '' },
      getElementById: () => null,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      createElement: () => ({ style: {} }),
    });
  });

  it('setLocale persists to localStorage and updates the DOM lang', () => {
    setLocale('en');
    expect(getLocale()).toBe('en');
    expect(storage['vivy.language']).toBe('en');
    expect((document.documentElement as { lang: string }).lang).toBe('en');
    expect(t('notebook.reports')).toBe('Reports');
  });

  it('locale option labels follow the current language while nativeLabel stays constant', () => {
    setLocale('zh');
    const zhLabel = localeOptions().find((option) => option.id === 'zh')!.label;
    setLocale('en');
    const enLabel = localeOptions().find((option) => option.id === 'zh')!.label;
    expect(enLabel).not.toBe(zhLabel);
    expect(localeOptions().find((option) => option.id === 'zh')!.nativeLabel).toBe('简体中文');
    expect(localeOptions().find((option) => option.id === 'en')!.nativeLabel).toBe('English');
  });
});
