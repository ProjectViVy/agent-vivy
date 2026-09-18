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
 *  - deferred 端点（openai-responses）行 disabled + 能力徽标；
 *  - 目录优先于同端点的注册表克隆，目录密钥落地条（catalog-*）不显示为自定义行。
 */
const CATALOG: ProviderCatalogEntry[] = [
  {
    vendor: 'deepseek',
    display_name: 'DeepSeek',
    endpoints: [
      { adapter: 'openai-completions', base_url: 'https://api.deepseek.com', default_model: 'deepseek-flash', models: ['deepseek-flash', 'deepseek-v4-pro'], executable: true, state: 'SUPPORTED' },
      { adapter: 'anthropic-messages', base_url: 'https://api.deepseek.com/anthropic', default_model: 'deepseek-flash', models: ['deepseek-flash'], executable: true, state: 'SUPPORTED' },
    ],
  },
  {
    vendor: 'openai',
    display_name: 'OpenAI',
    endpoints: [
      { adapter: 'openai-completions', base_url: 'https://api.openai.com/v1', default_model: 'gpt-4o', models: ['gpt-4o'], executable: true, state: 'SUPPORTED' },
      { adapter: 'openai-responses', base_url: 'https://api.openai.com/v1', default_model: 'gpt-4o', models: ['gpt-4o'], executable: false, state: 'DEFERRED-INDEFINITE' },
    ],
  },
];

const PROFILES: ProviderProfileStatus[] = [
  { id: 'openai-completions', adapter_family: 'openai-completions', endpoint_class: 'native', model_ids: [], state: 'READY' },
  { id: 'openai-responses', adapter_family: 'openai-responses', endpoint_class: 'native', model_ids: [], state: 'DEFERRED-INDEFINITE' },
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

  it('deferred 端点行 disabled 且带能力徽标；可执行端点行可用', async () => {
    await render();
    const deferred = testId('provider-row-openai-openai-responses') as HTMLButtonElement | null;
    expect(deferred).not.toBeNull();
    expect(deferred?.disabled).toBe(true);
    expect(testId('provider-capability-openai-openai-responses')?.textContent).toBe('DEFERRED-INDEFINITE');
    const executable = testId('provider-row-openai-openai-completions') as HTMLButtonElement | null;
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
});