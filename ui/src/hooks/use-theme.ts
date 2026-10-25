import { useSyncExternalStore } from 'react';

// 界面皮肤单一入口：主题注册表、data-theme 应用、localStorage 持久化。
// 色值本体只在 ../styles.css；本文件不持有任何颜色字面量。选择卡展示文案经
// i18n 词条 themes.<id>.* 解析，预览色块由 [data-theme-preview] 真实令牌渲染。
//
// 存储 id 保持 default|dark|pink|miku 以零迁移兼容旧安装；对外名称是
// Vivy 浅色/深色、Diva、Miku，以及三条新家族 rice/fluor/indigo。
export const THEME_IDS = ['default', 'dark', 'pink', 'miku', 'rice', 'fluor', 'indigo'] as const;
export type ThemeId = (typeof THEME_IDS)[number];
export type ThemeAppearance = 'light' | 'dark';

export type VivyTheme = {
  id: ThemeId;
  appearance: ThemeAppearance;
};

export const THEMES: readonly VivyTheme[] = [
  { id: 'default', appearance: 'light' },
  { id: 'dark', appearance: 'dark' },
  { id: 'pink', appearance: 'light' },
  { id: 'miku', appearance: 'dark' },
  { id: 'rice', appearance: 'light' },
  { id: 'fluor', appearance: 'dark' },
  { id: 'indigo', appearance: 'dark' },
];

export const DEFAULT_THEME_ID: ThemeId = 'default';
const STORAGE_KEY = 'vivy.theme';
// 预绘制脚本（index.html）只读外观，不认识任何主题 id；setTheme 会同步写入。
export const APPEARANCE_STORAGE_KEY = 'vivy.appearance';

export function isThemeId(value: unknown): value is ThemeId {
  return typeof value === 'string' && (THEME_IDS as readonly string[]).includes(value);
}

export function themeAppearance(id: ThemeId): ThemeAppearance {
  return THEMES.find((item) => item.id === id)?.appearance ?? 'light';
}

export function readStoredTheme(): ThemeId {
  try {
    const saved = window.localStorage.getItem(STORAGE_KEY);
    return isThemeId(saved) ? saved : DEFAULT_THEME_ID;
  } catch {
    return DEFAULT_THEME_ID;
  }
}

/** 把主题写成根元素上的两个属性；不写内联样式，避免与模块主题互相覆盖。 */
export function applyThemeToDOM(id: ThemeId): void {
  if (typeof document === 'undefined') return;
  const appearance = themeAppearance(id);
  const root = document.documentElement;
  root.setAttribute('data-theme', id);
  root.setAttribute('data-appearance', appearance);
  // .dark 只驱动 Tailwind dark: 变体；令牌值由 [data-appearance] 块提供。
  root.classList.toggle('dark', appearance === 'dark');
}

function persistTheme(id: ThemeId): void {
  try {
    window.localStorage.setItem(STORAGE_KEY, id);
    window.localStorage.setItem(APPEARANCE_STORAGE_KEY, themeAppearance(id));
  } catch {
    /* storage 不可用（测试/隐私模式）时主题仍已应用 */
  }
}

let currentTheme: ThemeId = readStoredTheme();
const listeners = new Set<() => void>();

applyThemeToDOM(currentTheme);
// 旧安装只有 vivy.theme；在首次加载时补写外观，后续硬刷新才能走无闪烁的预绘制路径。
persistTheme(currentTheme);

function emit(): void {
  for (const listener of listeners) listener();
}

/** 设置全局主题：立即应用到 DOM、持久化并通知所有订阅者。非法值被忽略。 */
export function setTheme(value: string): void {
  if (!isThemeId(value) || value === currentTheme) return;
  currentTheme = value;
  applyThemeToDOM(value);
  persistTheme(value);
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
