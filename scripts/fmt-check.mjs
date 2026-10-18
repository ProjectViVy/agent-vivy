#!/usr/bin/env node
// Check tracked and untracked Go files with gofmt without relying on a
// platform shell. Exits 1 and prints every unformatted path.
import { spawnSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { join } from 'node:path';
import { fail, repoRoot } from './lib/process.mjs';

const listed = spawnSync(
  'git',
  ['ls-files', '--cached', '--others', '--exclude-standard', '-z', '--', '*.go'],
  { cwd: repoRoot, encoding: 'buffer' },
);
if (listed.error) {
  fail('fmt-check', `git could not start: ${listed.error.message}`);
}
if (listed.status !== 0) {
  fail('fmt-check', `git ls-files failed (exit ${listed.status})`);
}

const files = listed.stdout
  .toString('utf8')
  .split('\0')
  .filter(Boolean)
  .filter((file) => existsSync(join(repoRoot, file)));

const unformatted = [];
const batchSize = 100;
for (let start = 0; start < files.length; start += batchSize) {
  const batch = files.slice(start, start + batchSize);
  const result = spawnSync('gofmt', ['-l', ...batch], { cwd: repoRoot, encoding: 'utf8' });
  if (result.error) {
    fail('fmt-check', `gofmt could not start: ${result.error.message}`);
  }
  if (result.status !== 0) {
    fail('fmt-check', `gofmt failed (exit ${result.status}): ${(result.stderr || '').trim()}`);
  }
  unformatted.push(
    ...result.stdout
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter(Boolean),
  );
}

if (unformatted.length > 0) {
  console.error('unformatted Go files:');
  for (const file of unformatted) {
    console.error(`  ${file}`);
  }
  process.exit(1);
}
console.log(`fmt-check: ${files.length} Go files formatted`);
