// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { hydrateLocale } from '@/i18n';
import type { ProviderCatalogEntry, ProviderEntry, ProviderProfileStatus } from '@/lib/api';
import { ModelSettingsCard } from './ModelSettingsCard';

/**
 * PROV-P4 目录渲染的组件级回归（RED 场景）：
 *  - 目录未到达（无数据 + idle/loading）时不渲染任何厂商行，只渲染 loading 占位；
 *  - DeepSeek 的两个端点各一行、共用一个显示名；
 *  - 目录只渲染密封可执行的端点行（openai-responses 已删除）；
 *  - 目录优先于同端点的注册表克隆，目录密钥落地条（catalog-*）不显示为自定义行。
 */
const CATALOG: ProviderCatalogEntry[] = [
  {
    vendor: 'deepseek',
    display_name: 'DeepSeek',
    endpoints: [
      { adapter: 'openai-completions', base_url: 'https://api.deepseek.com', default_model: 'deepseek-flash', models: ['deepseek-flash', 'deepseek-v4-pro'] },
      { adapter: 'anthropic-messages', base_url: 'https://api.deepseek.com/anthropic', default_model: 'deepseek-flash', models: ['deepseek-flash'] },
    ],
  },
  {
    vendor: 'openai',
    display_name: 'OpenAI',
    endpoints: [
      { adapter: 'openai-completions', base_url: 'https://api.openai.com/v1', default_model: 'gpt-4o', models: ['gpt-4o'] },
    ],
  },
];

const PROFILES: ProviderProfileStatus[] = [
  { id: 'openai-completions', adapter_family: 'openai-completions', endpoint_class: 'native', model_ids: [], state: 'READY' },
  { id: 'anthropic-messages', adapter_family: 'anthropic-messages', endpoint_class: 'native', model_ids: [], state: 'COMPILED' },
];

const view = vi.hoisted(() => ({
  settings: {
    provider: 'openai-completions',
    default_model: 'deepseek-flash',
    base_url: 'https://api.deepseek.com',
    read_only: false,
    frozen: false,
    config_provider: '',
    config_model: '',
    provider_profiles: [] as unknown[],
  } as Record<string, unknown> | null,
  settingsPhase: 'ready',
  settingsError: null,
  providers: [] as unknown[],
  catalog: [] as unknown[],
  providersPhase: 'ready',
  providersError: null,
  loadSettings: vi.fn().mockResolvedValue(undefined),
  saveSettings: vi.fn().mockResolvedValue(undefined),
  loadProviders: vi.fn().mockResolvedValue(undefined),
  saveProvider: vi.fn().mockResolvedValue(undefined),
  removeProvider: vi.fn().mockResolvedValue(undefined),
  refreshProvider: vi.fn().mockResolvedValue({ models: [] }),
}));

vi.mock('@/lib/store', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/store')>();
  return {
    ...original,
    useVivyStore: (selector: (state: typeof view) => unknown) => selector(view),
  };
});

describe('ModelSettingsCard 目录渲染（PROV-P4）', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    hydrateLocale('en');
    localStorage.clear();
    view.settings = {
      provider: 'openai-completions', default_model: 'deepseek-flash', base_url: 'https://api.deepseek.com',
      read_only: false, frozen: false, config_provider: '', config_model: '', provider_profiles: PROFILES,
    };
    view.settingsPhase = 'ready';
    view.providers = [];
    view.catalog = CATALOG;
    view.providersPhase = 'ready';
    view.providersError = null;
    view.saveSettings.mockClear();
    view.saveProvider.mockClear();
    view.saveProvider.mockResolvedValue(undefined);
    view.refreshProvider.mockClear();
    view.loadProviders.mockClear();
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    localStorage.clear();
    vi.unstubAllGlobals();
  });

  async function render() {
    await act(async () => root.render(<ModelSettingsCard />));
  }

  function testId(id: string): HTMLElement | null {
    return container.querySelector<HTMLElement>(`[data-testid="${id}"]`);
  }

  async function settle() {
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
  }

  async function typeInto(input: HTMLInputElement, value: string) {
    await act(async () => {
      // React 跟踪节点 value，需通过原生 setter 触发受控更新。
      const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')!.set!;
      setter.call(input, value);
      input.dispatchEvent(new Event('input', { bubbles: true }));
    });
  }

  it('目录未到达（idle/loading）时不渲染厂商行，只渲染 loading 占位', async () => {
    for (const phase of ['idle', 'loading']) {
      view.catalog = [];
      view.providersPhase = phase;
      await render();
      expect(testId('provider-catalog-loading')).not.toBeNull();
      expect(testId('provider-catalog-loading')?.getAttribute('aria-busy')).toBe('true');
      expect(container.querySelectorAll('[data-testid^="provider-row-"]')).toHaveLength(0);
      // 加载期间不渲染任何厂商显示名（UI 不持有任何目录数据）。
      expect(container.textContent).not.toContain('DeepSeek');
    }
  });

  it('目录到达后 DeepSeek 的两个协议端点各一行且共用一个显示名', async () => {
    await render();
    expect(testId('provider-catalog-loading')).toBeNull();
    const completions = testId('provider-row-deepseek-openai-completions');
    const messages = testId('provider-row-deepseek-anthropic-messages');
    expect(completions).not.toBeNull();
    expect(messages).not.toBeNull();
    expect(testId('provider-row-deepseek-openai-completions-name')?.textContent).toBe('DeepSeek');
    expect(testId('provider-row-deepseek-anthropic-messages-name')?.textContent).toBe('DeepSeek');
    // 两行同为 DeepSeek：靠行内协议适配器区分（多端点厂商才显示）。
    expect(completions?.textContent).toContain('openai-completions');
    expect(messages?.textContent).toContain('anthropic-messages');
  });

  it('目录只渲染密封可执行的端点行，且不出现适配器能力徽标', async () => {
    await render();
    // openai-responses 已随 agentic 迁移删除：目录里不再有这一行。
    expect(testId('provider-row-openai-openai-responses')).toBeNull();
    const executable = testId('provider-row-openai-openai-completions') as HTMLButtonElement | null;
    expect(executable).not.toBeNull();
    expect(executable?.disabled).toBe(false);
    expect(testId('provider-capability-openai-openai-completions')).toBeNull();
  });

  it('目录优先于同端点的注册表克隆，且 catalog-* 落地条不显示为自定义行', async () => {
    const clone: ProviderEntry = {
      id: 'custom-clone', display_name: '目录克隆', bundle: 'openai-completions',
      base_url: 'https://api.deepseek.com', default_model: 'deepseek-flash', models: ['deepseek-flash'], api_key_set: true,
    };
    view.providers = [
      clone,
      {
        id: 'catalog-deepseek-openai-completions', display_name: '目录密钥条', bundle: 'openai-completions',
        base_url: 'https://api.deepseek.com', default_model: 'deepseek-flash', models: ['deepseek-flash'], api_key_set: true,
      },
    ];
    await render();
    // 当前选择命中目录行（aria-pressed），而不是同端点的注册表克隆。
    expect(testId('provider-row-deepseek-openai-completions')?.getAttribute('aria-pressed')).toBe('true');
    expect(testId('provider-row-custom-clone-openai-completions')?.getAttribute('aria-pressed')).toBe('false');
    // 落地条不出现在列表里。
    expect(testId('provider-row-catalog-deepseek-openai-completions-openai-completions')).toBeNull();
  });

  it('点击端点行 + 模型行按适配器写回选择三元组', async () => {
    view.settings = {
      provider: '', default_model: '', base_url: '', read_only: false, frozen: false,
      config_provider: '', config_model: '', provider_profiles: PROFILES,
    };
    await render();
    await act(async () => testId('provider-row-deepseek-anthropic-messages')!.click());
    await act(async () => testId('provider-model-deepseek-flash')!.click());
    expect(view.saveSettings).toHaveBeenCalledWith({
      provider: 'anthropic-messages',
      base_url: 'https://api.deepseek.com/anthropic',
      default_model: 'deepseek-flash',
    });
  });

  async function openAddModel() {
    await act(async () => container.querySelector<HTMLButtonElement>('button[aria-label="Add model"]')!.click());
    return container.querySelector<HTMLInputElement>('input[aria-label="Add model"]')!;
  }

  async function confirmAddModel() {
    await act(async () => container.querySelector<HTMLButtonElement>('button[aria-label="Add and apply"]')!.click());
    await settle();
  }

  it('目录行手加模型：id 写入稳定的 catalog 落地条并立即选用', async () => {
    await render();
    await typeInto(await openAddModel(), 'deepseek-manual');
    await confirmAddModel();

    // 目录端点无注册表条目：写稳定 id（与 API Key 面板同一条目），
    // models 以内嵌目录为基线追加新 id，且不带 api_key。
    expect(view.saveProvider).toHaveBeenCalledWith({
      id: 'catalog-deepseek-openai-completions',
      display_name: 'DeepSeek',
      bundle: 'openai-completions',
      base_url: 'https://api.deepseek.com',
      default_model: 'deepseek-flash',
      models: ['deepseek-flash', 'deepseek-v4-pro', 'deepseek-manual'],
    });
    expect(view.saveSettings).toHaveBeenCalledWith({
      provider: 'openai-completions',
      base_url: 'https://api.deepseek.com',
      default_model: 'deepseek-manual',
    });
  });

  it('目录行手加已存在的模型：不写注册表，只切换当前选择', async () => {
    await render();
    await typeInto(await openAddModel(), 'deepseek-v4-pro');
    await confirmAddModel();

    expect(view.saveProvider).not.toHaveBeenCalled();
    expect(view.saveSettings).toHaveBeenCalledWith({
      provider: 'openai-completions',
      base_url: 'https://api.deepseek.com',
      default_model: 'deepseek-v4-pro',
    });
  });

  it('落地条已存在时按其 models 追加，且手加失败保留输入框不切换模型', async () => {
    view.providers = [{
      id: 'catalog-deepseek-openai-completions', display_name: 'DeepSeek', bundle: 'openai-completions',
      base_url: 'https://api.deepseek.com', default_model: 'deepseek-flash',
      models: ['upstream-a'], api_key_set: true,
    }];
    await render();
    await typeInto(await openAddModel(), 'manual-b');
    await confirmAddModel();
    expect(view.saveProvider).toHaveBeenCalledWith(expect.objectContaining({
      id: 'catalog-deepseek-openai-completions',
      models: ['upstream-a', 'manual-b'],
    }));

    // 失败路径：保留输入框与草稿，不切换当前模型。
    view.saveProvider.mockClear();
    view.saveSettings.mockClear();
    view.saveProvider.mockRejectedValueOnce(new Error('settings write failed'));
    const input = await openAddModel();
    await typeInto(input, 'manual-c');
    await confirmAddModel();
    expect(view.saveProvider).toHaveBeenCalledTimes(1);
    expect(view.saveSettings).not.toHaveBeenCalled();
    const stillOpen = container.querySelector<HTMLInputElement>('input[aria-label="Add model"]');
    expect(stillOpen).not.toBeNull();
    expect(stillOpen?.value).toBe('manual-c');
  });

  it('刷新反馈在注册表重新载入后仍显示', async () => {
    view.refreshProvider.mockResolvedValueOnce({
      id: 'catalog-deepseek-openai-completions', display_name: 'DeepSeek', bundle: 'openai-completions',
      base_url: 'https://api.deepseek.com', default_model: 'deepseek-flash', models: ['a', 'b'], api_key_set: true,
    });
    await render();
    await act(async () => container.querySelector<HTMLButtonElement>('button[aria-label="Refresh model list"]')!.click());
    await settle();
    expect(container.textContent).toContain('Synced 2 models from upstream');

    // 刷新成功后 store 会重载 providers：反馈不得被这次变化清掉。
    view.providers = [{
      id: 'catalog-deepseek-openai-completions', display_name: 'DeepSeek', bundle: 'openai-completions',
      base_url: 'https://api.deepseek.com', default_model: 'deepseek-flash', models: ['a', 'b'], api_key_set: true,
    }];
    await render();
    expect(container.textContent).toContain('Synced 2 models from upstream');
  });
});