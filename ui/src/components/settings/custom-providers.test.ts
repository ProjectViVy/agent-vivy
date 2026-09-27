import { describe, expect, it } from 'vitest';
import type { ProviderCatalogEntry, ProviderEntry, ProviderProfileStatus } from '@/lib/api';
import {
  CATALOG_OVERLAY_PREFIX,
  allProviderEntries,
  catalogOverlayId,
  customApiKeySetFor,
  hasBaseUrlConflict,
  isCatalogOverlayEntry,
  isProviderRegistryBundle,
  isValidCustomProvider,
  matchMergedProviderEntry,
  newCustomProviderId,
  parseCustomModels,
  providerEntryByEndpoint,
  providerEntryById,
  searchMergedProviders,
  supportsModelRefresh,
  type CustomProviderInput,
} from './custom-providers';

/** 合成目录：一厂商两端点（deepseek）、可执行 + deferred（openai）、第三方网关。 */
const CATALOG: ProviderCatalogEntry[] = [
  {
    vendor: 'deepseek',
    display_name: 'DeepSeek',
    endpoints: [
      { adapter: 'openai-completions', base_url: 'https://api.deepseek.com', default_model: 'deepseek-flash', models: ['deepseek-flash'], executable: true, state: 'SUPPORTED' },
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
  {
    vendor: 'anthropic',
    display_name: 'Anthropic',
    endpoints: [
      { adapter: 'anthropic-messages', base_url: 'https://api.anthropic.com', default_model: 'claude-sonnet-4-5', models: ['claude-sonnet-4-5'], executable: true, state: 'SUPPORTED' },
    ],
  },
  {
    vendor: 'my-gateway',
    display_name: 'My Gateway',
    endpoints: [
      { adapter: 'openai-completions', base_url: 'https://gw.example.com/v1', default_model: 'my-model', models: ['my-model'], executable: true, state: 'SUPPORTED' },
    ],
  },
];

const PROFILES: ProviderProfileStatus[] = [
  { id: 'openai-completions', adapter_family: 'openai-completions', endpoint_class: 'native', model_ids: [], state: 'READY' },
  { id: 'openai-responses', adapter_family: 'openai-responses', endpoint_class: 'native', model_ids: [], state: 'DEFERRED-INDEFINITE' },
  { id: 'anthropic-messages', adapter_family: 'anthropic-messages', endpoint_class: 'native', model_ids: [], state: 'COMPILED' },
];

function wireEntry(over: Partial<ProviderEntry> = {}): ProviderEntry {
  return {
    id: 'custom-1',
    display_name: '我的网关',
    bundle: 'openai-completions',
    base_url: 'https://custom.example.com/v1',
    default_model: 'm',
    models: ['m'],
    api_key_set: false,
    ...over,
  };
}

function input(over: Partial<CustomProviderInput> = {}): CustomProviderInput {
  return {
    displayName: '我的网关',
    bundle: 'openai-completions',
    baseUrl: 'https://custom.example.com/v1',
    defaultModel: 'm',
    models: ['m'],
    apiKey: '',
    ...over,
  };
}

describe('注册表条目校验（wire 取值）', () => {
  it('bundle 接受密封适配器 id 与旧运行束名，拒绝其它取值', () => {
    expect(isProviderRegistryBundle('openai-completions')).toBe(true);
    expect(isProviderRegistryBundle('openai-responses')).toBe(true);
    expect(isProviderRegistryBundle('anthropic-messages')).toBe(true);
    expect(isProviderRegistryBundle('deepseek')).toBe(true);
    expect(isProviderRegistryBundle('unsupported')).toBe(false);
    expect(isProviderRegistryBundle(undefined)).toBe(false);
  });

  it('坏数据整条丢弃：缺 id / 空白显示名 / 非法 bundle / 空地址 / models 非字符串数组', () => {
    expect(isValidCustomProvider(wireEntry())).toBe(true);
    expect(isValidCustomProvider(wireEntry({ bundle: 'deepseek' }))).toBe(true);
    expect(isValidCustomProvider(wireEntry({ id: '  ' }))).toBe(false);
    expect(isValidCustomProvider(wireEntry({ display_name: '' }))).toBe(false);
    expect(isValidCustomProvider(wireEntry({ bundle: 'unsupported' as ProviderEntry['bundle'] }))).toBe(false);
    expect(isValidCustomProvider(wireEntry({ base_url: ' ' }))).toBe(false);
    expect(isValidCustomProvider(wireEntry({ models: [1 as unknown as string] }))).toBe(false);
    expect(isValidCustomProvider({ ...wireEntry(), api_key_set: undefined })).toBe(false);
  });

  it('模型录入口径：换行 / 中英文逗号分隔，去空去重', () => {
    expect(parseCustomModels('a, b\nc，d\n\na')).toEqual(['a', 'b', 'c', 'd']);
    expect(parseCustomModels('')).toEqual([]);
  });

  it('自动 id 带 custom- 前缀且互不相同', () => {
    const ids = new Set([newCustomProviderId(), newCustomProviderId()]);
    expect(ids.size).toBe(2);
    for (const id of ids) expect(id.startsWith('custom-')).toBe(true);
  });
});

describe('supportsModelRefresh 门控（与后端同一适配器属性）', () => {
  it('openai-completions + http(s) 才可刷新（旧运行束名归一后同样通过）', () => {
    expect(supportsModelRefresh('openai-completions', 'https://gw.example.com/v1')).toBe(true);
    expect(supportsModelRefresh('openai-completions', 'http://127.0.0.1:8080/v1')).toBe(true);
    expect(supportsModelRefresh('openai', 'https://gw.example.com/v1')).toBe(true);
    expect(supportsModelRefresh('deepseek', 'https://api.deepseek.com')).toBe(true);
  });

  it('原生 Anthropic / responses / 非 http(s) 地址一律不可刷新', () => {
    expect(supportsModelRefresh('anthropic-messages', 'https://api.anthropic.com')).toBe(false);
    expect(supportsModelRefresh('anthropic', 'https://api.anthropic.com')).toBe(false);
    expect(supportsModelRefresh('openai-responses', 'https://api.openai.com/v1')).toBe(false);
    expect(supportsModelRefresh('openai-completions', '')).toBe(false);
    expect(supportsModelRefresh('openai-completions', 'ftp://gw.example.com')).toBe(false);
  });
});

describe('hasBaseUrlConflict 端点冲突', () => {
  it('注册表同 (adapter, base_url) 冲突，旧运行束名归一后同样命中', () => {
    const providers = [wireEntry({ bundle: 'openai' })];
    expect(hasBaseUrlConflict([], providers, input())).toBe(true);
    expect(hasBaseUrlConflict([], providers, input({ bundle: 'anthropic-messages' }))).toBe(false);
  });

  it('目录端点同样占用 (adapter, base_url)', () => {
    expect(hasBaseUrlConflict(CATALOG, [], input({ baseUrl: 'https://api.deepseek.com' }))).toBe(true);
    expect(hasBaseUrlConflict(CATALOG, [], input({ baseUrl: 'https://api.deepseek.com', bundle: 'anthropic-messages' }))).toBe(false);
    // 同地址不同适配器可共存（DeepSeek 的两种协议端点）。
    expect(hasBaseUrlConflict(CATALOG, [], input({ baseUrl: 'https://api.deepseek.com/anthropic', bundle: 'anthropic-messages' }))).toBe(true);
  });

  it('编辑自身时不与自己的旧条目冲突', () => {
    const providers = [wireEntry({ id: 'custom-9' })];
    expect(hasBaseUrlConflict([], providers, input(), 'custom-9')).toBe(false);
    expect(hasBaseUrlConflict([], providers, input(), 'custom-1')).toBe(true);
  });
});

describe('allProviderEntries 合并（目录端点在前，注册表在后）', () => {
  it('一个端点一行：目录 6 行 + 注册表行，DeepSeek 两行共用一个显示名', () => {
    const merged = allProviderEntries(CATALOG, [wireEntry()], PROFILES);
    expect(merged.filter((entry) => !entry.custom)).toHaveLength(6);
    const deepseek = merged.filter((entry) => entry.vendor === 'deepseek');
    expect(deepseek.map((entry) => entry.adapter)).toEqual(['openai-completions', 'anthropic-messages']);
    expect(new Set(deepseek.map((entry) => entry.displayName)).size).toBe(1);
    const custom = merged.filter((entry) => entry.custom);
    expect(custom).toHaveLength(1);
    expect(custom[0]).toMatchObject({ registryId: 'custom-1', vendor: 'custom-1', apiKeySet: false });
  });

  it('目录密钥落地条（catalog-*）不显示为自定义行；坏条目直接丢弃', () => {
    const merged = allProviderEntries(CATALOG, [
      wireEntry({ id: catalogOverlayId('deepseek', 'openai-completions'), api_key_set: true }),
      wireEntry({ id: 'broken', bundle: 'unsupported' as ProviderEntry['bundle'] }),
    ], PROFILES);
    expect(merged.filter((entry) => entry.custom)).toHaveLength(0);
    // 但目录行本身仍在，且可执行性只由目录 + Profile 决定。
    expect(merged.find((entry) => entry.vendor === 'deepseek' && entry.adapter === 'openai-completions')?.executable).toBe(true);
  });

  it('deferred 端点行不可执行（无 Profile 也保持端点声明的基线）', () => {
    const merged = allProviderEntries(CATALOG, [], []);
    expect(merged.find((entry) => entry.adapter === 'openai-responses')).toMatchObject({ executable: false, capabilityState: 'DEFERRED-INDEFINITE' });
  });
});

describe('searchMergedProviders 检索', () => {
  it('按显示名 / 厂商大小写不敏感检索，空串返回全部', () => {
    const merged = allProviderEntries(CATALOG, [wireEntry()], PROFILES);
    expect(searchMergedProviders(merged, '')).toHaveLength(merged.length);
    expect(searchMergedProviders(merged, 'deepseek').every((entry) => entry.vendor === 'deepseek')).toBe(true);
    expect(searchMergedProviders(merged, 'GATEWAY').map((entry) => entry.vendor)).toContain('my-gateway');
    expect(searchMergedProviders(merged, '不存在的厂商')).toEqual([]);
  });
});

describe('matchMergedProviderEntry / providerEntryByEndpoint 端点反查', () => {
  it('目录优先：注册表同端点的自定义克隆不会顶掉目录行', () => {
    const clone = wireEntry({ base_url: 'https://api.deepseek.com' });
    const match = matchMergedProviderEntry(CATALOG, [clone], 'openai-completions', 'https://api.deepseek.com', PROFILES);
    expect(match).toMatchObject({ vendor: 'deepseek', displayName: 'DeepSeek', custom: false });
  });

  it('目录未命中时回落到注册表条目；旧运行束名归一后仍命中', () => {
    const match = matchMergedProviderEntry(CATALOG, [wireEntry({ bundle: 'openai' })], 'openai-completions', 'https://custom.example.com/v1', PROFILES);
    expect(match).toMatchObject({ vendor: 'custom-1', custom: true, registryId: 'custom-1' });
    expect(providerEntryByEndpoint([wireEntry({ bundle: 'openai' })], 'openai-completions', 'https://custom.example.com/v1')?.id).toBe('custom-1');
  });

  it('旧文档不带地址：回落该厂商在对应适配器下的目录端点；无匹配返回 undefined', () => {
    expect(matchMergedProviderEntry(CATALOG, [], 'deepseek', '', PROFILES)).toMatchObject({ vendor: 'deepseek', adapter: 'openai-completions' });
    expect(matchMergedProviderEntry(CATALOG, [], 'anthropic', '', PROFILES)).toMatchObject({ vendor: 'anthropic', adapter: 'anthropic-messages' });
    expect(matchMergedProviderEntry(CATALOG, [], 'openai-completions', '', PROFILES)).toBeUndefined();
    expect(matchMergedProviderEntry(CATALOG, [], 'anything', 'https://nope.example/v1', PROFILES)).toBeUndefined();
    expect(providerEntryById([wireEntry()], 'custom-1')?.display_name).toBe('我的网关');
    expect(providerEntryById([wireEntry()], 'custom-2')).toBeUndefined();
  });
});

describe('目录密钥落地条与 key 提示', () => {
  it('落地条 id 按 (vendor, adapter) 稳定生成且可识别', () => {
    expect(catalogOverlayId('deepseek', 'openai-completions')).toBe(`${CATALOG_OVERLAY_PREFIX}deepseek-openai-completions`);
    expect(catalogOverlayId('deepseek', 'anthropic-messages')).not.toBe(catalogOverlayId('deepseek', 'openai-completions'));
    expect(isCatalogOverlayEntry(wireEntry({ id: catalogOverlayId('deepseek', 'openai-completions') }))).toBe(true);
    expect(isCatalogOverlayEntry(wireEntry())).toBe(false);
  });

  it('「已配置 API Key」按端点判定且只读 api_key_set 布尔', () => {
    const providers = [wireEntry({ bundle: 'openai', api_key_set: true })];
    expect(customApiKeySetFor(providers, 'openai-completions', 'https://custom.example.com/v1')).toBe(true);
    expect(customApiKeySetFor(providers, 'anthropic-messages', 'https://custom.example.com/v1')).toBe(false);
    expect(customApiKeySetFor(providers, 'openai-completions', 'https://other.example.com/v1')).toBe(false);
  });
});