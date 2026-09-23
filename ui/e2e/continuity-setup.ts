import { createHash } from 'node:crypto';
import fs from 'node:fs';
import http from 'node:http';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import type { Download } from '@playwright/test';

// Continuity E2E harness (T12): an isolated temp config/workspace/database,
// a deterministic loopback model speaking the OpenAI chat.completions wire
// protocol, and shared helpers. The backend only reaches the model lazily
// when a turn starts, so the spec owns the server lifecycle in beforeAll.
export const CONTINUITY_ADDR = '127.0.0.1:8798';
export const MODEL_ADDR = '127.0.0.1:8797';
export const UI_ADDR = '127.0.0.1:3015';

const here = path.dirname(fileURLToPath(import.meta.url));
export const continuityWorkdir = path.resolve(here, '..', '.continuity-workdir');
export const continuityConfig = path.join(continuityWorkdir, 'config.yaml');
export const continuityWorkspaceRoot = path.join(continuityWorkdir, 'workspaces');

// The five continuity tools plus the filesystem/shell set the coding loop
// needs; the baseline E2E config's echo_info/write_note/ask_user alone is
// insufficient per the T12 interface contract.
const CONTINUITY_TOOLS = [
  'echo_info', 'write_note', 'ask_user',
  'list_dir', 'read_file', 'search_files', 'write_file', 'patch', 'multiedit',
  'execute', 'bash',
  'history_search', 'history_read', 'history_trace', 'reference_context', 'present_files',
];

export function prepareContinuityWorkdir(): void {
  fs.rmSync(continuityWorkdir, { recursive: true, force: true });
  fs.mkdirSync(path.join(continuityWorkdir, 'state'), { recursive: true });
  const dbPath = path.join(continuityWorkdir, 'state', 'continuity.db').replace(/\\/g, '/');
  const workspaceRoot = continuityWorkspaceRoot.replace(/\\/g, '/');
  fs.writeFileSync(continuityConfig, [
    'server:', `  addr: "${CONTINUITY_ADDR}"`,
    'storage:', '  backend: sqlite', '  sqlite:', `    path: "${dbPath}"`,
    'providers:', '  active: deepseek',
    'runtime:', `  workspace_root: "${workspaceRoot}"`, '  stream_buffer: 256', '  max_event_payload_bytes: 65536',
    'tools:', '  enabled:',
    ...CONTINUITY_TOOLS.map((name) => `    - ${name}`),
    '  approval:', '    expiration: 5m',
    'governance:', '  profile: full_auto', '',
  ].join('\n'), 'utf8');
}

export type LoopbackReply =
  | { kind: 'tool'; id: string; name: string; args: Record<string, unknown> }
  | { kind: 'text'; text: string };

export const toolCall = (id: string, name: string, args: Record<string, unknown>): LoopbackReply => ({ kind: 'tool', id, name, args });
export const finalText = (text: string): LoopbackReply => ({ kind: 'text', text });

const modelBase = `http://${MODEL_ADDR}`;

export async function setModelScript(script: LoopbackReply[]): Promise<void> {
  const res = await fetch(`${modelBase}/__script`, { method: 'POST', body: JSON.stringify(script) });
  if (!res.ok) throw new Error(`set loopback script failed: ${res.status}`);
}

export async function modelRequestBodies(): Promise<string[]> {
  const res = await fetch(`${modelBase}/__requests`);
  if (!res.ok) throw new Error(`read loopback requests failed: ${res.status}`);
  return (await res.json()) as string[];
}

function writeChunk(w: http.ServerResponse, payload: string): void {
  w.write(`data: ${payload}\n\n`);
}

function answerRequest(res: http.ServerResponse, reply: LoopbackReply, streaming: boolean, seq: number): void {
  const base = `"id":"chatcmpl-${seq}","object":"chat.completion${streaming ? '.chunk' : ''}","created":1,"model":"deepseek-flash"`;
  if (reply.kind === 'tool') {
    const argsJSON = JSON.stringify(JSON.stringify(reply.args));
    const name = JSON.stringify(reply.name);
    const callID = JSON.stringify(reply.id);
    if (!streaming) {
      res.end(`{${base},"choices":[{"index":0,"message":{"role":"assistant","content":null,"tool_calls":[{"id":${callID},"type":"function","function":{"name":${name},"arguments":${argsJSON}}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`);
      return;
    }
    writeChunk(res, `{${base},"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":${callID},"type":"function","function":{"name":${name},"arguments":""}}]},"finish_reason":null}]}`);
    writeChunk(res, `{${base},"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":${argsJSON}}}]},"finish_reason":null}]}`);
    writeChunk(res, `{${base},"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`);
    res.end('data: [DONE]\n\n');
    return;
  }
  const content = JSON.stringify(reply.text);
  if (!streaming) {
    res.end(`{${base},"choices":[{"index":0,"message":{"role":"assistant","content":${content}},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`);
    return;
  }
  writeChunk(res, `{${base},"choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`);
  writeChunk(res, `{${base},"choices":[{"index":0,"delta":{"content":${content}},"finish_reason":null}]}`);
  writeChunk(res, `{${base},"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`);
  res.end('data: [DONE]\n\n');
}

// startLoopbackModel answers every chat.completions request with the next
// queued reply (FIFO per test) and records raw request bodies so specs can
// assert cross-session content never reached the model.
export function startLoopbackModel(): Promise<http.Server> {
  let queue: LoopbackReply[] = [];
  const captured: string[] = [];
  let seq = 0;
  const server = http.createServer((req, res) => {
    if (req.method === 'GET' && req.url === '/__healthz') { res.end('ok'); return; }
    if (req.method === 'GET' && req.url === '/__requests') {
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify(captured));
      return;
    }
    if (req.method === 'POST' && req.url === '/__script') {
      let body = '';
      req.on('data', (chunk) => { body += chunk; });
      req.on('end', () => { queue = JSON.parse(body) as LoopbackReply[]; res.end('ok'); });
      return;
    }
    if (req.method !== 'POST') { res.statusCode = 404; res.end(); return; }
    let raw = '';
    req.on('data', (chunk) => { raw += chunk; });
    req.on('end', () => {
      captured.push(raw);
      const reply = queue.shift();
      if (!reply) {
        res.statusCode = 500;
        res.end('loopback script exhausted');
        return;
      }
      seq += 1;
      const streaming = raw.includes('"stream":true');
      res.setHeader('Content-Type', streaming ? 'text/event-stream' : 'application/json');
      answerRequest(res, reply, streaming, seq);
    });
  });
  return new Promise((resolve, reject) => {
    server.once('error', reject);
    const [host, port] = MODEL_ADDR.split(':');
    server.listen(Number(port), host, () => resolve(server));
  });
}

export async function sha256DownloadedFile(download: Download): Promise<string> {
  const target = await download.path();
  if (!target) throw new Error('download path unavailable');
  const failure = await download.failure();
  if (failure) throw new Error(`download failed: ${failure}`);
  return createHash('sha256').update(fs.readFileSync(target)).digest('hex');
}

export function sha256Text(text: string): string {
  return createHash('sha256').update(text, 'utf8').digest('hex');
}

// runWorkspaceDirs lists the per-run private workspace directories the
// backend allocated, so specs can tamper with delivered bytes physically.
export function runWorkspaceDirs(): string[] {
  if (!fs.existsSync(continuityWorkspaceRoot)) return [];
  return fs.readdirSync(continuityWorkspaceRoot, { withFileTypes: true })
    .filter((entry) => entry.isDirectory())
    .map((entry) => path.join(continuityWorkspaceRoot, entry.name));
}
