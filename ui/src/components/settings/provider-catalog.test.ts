import { describe, expect, it } from 'vitest';
import type { ProviderCatalogEntry, ProviderProfileStatus } from '@/lib/api';
import {
  catalogRows,
  isProviderAdapter,
  isProviderValue,
  matchCatalogRow,
  normalizeProviderAdapter,
  normalizeProviderValue,
  projectProviderRow,
  providerRowKey,
  providerSelection,
} from './provider-catalog';

/**
 * 合成目录（PROV-P4：后端 settings/providers 的 catalog 片段）。测试只用这一
 * 个小对象，不引入真实 45 厂商目录；入选厂商覆盖四种形态：一厂商两端点
 * （deepseek）、可执行 + deferred 两端点（openai）、单端点（anthropic）、
 * 第三方网关（my-gateway）。
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
  {
    vendor: 'anthropic',
    display_name: 'Anthropic',
    endpoints: [
      { adapter: 'anthropic-messages', base_url: 'https://api.anthropic.com', default_model: 'claude-sonnet-4-5', models: ['claude-sonnet-4-5'], executable: true, state: 'SUPPORTED' },
    ],
  },
];

const PROFILES: ProviderProfileStatus[] = [
  { id: 'openai-completions', adapter_family: 'openai-completions', endpoint_class: 'native', model_ids: [], state: 'READY' },
  { id: 'openai-responses', adapter_family: 'openai-responses', endpoint_class: 'native', model_ids: [], state: 'DEFERRED-INDEFINITE' },
  { id: 'anthropic-messages', adapter_family: 'anthropic-messages', endpoint_class: 'native', model_ids: [], state: 'COMPILED' },
];

describe('provider value 归一（与后端 NormalizeAdapter 同口径）', () => {
  it('旧运行束名映射到密封适配器，并保留原厂商名', () => {
    expect(normalizeProviderValue('deepseek')).toEqual({ adapter: 'openai-completions', legacyVendor: 'deepseek' });
    expect(normalizeProviderValue('openai')).toEqual({ adapter: 'openai-completions', legacyVendor: 'openai' });
    expect(normalizeProviderValue('anthropic')).toEqual({ adapter: 'anthropic-messages', legacyVendor: 'anthropic' });
  });

  it('适配器 id 原样通过；未知取值原样返回（写侧由后端拒绝）', () => {
    expect(normalizeProviderValue('openai-responses')).toEqual({ adapter: 'openai-responses', legacyVendor: '' });
    expect(normalizeProviderAdapter('openai-completions')).toBe('openai-completions');
    expect(normalizeProviderAdapter('unknown')).toBe('unknown');
  });

  it('isProviderValue 接受适配器与旧运行束名，拒绝其它取值', () => {
    for (const value of ['openai-completions', 'openai-responses', 'anthropic-messages', 'openai', 'anthropic', 'deepseek']) {
      expect(isProviderValue(value)).toBe(true);
    }
    for (const value of ['unsupported', '', 42, null, undefined]) expect(isProviderValue(value)).toBe(false);
    expect(isProviderAdapter('openai-completions')).toBe(true);
    expect(isProviderAdapter('openai')).toBe(false);
  });
});

describe('catalogRows 目录投影', () => {
  it('一个端点一行：DeepSeek 贡献两行并共用一个 display_name', () => {
    const rows = catalogRows(CATALOG);
    expect(rows).toHaveLength(5);
    const deepseek = rows.filter((row) => row.vendor === 'deepseek');
    expect(deepseek.map((row) => row.adapter)).toEqual(['openai-completions', 'anthropic-messages']);
    expect(new Set(deepseek.map((row) => row.displayName))).toEqual(new Set(['DeepSeek']));
    expect(deepseek.map((row) => row.baseUrl)).toEqual(['https://api.deepseek.com', 'https://api.deepseek.com/anthropic']);
  });

  it('端点自带 executable/state 是基线：无 Profile 时 deferred 端点不可执行', () => {
    const rows = catalogRows(CATALOG);
    const deferred = rows.find((row) => row.adapter === 'openai-responses');
    expect(deferred).toMatchObject({ executable: false, capabilityState: 'DEFERRED-INDEFINITE' });
    const completions = rows.find((row) => row.vendor === 'openai' && row.adapter === 'openai-completions');
    expect(completions).toMatchObject({ executable: true, capabilityState: undefined });
  });

  it('Profile 叠加以适配器 id 为键：UNAVAILABLE 覆盖端点的可执行声明', () => {
    const rows = catalogRows(CATALOG, [{
      id: 'openai-completions', adapter_family: 'openai-completions', endpoint_class: 'native',
      model_ids: [], state: 'UNAVAILABLE',
    }]);
    const completions = rows.find((row) => row.vendor === 'openai' && row.adapter === 'openai-completions');
    expect(completions?.executable).toBe(false);
    expect(completions?.capabilityState).toBe('UNAVAILABLE');
    // 未列出 Profile 的适配器保持端点基线。
    expect(rows.find((row) => row.adapter === 'openai-responses')?.capabilityState).toBe('DEFERRED-INDEFINITE');
  });

  it('注册表行（无端点声明）只有 Profile 能判定可执行性', () => {
    const row = projectProviderRow({
      vendor: 'custom-1', displayName: '我的网关', adapter: 'openai-completions',
      baseUrl: 'https://gw.example.com/v1', defaultModel: 'm', models: ['m'],
    }, {}, PROFILES);
    expect(row).toMatchObject({ executable: true, capabilityState: 'READY' });
    const deferred = projectProviderRow({
      vendor: 'custom-2', displayName: 'Response 网关', adapter: 'openai-responses',
      baseUrl: 'https://gw.example.com/v1', defaultModel: 'gpt', models: ['gpt'],
    }, {}, PROFILES);
    expect(deferred).toMatchObject({ executable: false, capabilityState: 'DEFERRED-INDEFINITE' });
  });
});

describe('matchCatalogRow 与 providerSelection', () => {
  it('带地址按 (adapter, base_url) 精确命中；旧运行束名先归一', () => {
    const rows = catalogRows(CATALOG);
    expect(matchCatalogRow(rows, 'openai-completions', 'https://api.deepseek.com')?.vendor).toBe('deepseek');
    // 旧文档的 provider=openai + 网关地址：适配器归一后仍按地址命中。
    expect(matchCatalogRow(rows, 'openai', 'https://api.openai.com/v1')?.adapter).toBe('openai-completions');
    expect(matchCatalogRow(rows, 'openai-completions', 'https://unknown.example/v1')).toBeUndefined();
    // 同地址不同适配器各自命中（OpenAI 的 responses 变体）。
    expect(matchCatalogRow(rows, 'openai-responses', 'https://api.openai.com/v1')?.vendor).toBe('openai');
  });

  it('不带地址时旧运行束名回落到该厂商在该适配器下的端点', () => {
    const rows = catalogRows(CATALOG);
    expect(matchCatalogRow(rows, 'deepseek', '')).toMatchObject({ vendor: 'deepseek', adapter: 'openai-completions', baseUrl: 'https://api.deepseek.com' });
    expect(matchCatalogRow(rows, 'openai', '')).toMatchObject({ vendor: 'openai', adapter: 'openai-completions' });
    expect(matchCatalogRow(rows, 'anthropic', '')).toMatchObject({ vendor: 'anthropic', adapter: 'anthropic-messages' });
    // 已经是适配器 id 且没有地址：不回落到任意厂商。
    expect(matchCatalogRow(rows, 'openai-completions', '')).toBeUndefined();
    expect(matchCatalogRow(rows, '', '')).toBeUndefined();
  });

  it('选择三元组写适配器 id + 端点地址；不可执行行不给选择', () => {
    const rows = catalogRows(CATALOG);
    const deepseek = rows.find((row) => row.vendor === 'deepseek' && row.adapter === 'openai-completions')!;
    expect(providerSelection(deepseek, 'deepseek-v4-pro')).toEqual({
      provider: 'openai-completions', base_url: 'https://api.deepseek.com', default_model: 'deepseek-v4-pro',
    });
    const deferred = rows.find((row) => row.adapter === 'openai-responses')!;
    expect(deferred.executable).toBe(false);
    expect(providerSelection(deferred, 'gpt-4o')).toBeUndefined();
  });

  it('行身份是 (vendor, adapter)：厂商内适配器唯一', () => {
    const rows = catalogRows(CATALOG);
    expect(rows.map(providerRowKey)).toEqual([
      'deepseek/openai-completions',
      'deepseek/anthropic-messages',
      'openai/openai-completions',
      'openai/openai-responses',
      'anthropic/anthropic-messages',
    ]);
    expect(new Set(rows.map(providerRowKey)).size).toBe(rows.length);
  });
});