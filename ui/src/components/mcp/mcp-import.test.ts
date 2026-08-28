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
      { name: 'docs', endpoint: 'https://docs.example.com/mcp', auth_env: 'MCP_DOCS_TOKEN', enabled: false },
    ]);
    expect(parsed.skippedStdio).toEqual([]);
  });

  it('skips stdio entries instead of pretending they are HTTP', () => {
    const parsed = parseMcpImport({
      files: { command: 'npx -y @modelcontextprotocol/server-filesystem .' },
      remote: { endpoint: 'https://remote.example.com/mcp' },
    });
    expect(parsed.servers).toEqual([
      { name: 'remote', endpoint: 'https://remote.example.com/mcp', auth_env: undefined, enabled: true },
    ]);
    expect(parsed.skippedStdio).toEqual(['files']);
  });
});

describe('exportMcpConfig', () => {
  it('writes kernel-shaped HTTP config without secrets', () => {
    expect(exportMcpConfig([
      { name: 'docs', endpoint: 'https://docs.example.com/mcp', auth_env: 'MCP_DOCS_TOKEN', auth_env_set: true, enabled: true, tool_count: 2, status: 'ok' },
    ])).toEqual({
      mcp_servers: {
        docs: { endpoint: 'https://docs.example.com/mcp', auth_env: 'MCP_DOCS_TOKEN', enabled: true },
      },
    });
  });
});
