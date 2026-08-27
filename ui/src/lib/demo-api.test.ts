import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { addDemoMcpServer, exportDemoMcpConfig, formatTokenCost, formatTokenCount, getDemoDashboard, getDemoGenParams, getDemoMemories, getDemoMcpServers, getDemoComposerState, getDemoTokenUsage, importDemoMcpConfig, removeDemoMcpServer, saveDemoGenParams, searchSessions, toggleDemoMcpServer, updateDemoComposerState, updateDemoMcpServer } from './demo-api';

const values = new Map<string, string>();

beforeEach(() => {
  values.clear();
  vi.useFakeTimers();
  vi.stubGlobal('localStorage', {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => values.set(key, value),
    removeItem: (key: string) => values.delete(key),
    clear: () => values.clear(),
    key: (index: number) => [...values.keys()][index] ?? null,
    get length() { return values.size; },
  });
});

afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals(); });

async function settle<T>(promise: Promise<T>): Promise<T> {
  await vi.runAllTimersAsync();
  return promise;
}

describe('restored local demo API', () => {
  it('initializes every restored surface under vivy.demo.* keys', async () => {
    await settle(Promise.all([getDemoDashboard(), getDemoMemories(), getDemoMcpServers()]));
    expect([...values.keys()].sort()).toEqual(['vivy.demo.dashboard', 'vivy.demo.mcp', 'vivy.demo.memory']);
  });

  it('persists MCP and composer interactions', async () => {
    const servers = await settle(toggleDemoMcpServer('mcp-browser'));
    expect(servers.find((server) => server.id === 'mcp-browser')?.enabled).toBe(true);
    await getDemoComposerState();
    const composer = await updateDemoComposerState({ secure: false, recording: true });
    expect(composer).toMatchObject({ secure: false, recording: true });
    expect([...values.keys()].every((key) => key.startsWith('vivy.demo.'))).toBe(true);
  });

  it('adds MCP servers and round-trips nested tools.mcpServers config', async () => {
    const added = await settle(addDemoMcpServer({ name: 'Local Search', transport: 'stdio', command: 'mcp-local-search' }));
    expect(added.find((server) => server.name === 'Local Search')).toMatchObject({ enabled: true, command: 'mcp-local-search' });

    const imported = await settle(importDemoMcpConfig({
      tools: {
        mcpServers: {
          'Remote Docs': { url: 'https://example.test/mcp', enabled: false, toolCount: 3 },
        },
      },
    }));
    expect(imported.find((server) => server.name === 'Remote Docs')).toMatchObject({ transport: 'http', url: 'https://example.test/mcp', enabled: false, toolCount: 3 });

    const exported = await settle(exportDemoMcpConfig());
    expect(exported.mcpServers['Remote Docs']).toMatchObject({ transport: 'http', url: 'https://example.test/mcp', enabled: false, toolCount: 3 });
    expect(exported.mcpServers['Local Search']).toMatchObject({ transport: 'stdio', command: 'mcp-local-search', enabled: true });
  });

  it('updates, removes and validates MCP servers', async () => {
    const updated = await settle(updateDemoMcpServer('mcp-browser', { name: 'Browser Tools', transport: 'http', url: 'https://browser.test/mcp' }));
    expect(updated.find((server) => server.id === 'mcp-browser')).toMatchObject({ url: 'https://browser.test/mcp', enabled: false, toolCount: 5 });

    const invalidUrl = addDemoMcpServer({ name: 'Broken', transport: 'http', url: 'not a url' });
    const missingCommand = addDemoMcpServer({ name: 'No Command', transport: 'stdio' });
    const duplicate = addDemoMcpServer({ name: 'Workspace Files', transport: 'stdio', command: 'dup' });
    const assertions = [
      expect(invalidUrl).rejects.toThrow('HTTP 服务地址不是有效的 URL'),
      expect(missingCommand).rejects.toThrow('请输入 STDIO 启动命令'),
      expect(duplicate).rejects.toThrow('已存在同名 MCP 服务'),
    ];
    await vi.runAllTimersAsync();
    await Promise.all(assertions);

    const removed = await settle(removeDemoMcpServer('mcp-files'));
    expect(removed.map((server) => server.id)).toEqual(['mcp-browser']);
  });

  it('recovers a restored surface from malformed local data', async () => {
    values.set('vivy.demo.dashboard', '{not-json');
    const dashboard = await settle(getDemoDashboard());
    expect(dashboard.sessionCount).toBe(12);
    expect(() => JSON.parse(values.get('vivy.demo.dashboard') ?? '')).not.toThrow();
  });

  it('returns a period-scoped token snapshot without writing localStorage', async () => {
    const day = await settle(getDemoTokenUsage('1d'));
    const week = await settle(getDemoTokenUsage('1w'));
    expect(day.period).toBe('1d');
    expect(week.period).toBe('1w');
    expect(week.total.total_tokens).toBeGreaterThan(day.total.total_tokens);
    expect(day.total.total_tokens).toBe(day.sessions.reduce((sum, session) => sum + session.total_tokens, 0));
    expect(day.sessions.map((session) => session.title)).toEqual([
      '欢迎使用 Vivy 演示',
      '中控台设计讨论',
      '插件打包排障',
      '人格文档整理',
      '定时任务验收',
    ]);
    expect(day.models[0]?.model).toBe('deepseek-chat');
    expect(day.endpoints.map((endpoint) => endpoint.key)).toEqual(['对话补全', '工具调用', '上下文压缩']);
    expect([...values.keys()]).not.toContain('vivy.demo.tokens');
  });

  it('formats token counts and costs for the dashboard', () => {
    expect(formatTokenCount(42860)).toBe('42.9K');
    expect(formatTokenCount(1_250_000)).toBe('1.25M');
    expect(formatTokenCost(0.0042)).toBe('$0.0042');
    expect(formatTokenCost(1.2)).toBe('$1.20');
  });

  it('searches only the local demo session collection', async () => {
    values.set('vivy.demo.sessions', JSON.stringify([{ id: 's1', title: 'Demo', created_at: '', updated_at: '', message_count: 1 }]));
    values.set('vivy.demo.messages-s1', JSON.stringify([{ id: 'm1', role: 'user', content: 'needle in demo', timestamp: 1 }]));
    const result = await settle(searchSessions('needle'));
    expect(result.hits).toHaveLength(1);
    expect(result.hits[0].session_id).toBe('s1');
  });

  it('returns default demo generation params without writing localStorage', () => {
    expect(getDemoGenParams('openai//gpt-4o')).toEqual({ temperature: 0.7, max_tokens: 4096 });
    expect([...values.keys()]).not.toContain('vivy.demo.gen-params');
  });

  it('keeps demo generation params independent per model and persists them', () => {
    const saved = saveDemoGenParams('openai//gpt-4o', { temperature: 1.2, max_tokens: 8192 });
    expect(saved).toEqual({ temperature: 1.2, max_tokens: 8192 });
    expect(getDemoGenParams('openai//gpt-4o')).toEqual({ temperature: 1.2, max_tokens: 8192 });
    // 其他模型不受影响，仍为默认值。
    expect(getDemoGenParams('openai//gpt-4o-mini')).toEqual({ temperature: 0.7, max_tokens: 4096 });

    const store = JSON.parse(values.get('vivy.demo.gen-params') ?? '{}') as Record<string, unknown>;
    expect(store['openai//gpt-4o']).toEqual({ temperature: 1.2, max_tokens: 8192 });
    expect(store['openai//gpt-4o-mini']).toBeUndefined();
    expect([...values.keys()].every((key) => key.startsWith('vivy.demo.'))).toBe(true);
  });

  it('normalizes saved demo generation params and recovers from malformed data', () => {
    expect(saveDemoGenParams('mock//mock:hitl', { temperature: 9, max_tokens: 0 })).toEqual({ temperature: 2, max_tokens: 1 });

    values.set('vivy.demo.gen-params', '{not-json');
    expect(getDemoGenParams('openai//gpt-4o')).toEqual({ temperature: 0.7, max_tokens: 4096 });

    values.set('vivy.demo.gen-params', JSON.stringify({ 'openai//gpt-4o': { temperature: 'hot', max_tokens: 128 } }));
    expect(getDemoGenParams('openai//gpt-4o')).toEqual({ temperature: 0.7, max_tokens: 128 });
  });
});
