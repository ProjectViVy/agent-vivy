import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  SAVED_MODELS_KEY,
  addSavedModel,
  getSavedModels,
  removeSavedModel,
  savedModelVendorLabel,
} from './saved-models';

type ListenerMap = Record<string, Array<() => void>>;

/** stub window.localStorage + 事件监听，模拟浏览器环境（模块按需读取，不依赖 react 渲染）。 */
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

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('getSavedModels 读取与坏数据过滤', () => {
  it('SSR guard：无 window 时返回空数组且不抛错', () => {
    vi.stubGlobal('window', undefined);
    expect(getSavedModels()).toEqual([]);
    expect(() => addSavedModel({ provider: 'openai', baseUrl: '', model: 'gpt-4o' })).not.toThrow();
    expect(() => removeSavedModel('openai', '', 'gpt-4o')).not.toThrow();
  });

  it('存储为空或未设置时返回空数组', () => {
    stubWindow();
    expect(getSavedModels()).toEqual([]);
  });

  it('坏 JSON 返回空数组', () => {
    stubWindow({ [SAVED_MODELS_KEY]: '{broken json' });
    expect(getSavedModels()).toEqual([]);
  });

  it('非数组 JSON 返回空数组', () => {
    stubWindow({ [SAVED_MODELS_KEY]: '{"provider":"openai"}' });
    expect(getSavedModels()).toEqual([]);
  });

  it('缺字段/非字符串/空白条目整条丢弃，合法条目保留且顺序不变', () => {
    stubWindow({
      [SAVED_MODELS_KEY]: JSON.stringify([
        { provider: 'openai', baseUrl: 'https://api.deepseek.com/v1', model: 'deepseek-chat' },
        { provider: 'anthropic', baseUrl: 'https://api.anthropic.com', model: 'claude-sonnet-4-5' },
        { provider: 'openai', model: 'gpt-4o' }, // 缺 baseUrl
        { provider: 42, baseUrl: 'https://x', model: 'm' }, // provider 非字符串
        { provider: '  ', baseUrl: 'https://x', model: 'm' }, // provider 空白
        { provider: 'mock', baseUrl: '', model: '  ' }, // model 空白
        null,
        'not-an-object',
        { provider: 'openai', baseUrl: '', model: 'gpt-4o-mini' },
      ]),
    });
    expect(getSavedModels()).toEqual([
      { provider: 'openai', baseUrl: 'https://api.deepseek.com/v1', model: 'deepseek-chat' },
      { provider: 'anthropic', baseUrl: 'https://api.anthropic.com', model: 'claude-sonnet-4-5' },
      { provider: 'openai', baseUrl: '', model: 'gpt-4o-mini' },
    ]);
  });
});

describe('addSavedModel 去重与追加顺序', () => {
  it('新条目尾部追加，读取顺序与添加顺序一致', () => {
    const { storage } = stubWindow();
    addSavedModel({ provider: 'openai', baseUrl: '', model: 'gpt-4o' });
    addSavedModel({ provider: 'anthropic', baseUrl: 'https://api.anthropic.com', model: 'claude-sonnet-4-5' });
    expect(getSavedModels()).toEqual([
      { provider: 'openai', baseUrl: '', model: 'gpt-4o' },
      { provider: 'anthropic', baseUrl: 'https://api.anthropic.com', model: 'claude-sonnet-4-5' },
    ]);
    expect(storage.get(SAVED_MODELS_KEY)).toBe(
      JSON.stringify([
        { provider: 'openai', baseUrl: '', model: 'gpt-4o' },
        { provider: 'anthropic', baseUrl: 'https://api.anthropic.com', model: 'claude-sonnet-4-5' },
      ]),
    );
  });

  it('同 triple 幂等：重复添加不新增、不移动位置', () => {
    const { storage } = stubWindow();
    addSavedModel({ provider: 'openai', baseUrl: '', model: 'gpt-4o' });
    addSavedModel({ provider: 'openai', baseUrl: '', model: 'gpt-4o' });
    addSavedModel({ provider: 'mock', baseUrl: '', model: 'mock' });
    addSavedModel({ provider: 'openai', baseUrl: '', model: 'gpt-4o' });
    expect(getSavedModels()).toEqual([
      { provider: 'openai', baseUrl: '', model: 'gpt-4o' },
      { provider: 'mock', baseUrl: '', model: 'mock' },
    ]);
    expect(storage.get(SAVED_MODELS_KEY)).toBe(
      JSON.stringify([
        { provider: 'openai', baseUrl: '', model: 'gpt-4o' },
        { provider: 'mock', baseUrl: '', model: 'mock' },
      ]),
    );
  });

  it('base_url 不同视为不同条目（同 provider + 同 model）', () => {
    stubWindow();
    addSavedModel({ provider: 'openai', baseUrl: 'https://api.deepseek.com/v1', model: 'deepseek-chat' });
    addSavedModel({ provider: 'openai', baseUrl: 'https://api.openai.com/v1', model: 'deepseek-chat' });
    expect(getSavedModels()).toHaveLength(2);
  });
});

describe('removeSavedModel 按三元组过滤', () => {
  it('只移除命中的 triple，其余保持顺序', () => {
    const { storage } = stubWindow({
      [SAVED_MODELS_KEY]: JSON.stringify([
        { provider: 'openai', baseUrl: '', model: 'gpt-4o' },
        { provider: 'anthropic', baseUrl: 'https://api.anthropic.com', model: 'claude-sonnet-4-5' },
        { provider: 'mock', baseUrl: '', model: 'mock' },
      ]),
    });
    removeSavedModel('anthropic', 'https://api.anthropic.com', 'claude-sonnet-4-5');
    expect(getSavedModels()).toEqual([
      { provider: 'openai', baseUrl: '', model: 'gpt-4o' },
      { provider: 'mock', baseUrl: '', model: 'mock' },
    ]);
    expect(storage.get(SAVED_MODELS_KEY)).toBe(
      JSON.stringify([
        { provider: 'openai', baseUrl: '', model: 'gpt-4o' },
        { provider: 'mock', baseUrl: '', model: 'mock' },
      ]),
    );
  });

  it('未命中三元组时列表与写回均不变', () => {
    const initial = JSON.stringify([{ provider: 'openai', baseUrl: '', model: 'gpt-4o' }]);
    const { storage } = stubWindow({ [SAVED_MODELS_KEY]: initial });
    removeSavedModel('openai', 'https://api.other.example/v1', 'gpt-4o');
    removeSavedModel('openai', '', 'not-a-model');
    expect(getSavedModels()).toEqual([{ provider: 'openai', baseUrl: '', model: 'gpt-4o' }]);
    expect(storage.get(SAVED_MODELS_KEY)).toBe(initial);
  });
});

describe('savedModelVendorLabel 厂商标签解析', () => {
  it('目录命中返回厂商 displayName（base_url 精确匹配）', () => {
    expect(savedModelVendorLabel({ provider: 'openai', baseUrl: 'https://api.deepseek.com/v1', model: 'deepseek-chat' })).toBe('DeepSeek');
    expect(savedModelVendorLabel({ provider: 'anthropic', baseUrl: 'https://api.anthropic.com', model: 'claude-sonnet-4-5' })).toBe('Anthropic');
  });

  it('base_url 空且与运行束同名的目录条目也命中', () => {
    expect(savedModelVendorLabel({ provider: 'openai', baseUrl: '', model: 'gpt-4o' })).toBe('OpenAI');
    expect(savedModelVendorLabel({ provider: 'mock', baseUrl: '', model: 'mock' })).toBe('Mock');
  });

  it('目录未命中（自定义网关/未知束名）回退到原始 provider', () => {
    expect(savedModelVendorLabel({ provider: 'openai', baseUrl: 'https://my-gateway.example.com/v1', model: 'custom-model' })).toBe('openai');
    expect(savedModelVendorLabel({ provider: 'my-bundle', baseUrl: '', model: 'm' })).toBe('my-bundle');
  });
});

describe('事件广播与跨标签页同步', () => {
  it('addSavedModel 触发 vivy.ui.savedModels.changed 自定义事件监听器', () => {
    const { listeners } = stubWindow();
    const onChange = vi.fn();
    window.addEventListener('vivy.ui.savedModels.changed', onChange);
    addSavedModel({ provider: 'openai', baseUrl: '', model: 'gpt-4o' });
    expect(onChange).toHaveBeenCalledTimes(1);
    // 同 triple 幂等：不写回也就无需广播。
    addSavedModel({ provider: 'openai', baseUrl: '', model: 'gpt-4o' });
    expect(onChange).toHaveBeenCalledTimes(1);
  });

  it('storage 事件（另一标签页写入）后重新读取到新列表', () => {
    const { storage, listeners } = stubWindow({ [SAVED_MODELS_KEY]: '[]' });
    expect(getSavedModels()).toEqual([]);
    storage.set(SAVED_MODELS_KEY, JSON.stringify([{ provider: 'anthropic', baseUrl: '', model: 'claude-sonnet-4-5' }]));
    for (const listener of listeners['storage'] ?? []) listener();
    expect(getSavedModels()).toEqual([{ provider: 'anthropic', baseUrl: '', model: 'claude-sonnet-4-5' }]);
  });

  it('removeSavedModel 触发自定义事件广播', () => {
    const { listeners } = stubWindow({
      [SAVED_MODELS_KEY]: JSON.stringify([{ provider: 'mock', baseUrl: '', model: 'mock' }]),
    });
    const onChange = vi.fn();
    window.addEventListener('vivy.ui.savedModels.changed', onChange);
    removeSavedModel('mock', '', 'mock');
    expect(onChange).toHaveBeenCalledTimes(1);
    // 未命中不写回，不广播。
    removeSavedModel('mock', '', 'absent');
    expect(onChange).toHaveBeenCalledTimes(1);
  });
});
