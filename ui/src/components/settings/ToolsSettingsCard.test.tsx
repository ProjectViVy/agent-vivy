// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { hydrateLocale } from '@/i18n';
import * as api from '@/lib/api';
import { ToolsSettingsCard } from './ToolsSettingsCard';

function catalog(active: string[], overlayWritten = false): api.ToolsCatalogView {
  return {
    tools: ['read_file', 'search_files'].map((name) => ({
      name,
      description: `Backend description for ${name}`,
      readonly: true,
      active: active.includes(name),
    })),
    active,
    config_enabled: ['read_file'],
    overlay_written: overlayWritten,
  };
}

describe('ToolsSettingsCard mounted feedback', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    hydrateLocale('en');
    vi.spyOn(api, 'listTools').mockResolvedValue(catalog(['read_file']));
    vi.spyOn(api, 'setActiveTools');
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

  async function toggleAndSave(tool: string) {
    await act(async () => root.render(<ToolsSettingsCard />));
    const toggle = container.querySelector<HTMLButtonElement>(`[role="switch"][aria-label^="${tool} "]`);
    expect(toggle).not.toBeNull();
    await act(async () => toggle!.click());
    const save = button('Save tool configuration');
    expect(save.disabled).toBe(false);
    await act(async () => save.click());
  }

  it.each([
    {
      tool: 'read_file',
      active: [],
      english: 'Saved: chat-only mode (0 tools).',
      chinese: '已保存：纯对话模式（0 个工具）。',
    },
    {
      tool: 'search_files',
      active: ['read_file', 'search_files'],
      english: 'Saved: 2 tools active; applies to the next run.',
      chinese: '已保存：2 个工具激活，对下一次运行生效。',
    },
  ])('retranslates saved feedback without remounting ($active)', async ({ tool, active, english, chinese }) => {
    vi.mocked(api.setActiveTools).mockResolvedValue(catalog(active, true));
    await toggleAndSave(tool);
    expect(api.setActiveTools).toHaveBeenCalledExactlyOnceWith(active);
    const feedback = container.querySelector('[aria-live="polite"]');
    expect(feedback?.textContent).toBe(english);
    expect(button('Save tool configuration').disabled).toBe(true);

    await act(async () => hydrateLocale('zh'));
    expect(button('保存工具配置').disabled).toBe(true);
    expect(container.querySelector('[aria-live="polite"]')).toBe(feedback);
    expect(feedback?.textContent).toBe(chinese);

    await act(async () => hydrateLocale('en'));
    expect(feedback?.textContent).toBe(english);
    expect(api.listTools).toHaveBeenCalledTimes(1);
    expect(api.setActiveTools).toHaveBeenCalledTimes(1);
  });

  it.each([
    { error: new Error('后端拒绝：settings.yaml is read-only'), message: '后端拒绝：settings.yaml is read-only' },
    { error: 'toolsSettings.savedEmpty', message: 'toolsSettings.savedEmpty' },
  ])('preserves raw backend save errors across locale changes ($message)', async ({ error, message }) => {
    vi.mocked(api.setActiveTools).mockRejectedValue(error);
    await toggleAndSave('read_file');
    expect(api.setActiveTools).toHaveBeenCalledExactlyOnceWith([]);
    const feedback = container.querySelector('[aria-live="polite"]');
    expect(feedback?.textContent).toBe(message);

    await act(async () => hydrateLocale('zh'));
    expect(button('保存工具配置').disabled).toBe(false);
    expect(container.querySelector('[aria-live="polite"]')).toBe(feedback);
    expect(feedback?.textContent).toBe(message);
    expect(api.listTools).toHaveBeenCalledTimes(1);
    expect(api.setActiveTools).toHaveBeenCalledTimes(1);
  });
});
