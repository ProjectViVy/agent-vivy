#!/usr/bin/env node
// Exercise opt-in native recall through the official SDK-generated DIVA recipe.
// Every selected test must execute and pass, so a skip can never be reported
// as success.
import { randomUUID } from 'node:crypto';
import { mkdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import { fail, repoRoot, runInherit } from './lib/process.mjs';

const prefix = 'diva-recall';
const scratch = join(tmpdir(), `vivy-recall-${randomUUID().replaceAll('-', '')}`);
const generated = join(scratch, 'zz_diva.go');
const overlay = join(scratch, 'overlay.json');
const results = join(scratch, 'results.jsonl');
const tests = [
  'TestMemoryLoopMemoryInjection',
  'TestMemoryLoopRecallAfterProcessRestart',
  'TestMemoryLoopRecallNegativeControls',
  'TestMemoryLoopOrdinaryRecallAuthorityBoundary',
  'TestMemoryLoopRecallDeadlineDegradesSafely',
  'TestMemoryLoopCorrectionAndDeletionInModelInput',
];

try {
  mkdirSync(scratch, { recursive: true });
  const generate = runInherit(
    'go',
    [
      'run',
      './sdk/internal/cmd/generate-default',
      '--repo',
      repoRoot,
      '--recipe',
      'recipes/diva.vivy.yml',
      '--output',
      generated,
    ],
    { cwd: repoRoot },
  );
  if (generate.error) {
    fail(prefix, `go could not start: ${generate.error.message}`);
  }
  if (generate.status !== 0) {
    fail(prefix, `DIVA assembly generation failed (exit ${generate.status})`);
  }

  writeFileSync(
    overlay,
    JSON.stringify({
      Replace: {
        [join(repoRoot, 'internal/generated/assembly/zz_default.go')]: generated,
      },
    }),
    'utf8',
  );

  const selection = `^(${tests.join('|')})$`;
  const result = spawnSync(
    'go',
    [
      'test',
      '-timeout',
      '10m',
      '-overlay',
      overlay,
      '-tags',
      'vivy_diva_integration',
      '-json',
      './internal/app',
      '-run',
      selection,
      '-count=1',
    ],
    { cwd: repoRoot, encoding: 'utf8', maxBuffer: 128 * 1024 * 1024 },
  );
  writeFileSync(results, `${result.stdout || ''}${result.stderr || ''}`, 'utf8');
  if (result.error) {
    fail(prefix, `go could not start: ${result.error.message}`);
  }
  if (result.status !== 0) {
    console.error(`${result.stdout || ''}${result.stderr || ''}`);
    fail(prefix, `DIVA recall tests failed (exit ${result.status}). Raw output: ${results}`);
  }

  const events = [];
  for (const line of (result.stdout || '').split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed.startsWith('{')) {
      continue;
    }
    try {
      events.push(JSON.parse(trimmed));
    } catch {
      // Non-JSON build noise is preserved in results.jsonl.
    }
  }
  if (events.some((event) => event.Action === 'skip')) {
    fail(prefix, 'DIVA recall gate must execute every selected test without skips');
  }
  for (const test of tests) {
    const passed = events.filter((event) => event.Test === test && event.Action === 'pass').length;
    if (passed !== 1) {
      fail(prefix, `DIVA recall gate did not pass required test ${test}`);
    }
    console.log(`PASS ${test} (SDK-generated DIVA assembly)`);
  }
} finally {
  rmSync(scratch, { recursive: true, force: true });
}
