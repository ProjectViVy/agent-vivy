#!/usr/bin/env node
// Prepare the pinned ProjectViVy/laputa source closure required by Go builds.
// One implementation for Windows, WSL, and CI; no PowerShell dependency.
import { spawnSync } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const MODULES = ['garden', 'mentle', 'laputa'];

function usage() {
  return `usage: node scripts/ensure-laputa.mjs [--repo-root <path>] [--quiet]

Clones or advances the sibling ../laputa checkout to the commit pinned by
laputa-source.lock.json, then verifies the three module identities and the
Go toolchain version. Existing local changes are never overwritten.`;
}

function parseArgs(argv) {
  let repoRoot = '';
  let quiet = false;
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '--repo-root' || arg === '-RepoRoot') {
      repoRoot = argv[i + 1] ?? '';
      i += 1;
    } else if (arg === '--quiet' || arg === '-Quiet') {
      quiet = true;
    } else if (arg === '--help' || arg === '-h') {
      console.log(usage());
      process.exit(0);
    } else {
      fail(`unknown argument: ${arg}`);
    }
  }
  return { repoRoot, quiet };
}

function fail(message) {
  console.error(`ensure-laputa: ${message}`);
  process.exit(1);
}

function git(args, options = {}) {
  const result = spawnSync('git', args, { encoding: 'utf8', ...options });
  if (result.error) {
    if (result.error.code === 'ENOENT') {
      fail('git is not on PATH; install Git and retry');
    }
    fail(`could not run git: ${result.error.message}`);
  }
  if (result.status !== 0) {
    const detail = `${result.stderr || ''}${result.stdout || ''}`.trim();
    fail(
      `git ${args.join(' ')} failed (exit ${result.status})` +
        `${detail ? `: ${detail}` : ''}. Check access to the pinned Laputa repository, then retry.`,
    );
  }
  return result.stdout.trim();
}

function normalizeRemote(url) {
  return url
    .trim()
    .replace(/\/+$/, '')
    .replace(/^git@github\.com:/, 'https://github.com/')
    .replace(/\.git$/, '');
}

function readLock(root) {
  const lockPath = join(root, 'laputa-source.lock.json');
  let lock;
  try {
    lock = JSON.parse(readFileSync(lockPath, 'utf8'));
  } catch (error) {
    fail(`cannot read ${lockPath}: ${error.message}`);
  }
  if (!lock.repository || !/^[0-9a-f]{40}$/.test(lock.commit ?? '')) {
    fail('laputa-source.lock.json must declare a repository and a full commit SHA');
  }
  return lock;
}

function readHostToolchain(root, lock) {
  const goModPath = join(root, 'go.mod');
  const hostMod = readFileSync(goModPath, 'utf8');
  const revision = lock.commit.slice(0, 12);
  for (const name of MODULES) {
    const pattern = new RegExp(
      `^\\s*github\\.com/ProjectViVy/laputa/${name}\\s+v0\\.0\\.0-\\d{14}-${revision}(?:\\s|$)`,
      'm',
    );
    if (!pattern.test(hostMod)) {
      fail(
        `Laputa ${name} Go revision in go.mod must match laputa-source.lock.json at ${revision}`,
      );
    }
  }
  const goVersion = hostMod.match(/^go\s+(\S+)\s*$/m);
  if (!goVersion) {
    fail('go.mod must declare its Go version');
  }
  return goVersion[1];
}

function ensureCheckout(checkout, lock) {
  if (!existsSync(checkout)) {
    console.log(`installing Laputa from ${lock.repository} at ${lock.commit}`);
    git(['clone', '--no-checkout', lock.repository, checkout]);
    git(['-C', checkout, 'checkout', '--detach', lock.commit]);
    return;
  }
  if (!existsSync(join(checkout, '.git'))) {
    fail(
      `Laputa path already exists without a Git checkout: ${checkout}. Move it aside, then retry.`,
    );
  }
  const origin = normalizeRemote(git(['-C', checkout, 'remote', 'get-url', 'origin']));
  const expectedOrigin = normalizeRemote(lock.repository);
  if (origin !== expectedOrigin) {
    fail(
      `Laputa origin is ${origin}; expected ${expectedOrigin}. Move the unrelated checkout aside, then retry.`,
    );
  }
  const head = git(['-C', checkout, 'rev-parse', 'HEAD']);
  const changes = git(['-C', checkout, 'status', '--porcelain']);
  if (changes) {
    fail(
      `Laputa module source has local changes at ${head}; required commit is ${lock.commit}. ` +
        'Commit or stash local changes, then retry.',
    );
  }
  if (head !== lock.commit) {
    git(['-C', checkout, 'fetch', 'origin', lock.commit]);
    git(['-C', checkout, 'checkout', '--detach', lock.commit]);
  }
}

function verifyModules(checkout, goVersion) {
  for (const name of MODULES) {
    const modfile = join(checkout, name, 'go.mod');
    if (!existsSync(modfile)) {
      fail(`Laputa module is missing: ${modfile}. Check the pinned checkout before retrying.`);
    }
    const module = readFileSync(modfile, 'utf8');
    const identity = new RegExp(`^module\\s+github\\.com/ProjectViVy/laputa/${name}\\s*$`, 'm');
    if (!identity.test(module)) {
      fail(
        `Laputa module identity in ${modfile} must be github.com/ProjectViVy/laputa/${name}. ` +
          'Preserve local changes and restore the pinned module declaration.',
      );
    }
    const declared = module.match(/^go\s+(\S+)\s*$/m);
    if (!declared || declared[1] !== goVersion) {
      fail(`Laputa Go version in ${modfile} must match host Go version ${goVersion}`);
    }
  }
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  const scriptRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
  const root = resolve(args.repoRoot || scriptRoot);
  const lock = readLock(root);
  const goVersion = readHostToolchain(root, lock);
  const checkout = resolve(dirname(root), 'laputa');
  ensureCheckout(checkout, lock);
  verifyModules(checkout, goVersion);
  if (!args.quiet) {
    console.log(`Laputa source ready at ${lock.commit}: ${checkout}`);
  }
}

main();
