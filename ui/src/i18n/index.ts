// 界面语言单一入口：语言注册表、localStorage 持久化、t() 查询与 useTranslation。
// 词典本体在 ./zh.ts（权威）与 ./en.ts（结构必须与 zh 完全一致，由 i18n.test.ts 校验）。
// 与 hooks/use-theme.ts 同构：模块级状态 + useSyncExternalStore，不引入外部依赖。
import { useSyncExternalStore } from 'react';
import { zh } from './zh';
import { en } from './en';

export const LOCALES = ['zh', 'en'] as const;
export type Locale = (typeof LOCALES)[number];

export type LocaleOption = {
  id: Locale;
  /** 语言自身名称（切换后不变，供选择卡展示） */
  nativeLabel: string;
  /** 当前界面语言下的名称 */
  label: string;
  code: string;
};

const STORAGE_KEY = 'vivy.language';
const DEFAULT_LOCALE: Locale = 'zh';

export type Dictionary = typeof zh;

const DICTS: Record<Locale, Dictionary> = { zh, en };

/** 选择卡展示数据：label 跟随当前语言，nativeLabel 恒定。 */
export function localeOptions(): LocaleOption[] {
  return [
    { id: 'zh', nativeLabel: '简体中文', label: t('language.names.zh'), code: 'CN' },
    { id: 'en', nativeLabel: 'English', label: t('language.names.en'), code: 'EN' },
  ];
}

export function isLocale(value: unknown): value is Locale {
  return typeof value === 'string' && (LOCALES as readonly string[]).includes(value);
}

function detectInitialLocale(): Locale {
  try {
    const saved = window.localStorage.getItem(STORAGE_KEY);
    if (isLocale(saved)) return saved;
  } catch {
    /* storage 不可用（测试/隐私模式）时回退浏览器语言 */
  }
  try {
    const languages = typeof navigator !== 'undefined' && navigator.languages?.length
      ? navigator.languages
      : typeof navigator !== 'undefined' ? [navigator.language] : [];
    for (const tag of languages) {
      const normalized = tag.toLowerCase();
      if (normalized.startsWith('zh')) return 'zh';
      if (normalized.startsWith('en')) return 'en';
    }
  } catch {
    /* 无 navigator 时使用默认语言 */
  }
  return DEFAULT_LOCALE;
}

function applyLocaleToDOM(locale: Locale): void {
  if (typeof document === 'undefined') return;
  document.documentElement.lang = locale === 'zh' ? 'zh-CN' : 'en';
}

let currentLocale: Locale = detectInitialLocale();
const listeners = new Set<() => void>();

applyLocaleToDOM(currentLocale);

function emit(): void {
  for (const listener of listeners) listener();
}

/** 设置全局界面语言：立即应用、持久化并通知所有订阅者。非法值被忽略。 */
export function setLocale(locale: Locale): void {
  if (!isLocale(locale) || locale === currentLocale) return;
  currentLocale = locale;
  applyLocaleToDOM(locale);
  try {
    window.localStorage.setItem(STORAGE_KEY, locale);
  } catch {
    /* storage 不可用时语言仍已应用 */
  }
  emit();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function getLocale(): Locale {
  return currentLocale;
}

/** toLocaleString 等 Intl API 使用的语言标签。 */
export function dateTimeLocale(): string {
  return currentLocale === 'zh' ? 'zh-CN' : 'en-US';
}

function lookup(dict: unknown, key: string): string | undefined {
  let node: unknown = dict;
  for (const part of key.split('.')) {
    if (typeof node !== 'object' || node === null) return undefined;
    node = (node as Record<string, unknown>)[part];
  }
  return typeof node === 'string' ? node : undefined;
}

function interpolate(template: string, params?: Record<string, string | number>): string {
  if (!params) return template;
  return template.replace(/\{\{(\w+)\}\}/g, (match, name: string) => (
    name in params ? String(params[name]) : match
  ));
}

/**
 * 按点号路径查询词条并插值 {{param}}。当前语言缺项时回退 zh，仍缺则返回 key 本身。
 * 非组件环境（错误消息、工具函数）直接调用；组件内改用 useTranslation().t，
 * 以便语言切换时重渲染。
 */
export function t(key: string, params?: Record<string, string | number>): string {
  const value = lookup(DICTS[currentLocale], key) ?? lookup(DICTS[DEFAULT_LOCALE], key);
  return value === undefined ? interpolate(key, params) : interpolate(value, params);
}

export function useTranslation(): {
  t: typeof t;
  locale: Locale;
  setLocale: typeof setLocale;
  locales: () => LocaleOption[];
} {
  useSyncExternalStore(subscribe, () => currentLocale, () => DEFAULT_LOCALE);
  return { t, locale: currentLocale, setLocale, locales: localeOptions };
}

/** 仅供测试：重置模块内当前语言，不触碰 DOM 与 localStorage。 */
export function resetLocaleForTests(): void {
  currentLocale = DEFAULT_LOCALE;
}
