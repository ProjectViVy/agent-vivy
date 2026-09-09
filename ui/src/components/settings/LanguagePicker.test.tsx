import type { ReactElement, ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const view = vi.hoisted(() => ({
  locale: 'en' as 'en' | 'zh',
  settings: {
    locale: 'en' as 'en' | 'zh',
    locale_read_only: false,
  } as { locale: 'en' | 'zh'; locale_read_only: boolean } | null,
  settingsPhase: 'ready',
  settingsError: null as string | null,
  saveLocale: vi.fn().mockResolvedValue(undefined),
}));

vi.mock('@/lib/store', () => ({
  useVivyStore: (selector: (state: typeof view) => unknown) => selector(view),
}));

vi.mock('@/i18n', () => ({
  useTranslation: () => ({
    t: (key: string) => ({
      'language.title': 'Language',
      'language.description': 'Choose the interface language.',
      'language.currentLanguage': 'Current language',
      'language.switchHint': 'Saved for every interface.',
      'settings.themeSelected': 'Selected',
      'settingsModel.errors.saveFailed': 'Save failed; please retry',
    })[key] ?? key,
    locale: view.locale,
    locales: () => [
      { id: 'en', nativeLabel: 'English', label: 'English', code: 'EN' },
      { id: 'zh', nativeLabel: '简体中文', label: 'Simplified Chinese', code: 'CN' },
    ],
  }),
}));

import { LanguagePicker } from './LanguagePicker';

type TestElement = ReactElement<Record<string, unknown> & { children?: ReactNode }>;

function elements(node: ReactNode, type: string): TestElement[] {
  if (!node || typeof node !== 'object' || !('type' in node)) return [];
  const element = node as TestElement;
  const matches = element.type === type ? [element] : [];
  const children = Array.isArray(element.props.children) ? element.props.children : [element.props.children];
  return [...matches, ...children.flatMap((child) => elements(child, type))];
}

function textContent(node: ReactNode): string {
  if (typeof node === 'string' || typeof node === 'number') return String(node);
  if (!node || typeof node !== 'object' || !('type' in node)) return '';
  const children = (node as ReactElement<{ children?: ReactNode }>).props.children;
  return (Array.isArray(children) ? children : [children]).map(textContent).join('');
}

describe('LanguagePicker', () => {
  beforeEach(() => {
    view.locale = 'en';
    view.settings = { locale: 'en', locale_read_only: false };
    view.settingsPhase = 'ready';
    view.settingsError = null;
    view.saveLocale.mockClear();
  });

  it('saves through the store and marks the backend-confirmed locale selected', () => {
    view.locale = 'en';
    view.settings!.locale = 'zh';
    const buttons = elements(LanguagePicker(), 'button');

    expect(buttons).toHaveLength(2);
    expect(buttons[0].props['aria-pressed']).toBe(false);
    expect(buttons[1].props['aria-pressed']).toBe(true);

    (buttons[1].props as { onClick: () => void }).onClick();
    expect(view.saveLocale).toHaveBeenCalledWith('zh');
  });

  it('disables both controls without marking a cached locale selected until settings is known', () => {
    view.locale = 'zh';
    view.settings = null;
    const buttons = elements(LanguagePicker(), 'button');

    expect(buttons).toHaveLength(2);
    expect(buttons.every((button) => button.props.disabled)).toBe(true);
    expect(buttons.every((button) => button.props['aria-pressed'] === false)).toBe(true);
  });

  it('disables language changes while saving or when locale settings are read-only', () => {
    view.settingsPhase = 'processing';
    expect(elements(LanguagePicker(), 'button').every((button) => button.props.disabled)).toBe(true);

    view.settingsPhase = 'ready';
    view.settings!.locale_read_only = true;
    expect(elements(LanguagePicker(), 'button').every((button) => button.props.disabled)).toBe(true);
  });

  it('disables an already-hydrated picker while authoritative settings are loading', () => {
    view.settings = { locale: 'en', locale_read_only: false };
    view.settingsPhase = 'loading';

    expect(elements(LanguagePicker(), 'button').every((button) => button.props.disabled)).toBe(true);
  });

  it('shows a translated save error with the backend detail', () => {
    view.settingsError = 'settings are read-only';
    const alerts = elements(LanguagePicker(), 'p').filter((element) => element.props.role === 'alert');

    expect(alerts).toHaveLength(1);
    expect(textContent(alerts[0])).toBe('Save failed; please retry: settings are read-only');
  });
});
