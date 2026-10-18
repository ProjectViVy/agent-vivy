#!/usr/bin/env node
// Run the memory-loop suite against the DIVA Recipe it exercises, using the
// SDK's packed-artifact identity check and generated Go overlay. Cross-platform
// port of the former PowerShell gate; no behavior change.
import { randomUUID } from 'node:crypto';
import { mkdirSync, readFileSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { fail, repoRoot, run, runInherit } from './lib/process.mjs';

const prefix = 'memory-loop';
const scratch = join(repoRoot, '.workspace', `memory-loop-ci-${randomUUID().replaceAll('-', '')}`);
const artifact = join(scratch, 'artifact');
const overlay = join(scratch, 'overlay');

function goInherit(args, options = {}) {
  return runInherit('go', args, { cwd: repoRoot, ...options });
}

function requireGo(args, label, options = {}) {
  const result = goInherit(args, options);
  if (result.error) {
    fail(prefix, `go could not start: ${result.error.message}`);
  }
  if (result.status !== 0) {
    fail(prefix, `${label} failed (exit ${result.status})`);
  }
}

try {
  const listed = run('go', ['list', './...'], { cwd: repoRoot });
  if (listed.error) {
    fail(prefix, `go could not start: ${listed.error.message}`);
  }
  if (listed.status !== 0) {
    fail(prefix, `go list ./... failed (exit ${listed.status})`);
  }
  const packages = listed.stdout
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((name) => name && name !== 'agent-vivy/internal/app');
  console.log(`${prefix}: ordinary packages (${packages.length})`);
  requireGo(
    ['test', '-timeout', '35m', '-run', '^TestMemoryLoop', ...packages],
    'ordinary-package memory-loop tests',
  );

  mkdirSync(scratch, { recursive: true });
  console.log(`${prefix}: packing recipes/diva.vivy.yml`);
  requireGo(
    ['run', './sdk', 'pack', '--recipe', 'recipes/diva.vivy.yml', '--output', artifact],
    'SDK pack',
  );
  requireGo(['run', './sdk', 'inspect-artifact', artifact], 'artifact inspection');

  const childEnv = {
    ...process.env,
    VIVY_MEMORY_LOOP_ARTIFACT: artifact,
    VIVY_MEMORY_LOOP_OVERLAY_DIR: overlay,
  };
  const sdkTest = runInherit(
    'go',
    ['test', './sdk/internal', '-run', '^TestMemoryLoop', '-count=1'],
    { cwd: repoRoot, env: childEnv },
  );
  if (sdkTest.error) {
    fail(prefix, `go could not start: ${sdkTest.error.message}`);
  }
  if (sdkTest.status !== 0) {
    fail(prefix, `SDK memory-loop tests failed (exit ${sdkTest.status})`);
  }

  const appDir = join(repoRoot, 'internal/app');
  const appListed = run('go', ['list', '-json', '.'], { cwd: appDir });
  if (appListed.error) {
    fail(prefix, `go could not start: ${appListed.error.message}`);
  }
  if (appListed.status !== 0) {
    fail(prefix, `go list -json internal/app failed (exit ${appListed.status})`);
  }
  const pkg = JSON.parse(appListed.stdout);
  const defaultBodyTests = [
    'default_generation_test.go',
    'headless_mask_arming_test.go',
    'assembly_notebook_test.go',
  ];
  const goFiles = Array.isArray(pkg.GoFiles) ? pkg.GoFiles : [];
  const testFiles = Array.isArray(pkg.TestGoFiles) ? pkg.TestGoFiles : [];
  for (const file of defaultBodyTests) {
    if (!testFiles.includes(file)) {
      continue;
    }
    const source = readFileSync(join(appDir, file), 'utf8');
    if (/^func TestMemoryLoop/m.test(source)) {
      fail(prefix, `default-body exclusion would omit memory-loop tests: ${file}`);
    }
  }
  const tests = testFiles.filter((file) => !defaultBodyTests.includes(file));
  const sources = [...goFiles, ...tests];
  console.log(`${prefix}: app tests against the DIVA overlay (${sources.length} sources)`);
  requireGo(
    [
      'test',
      '-overlay',
      join(overlay, 'overlay.json'),
      '-timeout',
      '35m',
      '-run',
      '^TestMemoryLoop',
      ...sources,
    ],
    'app memory-loop tests',
    { cwd: appDir },
  );
  console.log(`${prefix}: passed`);
} finally {
  rmSync(scratch, { recursive: true, force: true });
}
