import assert from 'node:assert/strict';
import test from 'node:test';
import { executedTests } from './verify-kernel.mjs';
import { buildGates } from '../.agents/skills/vivy-advance-dev-env/scripts/advance-dev-env.mjs';

test('a successful go command with no selected tests cannot accept a gate', () => {
  assert.throws(() => executedTests('{"Action":"pass","Package":"example"}\n'), /no tests/);
});

test('a skipped required case cannot borrow another passing case', () => {
  const events = [{ Action: 'skip', Test: 'TestRequired' }, { Action: 'pass', Test: 'TestOther' }];
  assert.throws(() => executedTests(events.map(JSON.stringify).join('\n'), ['TestRequired']), /TestRequired/);
});

test('failure remains failure even when a sibling passed', () => {
  const events = [{ Action: 'fail', Test: 'TestBroken' }, { Action: 'pass', Test: 'TestOther' }];
  assert.throws(() => executedTests(events.map(JSON.stringify).join('\n')), /TestBroken/);
});

test('actual executed cases and skips are reported separately', () => {
  const events = [{ Action: 'pass', Test: 'TestRequired' }, { Action: 'skip', Test: 'TestExternal' }];
  assert.deepEqual(executedTests(events.map(JSON.stringify).join('\n'), ['TestRequired']), {
    passed: ['TestRequired'], skipped: ['TestExternal'],
  });
});

test('scoped Linux readiness does not claim the legacy Windows full gate', () => {
  const gates = buildGates({ os: 'wsl', docker: { daemon: true }, cc: { present: true }, postgres: {} }, {
    found: true, root: '/home/test/agent-vivy', just: true, pnpm: false, scopedKernel: true,
    doctor: { summary: { failures: 0 } },
  });
  assert.equal(gates.find(g => g.id === 'minimal-kernel').ready, true);
  assert.equal(gates.find(g => g.id === 'full-ci').ready, false);
  assert.match(gates.find(g => g.id === 'full-ci').requires, /Windows/);
});

test('Windows readiness routes all scoped verification to Linux', () => {
  const gates = buildGates({ os: 'windows', docker: {}, cc: { present: true }, postgres: { dsnSet: true, reachable: true } }, {
    found: true, root: 'C:/work/agent-vivy', just: true, pnpm: true, scopedKernel: true,
    doctor: { summary: { failures: 0 } },
  });
  assert.equal(gates.find(g => g.id === 'minimal-kernel').ready, false);
  assert.equal(gates.find(g => g.id === 'minimal-kernel').external, true);
  assert.equal(gates.find(g => g.id === 'race').ready, false);
  assert.equal(gates.find(g => g.id === 'fast').ready, false);
  assert.equal(gates.find(g => g.id === 'postgres').ready, false);
});

test('WSL mounted Windows paths cannot advertise scoped readiness', () => {
  const gates = buildGates({ os: 'wsl', docker: {}, cc: { present: true }, postgres: { dsnSet: true, reachable: true } }, {
    found: true, root: '/mnt/c/work/agent-vivy', just: true, pnpm: true, scopedKernel: true,
    doctor: { summary: { failures: 0 } },
  });
  for (const id of ['fast', 'minimal-kernel', 'race', 'postgres']) {
    assert.equal(gates.find(g => g.id === id).ready, false, id);
    assert.equal(gates.find(g => g.id === id).external, true, id);
  }
});
