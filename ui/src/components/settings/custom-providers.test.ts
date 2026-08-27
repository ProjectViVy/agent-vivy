import { afterEach, describe, expect, it, vi } from 'vitest';
import { PROVIDER_CATALOG } from './provider-catalog';
import {
  CUSTOM_PROVIDERS_KEY,
  addCustomProvider,
  allProviderEntries,
  customApiKeyFor,
  getCustomProviders,
  matchMergedProviderEntry,
  newCustomProviderId,
  parseCustomModels,
  removeCustomProvider,
  searchMergedProviders,
  splitMergedByFold,
  updateCustomProvider,
} from './custom-providers';

type ListenerMap = Record<string, Array<() => void>>;

function stubWindow(initial: Record<string, string> = {}) {
  const storage = new Map(Object.entries(initial));
  const listeners: ListenerMap = {};
  vi.stubGlobal('window', {
    localStorage: {
      getItem: (key: string) => (storage.has(key) ? storage.get(key)! : null),
      setItem: (key: string, value: string) => { storage.set(key, value); },
    },
    addEventListener: (type: string, onChange: () => void) => {
      (listeners[type] ??= []).push(onChange);
    },
    removeEventListener: (type: string, onChange: () => void) => {
      listeners[type] = (listeners[type] ?? []).filter((fn) => fn !== onChange);
    },
    dispatchEvent: (event: Event) => {
      for (const listener of listeners[event.type] ?? []) listener();
      return true;
    },
  });
  return { storage, listeners };
}

const INPUT = {
  displayName: '我的网关',
  bundle: 'openai' as const,
  baseUrl: 'https://my-gateway.example.com/v1',
  defaultModel: 'my-model',
  models: ['my-model', 'my-model-2'],
  apiKey: '',
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('getCustomProviders 读取与坏数据过滤', () => {
  it('SSR guard：无 window 时返回空数组且 CRUD 不抛错', () => {
    vi.stubGlobal('window', undefined);
    expect(getCustomProviders()).toEqual([]);
    expect(() => addCustomProvider(INPUT)).not.toThrow();
    expect(() => removeCustomProvider('custom-x')).not.toThrow();
  });

  it('存储为空时返回空数组', () => {
    stubWindow();
    expect(getCustomProviders()).toEqual([]);
  });

  it('坏 JSON / 非数组返回空数组', () => {
    stubWindow({ [CUSTOM_PROVIDERS_KEY]: '{broken json' });
    expect(getCustomProviders()).toEqual([]);
    stubWindow({ [CUSTOM_PROVIDERS_KEY]: '{"displayName":"x"}' });
    expect(getCustomProviders()).toEqual([]);
  });

  it('缺字段/空白显示名/非法束名/非字符串数组模型条目整条丢弃，合法条目保留', () => {
    stubWindow({
      [CUSTOM_PROVIDERS_KEY]: JSON.stringify([
        { id: 'custom-a', displayName: 'A', bundle: 'openai', baseUrl: 'https://a.example/v1', defaultModel: '', models: [] },
        { id: 'custom-b', displayName: '  ', bundle: 'openai', baseUrl: 'https://b.example/v1', defaultModel: '', models: [] },
        { id: 'custom-c', displayName: 'C', bundle: 'mock', baseUrl: 'https://c.example/v1', defaultModel: '', models: [] },
        { id: 'custom-d', displayName: 'D', bundle: 'anthropic', baseUrl: 'https://d.example/v1', defaultModel: 'claude', models: ['claude', 42] },
        { displayName: 'E', bundle: 'openai', baseUrl: 'https://e.example/v1', defaultModel: '', models: [] },
        { id: 'custom-g', displayName: 'G', bundle: 'openai', baseUrl: 'https://g.example/v1', defaultModel: '', models: ['g1'] },
      ]),
    });
    expect(getCustomProviders().map((entry) => entry.id)).toEqual(['custom-a', 'custom-g']);
  });
});

describe('addCustomProvider / updateCustomProvider / removeCustomProvider', () => {
  it('新增尾部追加、id 自动生成、写回 trim 归一后的 JSON', () => {
    const { storage } = stubWindow();
    const created = addCustomProvider({
      displayName: '  我的网关  ',
      bundle: 'openai',
      baseUrl: ' https://my-gateway.example.com/v1 ',
      defaultModel: ' my-model ',
      models: ['my-model', ' my-model ', 'my-model-2'],
      apiKey: '',
    });
    expect(created).not.toBeNull();
    expect(created!.id.startsWith('custom-')).toBe(true);
    const next = addCustomProvider({ ...INPUT, displayName: '网关二', baseUrl: 'https://gateway2.example.com/v1' });
    expect(next?.id).not.toBe(created!.id);
    expect(getCustomProviders()).toEqual([
      { id: created!.id, displayName: '我的网关', bundle: 'openai', baseUrl: 'https://my-gateway.example.com/v1', defaultModel: 'my-model', models: ['my-model', 'my-model-2'], apiKey: '' },
      { id: next!.id, displayName: '网关二', bundle: 'openai', baseUrl: 'https://gateway2.example.com/v1', defaultModel: 'my-model', models: ['my-model', 'my-model-2'], apiKey: '' },
    ]);
    expect(storage.get(CUSTOM_PROVIDERS_KEY)).toBe(JSON.stringify(getCustomProviders()));
  });

  it('同 (bundle, baseUrl) 与既有自定义冲突时新增返回 null', () => {
    stubWindow();
    expect(addCustomProvider(INPUT)).not.toBeNull();
    expect(addCustomProvider(INPUT)).toBeNull();
  });

  it('与静态目录同 (bundle, baseUrl) 冲突时新增返回 null', () => {
    stubWindow();
    expect(addCustomProvider({ ...INPUT, displayName: '重复目录', baseUrl: 'https://api.deepseek.com/v1' })).toBeNull();
  });

  it('编辑保留 id、改显示名即改标签源；编辑冲突/不存在返回 false', () => {
    const { storage } = stubWindow();
    const created = addCustomProvider(INPUT)!;
    expect(updateCustomProvider(created.id, { ...INPUT, displayName: '新名字', models: ['x'] })).toBe(true);
    expect(getCustomProviders()[0]).toEqual({ id: created.id, displayName: '新名字', bundle: 'openai', baseUrl: 'https://my-gateway.example.com/v1', defaultModel: 'my-model', models: ['x'], apiKey: '' });
    // 冲突：改成另一个已存在条目的 baseUrl（openai 目录里的 deepseek 网关）。
    expect(updateCustomProvider(created.id, { ...INPUT, baseUrl: 'https://api.deepseek.com/v1' })).toBe(false);
    expect(updateCustomProvider('custom-nope', INPUT)).toBe(false);
    expect(storage.get(CUSTOM_PROVIDERS_KEY)).toBe(JSON.stringify(getCustomProviders()));
  });

  it('按 id 移除；未命中 no-op 且写回不变', () => {
    const second = { ...INPUT, displayName: 'B', baseUrl: 'https://b.example/v1' };
    stubWindow();
    const a = addCustomProvider(INPUT)!;
    const b = addCustomProvider(second)!;
    removeCustomProvider(a.id);
    expect(getCustomProviders().map((entry) => entry.id)).toEqual([b.id]);
    removeCustomProvider('custom-nope');
    expect(getCustomProviders().map((entry) => entry.id)).toEqual([b.id]);
  });
});

describe('apiKey 与 customApiKeyFor', () => {
  it('apiKey 随注册表存取：新增/编辑保留，写回包含 trim 后的值', () => {
    const { storage } = stubWindow();
    const created = addCustomProvider({ ...INPUT, apiKey: ' sk-abc ' })!;
    expect(getCustomProviders()[0].apiKey).toBe('sk-abc');
    expect(updateCustomProvider(created.id, { ...INPUT, apiKey: 'sk-new' })).toBe(true);
    expect(getCustomProviders()[0].apiKey).toBe('sk-new');
    expect(storage.get(CUSTOM_PROVIDERS_KEY)).toBe(JSON.stringify(getCustomProviders()));
  });

  it('字段引入前保存的旧条目（无 apiKey）仍有效并补空串', () => {
    stubWindow({
      [CUSTOM_PROVIDERS_KEY]: JSON.stringify([
        { id: 'custom-old', displayName: '旧网关', bundle: 'openai', baseUrl: 'https://old.example/v1', defaultModel: '', models: [] },
      ]),
    });
    expect(getCustomProviders()).toEqual([
      { id: 'custom-old', displayName: '旧网关', bundle: 'openai', baseUrl: 'https://old.example/v1', defaultModel: '', models: [], apiKey: '' },
    ]);
  });

  it('customApiKeyFor：自定义命中返回密钥，目录/未知/空 baseUrl 返回空串', () => {
    stubWindow();
    addCustomProvider({ ...INPUT, apiKey: 'sk-custom' });
    expect(customApiKeyFor('openai', 'https://my-gateway.example.com/v1')).toBe('sk-custom');
    expect(customApiKeyFor('openai', 'https://api.deepseek.com/v1')).toBe('');
    expect(customApiKeyFor('openai', 'https://unknown.example/v1')).toBe('');
    expect(customApiKeyFor('mock', '')).toBe('');
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

describe('合并视图（目录 + 自定义）', () => {
  it('allProviderEntries：目录在前、自定义在后；自定义带 custom 标记与 registryId', () => {
    stubWindow();
    const created = addCustomProvider(INPUT)!;
    const entries = allProviderEntries();
    expect(entries).toHaveLength(PROVIDER_CATALOG.length + 1);
    expect(entries[PROVIDER_CATALOG.length].custom).toBe(true);
    expect(entries[PROVIDER_CATALOG.length].registryId).toBe(created.id);
    expect(entries[PROVIDER_CATALOG.length].displayName).toBe('我的网关');
    expect(entries.slice(0, PROVIDER_CATALOG.length).every((entry) => entry.custom === false)).toBe(true);
  });

  it('searchMergedProviders：按显示名/名字检索自定义条目，空串返回全部', () => {
    stubWindow();
    addCustomProvider(INPUT);
    expect(searchMergedProviders('我的网关')[0]?.custom).toBe(true);
    expect(searchMergedProviders('deepseek').some((entry) => entry.name === 'deepseek')).toBe(true);
    expect(searchMergedProviders('')).toHaveLength(PROVIDER_CATALOG.length + 1);
  });

  it('matchMergedProviderEntry：目录优先；自定义命中；base_url 空沿用束名回退；未知返回 undefined', () => {
    stubWindow({ [CUSTOM_PROVIDERS_KEY]: JSON.stringify([{ id: 'custom-c', displayName: '自定义 DeepSeek', bundle: 'openai', baseUrl: 'https://api.deepseek.com/v1', defaultModel: '', models: [] }]) });
    // 目录优先（即使用户注册同 URL，也返回目录条目）。
    expect(matchMergedProviderEntry('openai', 'https://api.deepseek.com/v1')).toMatchObject({ name: 'deepseek', custom: false });
    // 自定义独有 URL → 自定义条目。
    const custom = addCustomProvider(INPUT) ? matchMergedProviderEntry('openai', 'https://my-gateway.example.com/v1') : undefined;
    expect(custom).toMatchObject({ displayName: '我的网关', custom: true });
    // base_url 空：束名同名回退（目录）。
    expect(matchMergedProviderEntry('openai', '')).toMatchObject({ name: 'openai' });
    expect(matchMergedProviderEntry('unknown-bundle', '')).toBeUndefined();
  });

  it('splitMergedByFold：自定义永不入 more；搜索态平铺', () => {
    stubWindow();
    addCustomProvider(INPUT);
    const entries = allProviderEntries();
    const folded = splitMergedByFold(entries, false);
    expect(folded.custom.map((entry) => entry.registryId)).toEqual([entries[PROVIDER_CATALOG.length].registryId]);
    expect(folded.more.some((entry) => entry.custom)).toBe(false);
    expect(folded.visible.length + folded.custom.length + folded.more.length).toBe(entries.length);
    const searching = splitMergedByFold(searchMergedProviders('我的网关'), true);
    expect(searching.visible.length).toBe(1);
    expect(searching.custom).toEqual([]);
    expect(searching.more).toEqual([]);
  });
});
