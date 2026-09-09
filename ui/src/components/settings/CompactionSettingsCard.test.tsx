// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { hydrateLocale } from '@/i18n';
import * as api from '@/lib/api';
import { CompactionSettingsCard } from './CompactionSettingsCard';

const view = vi.hoisted(() => ({
  settings: {
    provider: 'openai', default_model: 'gpt-4o-mini', base_url: '', read_only: false,
    config_provider: '', config_model: '', locale: 'en' as const, generation_locale: 'en' as const,
    workspace_locale: '' as const, locale_read_only: false,
    compaction: {
      enabled: true, max_tokens: 0, trigger_percent: 80, keep_recent: 12,
      config_enabled: true, config_max_tokens: 0, config_trigger_percent: 80, config_keep_recent: 12,
    },
  },
  sessionContext: null,
  activeSessionId: 'session-1',
  currentRun: null,
  backgroundRuns: [],
  saveSettings: vi.fn().mockResolvedValue(undefined),
  compactSession: vi.fn(),
  loadSessionContext: vi.fn().mockResolvedValue(undefined),
  loadBackgroundRuns: vi.fn().mockResolvedValue(undefined),
}));

vi.mock('@/lib/store', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/store')>();
  return {
    ...original,
    useVivyStore: (selector: (state: typeof view) => unknown) => selector(view),
  };
});

describe('CompactionSettingsCard mounted feedback', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    hydrateLocale('en');
    vi.spyOn(api, 'listSessionCompactions').mockResolvedValue({ compactions: [] });
    view.saveSettings.mockClear();
    view.compactSession.mockReset();
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    hydrateLocale('en');
    localStorage.clear();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  function button(label: string): HTMLButtonElement {
    const match = [...container.querySelectorAll('button')].find((node) => node.textContent === label);
    expect(match, `button: ${label}`).toBeDefined();
    return match!;
  }

  async function render() {
    await act(async () => root.render(<CompactionSettingsCard />));
  }

  it('retranslates saved-settings feedback without remounting', async () => {
    await render();
    await act(async () => button('Save config').click());
    const feedback = container.querySelector('[aria-live="polite"]');
    expect(feedback?.textContent).toBe('Compaction config saved; it applies to the next run.');

    await act(async () => hydrateLocale('zh'));
    expect(container.querySelector('[aria-live="polite"]')).toBe(feedback);
    expect(feedback?.textContent).toBe('压缩配置已保存，将作用于下一次运行。');
    expect(view.saveSettings).toHaveBeenCalledTimes(1);
  });

  it('retranslates completed feedback from raw token counts without remounting', async () => {
    view.compactSession.mockResolvedValue({ before_tokens: 12345, after_tokens: 6789, folded_messages: 4, skipped: false });
    await render();
    await act(async () => button('Compact now').click());
    const feedback = [...container.querySelectorAll('[aria-live="polite"]')].at(-1);
    expect(feedback?.textContent).toBe('Compacted: 12,345 → 6,789 tokens.');

    await act(async () => hydrateLocale('zh'));
    expect([...container.querySelectorAll('[aria-live="polite"]')].at(-1)).toBe(feedback);
    expect(feedback?.textContent).toBe('压缩完成：12,345 → 6,789 tokens。');
    expect(view.compactSession).toHaveBeenCalledTimes(1);
  });

  it('preserves raw backend errors across locale changes', async () => {
    view.compactSession.mockRejectedValue(new Error('后端拒绝：session is busy'));
    await render();
    await act(async () => button('Compact now').click());
    const feedback = [...container.querySelectorAll('[aria-live="polite"]')].at(-1);
    expect(feedback?.textContent).toBe('后端拒绝：session is busy');

    await act(async () => hydrateLocale('zh'));
    expect([...container.querySelectorAll('[aria-live="polite"]')].at(-1)).toBe(feedback);
    expect(feedback?.textContent).toBe('后端拒绝：session is busy');
    expect(view.compactSession).toHaveBeenCalledTimes(1);
  });
});
