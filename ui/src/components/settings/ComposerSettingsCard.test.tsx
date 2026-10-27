// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { hydrateLocale } from '@/i18n';
import { ComposerSettingsCard } from './ComposerSettingsCard';

const view = vi.hoisted(() => ({
  settings: {
    provider: 'openai', default_model: 'gpt-4o-mini', base_url: '', read_only: false,
    config_provider: '', config_model: '', locale: 'en' as const, generation_locale: 'en' as const,
    workspace_locale: '' as const, locale_read_only: false,
    busy_enter: 'queue' as 'queue' | 'steer',
  },
  saveSettings: vi.fn().mockResolvedValue(undefined),
}));

vi.mock('@/lib/store', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/store')>();
  return {
    ...original,
    useVivyStore: (selector: (state: typeof view) => unknown) => selector(view),
  };
});

describe('ComposerSettingsCard', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    hydrateLocale('en');
    view.saveSettings.mockClear();
    view.settings.busy_enter = 'queue';
    view.settings.read_only = false;
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    hydrateLocale('en');
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  function button(label: string): HTMLButtonElement {
    const match = [...container.querySelectorAll('button')].find((node) => node.textContent === label);
    expect(match, `button: ${label}`).toBeDefined();
    return match!;
  }

  async function render() {
    await act(async () => root.render(<ComposerSettingsCard />));
  }

  it('shows the stored mode and saves the flipped preference', async () => {
    await render();
    expect(container.querySelector('[data-testid="busy-enter-effective"]')?.textContent).toBe('In force now: Queue');
    await act(async () => (container.querySelector('[data-testid="busy-enter-steer"]') as HTMLButtonElement).click());
    await act(async () => button('Save send mode').click());
    expect(view.saveSettings).toHaveBeenCalledTimes(1);
    expect(view.saveSettings.mock.calls[0][0].busy_enter).toBe('steer');
    expect(container.querySelector('[aria-live="polite"]')?.textContent).toBe('Send mode saved; the composer uses it immediately.');
  });

  it('keeps the saved mode when nothing changed', async () => {
    await render();
    expect(button('Save send mode').disabled).toBe(true);
  });

  it('treats an empty stored value as the queue default', async () => {
    view.settings.busy_enter = '' as never;
    await render();
    expect(container.querySelector('[data-testid="busy-enter-effective"]')?.textContent).toBe('In force now: Queue');
    expect(container.querySelector('[data-testid="busy-enter-queue"]')?.getAttribute('aria-checked')).toBe('true');
    view.settings.busy_enter = 'queue';
  });

  it('disables the controls when settings are read-only', async () => {
    view.settings.read_only = true;
    await render();
    expect(button('Save send mode').disabled).toBe(true);
    expect((container.querySelector('[data-testid="busy-enter-steer"]') as HTMLButtonElement).disabled).toBe(true);
    view.settings.read_only = false;
  });
});
