import { describe, expect, it } from 'vitest';
import { PROVIDER_CATALOG } from './provider-catalog';
import {
  allProviderEntries,
  customApiKeySetFor,
  hasBaseUrlConflict,
  isValidCustomProvider,
  matchMergedProviderEntry,
  newCustomProviderId,
  parseCustomModels,
  providerEntryById,
  searchMergedProviders,
  splitMergedByFold,
  type ProviderEntry,
} from './custom-providers';

const ENTRY: ProviderEntry = {
  id: 'custom-1',
  display_name: '我的网关',
  bundle: 'openai',
  base_url: 'https://my-gateway.example.com/v1',
  default_model: 'my-model',
  models: ['my-model', 'my-model-2'],
  api_key_set: true,
};

const INPUT = {
  displayName: '我的网关',
  bundle: 'openai' as const,
  baseUrl: 'https://my-gateway.example.com/v1',
  defaultModel: 'my-model',
  models: ['my-model', 'my-model-2'],
  apiKey: '',
};

describe('isValidCustomProvider 校验 wire 条目', () => {
  it('合法条目通过', () => {
    expect(isValidCustomProvider(ENTRY)).toBe(true);
  });

  it('缺字段/空白显示名/非法束名/模型列表非字符串数组整条拒绝', () => {
    const bad: unknown[] = [
      { id: '', display_name: 'A', bundle: 'openai', base_url: 'https://a.example/v1', default_model: '', models: [], api_key_set: false },
      { id: 'custom-b', display_name: '  ', bundle: 'openai', base_url: 'https://b.example/v1', default_model: '', models: [], api_key_set: false },
      { id: 'custom-c', display_name: 'C', bundle: 'mock', base_url: 'https://c.example/v1', default_model: '', models: [], api_key_set: false },
      { id: 'custom-d', display_name: 'D', bundle: 'anthropic', base_url: 'https://d.example/v1', default_model: 'claude', models: ['claude', 42], api_key_set: false },
      { display_name: 'E', bundle: 'openai', base_url: 'https://e.example/v1', default_model: '', models: [], api_key_set: false },
      { ...ENTRY, api_key_set: 'yes' },
    ];
    for (const entry of bad) expect(isValidCustomProvider(entry)).toBe(false);
  });
});

describe('hasBaseUrlConflict', () => {
  it('注册表重名 (bundle, base_url) 冲突（编辑时排除自身）', () => {
    expect(hasBaseUrlConflict([ENTRY], INPUT)).toBe(true);
    expect(hasBaseUrlConflict([ENTRY], INPUT, 'custom-1')).toBe(false);
  });

  it('与静态目录同 (bundle, base_url) 冲突', () => {
    expect(hasBaseUrlConflict([], { ...INPUT, displayName: '重复目录', baseUrl: 'https://api.deepseek.com/v1' })).toBe(true);
  });

  it('不同 bundle 同 URL 不冲突', () => {
    expect(hasBaseUrlConflict([], { ...INPUT, bundle: 'anthropic' })).toBe(false);
  });
});

describe('parseCustomModels 录入解析', () => {
  it('换行 / 逗号 / 中文逗号分隔，去空白去重', () => {
    expect(parseCustomModels('gpt-4o\ndeepseek-chat, claude-3-5')).toEqual(['gpt-4o', 'deepseek-chat', 'claude-3-5']);
    expect(parseCustomModels('gpt-4o, gpt-4o，deepseek-chat')).toEqual(['gpt-4o', 'deepseek-chat']);
  });

  it('空/纯空白返回空数组', () => {
    expect(parseCustomModels('')).toEqual([]);
    expect(parseCustomModels('   \n,， ')).toEqual([]);
  });
});

describe('id 生成', () => {
  it('两次生成不相等且都带 custom- 前缀', () => {
    const a = newCustomProviderId();
    const b = newCustomProviderId();
    expect(a.startsWith('custom-')).toBe(true);
    expect(b.startsWith('custom-')).toBe(true);
    expect(a).not.toBe(b);
  });
});

describe('合并视图（目录 + 注册表）', () => {
  it('allProviderEntries：目录在前、注册表在后；注册表带 custom 标记与 registryId', () => {
    const entries = allProviderEntries([ENTRY]);
    expect(entries).toHaveLength(PROVIDER_CATALOG.length + 1);
    expect(entries[PROVIDER_CATALOG.length].custom).toBe(true);
    expect(entries[PROVIDER_CATALOG.length].registryId).toBe(ENTRY.id);
    expect(entries[PROVIDER_CATALOG.length].displayName).toBe('我的网关');
    expect(entries[PROVIDER_CATALOG.length].apiKeySet).toBe(true);
    expect(entries.slice(0, PROVIDER_CATALOG.length).every((entry) => entry.custom === false)).toBe(true);
    expect(entries.slice(0, PROVIDER_CATALOG.length).every((entry) => entry.apiKeySet === false)).toBe(true);
  });

  it('坏 wire 条目被过滤，不进入合并视图', () => {
    const bogus = { id: 'custom-x', display_name: '  ', bundle: 'mock', base_url: 'https://x.example/v1', default_model: '', models: [], api_key_set: false } as unknown as ProviderEntry;
    const entries = allProviderEntries([ENTRY, bogus]);
    expect(entries).toHaveLength(PROVIDER_CATALOG.length + 1);
  });

  it('searchMergedProviders：按显示名/名字检索注册表条目，空串返回全部', () => {
    const entries = allProviderEntries([ENTRY]);
    expect(searchMergedProviders(entries, '我的网关')[0]?.custom).toBe(true);
    expect(searchMergedProviders(entries, 'deepseek').some((entry) => entry.name === 'deepseek')).toBe(true);
    expect(searchMergedProviders(entries, '')).toHaveLength(PROVIDER_CATALOG.length + 1);
  });

  it('matchMergedProviderEntry：目录优先；注册表命中；base_url 空沿用束名回退；未知返回 undefined', () => {
    const custom = { ...ENTRY, display_name: '自定义 DeepSeek', base_url: 'https://api.deepseek.com/v1' };
    expect(matchMergedProviderEntry([custom], 'openai', 'https://api.deepseek.com/v1')).toMatchObject({ name: 'deepseek', custom: false });
    expect(matchMergedProviderEntry([ENTRY], 'openai', 'https://my-gateway.example.com/v1')).toMatchObject({ displayName: '我的网关', custom: true });
    expect(matchMergedProviderEntry([ENTRY], 'openai', '')).toMatchObject({ name: 'openai' });
    expect(matchMergedProviderEntry([ENTRY], 'unknown-bundle', '')).toBeUndefined();
  });

  it('providerEntryById：按 id 查注册表条目', () => {
    expect(providerEntryById([ENTRY], 'custom-1')).toEqual(ENTRY);
    expect(providerEntryById([ENTRY], 'custom-nope')).toBeUndefined();
  });

  it('splitMergedByFold：注册表永不入 more；搜索态平铺', () => {
    const entries = allProviderEntries([ENTRY]);
    const folded = splitMergedByFold(entries, false);
    expect(folded.custom.map((entry) => entry.registryId)).toEqual([ENTRY.id]);
    expect(folded.more.some((entry) => entry.custom)).toBe(false);
    expect(folded.visible.length + folded.custom.length + folded.more.length).toBe(entries.length);
    const searching = splitMergedByFold(searchMergedProviders(entries, '我的网关'), true);
    expect(searching.visible.length).toBe(1);
    expect(searching.custom).toEqual([]);
    expect(searching.more).toEqual([]);
  });
});

describe('customApiKeySetFor', () => {
  it('自定义命中且已配密钥返回 true；未配/目录/未知/空 baseUrl 返回 false', () => {
    expect(customApiKeySetFor([ENTRY], 'openai', 'https://my-gateway.example.com/v1')).toBe(true);
    expect(customApiKeySetFor([{ ...ENTRY, api_key_set: false }], 'openai', 'https://my-gateway.example.com/v1')).toBe(false);
    expect(customApiKeySetFor([ENTRY], 'openai', 'https://api.deepseek.com/v1')).toBe(false);
    expect(customApiKeySetFor([ENTRY], 'openai', 'https://unknown.example/v1')).toBe(false);
    expect(customApiKeySetFor([ENTRY], 'mock', '')).toBe(false);
  });
});