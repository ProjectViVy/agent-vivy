// @vitest-environment happy-dom
import { afterEach, expect, it, vi } from 'vitest';
import { hydrateLocale } from '../i18n';
import { initRevealEngine } from './reveal-engine';

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  document.getElementById('reveal-engine-style')?.remove();
  document.documentElement.classList.remove('reveal-ready');
  hydrateLocale('en');
  window.localStorage.clear();
});

it.each([
  ['en', '[reveal-engine] Initialization failed; all content remains visible:'],
  ['zh', '[reveal-engine] 初始化失败，已降级为全部可见：'],
] as const)('localizes the failure diagnostic in %s and leaves content visible', (locale, warning) => {
  hydrateLocale(locale);
  const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
  vi.stubGlobal('IntersectionObserver', class {
    constructor() { throw new Error('observer unavailable'); }
  });

  initRevealEngine();

  expect(warn).toHaveBeenCalledExactlyOnceWith(warning, 'observer unavailable');
  expect(document.documentElement.classList.contains('reveal-ready')).toBe(false);
});
