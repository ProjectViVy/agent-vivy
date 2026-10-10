import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const script = resolve(dirname(fileURLToPath(import.meta.url)), 'ensure-laputa.ps1');
const powershell = process.env.PWSH ?? (process.platform === 'win32' ? 'powershell.exe' : 'pwsh');

function git(dir, ...args) {
  const result = spawnSync('git', ['-C', dir, ...args], { encoding: 'utf8' });
  assert.equal(result.status, 0, result.stderr);
  return result.stdout.trim();
}

function fixture(t) {
  const dir = mkdtempSync(join(tmpdir(), 'vivy-laputa-bootstrap-'));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  const source = join(dir, 'source').replaceAll('\\', '/');
  const root = join(dir, 'vivy');
  mkdirSync(source);
  mkdirSync(root);
  git(source, 'init', '-q');
  for (const name of ['garden', 'mentle', 'laputa']) {
    mkdirSync(join(source, name));
    writeFileSync(join(source, name, 'go.mod'), `module github.com/ProjectViVy/laputa/${name}\n\ngo 1.26.4\n`);
  }
  git(source, 'add', '.');
  git(source, '-c', 'user.name=fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '-qm', 'modules');
  const commit = git(source, 'rev-parse', 'HEAD');
  writeFileSync(join(root, 'laputa-source.lock.json'), JSON.stringify({ repository: source, commit }));
  return { dir, source, root, commit, checkout: join(dir, 'laputa') };
}

function ensure(f) {
  return spawnSync(powershell, ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', script, '-RepoRoot', f.root, '-Quiet'], { encoding: 'utf8' });
}

test('the direct-script entry locates its repository without RepoRoot or a matching cwd', t => {
  const f = fixture(t);
  mkdirSync(join(f.root, 'scripts'));
  const entry = join(f.root, 'scripts', 'ensure-laputa.ps1');
  writeFileSync(entry, readFileSync(script));
  const result = spawnSync(powershell, ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', entry, '-Quiet'], {
    cwd: f.source, encoding: 'utf8',
  });
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.equal(git(f.checkout, 'rev-parse', 'HEAD'), f.commit);
});

test('a fresh checkout installs all three modules at the pinned commit and is repeatable', t => {
  const f = fixture(t);
  let result = ensure(f);
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.equal(git(f.checkout, 'rev-parse', 'HEAD'), f.commit);
  for (const name of ['garden', 'mentle', 'laputa']) {
    assert.match(readFileSync(join(f.checkout, name, 'go.mod'), 'utf8'), new RegExp(`^module github.com/ProjectViVy/laputa/${name}\\r?\\n`));
  }
  result = ensure(f);
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.equal(git(f.checkout, 'status', '--porcelain'), '');
});

test('a clean old checkout advances to the pin', t => {
  const f = fixture(t);
  assert.equal(ensure(f).status, 0);
  writeFileSync(join(f.source, 'new.txt'), 'new revision');
  git(f.source, 'add', '.');
  git(f.source, '-c', 'user.name=fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '-qm', 'next');
  const commit = git(f.source, 'rev-parse', 'HEAD');
  writeFileSync(join(f.root, 'laputa-source.lock.json'), JSON.stringify({ repository: f.source, commit }));
  const result = ensure(f);
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.equal(git(f.checkout, 'rev-parse', 'HEAD'), commit);
});

test('an old checkout with local changes is preserved and fails with an actionable error', t => {
  const f = fixture(t);
  assert.equal(ensure(f).status, 0);
  writeFileSync(join(f.checkout, 'garden', 'go.mod'), 'local work');
  writeFileSync(join(f.root, 'laputa-source.lock.json'), JSON.stringify({ repository: f.source, commit: 'a'.repeat(40) }));
  const result = ensure(f);
  assert.notEqual(result.status, 0);
  assert.match(result.stdout + result.stderr, /local changes/i);
  assert.equal(readFileSync(join(f.checkout, 'garden', 'go.mod'), 'utf8'), 'local work');
  assert.equal(git(f.checkout, 'rev-parse', 'HEAD'), f.commit);
});

test('an unrelated repository in the sibling path is rejected without changing it', t => {
  const f = fixture(t);
  assert.equal(ensure(f).status, 0);
  git(f.checkout, 'remote', 'set-url', 'origin', 'https://example.invalid/unrelated');
  const result = ensure(f);
  assert.notEqual(result.status, 0);
  assert.match(result.stdout + result.stderr, /origin/i);
  assert.equal(git(f.checkout, 'rev-parse', 'HEAD'), f.commit);
});

test('an invalid module identity at the pinned commit fails before Go starts', t => {
  const f = fixture(t);
  assert.equal(ensure(f).status, 0);
  writeFileSync(join(f.checkout, 'garden', 'go.mod'), 'module github.com/elsewhere/garden\n');
  const result = ensure(f);
  assert.notEqual(result.status, 0);
  assert.match(result.stdout + result.stderr, /module/i);
});
