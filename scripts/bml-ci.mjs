#!/usr/bin/env node
// Root ./... excludes BML, which declares its own Go module.
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { repoRoot, runInherit } from './lib/process.mjs';

export function runBmlCI(go = 'go', run = runInherit) {
  for (const args of [
    ['vet', '-p=1', './...'],
    ['test', '-p=1', '-count=1', '-timeout=5m', './...'],
  ]) {
    const result = run(go, args, { cwd: join(repoRoot, 'bml') });
    if (result.error) {
      throw new Error(`bml-ci: go ${args[0]} could not start`, { cause: result.error });
    }
    if (result.signal) {
      throw new Error(`bml-ci: go ${args[0]} terminated by ${result.signal}`);
    }
    if (result.status !== 0) {
      return result.status ?? 1;
    }
  }
  return 0;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    process.exitCode = runBmlCI(process.argv[2]);
  } catch (error) {
    console.error(error);
    process.exitCode = 1;
  }
}
