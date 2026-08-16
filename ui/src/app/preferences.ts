export type Locale = "zh-CN" | "en";
export type ThemeMode = "system" | "light" | "dark";

const LOCALE_KEY = "vivy.locale";
const THEME_KEY = "vivy.theme";

function systemLocale(): Locale {
  return navigator.language.toLowerCase().startsWith("zh") ? "zh-CN" : "en";
}

export function loadLocale(): Locale {
  const value = window.localStorage.getItem(LOCALE_KEY);
  return value === "zh-CN" || value === "en" ? value : systemLocale();
}

export function loadTheme(): ThemeMode {
  const value = window.localStorage.getItem(THEME_KEY);
  return value === "system" || value === "light" || value === "dark" ? value : "system";
}

export function saveLocale(locale: Locale): void {
  window.localStorage.setItem(LOCALE_KEY, locale);
}

export function saveTheme(theme: ThemeMode): void {
  window.localStorage.setItem(THEME_KEY, theme);
}

export function applyTheme(theme: ThemeMode): void {
  const root = document.documentElement;
  root.dataset.theme = theme;
  root.style.colorScheme = theme === "system" ? "light dark" : theme;
}

export function nextTheme(theme: ThemeMode): ThemeMode {
  if (theme === "system") return "light";
  if (theme === "light") return "dark";
  return "system";
}
