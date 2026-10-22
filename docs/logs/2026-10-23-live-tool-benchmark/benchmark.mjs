import { mkdir, writeFile, readFile } from 'node:fs/promises';
import path from 'node:path';
import { randomBytes } from 'node:crypto';
import { performance } from 'node:perf_hooks';

// External RPC observer only. The server owns model/tool execution and policy.
// No provider settings, credentials, bootstrap tokens or raw events are saved.
const base = process.env.BENCH_URL || 'http://127.0.0.1:8787';
const output = path.resolve(process.env.BENCH_OUTPUT || '.workspace/live-tool-benchmark');
const preflight = process.argv.includes('--preflight');
const rounds = Number(process.env.BENCH_ROUNDS || 2);
const timeoutMS = 180_000;
const startedAt = new Date().toISOString();
const terminal = new Set(['run.completed', 'run.failed', 'run.cancelled']);
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));

class RPC {
  pending = new Map();
  listeners = new Set();
  next = 0;
  async connect() {
    const boot = await (await fetch(`${base}/rpc/bootstrap`)).json();
    const url = new URL(boot.websocket_path, base);
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
    url.searchParams.set('token', boot.token);
    this.socket = new WebSocket(url);
    this.socket.addEventListener('message', message => {
      const item = JSON.parse(String(message.data));
      if (item.id !== undefined) {
        const pending = this.pending.get(item.id);
        if (!pending) return;
        this.pending.delete(item.id);
        clearTimeout(pending.timer);
        if (item.error) pending.reject(new Error(`RPC ${pending.method}: code ${item.error.code}`));
        else pending.resolve(item.result);
      } else {
        for (const listener of this.listeners) listener(item);
      }
    });
    await new Promise((resolve, reject) => {
      this.socket.addEventListener('open', resolve, { once: true });
      this.socket.addEventListener('error', () => reject(new Error('WebSocket connection failed')), { once: true });
    });
    await this.call('initialize', { protocol_version: boot.protocol_version });
  }
  call(method, params = {}) {
    const id = ++this.next;
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error(`RPC ${method}: timeout`));
      }, 20_000);
      this.pending.set(id, { resolve, reject, timer, method });
      this.socket.send(JSON.stringify({ jsonrpc: '2.0', id, method, params }));
    });
  }
  close() { this.socket?.close(); }
}

async function put(root, name, content) {
  const target = path.join(root, name);
  await mkdir(path.dirname(target), { recursive: true });
  await writeFile(target, content);
}

async function tasks(root) {
  const nonce = () => randomBytes(9).toString('hex');
  const a = nonce(), b = nonce(), dispatch = nonce(), leaf = nonce();
  await put(root, 'note.txt', `acceptance_code=${a}\n`);
  await put(root, 'next.txt', `next_code=${b}\n`);
  await put(root, 'README.md', 'The active dispatch manifest is records/manifest.json.\n');
  await put(root, 'records/manifest.json', JSON.stringify({ active: `dispatch/${dispatch}.txt` }));
  await put(root, `dispatch/${dispatch}.txt`, `shipment=Orion\nreceipt=${leaf}\n`);
  await put(root, 'orders.csv', 'id,status,quantity,unit_price\nA,paid,3,17\nB,pending,9,100\nC,paid,5,23\nD,paid,2,11\nE,cancelled,1,999\n');
  await put(root, 'src/last.mjs', 'export function last(items) {\n  return items[items.length];\n}\n');
  await put(root, 'tests/last.json', JSON.stringify({ input: ['red', 'green', 'blue'], expected: 'blue' }));
  await put(root, 'settings/current.txt', `release=R-${nonce()}\n`);
  const release = (await readFile(path.join(root, 'settings/current.txt'), 'utf8')).trim().slice(8);
  return [
    { id: 'read_nonce', text: 'Read note.txt using a file tool. Return only JSON {"code":"the exact acceptance_code from the file"}.', expected: { code: a }, reads: ['note.txt'] },
    { id: 'follow_manifest', text: 'Read README.md, follow the manifest and then its active dispatch file. Return only JSON with shipment and receipt. Use file tools and do not guess.', expected: { shipment: 'Orion', receipt: leaf }, reads: ['README.md', 'records/manifest.json', `dispatch/${dispatch}.txt`] },
    { id: 'aggregate_csv', text: 'Read orders.csv with a file tool. Sum quantity * unit_price for status paid only, and count the paid rows. Return only JSON {"paid_rows":number,"total":number}.', expected: { paid_rows: 3, total: 188 }, reads: ['orders.csv'] },
    { id: 'inspect_bug', text: 'Read src/last.mjs and tests/last.json. Do not edit files. Identify the faulty line and corrected return expression. Return only JSON {"line":number,"expression":"..."}.', expected: { line: 2, expression: 'items[items.length - 1]' }, reads: ['src/last.mjs', 'tests/last.json'] },
    { id: 'missing_read_recovery', text: 'First attempt to read settings/archived.txt with a file tool. If it is missing, read settings/current.txt instead. Return only JSON {"release":"the release value"}. Do not create files.', expected: { release }, reads: ['settings/current.txt'], missing: 'settings/archived.txt' },
    { id: 'context_followup', text: 'Keep the acceptance_code you read earlier in this conversation. Read next.txt now using a file tool. Return only JSON {"previous":"earlier acceptance_code","next":"next_code"}.', expected: { previous: a, next: b }, reads: ['next.txt'], followup: true },
  ];
}

function objectAnswer(text) {
  try { return JSON.parse(text.trim()); } catch { /* tolerate Markdown fences */ }
  const start = text.indexOf('{'), end = text.lastIndexOf('}');
  if (start < 0 || end < start) return null;
  try { return JSON.parse(text.slice(start, end + 1)); } catch { return null; }
}

function matches(actual, expected) {
  return actual && Object.entries(expected).every(([key, value]) => {
    if (key === 'expression') return String(actual[key] || '').replace(/\s/g, '').replace(/^return/, '').replace(/;$/, '') === value.replace(/\s/g, '');
    return actual[key] === value;
  });
}

async function runTask(rpc, task, round, sessionID) {
  const started = performance.now();
  const seen = new Map();
  let runID, firstDeltaMS = null, terminalMS = null, subscriptionID;
  const observe = item => {
    if (item.method !== 'run/event') return;
    const event = item.params?.event;
    if (event?.run_id !== runID) return;
    seen.set(event.seq, event);
    if (event.type === 'model.delta' && event.payload?.delta && firstDeltaMS === null) firstDeltaMS = performance.now() - started;
    if (terminal.has(event.type) && terminalMS === null) terminalMS = performance.now() - started;
  };
  rpc.listeners.add(observe);
  let timedOut = false;
  try {
    const admitted = await rpc.call('turn/start', { session_id: sessionID, text: task.text });
    runID = admitted.run_id;
    subscriptionID = (await rpc.call('run/subscribe', { run_id: runID, after_seq: 0 })).subscription_id;
    while (terminalMS === null && performance.now() - started < timeoutMS) await pause(100);
    if (terminalMS === null) {
      timedOut = true;
      await rpc.call('run/cancel', { run_id: runID });
      await pause(500);
    }
    const replay = await rpc.call('run/log', { run_id: runID, after_seq: 0 });
    for (const event of replay.events || []) seen.set(event.seq, event);
    const events = [...seen.values()].sort((a, b) => a.seq - b.seq);
    const messages = await rpc.call('session/messages', { session_id: sessionID, include_attachment_data: false });
    const answer = (messages.messages || []).filter(message => message.run_id === runID && message.role === 'assistant').map(message => message.content).join('\n');
    const requested = events.filter(event => event.type === 'tool.requested').map(event => event.payload);
    const finished = events.filter(event => event.type === 'tool.finished').map(event => event.payload);
    const modelRequests = events.filter(event => event.type === 'model.request');
    const models = events.filter(event => event.type === 'model.call.finished').map(event => event.payload);
    const ends = events.filter(event => terminal.has(event.type));
    const successful = new Set(finished.filter(item => !item.error).map(item => item.tool_call_id));
    const normalized = value => String(value || '').replaceAll('\\', '/');
    const readOK = name => requested.some(item => item.tool_name === 'read_file' && normalized(item.args?.path).endsWith(name) && successful.has(item.tool_call_id));
    const missingAttempted = !task.missing || requested.some(item => item.tool_name === 'read_file' && normalized(item.args?.path).endsWith(task.missing));
    const answerOK = Boolean(matches(objectAnswer(answer), task.expected));
    const evidenceOK = task.reads.every(readOK) && missingAttempted;
    const status = ends.at(-1)?.type.replace('run.', '') || 'unknown';
    const usages = models.map(item => item.usage).filter(Boolean);
    const usage = usages.length === models.length && models.length > 0 ? Object.fromEntries(['prompt_tokens', 'completion_tokens', 'total_tokens'].map(key => [key, usages.reduce((sum, item) => sum + item[key], 0)])) : null;
    return {
      round, task: task.id, session_id: sessionID, run_id: runID, status, timed_out: timedOut,
      passed: !timedOut && status === 'completed' && answerOK && evidenceOK && ends.length === 1,
      answer_correct: answerOK, tool_evidence_correct: evidenceOK,
      elapsed_ms: Math.round(terminalMS ?? performance.now() - started), first_text_delta_ms: firstDeltaMS === null ? null : Math.round(firstDeltaMS),
      model_requests: modelRequests.length, model_closures: models.length,
      model_statuses: models.map(item => item.status), routes: [...new Set(models.map(item => `${item.provider}/${item.model}`))],
      tools_requested: requested.length, tools_finished: finished.length,
      tool_names: requested.map(item => item.tool_name), tool_errors: finished.filter(item => item.error).length,
      usage, usage_reported_calls: usages.length, terminal_events: ends.length,
      provider_retries: events.filter(item => item.type === 'provider.retry').length,
      approval_requests: events.filter(item => item.type === 'tool.approval_required').length,
      failure_types: events.filter(item => item.type === 'run.failed').map(item => Object.keys(item.payload || {})),
      // Only synthetic task answers and expected values; no raw event payloads.
      answer: answer.slice(0, 1500), expected: task.expected,
    };
  } finally {
    rpc.listeners.delete(observe);
    if (subscriptionID) await rpc.call('run/unsubscribe', { subscription_id: subscriptionID }).catch(() => {});
  }
}

const rpc = new RPC();
try {
  await rpc.connect();
  const catalog = await rpc.call('tools/list');
  console.log(JSON.stringify({ phase: 'preflight', tools: catalog.tools.map(tool => ({ name: tool.name, active: tool.active, readonly: tool.readonly, exposure: tool.exposure })) }));
  if (!preflight) {
    await mkdir(output, { recursive: true });
    const results = [];
    for (let round = 1; round <= rounds; round++) {
      const root = path.join(output, `round-${round}`);
      const cases = await tasks(root);
      let initialSession;
      for (const task of cases) {
        const sessionID = task.followup ? initialSession : (await rpc.call('session/create', { title: `Benchmark ${round}: ${task.id}`, workspace_path: root })).id;
        if (task.id === 'read_nonce') initialSession = sessionID;
        console.log(JSON.stringify({ phase: 'start', round, task: task.id, session_id: sessionID }));
        const result = await runTask(rpc, task, round, sessionID);
        results.push(result);
        await writeFile(path.join(output, 'results.json'), JSON.stringify({ started_at: startedAt, updated_at: new Date().toISOString(), endpoint: base, rounds, timeout_ms: timeoutMS, results }, null, 2));
        console.log(JSON.stringify({ phase: 'result', ...result, answer: undefined, expected: undefined }));
      }
    }
    console.log(JSON.stringify({ phase: 'complete', passed: results.filter(result => result.passed).length, total: results.length, result_path: path.join(output, 'results.json') }));
  }
} catch (error) {
  console.error(String(error.message).replace(/token=[^&\s]+/g, 'token=[redacted]'));
  process.exitCode = 1;
} finally { rpc.close(); }
