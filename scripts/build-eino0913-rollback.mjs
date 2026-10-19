#!/usr/bin/env node
// Build the compatibility rollback binary from this source with the historical
// Eino pin. The scratch module files stay under .workspace; product go.mod is
// never rewritten.
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const args = new Map();
for (let i = 2; i < process.argv.length; i += 2) args.set(process.argv[i], process.argv[i + 1]);
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const output = path.resolve(args.get('--output') || path.join(root, '.workspace', 'pr80-binary', 'compat-rollback', 'vivy-eino0913.exe'));
const buildRoot = `${output}.build`;
assert(!fs.existsSync(output), `refusing to overwrite compatibility binary: ${output}`);
assert(!fs.existsSync(buildRoot), `refusing to reuse compatibility build scratch: ${buildRoot}`);

function run(command, commandArgs) {
  const result = spawnSync(command, commandArgs, { cwd: root, encoding: 'utf8', windowsHide: true });
  if (result.status !== 0) throw new Error(`${command} ${commandArgs.join(' ')} failed\n${result.stdout}${result.stderr}`);
  return result.stdout;
}

const go = args.get('--go') || 'go';
fs.mkdirSync(path.dirname(output), { recursive: true });
fs.mkdirSync(buildRoot, { recursive: true });
const claudeDir = path.join(buildRoot, 'eino-claude');
fs.cpSync(path.join(root, 'third_party', 'eino-claude'), claudeDir, { recursive: true });

const claudeMod = path.join(claudeDir, 'compat.mod');
fs.copyFileSync(path.join(claudeDir, 'go.mod'), claudeMod);
run(go, ['mod', 'edit', `-modfile=${claudeMod}`, '-require=github.com/cloudwego/eino@v0.9.13']);
fs.copyFileSync(claudeMod, path.join(claudeDir, 'go.mod'));

const modFile = path.join(buildRoot, 'compat.mod');
fs.copyFileSync(path.join(root, 'go.mod'), modFile);
fs.copyFileSync(path.join(root, 'go.sum'), path.join(buildRoot, 'compat.sum'));
const claudeRelative = `./${path.relative(root, claudeDir).split(path.sep).join('/')}`;
run(go, [
  'mod', 'edit', `-modfile=${modFile}`,
  '-require=github.com/cloudwego/eino@v0.9.13',
  `-replace=github.com/cloudwego/eino-ext/components/model/claude=${claudeRelative}`,
]);

const selected = run(go, ['list', `-modfile=${modFile}`, '-m', 'github.com/cloudwego/eino']).trim();
assert.equal(selected, 'github.com/cloudwego/eino v0.9.13', `unexpected compatibility Eino selection: ${selected}`);
run(go, ['build', '-mod=mod', `-modfile=${modFile}`, '-o', output, './cmd/vivy']);
const buildInfo = run(go, ['version', '-m', output]);
assert.match(buildInfo, /dep\s+github\.com\/cloudwego\/eino\s+v0\.9\.13\b/);
assert.doesNotMatch(buildInfo, /dep\s+github\.com\/cloudwego\/eino\s+v0\.9\.21\b/);
process.stdout.write(`built compatibility rollback binary: ${output}\n`);
process.stdout.write(buildInfo.split('\n').filter((line) => line.includes('github.com/cloudwego/eino') || line.includes('vcs.')).join('\n') + '\n');
