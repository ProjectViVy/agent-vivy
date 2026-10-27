import { describe, expect, it } from 'vitest';
import type {
  ProviderCatalogEntry,
  ProviderEntry,
  ProviderProfileStatus,
  Settings,
} from '@/lib/api';
import {
  canSaveWelcomeModel,
  prefillWelcomeModel,
  welcomeCatalogRows,
  welcomeKeyStatus,
  welcomeProviderInput,
  welcomeSelection,
  welcomeVendors,
} from './welcome-model';

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

/** 设置文档的最小夹具；只声明本模块读到的字段，其余保持 wire 的缺省形态。 */
function settings(overrides: Partial<Settings> = {}): Settings {
  return {
    locale: 'en',
    generation_locale: 'en',
    workspace_locale: '',
    locale_read_only: false,
    provider: '',
    default_model: '',
    busy_enter: 'queue',
    base_url: '',
    read_only: false,
    config_provider: '',
    config_model: '',
    provider_profiles: PROFILES,
    ...overrides,
  };
}

function providerEntry(overrides: Partial<ProviderEntry> = {}): ProviderEntry {
  return {
    id: 'catalog-deepseek-openai-completions',
    display_name: 'DeepSeek',
    bundle: 'openai-completions',
    base_url: 'https://api.deepseek.com',
    default_model: 'deepseek-flash',
    models: ['deepseek-flash'],
    api_key_set: true,
    ...overrides,
  };
}

function rows() {
  return welcomeCatalogRows(CATALOG, PROFILES);
}

function row(vendor: string, adapter: string) {
  const found = rows().find((candidate) => candidate.vendor === vendor && candidate.adapter === adapter);
  if (!found) throw new Error(`missing fixture row ${vendor}/${adapter}`);
  return found;
}

describe('welcomeCatalogRows', () => {
  it('只暴露 Profile 可执行的目录端点，UNAVAILABLE 适配器不出现', () => {
    expect(rows()).toHaveLength(3);
    expect(welcomeCatalogRows(CATALOG, [
      { id: 'openai-completions', adapter_family: 'openai-completions', endpoint_class: 'native', model_ids: [], state: 'UNAVAILABLE' },
      { id: 'anthropic-messages', adapter_family: 'anthropic-messages', endpoint_class: 'native', model_ids: [], state: 'COMPILED' },
    ]).map((entry) => entry.adapter)).toEqual(['anthropic-messages']);
  });
});

describe('welcomeVendors', () => {
  it('按目录顺序把端点变体归到厂商下', () => {
    const vendors = welcomeVendors(rows());
    expect(vendors.map((vendor) => vendor.vendor)).toEqual(['deepseek', 'openai']);
    expect(vendors[0].displayName).toBe('DeepSeek');
    expect(vendors[0].endpoints.map((endpoint) => endpoint.adapter)).toEqual([
      'openai-completions',
      'anthropic-messages',
    ]);
    expect(vendors[1].endpoints.map((endpoint) => endpoint.adapter)).toEqual(['openai-completions']);
  });
});

describe('prefillWelcomeModel', () => {
  it('迁移前的运行束名 + 空地址回落到同名厂商的端点（PROV-P4 预填口径）', () => {
    const draft = prefillWelcomeModel(
      rows(),
      settings({
        provider: 'deepseek',
        config_provider: 'deepseek',
        default_model: 'deepseek-flash',
      }),
    );
    expect(draft).toMatchObject({
      vendor: 'deepseek',
      adapter: 'openai-completions',
      model: 'deepseek-flash',
    });
  });

  it('密封适配器 + base_url 精确命中目录端点', () => {
    const draft = prefillWelcomeModel(
      rows(),
      settings({
        provider: 'openai-completions',
        base_url: 'https://api.deepseek.com',
        default_model: 'deepseek-chat',
      }),
    );
    expect(draft).toMatchObject({
      vendor: 'deepseek',
      adapter: 'openai-completions',
      model: 'deepseek-chat',
    });
  });

  it('只有 config_provider 时使用该厂商的默认可执行端点', () => {
    const draft = prefillWelcomeModel(
      rows(),
      settings({ config_provider: 'deepseek', config_model: 'deepseek-flash' }),
    );
    expect(draft).toMatchObject({ vendor: 'deepseek', adapter: 'openai-completions' });
  });

  it('存下的模型不在端点清单里时回落到目录默认模型', () => {
    const draft = prefillWelcomeModel(
      rows(),
      settings({
        provider: 'deepseek',
        default_model: 'not-in-catalog',
      }),
    );
    expect(draft?.model).toBe('deepseek-flash');
  });

  it('无法从后端数据判定厂商时返回 null，不发明供应商', () => {
    const draft = prefillWelcomeModel(
      rows(),
      settings({
        provider: 'openai-completions',
        base_url: 'https://custom.example.com/v1',
        config_provider: '',
        config_model: '',
      }),
    );
    expect(draft).toBeNull();
  });

  it('已存的自定义网关不回落到 config 目录厂商，避免向导覆盖它', () => {
    const draft = prefillWelcomeModel(
      rows(),
      settings({
        provider: 'openai-completions',
        base_url: 'https://custom.example.com/v1',
        config_provider: 'deepseek',
        config_model: 'deepseek-flash',
      }),
    );
    expect(draft).toBeNull();
  });

  it('目录为空时返回 null', () => {
    expect(prefillWelcomeModel([], settings({ provider: 'deepseek' }))).toBeNull();
    expect(prefillWelcomeModel(rows(), null)).toBeNull();
  });
});

describe('welcomeKeyStatus', () => {
  it('注册表条目命中端点且 api_key_set 时报告已配置，并复用条目 id', () => {
    const status = welcomeKeyStatus(rows(), [providerEntry()], settings(), row('deepseek', 'openai-completions'));
    expect(status).toEqual({
      configured: true,
      entryId: 'catalog-deepseek-openai-completions',
      overlayId: 'catalog-deepseek-openai-completions',
    });
  });

  it('目录端点没有注册表条目时，旧 api_key overlay 与当前选择匹配则视为已配置', () => {
    const status = welcomeKeyStatus(
      rows(),
      [],
      settings({
        provider: 'deepseek',
        base_url: 'https://api.deepseek.com',
        api_key_set: true,
      }),
      row('deepseek', 'openai-completions'),
    );
    expect(status.configured).toBe(true);
    expect(status.entryId).toBe('catalog-deepseek-openai-completions');
  });

  it('没有注册表条目也没有 overlay 时报告未配置', () => {
    const status = welcomeKeyStatus(rows(), [], settings({ provider: 'deepseek' }), row('deepseek', 'openai-completions'));
    expect(status.configured).toBe(false);
  });
});

describe('welcomeProviderInput', () => {
  it('已有条目：复用 id / 显示名 / 模型清单，只替换写-only 密钥', () => {
    const existing = providerEntry({ id: 'custom-1', display_name: '我的 DeepSeek', models: ['deepseek-flash', 'custom-model'] });
    const input = welcomeProviderInput(row('deepseek', 'openai-completions'), '  sk-new  ', existing);
    expect(input).toMatchObject({
      id: 'custom-1',
      display_name: '我的 DeepSeek',
      bundle: 'openai-completions',
      base_url: 'https://api.deepseek.com',
      models: ['deepseek-flash', 'custom-model'],
      api_key: 'sk-new',
    });
  });

  it('没有条目：落到稳定的目录占位 id，并带目录显示名与模型', () => {
    const input = welcomeProviderInput(row('deepseek', 'openai-completions'), 'sk-new', undefined);
    expect(input).toMatchObject({
      id: 'catalog-deepseek-openai-completions',
      display_name: 'DeepSeek',
      bundle: 'openai-completions',
      default_model: 'deepseek-flash',
      models: ['deepseek-flash', 'deepseek-chat'],
      api_key: 'sk-new',
    });
  });
});

describe('welcomeSelection', () => {
  it('写密封适配器 id + 端点地址 + 所选模型', () => {
    expect(welcomeSelection(row('deepseek', 'anthropic-messages'), 'deepseek-chat')).toEqual({
      provider: 'anthropic-messages',
      base_url: 'https://api.deepseek.com/anthropic',
      default_model: 'deepseek-chat',
    });
  });
});

describe('canSaveWelcomeModel', () => {
  const base = {
    readOnly: false,
    row: row('deepseek', 'openai-completions'),
    model: 'deepseek-flash',
    apiKey: '',
    keyConfigured: false,
    envKeyConfirmed: false,
  };

  it('没有密钥、未配置、未确认环境变量时拒绝保存', () => {
    expect(canSaveWelcomeModel(base)).toBe(false);
  });

  it('输入了密钥时允许保存', () => {
    expect(canSaveWelcomeModel({ ...base, apiKey: 'sk-1' })).toBe(true);
  });

  it('端点已配置密钥时允许留空保存', () => {
    expect(canSaveWelcomeModel({ ...base, keyConfigured: true })).toBe(true);
  });

  it('用户确认运行环境已注入密钥时允许留空保存', () => {
    expect(canSaveWelcomeModel({ ...base, envKeyConfirmed: true })).toBe(true);
  });

  it('没有可用端点或模型时拒绝保存', () => {
    expect(canSaveWelcomeModel({ ...base, row: undefined })).toBe(false);
    expect(canSaveWelcomeModel({ ...base, model: '  ' })).toBe(false);
  });

  it('read-only 时只前进不写入，始终允许', () => {
    expect(canSaveWelcomeModel({ ...base, readOnly: true, row: undefined })).toBe(true);
  });
});
