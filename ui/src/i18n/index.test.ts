import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import * as localeStore from './index';
import { getLocale, hydrateLocale, localeOptions, resetLocaleForTests, t } from './index';
import { zh } from './zh';
import { en } from './en';
import { CHANNEL_PLATFORMS } from '../components/settings/channel-platforms';
import { CHANNEL_CREDENTIAL_FIELDS } from '../components/settings/channel-schema';

function flatten(value: unknown, prefix = ''): Record<string, string> {
  if (typeof value === 'string') return { [prefix]: value };
  return Object.fromEntries(Object.entries(value as Record<string, unknown>).flatMap(([key, child]) =>
    Object.entries(flatten(child, prefix ? `${prefix}.${key}` : key))));
}

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
  it('has nonempty string leaves, identical keys and named placeholders including arrays', () => {
    const english = flatten(en);
    const chinese = flatten(zh);
    expect(Object.keys(english).sort()).toEqual(Object.keys(chinese).sort());
    for (const key of Object.keys(english)) {
      expect(english[key].trim(), `en:${key}`).not.toBe('');
      expect(chinese[key].trim(), `zh:${key}`).not.toBe('');
      const names = (text: string) => [...text.matchAll(/\{\{\s*(\w+)\s*\}\}/g)].map((match) => match[1]).sort();
      expect(names(english[key]), key).toEqual(names(chinese[key]));
    }
  });

  it.each(['en', 'zh'] as const)('resolves representative surfaces without fallback in %s', (locale) => {
    hydrateLocale(locale);
    const dictionary = flatten(locale === 'en' ? en : zh);
    for (const key of ['nav.settings', 'chat.copy', 'approvals.empty', 'cron.title',
      'trajectory.empty', 'files.empty', 'settingsModel.errors.saveFailed', 'toolsSettings.save',
      'ui.previousPage', 'channels.tutorialBody', 'dashboard.overview', 'lifecycle.speciesTab']) {
      expect(dictionary[key], key).toBeTypeOf('string');
      expect(t(key), key).not.toBe(key);
    }
  });

  it('describes backend-shared language persistence', () => {
    expect(t('language.description')).toContain('workspace');
    hydrateLocale('zh');
    expect(t('language.description')).toContain('工作区');
  });

  it('resolves channel host metadata on access while preserving protocol defaults and brands', () => {
    hydrateLocale('en');
    expect(CHANNEL_PLATFORMS.telegram.quickGuideSteps[0]).toContain('Search');
    expect(CHANNEL_CREDENTIAL_FIELDS.telegram[1].label).toBe('Allowed user IDs');
    expect(CHANNEL_PLATFORMS.feishu.displayName).toBe('飞书');
    hydrateLocale('zh');
    expect(CHANNEL_PLATFORMS.telegram.quickGuideSteps[0]).toContain('搜索');
    expect(CHANNEL_CREDENTIAL_FIELDS.telegram[1].label).toBe('允许的用户 ID');
    expect(CHANNEL_CREDENTIAL_FIELDS.dingtalk.find((field) => field.key === 'dm_policy')?.default).toBe('open');
  });
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
  it.each([
    ['en', 'Every 5 minutes', '5 tools'],
    ['zh', '每 5 分钟', '5 个工具'],
  ] as const)('interpolates cron and settings counts in %s', (locale, schedule, tools) => {
    hydrateLocale(locale);
    expect(t('cron.scheduleFormat.everyMinutes', { count: 5 })).toBe(schedule);
    expect(t('toolsSettings.description', { count: 5 })).toContain(tools);
    expect(t('toolsSettings.activeLabel', { name: 'bash' })).toContain('bash');
  });
  it('uses English with no saved or backend value', () => {
    resetLocaleForTests();
    expect(getLocale()).toBe('en');
    expect(t('notebook.reports')).toBe('Reports');
  });

  it('does not detect the locale from browser languages', async () => {
    vi.stubGlobal('window', { localStorage: { getItem: () => null, setItem: () => undefined } });
    vi.stubGlobal('navigator', { languages: ['zh-CN'], language: 'zh-CN' });
    vi.resetModules();

    const freshLocaleStore = await import('./index');

    expect(freshLocaleStore.getLocale()).toBe('en');
  });

  it('returns the key itself when no dictionary has it', () => {
    expect(t('does.not.exist')).toBe('does.not.exist');
  });

  it('interpolates {{params}}', () => {
    expect(t('masks.current', { name: 'Programmer' })).toBe('Current: Programmer');
    expect(t('masks.useMask', { name: 'Researcher' })).toBe('Use “Researcher”');
  });

  it('indexes array leaves by dotted path', () => {
    expect(t('demo.tokens.timeline.months.0')).toBe('Jan');
    expect(t('demo.tokens.timeline.months.11')).toBe('Dec');
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

  it('backend hydration replaces a stale browser cache', () => {
    storage['vivy.language'] = 'zh';
    expect(localeStore).toHaveProperty('hydrateLocale');
    const hydrateLocale = (localeStore as typeof localeStore & {
      hydrateLocale: (locale: 'en' | 'zh') => void;
    }).hydrateLocale;
    hydrateLocale('en');
    expect(getLocale()).toBe('en');
    expect(storage['vivy.language']).toBe('en');
    expect((document.documentElement as { lang: string }).lang).toBe('en');
    expect(t('notebook.reports')).toBe('Reports');
  });

  it('locale option labels follow the current language while nativeLabel stays constant', () => {
    expect(localeStore).toHaveProperty('hydrateLocale');
    const hydrateLocale = (localeStore as typeof localeStore & {
      hydrateLocale: (locale: 'en' | 'zh') => void;
    }).hydrateLocale;
    hydrateLocale('zh');
    const zhLabel = localeOptions().find((option) => option.id === 'zh')!.label;
    hydrateLocale('en');
    const enLabel = localeOptions().find((option) => option.id === 'zh')!.label;
    expect(enLabel).not.toBe(zhLabel);
    expect(localeOptions().find((option) => option.id === 'zh')!.nativeLabel).toBe('简体中文');
    expect(localeOptions().find((option) => option.id === 'en')!.nativeLabel).toBe('English');
  });
});
