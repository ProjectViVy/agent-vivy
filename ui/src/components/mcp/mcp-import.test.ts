import { describe, expect, it } from 'vitest';
import { exportMcpConfig, parseMcpImport } from './mcp-import';

describe('parseMcpImport', () => {
  it('reads nested mcpServers maps and endpoint aliases', () => {
    const parsed = parseMcpImport({
      tools: {
        mcpServers: {
          docs: { url: 'https://docs.example.com/mcp', enabled: false, auth_env: 'MCP_DOCS_TOKEN' },
        },
      },
    });
    expect(parsed.servers).toEqual([
      { name: 'docs', transport: 'http', endpoint: 'https://docs.example.com/mcp', auth_env: 'MCP_DOCS_TOKEN', enabled: false },
    ]);
    expect(parsed.warnings).toEqual([]);
  });

  it('preserves stdio entries with argv and environment mappings', () => {
    const parsed = parseMcpImport({
      files: { command: 'npx', args: ['-y', '@modelcontextprotocol/server-filesystem', '.'], env_from: { ROOT: 'MCP_ROOT' }, cwd: 'tools' },
      remote: { endpoint: 'https://remote.example.com/mcp' },
    });
    expect(parsed.servers).toEqual([
      { name: 'files', transport: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-filesystem', '.'], env_from: { ROOT: 'MCP_ROOT' }, cwd: 'tools', enabled: true },
      { name: 'remote', transport: 'http', endpoint: 'https://remote.example.com/mcp', auth_env: undefined, enabled: true },
    ]);
    expect(parsed.warnings).toEqual([]);
  });

  it('preserves argv bytes and converts legacy env values to same-name refs with a warning', () => {
    const parsed = parseMcpImport({
      local: { command: 'node', args: ['', '  keep spaces  '], env: { TOKEN: 'secret-value' } },
    });
    expect(parsed.servers[0]).toEqual({
      name: 'local', transport: 'stdio', command: 'node', args: ['', '  keep spaces  '],
      env_from: { TOKEN: 'TOKEN' }, cwd: undefined, enabled: true,
    });
    expect(parsed.warnings).toEqual(['local']);
    expect(JSON.stringify(parsed)).not.toContain('secret-value');
  });
});

describe('exportMcpConfig', () => {
  it('writes kernel-shaped HTTP config without secrets', () => {
    expect(exportMcpConfig([
      { name: 'docs', transport: 'http', endpoint: 'https://docs.example.com/mcp', auth_env: 'MCP_DOCS_TOKEN', auth_env_set: true, enabled: true, tool_count: 2, status: 'ok' },
    ])).toEqual({
      mcp_servers: {
        docs: { transport: 'http', endpoint: 'https://docs.example.com/mcp', auth_env: 'MCP_DOCS_TOKEN', enabled: true },
      },
    });
  });

  it('writes stdio config without exposing resolved environment values', () => {
    expect(exportMcpConfig([
      { name: 'local', transport: 'stdio', command: 'node', args: ['server.js'], env_from: { MCP_TOKEN: 'HOST_TOKEN' }, cwd: 'tools', enabled: true, auth_env_set: false, tool_count: -1, status: 'idle' },
    ])).toEqual({
      mcp_servers: {
        local: { transport: 'stdio', command: 'node', args: ['server.js'], env_from: { MCP_TOKEN: 'HOST_TOKEN' }, cwd: 'tools', enabled: true },
      },
    });
  });

  it('keeps empty and whitespace argv items in export', () => {
    expect(exportMcpConfig([
      { name: 'local', transport: 'stdio', command: 'node', args: ['', '  keep spaces  '], env_from: { TOKEN: 'HOST_TOKEN' }, auth_env_set: false, tool_count: -1, status: 'idle', enabled: true },
    ])).toEqual({
      mcp_servers: {
        local: { transport: 'stdio', command: 'node', args: ['', '  keep spaces  '], env_from: { TOKEN: 'HOST_TOKEN' }, enabled: true },
      },
    });
  });
});
