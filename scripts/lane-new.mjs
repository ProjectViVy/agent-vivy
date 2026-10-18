#!/usr/bin/env node
// Create an isolated parallel lane:
//   .worktrees/<slug>/agent-vivy  (git worktree on feat/<slug>)
//   .worktrees/<slug>/laputa      (its own pinned checkout)
// A lane never shares the root tree or its sibling Laputa, so two branches
// cannot flip each other's pinned revision.
import { spawnSync } from 'node:child_process';
import { existsSync, mkdirSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const prefix = 'lane-new';

function fail(message) {
  console.error(`${prefix}: ${message}`);
  process.exit(1);
}

function git(args, options = {}) {
  const result = spawnSync('git', args, { cwd: repoRoot, encoding: 'utf8', ...options });
  if (result.error) {
    fail(`could not run git: ${result.error.message}`);
  }
  return result;
}

function usage() {
  return `usage: node scripts/lane-new.mjs <slug> [--base <ref>]

Creates .worktrees/<slug>/agent-vivy on branch feat/<slug> and a matching
.worktrees/<slug>/laputa at the commit pinned by laputa-source.lock.json.`;
}

function main() {
  const args = process.argv.slice(2);
  let slug = '';
  let base = 'HEAD';
  for (let i = 0; i < args.length; i += 1) {
    const arg = args[i];
    if (arg === '--base') {
      base = args[i + 1] ?? '';
      i += 1;
      if (!base) fail('--base needs a ref');
    } else if (arg === '--help' || arg === '-h') {
      console.log(usage());
      process.exit(0);
    } else if (arg.startsWith('-')) {
      fail(`unknown argument: ${arg}`);
    } else if (!slug) {
      slug = arg;
    } else {
      fail(`unexpected argument: ${arg}`);
    }
  }
  if (!slug) fail(usage().split('\n')[0]);
  if (!/^[a-z0-9][a-z0-9-]*$/.test(slug)) {
    fail('slug must use lowercase letters, digits, and hyphens (for example: diva-recall)');
  }

  const laneRoot = join(repoRoot, '.worktrees', slug);
  const checkout = join(laneRoot, 'agent-vivy');
  const branch = `feat/${slug}`;
  if (existsSync(laneRoot)) {
    fail(`${laneRoot} already exists; choose another slug or remove the lane explicitly`);
  }
  const baseCheck = git(['rev-parse', '--verify', `${base}^{commit}`]);
  if (baseCheck.status !== 0) {
    fail(`base ref does not resolve to a commit: ${base}`);
  }
  const branchCheck = git(['show-ref', '--verify', '--quiet', `refs/heads/${branch}`]);
  if (branchCheck.status === 0) {
    fail(`branch ${branch} already exists; choose another slug`);
  }

  mkdirSync(laneRoot, { recursive: true });
  const added = git(['worktree', 'add', checkout, '-b', branch, base]);
  if (added.status !== 0) {
    fail(`git worktree add failed: ${(added.stderr || added.stdout || '').trim()}`);
  }

  const laputa = spawnSync(
    process.execPath,
    [join(checkout, 'scripts/ensure-laputa.mjs'), '--repo-root', checkout, '--quiet'],
    { cwd: checkout, encoding: 'utf8', stdio: 'inherit' },
  );
  if (laputa.error) {
    fail(`could not start the Laputa bootstrap: ${laputa.error.message}`);
  }
  if (laputa.status !== 0) {
    fail(`${checkout} was created, but its Laputa checkout failed; fix the error and run just ensure-laputa there`);
  }

  console.log('');
  console.log(`${prefix}: lane ${slug}`);
  console.log(`  checkout  ${checkout}`);
  console.log(`  laputa    ${join(laneRoot, 'laputa')}`);
  console.log(`  branch    ${branch} from ${base}`);
  console.log('');
  console.log('next:');
  console.log(`  cd ${checkout}`);
  console.log('  just doctor');
  console.log('  just dev');
  console.log('  # a second concurrent lane needs different ports:');
  console.log('  #   WSL:        VIVY_DEV_BACKEND_PORT=8789 VIVY_DEV_UI_PORT=3017 just dev');
  console.log('  #   PowerShell: $env:VIVY_DEV_BACKEND_PORT=8789; $env:VIVY_DEV_UI_PORT=3017; just dev');
}

main();
