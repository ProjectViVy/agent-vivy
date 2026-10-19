// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { hydrateLocale } from '@/i18n';
import type { ProviderCatalogEntry, ProviderEntry, ProviderProfileStatus } from '@/lib/api';
import { WelcomeWizard } from './WelcomeWizard';

const CATALOG: ProviderCatalogEntry[] = [
  {
    vendor: 'deepseek',
    display_name: 'DeepSeek',
    endpoints: [
      {
        adapter: 'openai-completions',
        base_url: 'https://api.deepseek.com',
        default_model: 'deepseek-flash',
        models: ['deepseek-flash', 'deepseek-chat'],
      },
      {
        adapter: 'anthropic-messages',
        base_url: 'https://api.deepseek.com/anthropic',
        default_model: 'deepseek-chat',
        models: ['deepseek-chat'],
      },
    ],
  },
  {
    vendor: 'openai',
    display_name: 'OpenAI',
    endpoints: [
      {
        adapter: 'openai-completions',
        base_url: 'https://api.openai.com/v1',
        default_model: 'gpt-4o',
        models: ['gpt-4o'],
      },
    ],
  },
];

const PROFILES: ProviderProfileStatus[] = [
  { id: 'openai-completions', adapter_family: 'openai-completions', endpoint_class: 'native', model_ids: [], state: 'READY' },
  { id: 'anthropic-messages', adapter_family: 'anthropic-messages', endpoint_class: 'native', model_ids: [], state: 'COMPILED' },
];

const view = vi.hoisted(() => ({
  settings: {
    provider: 'deepseek',
    default_model: 'deepseek-flash',
    base_url: '',
    read_only: false,
    frozen: false,
    config_provider: 'deepseek',
    config_model: 'deepseek-flash',
    api_key_set: false,
    provider_profiles: [] as unknown[],
  } as Record<string, unknown> | null,
  providers: [] as unknown[],
  catalog: [] as unknown[],
  providersPhase: 'ready',
  providersError: null as string | null,
  saveSettings: vi.fn().mockResolvedValue(undefined),
  loadProviders: vi.fn().mockResolvedValue(undefined),
  saveProvider: vi.fn().mockResolvedValue(undefined),
}));

const completeWelcome = vi.hoisted(() => vi.fn());

vi.mock('@/lib/store', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/store')>();
  return {
    ...original,
    useVivyStore: (selector: (state: typeof view) => unknown) => selector(view),
  };
});

vi.mock('@tanstack/react-router', () => ({ useNavigate: () => vi.fn() }));

vi.mock('@/hooks/use-welcome', () => ({
  useWelcomeOpen: () => true,
  completeWelcome,
}));

function providerEntry(overrides: Partial<ProviderEntry> = {}): ProviderEntry {
  return {
    id: 'catalog-deepseek-openai-completions',
    display_name: 'DeepSeek',
    bundle: 'openai-completions',
    base_url: 'https://api.deepseek.com',
    default_model: 'deepseek-flash',
    models: ['deepseek-flash', 'deepseek-chat'],
    api_key_set: true,
    ...overrides,
  };
}

describe('WelcomeWizard 模型步骤（PROV-P4）', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    hydrateLocale('en');
    localStorage.clear();
    view.settings = {
      provider: 'deepseek',
      default_model: 'deepseek-flash',
      base_url: '',
      read_only: false,
      frozen: false,
      config_provider: 'deepseek',
      config_model: 'deepseek-flash',
      api_key_set: false,
      provider_profiles: PROFILES,
    };
    view.providers = [];
    view.catalog = CATALOG;
    view.providersPhase = 'ready';
    view.providersError = null;
    view.saveSettings.mockClear();
    view.saveProvider.mockClear();
    view.loadProviders.mockClear();
    completeWelcome.mockClear();
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
    await act(async () => root.render(<WelcomeWizard />));
  }

  function testId(id: string): HTMLElement | null {
    return container.querySelector<HTMLElement>(`[data-testid="${id}"]`);
  }

  function buttonByText(text: string): HTMLButtonElement {
    const button = [...container.querySelectorAll('button')].find((candidate) => candidate.textContent?.includes(text));
    if (!button) throw new Error(`missing button: ${text}`);
    return button;
  }

  async function click(button: HTMLButtonElement) {
    await act(async () => {
      button.click();
      await Promise.resolve();
    });
  }

  async function next() {
    await click(buttonByText('Next'));
  }

  async function openModelStep() {
    await next();
  }

  async function typeKey(value: string) {
    const input = testId('welcome-api-key') as HTMLInputElement | null;
    if (!input) throw new Error('missing API key input');
    await act(async () => {
      const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set;
      setter?.call(input, value);
      input.dispatchEvent(new Event('input', { bubbles: true }));
    });
  }

  it('目录尚在应答时只渲染 loading 占位，不渲染选择器', async () => {
    view.catalog = [];
    view.providersPhase = 'loading';
    await render();
    await openModelStep();
    expect(testId('welcome-catalog-loading')).not.toBeNull();
    expect(testId('welcome-catalog-loading')?.getAttribute('aria-busy')).toBe('true');
    expect(testId('welcome-provider')).toBeNull();
  });

  it('目录失败时给出错误与重试，而不是空白步骤', async () => {
    view.catalog = [];
    view.providersPhase = 'error';
    view.providersError = 'rpc unavailable';
    await render();
    await openModelStep();
    expect(container.textContent).toContain('Could not load the provider catalog: rpc unavailable');
    await click(buttonByText('Retry'));
    expect(view.loadProviders).toHaveBeenCalledTimes(2);
  });

  it('从后端设置预填厂商/协议/模型；没有密钥时拒绝保存并提示', async () => {
    await render();
    await openModelStep();
    await next();
    expect(container.textContent).toContain(
      'Enter an API key, or confirm below that the runtime environment already provides it.',
    );
    expect(view.saveSettings).not.toHaveBeenCalled();
    expect(view.saveProvider).not.toHaveBeenCalled();
  });

  it('端点已配置密钥时留空即可保存，不重复 upsert 密钥', async () => {
    view.providers = [providerEntry()];
    await render();
    await openModelStep();
    await next();
    expect(view.saveProvider).not.toHaveBeenCalled();
    expect(view.saveSettings).toHaveBeenCalledTimes(1);
    expect(view.saveSettings).toHaveBeenCalledWith(expect.objectContaining({
      provider: 'openai-completions',
      base_url: 'https://api.deepseek.com',
      default_model: 'deepseek-flash',
    }));
  });

  it('用户确认运行环境已注入密钥后，留空保存且不调用 upsert', async () => {
    await render();
    await openModelStep();
    const checkbox = testId('welcome-env-key');
    if (!checkbox) throw new Error('missing env-key checkbox');
    await click(checkbox as unknown as HTMLButtonElement);
    await next();
    expect(view.saveProvider).not.toHaveBeenCalled();
    expect(view.saveSettings).toHaveBeenCalledTimes(1);
  });

  it('填写密钥后先写注册表再写选择，密钥只出现在 upsert 载荷里', async () => {
    await render();
    await openModelStep();
    await typeKey('sk-welcome-test');
    await next();
    expect(view.saveProvider).toHaveBeenCalledTimes(1);
    expect(view.saveProvider).toHaveBeenCalledWith(expect.objectContaining({
      id: 'catalog-deepseek-openai-completions',
      bundle: 'openai-completions',
      base_url: 'https://api.deepseek.com',
      api_key: 'sk-welcome-test',
    }));
    expect(view.saveSettings).toHaveBeenCalledTimes(1);
    const settingsPayload = view.saveSettings.mock.calls[0][0] as Record<string, unknown>;
    expect(JSON.stringify(settingsPayload)).not.toContain('sk-welcome-test');
  });

  it('read-only 部署不写任何设置，直接前进', async () => {
    view.settings = {
      ...view.settings,
      read_only: true,
      frozen: true,
    };
    await render();
    await openModelStep();
    expect(container.textContent).toContain('This process is locked to an environment-variable provider session');
    await next();
    expect(view.saveSettings).not.toHaveBeenCalled();
    expect(view.saveProvider).not.toHaveBeenCalled();
    expect(container.textContent).toContain('You are all set!');
  });
});
