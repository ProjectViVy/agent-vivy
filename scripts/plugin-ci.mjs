#!/usr/bin/env node
// Vet and test every independent Go module under plugins/ and faces/.
import { existsSync, readdirSync } from 'node:fs';
import { join, relative } from 'node:path';
import { repoRoot, runInherit } from './lib/process.mjs';

function findModules(root, rel, found) {
  const dir = join(root, rel);
  if (!existsSync(dir)) {
    return;
  }
  if (existsSync(join(dir, 'go.mod'))) {
    found.push(dir);
  }
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (!entry.isDirectory()) {
      continue;
    }
    if (entry.name === 'node_modules' || entry.name === '.git' || entry.name === 'dist') {
      continue;
    }
    findModules(root, join(rel, entry.name), found);
  }
}

const modules = [];
for (const root of ['plugins', 'faces']) {
  findModules(repoRoot, root, modules);
}
modules.sort();

let failed = false;
for (const module of modules) {
  console.log(`== plugin-ci: ${relative(repoRoot, module) || module}`);
  const vet = runInherit('go', ['vet', './...'], { cwd: module });
  if (vet.error) {
    console.error(`plugin-ci: go could not start: ${vet.error.message}`);
    failed = true;
    continue;
  }
  if (vet.status !== 0) {
    failed = true;
  }
  const test = runInherit('go', ['test', './...'], { cwd: module });
  if (test.error) {
    console.error(`plugin-ci: go could not start: ${test.error.message}`);
    failed = true;
    continue;
  }
  if (test.status !== 0) {
    failed = true;
  }
}

if (failed) {
  process.exit(1);
}
console.log(`plugin-ci: ${modules.length} modules passed`);
