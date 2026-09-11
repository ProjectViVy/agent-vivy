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

function asBoolean(value: unknown): boolean {
  return value === true;
}

function asArgs(value: unknown): string[] {
  if (Array.isArray(value)) return value.filter((item): item is string => typeof item === 'string');
  if (typeof value === 'string') return value.split(/\r?\n/);
  return [];
}

function asEnvFrom(value: unknown): Record<string, string> | undefined {
  if (!isRecord(value)) return undefined;
  const entries = Object.entries(value).filter(([, item]) => typeof item === 'string') as [string, string][];
  return entries.length ? Object.fromEntries(entries) : undefined;
}

function legacyEnvRefs(value: unknown, name: string, warnings: string[]): Record<string, string> | undefined {
  if (!isRecord(value)) return undefined;
  const keys = Object.keys(value);
  if (!keys.length) return undefined;
  warnings.push(name);
  return Object.fromEntries(keys.map((key) => [key, key]));
}

export interface ParsedMcpImport {
  servers: McpServerInput[];
  /** Server names whose legacy env values were discarded and converted to same-name refs. */
  warnings: string[];
}

/** 把常见 MCP JSON 规范成 HTTP/stdio upsert 列表。 */
export function parseMcpImport(config: unknown): ParsedMcpImport {
  const servers: McpServerInput[] = [];
  const warnings: string[] = [];

  const append = (nameHint: string, value: unknown) => {
    const record = isRecord(value) ? value : {};
    const name = asString(record.name) || nameHint.trim();
    if (!name) return;
    const command = asString(record.command);
    const endpoint = asString(record.endpoint) || asString(record.url);
    const transport = asString(record.transport).toLowerCase();
    const looksStdio = transport === 'stdio' || (command !== '' && endpoint === '');
    const resourceBridge = asBoolean(record.resource_bridge ?? record.resourceBridge);
    const deferredReason = asString(record.deferred_reason ?? record.deferredReason) || undefined;
    if (looksStdio) {
      servers.push({
        name,
        transport: 'stdio',
        command,
        args: asArgs(record.args ?? record.argv),
        env_from: asEnvFrom(record.env_from ?? record.envFrom) ?? legacyEnvRefs(record.env, name, warnings),
        cwd: asString(record.cwd) || undefined,
        ...(resourceBridge ? { resource_bridge: true } : {}),
        ...(deferredReason ? { deferred_reason: deferredReason } : {}),
        enabled: asEnabled(record.enabled),
      });
      return;
    }
    if (!endpoint) return;
    servers.push({
      name,
      transport: 'http',
      endpoint,
      auth_env: asString(record.auth_env) || asString(record.authEnv) || undefined,
      ...(resourceBridge ? { resource_bridge: true } : {}),
      ...(deferredReason ? { deferred_reason: deferredReason } : {}),
      enabled: asEnabled(record.enabled),
    });
  };

  if (Array.isArray(config)) {
    config.forEach((value) => append(isRecord(value) ? asString(value.name) : '', value));
    return { servers, warnings };
  }
  if (!isRecord(config)) return { servers, warnings };

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
    return { servers, warnings };
  }
  if ('name' in config || 'endpoint' in config || 'url' in config || 'command' in config) {
    append('', config);
    return { servers, warnings };
  }
  Object.entries(config).forEach(([name, value]) => append(name, value));
  return { servers, warnings };
}

export function exportMcpConfig(servers: McpServer[]): { mcp_servers: Record<string, Record<string, unknown>> } {
  return {
    mcp_servers: Object.fromEntries(servers.map((server) => [server.name, {
      transport: server.transport,
      ...(server.transport === 'stdio'
        ? {
            command: server.command,
            ...(server.args?.length ? { args: server.args } : {}),
            ...(server.env_from && Object.keys(server.env_from).length ? { env_from: server.env_from } : {}),
            ...(server.cwd ? { cwd: server.cwd } : {}),
          }
        : { endpoint: server.endpoint }),
      ...(server.transport === 'http' && server.auth_env ? { auth_env: server.auth_env } : {}),
      ...(server.resource_bridge ? { resource_bridge: true } : {}),
      ...(server.deferred_reason ? { deferred_reason: server.deferred_reason } : {}),
      enabled: server.enabled,
    }])),
  };
}
