import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { addDemoMcpServer, exportDemoMcpConfig, getDemoDashboard, getDemoMemories, getDemoMcpServers, getDemoPet, getDemoComposerState, importDemoMcpConfig, interactWithDemoPet, searchSessions, toggleDemoMcpServer, updateDemoComposerState } from './demo-api';

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
    await settle(Promise.all([getDemoPet(), getDemoDashboard(), getDemoMemories(), getDemoMcpServers()]));
    expect([...values.keys()].sort()).toEqual(['vivy.demo.dashboard', 'vivy.demo.mcp', 'vivy.demo.memory', 'vivy.demo.pet']);
  });

  it('persists pet, MCP and composer interactions', async () => {
    const pet = await settle(interactWithDemoPet('focused'));
    expect(pet.mood).toBe('focused');
    const servers = await settle(toggleDemoMcpServer('mcp-browser'));
    expect(servers.find((server) => server.id === 'mcp-browser')?.enabled).toBe(true);
    await getDemoComposerState();
    const composer = await updateDemoComposerState({ secure: false, recording: true });
    expect(composer).toMatchObject({ secure: false, recording: true });
    expect([...values.keys()].every((key) => key.startsWith('vivy.demo.'))).toBe(true);
  });

  it('adds MCP servers and round-trips nested tools.mcpServers config', async () => {
    const added = await settle(addDemoMcpServer({ name: 'Local Search', transport: 'stdio' }));
    expect(added.find((server) => server.name === 'Local Search')).toMatchObject({ enabled: true, status: 'connected' });

    const imported = await settle(importDemoMcpConfig({
      tools: {
        mcpServers: {
          'Remote Docs': { url: 'https://example.test/mcp', enabled: false, toolCount: 3 },
        },
      },
    }));
    expect(imported.find((server) => server.name === 'Remote Docs')).toMatchObject({ transport: 'http', enabled: false, status: 'disabled', toolCount: 3 });

    const exported = await settle(exportDemoMcpConfig());
    expect(exported.mcpServers['Remote Docs']).toMatchObject({ transport: 'http', enabled: false, toolCount: 3 });
  });

  it('recovers a restored surface from malformed local data', async () => {
    values.set('vivy.demo.pet', '{not-json');
    const pet = await settle(getDemoPet());
    expect(pet.mood).toBe('curious');
    expect(() => JSON.parse(values.get('vivy.demo.pet') ?? '')).not.toThrow();
  });

  it('searches only the local demo session collection', async () => {
    values.set('vivy.demo.sessions', JSON.stringify([{ id: 's1', title: 'Demo', created_at: '', updated_at: '', message_count: 1 }]));
    values.set('vivy.demo.messages-s1', JSON.stringify([{ id: 'm1', role: 'user', content: 'needle in demo', timestamp: 1 }]));
    const result = await settle(searchSessions('needle'));
    expect(result.hits).toHaveLength(1);
    expect(result.hits[0].session_id).toBe('s1');
  });
});
