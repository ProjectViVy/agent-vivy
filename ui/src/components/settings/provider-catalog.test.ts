import { describe, expect, it } from 'vitest';
import {
  FOLDED_PROVIDER_NAMES,
  PROVIDER_CATALOG,
  findProvider,
  isFoldedProvider,
  matchProviderEntry,
  searchProviders,
  splitByFold,
} from './provider-catalog';

/** Agent-Diva ProvidersSettings.vue hiddenProviderNames 的原样清单（20 个）。 */
const DIVA_HIDDEN_PROVIDER_NAMES = [
  '302ai', 'aihubmix', 'aionly', 'baichuan', 'burncloud', 'cephalon',
  'cerebras', 'cherryin', 'fireworks', 'hyperbolic', 'infini', 'jina',
  'lanyun', 'ocoolai', 'ph8', 'ppio', 'together', 'tokenflux',
  'voyageai', 'yi',
] as const;

describe('provider catalog data', () => {
  it('来自 Agent-Diva 注册表 47 家供应商', () => {
    expect(PROVIDER_CATALOG).toHaveLength(47);
    const names = PROVIDER_CATALOG.map((entry) => entry.name);
    expect(new Set(names).size).toBe(names.length);
    for (const required of ['openai', 'anthropic', 'deepseek', 'custom']) {
      expect(names).toContain(required);
    }
    expect(names).not.toContain('mock');
  });

  it('运行束名只取后端接受的 openai/anthropic', () => {
    for (const entry of PROVIDER_CATALOG) {
      expect(['openai', 'anthropic']).toContain(entry.bundle);
    }
    expect(findProvider('anthropic')?.bundle).toBe('anthropic');
    expect(findProvider('deepseek')?.bundle).toBe('openai');
  });

  it('默认模型是原始 id：剥离网关前缀，custom 无推荐', () => {
    expect(findProvider('openai')?.defaultModel).toBe('gpt-4o');
    expect(findProvider('dashscope')?.defaultModel).toBe('qwen-max');
    // OpenRouter 的模型 id 本身带厂商段，只剥离自家网关前缀。
    expect(findProvider('openrouter')?.defaultModel).toBe('anthropic/claude-sonnet-4');
    expect(findProvider('vllm')?.defaultModel).toBe('meta-llama/Llama-3.3-70B-Instruct');
    expect(findProvider('custom')?.defaultModel).toBe('');
  });

  it('修复 diva 数据 bug：aionly 的 base_url 不再带全角冒号前缀', () => {
    expect(findProvider('aionly')?.baseUrl).toBe('https://api.aiionly.com/v1');
  });
});

describe('provider fold logic（Agent-Diva 折叠移植）', () => {
  it('折叠名单与 Agent-Diva hiddenProviderNames 完全一致且都存在于目录', () => {
    expect([...FOLDED_PROVIDER_NAMES].sort()).toEqual([...DIVA_HIDDEN_PROVIDER_NAMES].sort());
    for (const name of FOLDED_PROVIDER_NAMES) expect(findProvider(name)).toBeDefined();
    expect(isFoldedProvider('deepseek')).toBe(false);
    expect(isFoldedProvider('ppio')).toBe(true);
  });

  it('非搜索态按折叠名单拆分，两组互斥且并集为全集', () => {
    const { visible, more } = splitByFold(PROVIDER_CATALOG, false);
    expect(more.every((entry) => FOLDED_PROVIDER_NAMES.has(entry.name))).toBe(true);
    expect(visible.every((entry) => !FOLDED_PROVIDER_NAMES.has(entry.name))).toBe(true);
    expect(visible.length + more.length).toBe(PROVIDER_CATALOG.length);
  });

  it('搜索态绕过折叠：结果平铺，more 恒为空', () => {
    const matches = searchProviders('deep');
    const { visible, more } = splitByFold(matches, true);
    expect(visible).toBe(matches);
    expect(more).toEqual([]);
  });

  it('搜索按 displayName 与 name 匹配，大小写不敏感；空串返回全部', () => {
    expect(searchProviders('').length).toBe(PROVIDER_CATALOG.length);
    expect(searchProviders('deepseek').map((entry) => entry.name)).toEqual(['deepseek']);
    expect(searchProviders('302').map((entry) => entry.name)).toEqual(['302ai']);
    // 子串匹配 + 去空白 + 大小写不敏感（cherryin 同样含 "yi"）。
    expect(searchProviders('  yi  ').map((entry) => entry.name)).toEqual(['cherryin', 'yi']);
    expect(searchProviders('不存在供应商')).toEqual([]);
  });
});

describe('matchProviderEntry', () => {
  it('按 bundle + base_url 精确反查厂商条目', () => {
    expect(matchProviderEntry('openai', 'https://api.deepseek.com/v1')?.name).toBe('deepseek');
    expect(matchProviderEntry('openai', 'https://api.openai.com/v1')?.name).toBe('openai');
    expect(matchProviderEntry('anthropic', 'https://api.anthropic.com')?.name).toBe('anthropic');
  });

  it('base_url 为空时回落到与运行束同名的规范条目', () => {
    expect(matchProviderEntry('openai', '')?.name).toBe('openai');
    expect(matchProviderEntry('anthropic', '')?.name).toBe('anthropic');
  });

  it('未知组合返回 undefined（自定义网关/未知束名走手工输入路径）', () => {
    expect(matchProviderEntry('openai', 'https://my-gateway.example.com/v1')).toBeUndefined();
    expect(matchProviderEntry('', '')).toBeUndefined();
    expect(matchProviderEntry('deepseek', '')).toBeUndefined();
  });
});
