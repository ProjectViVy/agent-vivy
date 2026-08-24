import { useSyncExternalStore } from 'react';

// 界面皮肤单一入口：主题注册表、data-theme 应用、localStorage 持久化。
// 主题色值本体在 ./../styles.css 的 [data-theme="..."] 块中，本文件只持有
// 选择卡所需的展示数据（预览色板为字面量：选择卡需同时展示所有皮肤，
// 不能跟随当前主题 token）。
export const THEME_IDS = ['default', 'pink', 'dark', 'miku'] as const;
export type ThemeId = (typeof THEME_IDS)[number];
export type ThemeAppearance = 'light' | 'dark';

export type VivyTheme = {
  id: ThemeId;
  label: string;
  description: string;
  appearance: ThemeAppearance;
  /** 选择卡预览背景（CSS background 字面量） */
  preview: string;
  /** 选择卡强调色（CSS color 字面量） */
  accent: string;
};

export const THEMES: readonly VivyTheme[] = [
  {
    id: 'default',
    label: 'Vivy 蓝',
    description: '默认的浅色蓝白界面。',
    appearance: 'light',
    preview: 'linear-gradient(135deg, #f6f7fb 0%, #e7ebf8 55%, #d9e1f6 100%)',
    accent: '#3e63dd',
  },
  {
    id: 'pink',
    label: '简约粉白',
    description: '白底细粉线的轻量界面，移植自 Agent-Diva。',
    appearance: 'light',
    preview: 'linear-gradient(135deg, #ffffff 0%, #fff5f7 45%, #ffe4ef 100%)',
    accent: '#ec4899',
  },
  {
    id: 'dark',
    label: '深蓝夜色',
    description: '低亮度蓝黑的深色界面。',
    appearance: 'dark',
    preview: 'linear-gradient(135deg, #14172a 0%, #1e2542 55%, #2a3355 100%)',
    accent: '#6f8dff',
  },
  {
    id: 'miku',
    label: 'Miku 青',
    description: '青绿应援色的深色界面，移植自 Agent-Diva。',
    appearance: 'dark',
    preview: 'linear-gradient(135deg, #0d1117 0%, #122129 45%, #123b39 100%)',
    accent: '#39c5bb',
  },
];

export const DEFAULT_THEME_ID: ThemeId = 'default';
const STORAGE_KEY = 'vivy.theme';

export function isThemeId(value: unknown): value is ThemeId {
  return typeof value === 'string' && (THEME_IDS as readonly string[]).includes(value);
}

export function readStoredTheme(): ThemeId {
  try {
    const saved = window.localStorage.getItem(STORAGE_KEY);
    return isThemeId(saved) ? saved : DEFAULT_THEME_ID;
  } catch {
    return DEFAULT_THEME_ID;
  }
}

export function applyThemeToDOM(id: ThemeId): void {
  if (typeof document === 'undefined') return;
  const theme = THEMES.find((item) => item.id === id);
  const root = document.documentElement;
  root.setAttribute('data-theme', id);
  // .dark 类只负责驱动 Tailwind 的 dark: 变体；token 值由 [data-theme] 块提供。
  root.classList.toggle('dark', theme?.appearance === 'dark');
}

let currentTheme: ThemeId = readStoredTheme();
const listeners = new Set<() => void>();

applyThemeToDOM(currentTheme);

function emit(): void {
  for (const listener of listeners) listener();
}

/** 设置全局主题：立即应用到 DOM、持久化并通知所有订阅者。非法值被忽略。 */
export function setTheme(value: string): void {
  if (!isThemeId(value) || value === currentTheme) return;
  currentTheme = value;
  applyThemeToDOM(value);
  try {
    window.localStorage.setItem(STORAGE_KEY, value);
  } catch {
    /* storage 不可用（测试/隐私模式）时主题仍已应用 */
  }
  emit();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function useTheme(): { theme: ThemeId; themes: readonly VivyTheme[]; setTheme: typeof setTheme } {
  const theme = useSyncExternalStore(subscribe, () => currentTheme, () => DEFAULT_THEME_ID);
  return { theme, themes: THEMES, setTheme };
}

/** 仅供测试：重置模块内当前主题，不触碰 DOM 与 localStorage。 */
export function resetThemeForTests(): void {
  currentTheme = DEFAULT_THEME_ID;
}
