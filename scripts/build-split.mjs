#!/usr/bin/env node
// Build the headless backend plus the standalone static UI under dist/.
import { mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { fail, repoRoot, runInherit } from './lib/process.mjs';

const backend = join('dist', process.platform === 'win32' ? 'vivy-backend.exe' : 'vivy-backend');
mkdirSync(join(repoRoot, 'dist'), { recursive: true });

console.log(`build-split: go build -tags vivy_headless -o ${backend} ./cmd/vivy`);
const build = runInherit('go', ['build', '-tags', 'vivy_headless', '-o', backend, './cmd/vivy'], {
  cwd: repoRoot,
});
if (build.error) {
  fail('build-split', `go could not start: ${build.error.message}`);
}
if (build.status !== 0) {
  fail('build-split', `backend build failed (exit ${build.status})`);
}

console.log('build-split: pnpm build --outDir ../dist/vivy-ui');
const ui = runInherit('pnpm', ['--dir', 'ui', 'build', '--', '--outDir', '../dist/vivy-ui', '--emptyOutDir'], {
  cwd: repoRoot,
});
if (ui.error) {
  fail('build-split', `pnpm could not start: ${ui.error.message}`);
}
if (ui.status !== 0) {
  fail('build-split', `UI build failed (exit ${ui.status})`);
}
console.log(`build-split: ${backend} + dist/vivy-ui ready`);
