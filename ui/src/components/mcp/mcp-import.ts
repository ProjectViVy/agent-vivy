import type { McpServer, McpServerInput } from '@/lib/api';

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function asString(value: unknown): string {
  return typeof value === 'string' ? value.trim() : '';
}

function asEnabled(value: unknown): boolean {
  return typeof value === 'boolean' ? value : true;
}

export interface ParsedMcpImport {
  servers: McpServerInput[];
  skippedStdio: string[];
}

/** 把常见 MCP JSON 规范成 HTTP upsert 列表；stdio 项记入 skippedStdio。 */
export function parseMcpImport(config: unknown): ParsedMcpImport {
  const servers: McpServerInput[] = [];
  const skippedStdio: string[] = [];

  const append = (nameHint: string, value: unknown) => {
    const record = isRecord(value) ? value : {};
    const name = asString(record.name) || nameHint.trim();
    if (!name) return;
    const command = asString(record.command);
    const endpoint = asString(record.endpoint) || asString(record.url);
    const transport = asString(record.transport).toLowerCase();
    const looksStdio = transport === 'stdio' || (command !== '' && endpoint === '');
    if (looksStdio) {
      skippedStdio.push(name);
      return;
    }
    if (!endpoint) return;
    servers.push({
      name,
      endpoint,
      auth_env: asString(record.auth_env) || asString(record.authEnv) || undefined,
      enabled: asEnabled(record.enabled),
    });
  };

  if (Array.isArray(config)) {
    config.forEach((value) => append(isRecord(value) ? asString(value.name) : '', value));
    return { servers, skippedStdio };
  }
  if (!isRecord(config)) return { servers, skippedStdio };

  const tools = isRecord(config.tools) ? config.tools : null;
  const serverMap = isRecord(config.mcpServers)
    ? config.mcpServers
    : isRecord(config.mcp_servers)
      ? config.mcp_servers
      : tools && isRecord(tools.mcpServers)
        ? tools.mcpServers
        : tools && isRecord(tools.mcp_servers)
          ? tools.mcp_servers
          : null;
  if (serverMap) {
    if (Array.isArray(serverMap)) serverMap.forEach((value) => append('', value));
    else Object.entries(serverMap).forEach(([name, value]) => append(name, value));
    return { servers, skippedStdio };
  }
  if ('name' in config || 'endpoint' in config || 'url' in config || 'command' in config) {
    append('', config);
    return { servers, skippedStdio };
  }
  Object.entries(config).forEach(([name, value]) => append(name, value));
  return { servers, skippedStdio };
}

export function exportMcpConfig(servers: McpServer[]): { mcp_servers: Record<string, Record<string, unknown>> } {
  return {
    mcp_servers: Object.fromEntries(servers.map((server) => [server.name, {
      endpoint: server.endpoint,
      ...(server.auth_env ? { auth_env: server.auth_env } : {}),
      enabled: server.enabled,
    }])),
  };
}
