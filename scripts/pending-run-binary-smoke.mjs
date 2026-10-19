#!/usr/bin/env node
// Actual process/checkpoint acceptance. Node >=24; no provider credentials,
// checkpoint conversion, engine-version override, or tenant data access.
import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import http from 'node:http';
import path from 'node:path';
import { DatabaseSync } from 'node:sqlite';

const args = new Map();
for (let i = 2; i < process.argv.length; i += 2) args.set(process.argv[i], process.argv[i + 1]);
if (!args.has('--old') || !args.has('--new') || !args.has('--output')) {
  console.error('usage: node scripts/pending-run-binary-smoke.mjs --old <binary> --new <binary> --output <new-scratch-directory> [--port 18780] [--go <go executable>] [--compat-rollback]');
  process.exit(2);
}
const oldBinary = path.resolve(args.get('--old'));
const newBinary = path.resolve(args.get('--new'));
const compatRollback = args.has('--compat-rollback');
const output = path.resolve(args.get('--output'));
assert(!fs.existsSync(output), `refusing to reuse scratch directory: ${output}`);
fs.mkdirSync(output, { recursive: true });
const port = Number(args.get('--port') || 18780);
const modelPort = port + 1;
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const report = { started_at: new Date().toISOString(), platform: `${process.platform}/${process.arch}`, compat_rollback: compatRollback, binaries: {}, cases: [] };
const children = new Set();
let script = [];
let modelRequests = 0;
const server = http.createServer(async (req, res) => {
  let raw = '';
  for await (const part of req) raw += part;
  modelRequests++;
  const request = JSON.parse(raw);
  assert.equal(request.stream, true, 'real runtime must request a streamed response');
  const reply = script.shift();
  if (!reply) { res.writeHead(500); res.end('test model script exhausted'); return; }
  res.writeHead(200, { 'content-type': 'text/event-stream' });
  const emit = (delta, finish = null) => res.write(`data: ${JSON.stringify({ id: `smoke-${modelRequests}`, object: 'chat.completion.chunk', created: 1, model: request.model, choices: [{ index: 0, delta, finish_reason: finish }] })}\n\n`);
  emit({ role: 'assistant', tool_calls: [{ index: 0, id: `call-${modelRequests}`, type: 'function', function: { name: reply.tool, arguments: '' } }] });
  emit({ tool_calls: [{ index: 0, function: { arguments: JSON.stringify(reply.args) } }] });
  emit({}, 'tool_calls');
  res.end('data: [DONE]\n\n');
});

function rows(dbPath, sql, ...params) {
  const db = new DatabaseSync(dbPath, { readOnly: true, timeout: 5000 });
  try { return db.prepare(sql).all(...params).map((row) => ({ ...row })); } finally { db.close(); }
}
async function eventually(probe, message, timeout = 60000) {
  const deadline = Date.now() + timeout;
  let last;
  while (Date.now() < deadline) {
    try { last = await probe(); if (last) return last; } catch (error) { if (error?.fatal) throw error; last = error; }
    await sleep(50);
  }
  throw new Error(`${message}: ${String(last)}`);
}
function describeBinary(binary) {
  const result = spawnSync(args.get('--go') || 'go', ['version', '-m', binary], { encoding: 'utf8' });
  assert.equal(result.status, 0, result.stderr);
  return { path: binary, sha256: createHash('sha256').update(fs.readFileSync(binary)).digest('hex'), build_info: result.stdout };
}
function prepare(name) {
  const root = path.join(output, name);
  fs.mkdirSync(root);
  fs.mkdirSync(path.join(root, 'workspace'));
  const db = path.join(root, 'vivy.db');
  const config = path.join(root, 'config.yaml');
  fs.writeFileSync(config, `server:\n  addr: "127.0.0.1:${port}"\nstorage:\n  backend: sqlite\n  data_dir: ${JSON.stringify(root)}\n  sqlite:\n    path: ${JSON.stringify(db)}\nproviders:\n  active: deepseek\nruntime:\n  workspace_root: ${JSON.stringify(path.join(root, 'workspaces'))}\n  sandbox:\n    approval:\n      timeout_seconds: 0\n      auto_approve_tools: [submit_plan]\ntools:\n  enabled: [write_note, ask_user, submit_plan]\n  approval:\n    expiration: 1h\n`);
  return { name, root, db, config, workspace: path.join(root, 'workspace'), starts: 0 };
}
async function start(binary, fixture) {
  // A crashed owner retains its ordinary 30-second organism lease. Observe
  // its expiry; never delete leases or mutate the checkpoint/database.
  if (fs.existsSync(fixture.db)) {
    await eventually(() => rows(fixture.db, "SELECT expires_at FROM leases WHERE key='vivy/organism'").every((lease) => Number(lease.expires_at) < Date.now()), 'crashed-owner lease did not expire', 35000);
  }
  fixture.starts++;
  const logPath = path.join(fixture.root, `process-${fixture.starts}.log`);
  const log = fs.openSync(logPath, 'w');
  const child = spawn(binary, [], { cwd: fixture.workspace, windowsHide: true, stdio: ['ignore', log, log], env: { ...process.env, VIVY_USER_HOME: path.join(fixture.root, 'home'), VIVY_CONFIG: fixture.config, VIVY_ADDR: `127.0.0.1:${port}`, VIVY_PROVIDER: 'deepseek', VIVY_API_BASE: `http://127.0.0.1:${modelPort}`, DEEPSEEK_API_KEY: 'isolated-loopback-test-key' } });
  children.add(child);
  child.once('exit', () => { children.delete(child); fs.closeSync(log); });
  await eventually(async () => {
    if (child.exitCode !== null) {
      const error = new Error(fs.readFileSync(logPath, 'utf8'));
      error.fatal = true;
      throw error;
    }
    const response = await fetch(`http://127.0.0.1:${port}/healthz`);
    return response.ok;
  }, `process did not become ready (${logPath})`);
  const bootstrap = await (await fetch(`http://127.0.0.1:${port}/rpc/bootstrap`)).json();
  const socket = new WebSocket(`ws://127.0.0.1:${port}/rpc?token=${encodeURIComponent(bootstrap.token)}`);
  await new Promise((resolve, reject) => { socket.addEventListener('open', resolve, { once: true }); socket.addEventListener('error', reject, { once: true }); });
  let sequence = 0;
  const pending = new Map();
  socket.addEventListener('message', (event) => {
    const message = JSON.parse(String(event.data));
    const entry = pending.get(message.id);
    if (!entry) return;
    pending.delete(message.id);
    clearTimeout(entry.timer);
    if (message.error) entry.reject(new Error(JSON.stringify(message.error))); else entry.resolve(message.result);
  });
  const rpc = (method, params = {}) => new Promise((resolve, reject) => {
    const id = String(++sequence);
    const timer = setTimeout(() => { pending.delete(id); reject(new Error(`${method} timed out`)); }, 15000);
    pending.set(id, { resolve, reject, timer });
    socket.send(JSON.stringify({ jsonrpc: '2.0', id, method, params }));
  });
  await rpc('initialize', { protocol_version: 'vivy.rpc.v1' });
  return { child, socket, rpc };
}
async function kill(process) {
  process.socket.close();
  await new Promise((resolve) => {
    if (process.child.exitCode !== null) { resolve(); return; }
    process.child.once('exit', resolve);
    process.child.kill('SIGKILL'); // Windows TerminateProcess; intentionally no App.Close cleanup.
  });
}
function snapshot(fixture, pauses) {
  return pauses.map((pause) => ({ ...pause,
    checkpoints: rows(fixture.db, 'SELECT id,generation,checksum FROM checkpoints WHERE id=?', `ckpt-${pause.run_id}`),
    cleanup_intents: rows(fixture.db, "SELECT id FROM checkpoints WHERE id LIKE 'runtime:checkpoint-cleanup:v1:%'"),
    run: rows(fixture.db, 'SELECT id,status FROM runs WHERE id=?', pause.run_id)[0],
    approvals: rows(fixture.db, 'SELECT id,decision FROM approvals WHERE run_id=?', pause.run_id),
    questions: rows(fixture.db, 'SELECT id,status FROM questions WHERE run_id=?', pause.run_id),
    events: rows(fixture.db, 'SELECT seq,type,CAST(payload AS TEXT) AS payload FROM run_events WHERE run_id=? ORDER BY seq', pause.run_id),
    work_events: rows(fixture.db, 'SELECT work_seq,kind,CAST(payload AS TEXT) AS payload FROM session_work_events WHERE session_id=? ORDER BY work_seq', pause.session_id),
  }));
}
async function seed(process, fixture) {
  const pauses = [];
  for (const kind of ['approval', 'question', 'plan']) {
    const session = await process.rpc('session/create', { title: `binary smoke ${kind}`, workspace_path: fixture.workspace });
    if (!pauses.length) {
      await process.rpc('module.action.invoke', { module_id: 'vivy/diva-cognitive', action_id: 'diva.cognitive.persona.initialize', input: { session_id: session.id, initialization: { identity: 'Binary smoke assistant', relationship: 'test partner', redline: 'isolated test only', user: 'test owner', world: 'scratch workspace' } } });
    }
    await process.rpc('session/set_permission', { session_id: session.id, preset: kind === 'approval' ? 'smart' : 'trusted' });
    if (kind === 'plan') await process.rpc('plan/enter', { session_id: session.id, request_id: `enter-${fixture.name}`, expected_version: 0 });
    script.push(kind === 'approval' ? { tool: 'write_note', args: { content: 'isolated pending approval' } } : kind === 'question' ? { tool: 'ask_user', args: { question: 'Isolated pending question?' } } : { tool: 'submit_plan', args: { markdown: '1. Review the isolated pending plan.' } });
    const turn = await process.rpc('turn/start', { session_id: session.id, text: `Create the ${kind} pause for the local test.` });
    const pause = { kind, session_id: session.id, run_id: turn.run_id };
    await eventually(async () => {
      if (kind === 'plan') {
        const work = await process.rpc('session/work/get', { session_id: session.id });
        return work.plan?.review_status === 'pending' && work.version >= 3;
      }
      const log = await process.rpc('run/log', { run_id: pause.run_id });
      return log.events.some((event) => event.type === (kind === 'approval' ? 'tool.approval_required' : 'user.question_required'));
    }, `${kind} did not suspend`);
    assert.equal((await process.rpc('run/get', { run_id: pause.run_id })).status, 'active');
    await sleep(100);
    assert.equal(rows(fixture.db, 'SELECT id FROM checkpoints WHERE id=?', `ckpt-${pause.run_id}`).length, 1);
    pauses.push(pause);
  }
  return pauses;
}
function assertSettled(state, terminal) {
  for (const pause of state) {
    assert.equal(pause.cleanup_intents.length, 0, 'durable cleanup intent was not retired');
    assert.equal(pause.run.status, terminal);
    assert.equal(pause.events.filter((event) => ['run.failed', 'run.cancelled', 'run.completed'].includes(event.type)).length, 1);
    if (pause.kind === 'plan') assert.equal(pause.work_events.filter((event) => event.kind === 'plan.review_cancelled').length, 1);
    else {
      const type = pause.kind === 'approval' ? 'tool.approval_cancelled' : 'user.question_cancelled';
      assert.equal(pause.kind === 'approval' ? pause.approvals[0].decision : pause.questions[0].status, 'cancelled');
      const cancelled = pause.events.filter((event) => event.type === type);
      assert.equal(cancelled.length, 1);
      assert(cancelled[0].seq < pause.events.find((event) => event.type === `run.${terminal}`).seq);
    }
  }
}
try {
  report.binaries.old = describeBinary(oldBinary);
  report.binaries.candidate = describeBinary(newBinary);
  assert.match(report.binaries.old.build_info, /github.com\/cloudwego\/eino\s+v0\.9\.13\b/);
  assert.match(report.binaries.candidate.build_info, /github.com\/cloudwego\/eino\s+v0\.9\.21\b/);
  await new Promise((resolve) => server.listen(modelPort, '127.0.0.1', resolve));
  for (const mode of ['upgrade', 'unsupported-direct-rollback', 'clean-before-rollback']) {
    console.log(`binary smoke: ${mode}`);
    const fixture = prepare(mode);
    let process = await start(mode === 'upgrade' ? oldBinary : newBinary, fixture);
    const pauses = await seed(process, fixture);
    const before = snapshot(fixture, pauses);
    const requestsAtPause = modelRequests;
    let preRollback = before;
    if (mode === 'clean-before-rollback') {
      for (const pause of pauses) await process.rpc('run/cancel', { run_id: pause.run_id });
      await eventually(async () => (await Promise.all(pauses.map((pause) => process.rpc('run/get', { run_id: pause.run_id })))).every((run) => run.status === 'cancelled'), 'same-version cleanup did not settle');
      const cleaned = snapshot(fixture, pauses);
      preRollback = cleaned;
      assertSettled(cleaned, 'cancelled');
      await kill(process);
      process = await start(newBinary, fixture);
      assert.deepEqual(snapshot(fixture, pauses), cleaned, 'same-version restart changed pre-rollback cleanup');
      assert.equal(modelRequests, requestsAtPause, 'pre-rollback cleanup or restart called the model');
      assert.equal(rows(fixture.db, "SELECT id FROM runs WHERE status IN ('active','accepted')").length, 0, 'active runs remain before historical rollback');
      assert.equal(rows(fixture.db, "SELECT id FROM approvals WHERE decision='pending'").length, 0);
      assert.equal(rows(fixture.db, "SELECT id FROM questions WHERE status='pending'").length, 0);
    }
    await kill(process);
    try {
      process = await start(mode === 'upgrade' ? newBinary : oldBinary, fixture);
    } catch (error) {
      if (mode !== 'upgrade' && !compatRollback) {
        assert.match(String(error), /database records unknown migration version 44/);
        const after = snapshot(fixture, pauses);
        assert.deepEqual(after, preRollback, 'historical binary changed request or checkpoint state before failing closed');
        assert.equal(modelRequests, requestsAtPause, 'rejected rollback called the model');
        report.cases.push({ mode, before: preRollback, after, startup_error: String(error), result: 'EXPECTED_FAIL_CLOSED' });
        fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify(report, null, 2));
        continue;
      }
      throw error;
    }
    const after = snapshot(fixture, pauses);
    assert.equal(modelRequests, requestsAtPause, 'first recovery called the model');
    for (let i = 0; i < pauses.length; i++) assert.deepEqual(after[i].checkpoints, before[i].checkpoints, 'checkpoint bytes/pointer changed across versions');
    if (mode === 'upgrade') {
      assertSettled(after, 'failed');
      for (const pause of after) {
        const failure = JSON.parse(pause.events.find((event) => event.type === 'run.failed').payload);
        assert.equal(failure.cause_category, 'internal_error');
        assert.match(failure.message, /different engine version/);
      }
    }
    else if (mode === 'clean-before-rollback') assertSettled(after, 'cancelled');
    else if (compatRollback) {
      assertSettled(after, 'failed');
      for (const pause of after) {
        const failure = JSON.parse(pause.events.find((event) => event.type === 'run.failed').payload);
        assert.equal(failure.cause_category, 'internal_error');
        assert.match(failure.message, /different engine version/);
      }
    }
    else throw new Error(`unexpected rollback mode reached recovery assertions: ${mode}`);
    const requestsAfterRecovery = modelRequests;
    const views = [];
    for (const pause of pauses) views.push({ run: await process.rpc('run/get', { run_id: pause.run_id }), work: await process.rpc('session/work/get', { session_id: pause.session_id }), reviews: await process.rpc('review/list', { session_id: pause.session_id }) });
    assert.equal(views[2].work.plan.review_status, 'cancelled');
    assert(views.every((view) => view.reviews.reviews.every((review) => review.status !== 'pending')));
    await kill(process);
    process = await start(mode === 'upgrade' ? newBinary : oldBinary, fixture);
    const repeated = snapshot(fixture, pauses);
    assert.deepEqual(repeated, after, 'restart duplicated or lost durable events');
    assert.equal(modelRequests, requestsAfterRecovery, 'recovery called the model');
    await kill(process);
    report.cases.push({ mode, before, after, repeated, views, result: 'PASS' });
    fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify(report, null, 2));
  }
  report.result = 'PASS';
} catch (error) {
  report.result = 'FAIL'; report.error = error.stack;
  console.error(error);
  process.exitCode = 1;
} finally {
  for (const child of children) child.kill('SIGKILL');
  server.closeAllConnections();
  server.close();
  report.finished_at = new Date().toISOString();
  report.model_requests = modelRequests;
  fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify(report, null, 2));
  console.log(`binary smoke ${report.result}: ${path.join(output, 'report.json')}`);
}
