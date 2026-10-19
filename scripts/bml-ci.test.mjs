import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';
import { runBmlCI } from './bml-ci.mjs';
import { repoRoot } from './lib/process.mjs';

const success = { status: 0, signal: null };

test('BML vets before testing, in its independent module with bounded test options', () => {
  const calls = [];
  const status = runBmlCI('custom-go', (command, args, options) => {
    calls.push({ command, args, options });
    return success;
  });
  assert.equal(status, 0);
  assert.deepEqual(calls, [
    { command: 'custom-go', args: ['vet', '-p=1', './...'], options: { cwd: join(repoRoot, 'bml') } },
    { command: 'custom-go', args: ['test', '-p=1', '-count=1', '-timeout=5m', './...'], options: { cwd: join(repoRoot, 'bml') } },
  ]);
});

test('a vet failure returns its exit code without starting tests', () => {
  const commands = [];
  const status = runBmlCI('go', (_command, args) => {
    commands.push(args[0]);
    return { status: 23, signal: null };
  });
  assert.equal(status, 23);
  assert.deepEqual(commands, ['vet']);
});

test('a test failure returns its exit code', () => {
  const commands = [];
  const status = runBmlCI('go', (_command, args) => {
    commands.push(args[0]);
    return { status: args[0] === 'vet' ? 0 : 29, signal: null };
  });
  assert.equal(status, 29);
  assert.deepEqual(commands, ['vet', 'test']);
});

for (const stage of ['vet', 'test']) {
  test(`a ${stage} spawn failure preserves the cause and stops the gate`, () => {
    const cause = new Error('spawn go ENOENT');
    const commands = [];
    assert.throws(() => runBmlCI('go', (_command, args) => {
      commands.push(args[0]);
      return args[0] === stage ? { error: cause, status: null, signal: null } : success;
    }), error => {
      assert.match(error.message, new RegExp(`go ${stage} could not start`));
      assert.equal(error.cause, cause);
      return true;
    });
    assert.deepEqual(commands, stage === 'vet' ? ['vet'] : ['vet', 'test']);
  });

  test(`a ${stage} signal is a diagnosed failure and stops the gate`, () => {
    const commands = [];
    assert.throws(() => runBmlCI('go', (_command, args) => {
      commands.push(args[0]);
      return args[0] === stage ? { status: null, signal: 'SIGTERM' } : success;
    }), new RegExp(`go ${stage} terminated by SIGTERM`));
    assert.deepEqual(commands, stage === 'vet' ? ['vet'] : ['vet', 'test']);
  });
}

test('the CLI reports a real spawn failure with its original error', () => {
  const result = spawnSync(process.execPath, [
    fileURLToPath(new URL('./bml-ci.mjs', import.meta.url)),
    join(repoRoot, 'scripts', 'missing-bml-go-executable'),
  ], { cwd: tmpdir(), encoding: 'utf8' });
  assert.equal(result.status, 1, result.stdout + result.stderr);
  assert.match(result.stderr, /go vet could not start/);
  assert.match(result.stderr, /ENOENT/);
});

test('subprocess stdout and stderr are inherited and a failing test remains nonzero', () => {
  const runner = new URL('./bml-ci.mjs', import.meta.url).href;
  const helper = new URL('./lib/process.mjs', import.meta.url).href;
  const result = spawnSync(process.execPath, ['--input-type=module', '-e', `
    import { runBmlCI } from ${JSON.stringify(runner)};
    import { runInherit } from ${JSON.stringify(helper)};
    process.exitCode = runBmlCI('go', (_command, args, options) =>
      runInherit(process.execPath, ['-e',
        'console.log(' + JSON.stringify(args[0] + ' stdout') + ');' +
        'console.error(' + JSON.stringify(args[0] + ' stderr') + ');' +
        'process.exit(' + (args[0] === 'vet' ? 0 : 31) + ');'
      ], options));
  `], { cwd: tmpdir(), encoding: 'utf8' });
  assert.equal(result.status, 31, result.stdout + result.stderr);
  assert.match(result.stdout, /vet stdout\r?\ntest stdout/);
  assert.match(result.stderr, /vet stderr\r?\ntest stderr/);
});
