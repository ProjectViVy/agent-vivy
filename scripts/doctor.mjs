#!/usr/bin/env node
// Diagnose the development environment. Read-only by design: it reports
// problems and the next command, but never changes go env, global proxy
// settings, ports, or another project's files.
import { spawnSync } from 'node:child_process';
import { existsSync, readFileSync, unlinkSync, writeFileSync } from 'node:fs';
import { createServer } from 'node:net';
import { homedir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { run as runProcess } from './lib/process.mjs';

const prefix = 'doctor';
const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const results = [];

function report(status, area, message) {
  results.push({ status, area, message });
  const label = status === 'ok' ? 'OK  ' : status === 'warn' ? 'WARN' : status === 'fail' ? 'FAIL' : 'INFO';
  console.log(`[${label}] ${area}: ${message}`);
}

function section(title) {
  console.log(`\n${title}`);
}

function spawn(command, args, options = {}) {
  return spawnSync(command, args, {
    cwd: repoRoot,
    encoding: 'utf8',
    timeout: 5000,
    ...options,
  });
}

function parseVersion(text) {
  const match = String(text || '').match(/(\d+)\.(\d+)(?:\.(\d+))?/);
  return match ? [Number(match[1]), Number(match[2]), Number(match[3] || 0)] : null;
}

function compareVersions(left, right) {
  for (let i = 0; i < 3; i += 1) {
    if ((left[i] || 0) !== (right[i] || 0)) {
      return (left[i] || 0) - (right[i] || 0);
    }
  }
  return 0;
}

function normalizeRemote(url) {
  return String(url || '')
    .trim()
    .replace(/\/+$/, '')
    .replace(/^git@github\.com:/, 'https://github.com/')
    .replace(/\.git$/, '');
}

function git(args, cwd = repoRoot) {
  return spawn('git', args, { cwd });
}

function readJSON(path) {
  return JSON.parse(readFileSync(path, 'utf8'));
}

function findGo() {
  let result = spawn('go', ['version']);
  let command = 'go';
  if (result.error && process.platform === 'win32') {
    const fallback = 'C:\\Program Files\\Go\\bin\\go.exe';
    if (existsSync(fallback)) {
      result = spawn(fallback, ['version']);
      command = fallback;
    }
  }
  return { command, result };
}

function portState(port) {
  return new Promise((resolvePort) => {
    const server = createServer();
    server.once('error', (error) => resolvePort({ free: false, error: error.code || error.message }));
    server.once('listening', () => {
      server.close(() => resolvePort({ free: true }));
    });
    server.listen({ host: '127.0.0.1', port });
  });
}

function canWrite(dir) {
  if (!existsSync(dir)) {
    return { ok: false, reason: 'missing' };
  }
  const file = join(dir, `.${prefix}-${process.pid}-${Date.now()}.tmp`);
  try {
    writeFileSync(file, 'ok', 'utf8');
    unlinkSync(file);
    return { ok: true };
  } catch (error) {
    return { ok: false, reason: error.message };
  }
}

function parseGoMod() {
  const path = join(repoRoot, 'go.mod');
  if (!existsSync(path)) {
    report('fail', 'go.mod', `${path} is missing`);
    return null;
  }
  const text = readFileSync(path, 'utf8');
  const goVersion = text.match(/^go\s+(\S+)\s*$/m)?.[1] || '';
  const replaces = [];
  for (const line of text.split(/\r?\n/)) {
    const match = line.match(/^replace\s+(\S+)(?:\s+v\S+)?\s+=>\s+(\S+)/);
    if (match) {
      replaces.push({ module: match[1], target: match[2] });
    }
  }
  return { text, goVersion, replaces };
}

function checkLaputa(goVersion, lock) {
  const checkout = resolve(dirname(repoRoot), 'laputa');
  if (!existsSync(checkout)) {
    report('fail', 'laputa', `missing ${checkout}; run 'just setup'`);
    return;
  }
  if (!existsSync(join(checkout, '.git'))) {
    report('fail', 'laputa', `${checkout} exists without a Git checkout; move it aside`);
    return;
  }
  const head = git(['rev-parse', 'HEAD'], checkout);
  const origin = git(['remote', 'get-url', 'origin'], checkout);
  const changes = git(['status', '--porcelain'], checkout);
  if (head.error || origin.error || changes.error) {
    report('fail', 'laputa', 'git could not inspect the checkout');
    return;
  }
  const actualHead = head.stdout.trim();
  const actualOrigin = normalizeRemote(origin.stdout);
  if (actualOrigin !== normalizeRemote(lock.repository)) {
    report('fail', 'laputa', `origin is ${actualOrigin}; expected ${lock.repository}`);
    return;
  }
  if (changes.stdout.trim()) {
    report('fail', 'laputa', `local changes at ${actualHead.slice(0, 12)}; the pinned commit is ${lock.commit.slice(0, 12)}`);
  } else if (actualHead !== lock.commit) {
    report('fail', 'laputa', `HEAD is ${actualHead.slice(0, 12)}; run 'just setup' to advance to ${lock.commit.slice(0, 12)}`);
  } else {
    report('ok', 'laputa', `${actualHead.slice(0, 12)} clean at ${checkout}`);
  }
  for (const name of ['garden', 'mentle', 'laputa']) {
    const modfile = join(checkout, name, 'go.mod');
    if (!existsSync(modfile)) {
      report('fail', 'laputa', `module ${name} is missing at the pinned checkout`);
      continue;
    }
    const text = readFileSync(modfile, 'utf8');
    if (!new RegExp(`^module\\s+github\\.com/ProjectViVy/laputa/${name}\\s*$`, 'm').test(text)) {
      report('fail', 'laputa', `${name}/go.mod has a different module identity`);
      continue;
    }
    const declared = text.match(/^go\s+(\S+)\s*$/m)?.[1];
    if (declared !== goVersion) {
      report('fail', 'laputa', `${name}/go.mod declares Go ${declared}; host go.mod requires ${goVersion}`);
    }
  }
  const flatWorktree = repoRoot.replaceAll('\\', '/').match(/^(.*\/\.worktrees)\/[^/]+$/);
  if (flatWorktree) {
    report(
      'warn',
      'lane',
      'flat .worktrees/<lane>/ layout puts this checkout beside a shared .worktrees/laputa; ' +
        'create new lanes with just lane-new <slug> (nested lane/laputa)',
    );
  }
}

function checkProxy(goCommand) {
  const projectProxy =
    (process.env.VIVY_GOPROXY || process.env.GOPROXY || 'https://goproxy.cn,direct').trim();
  report('info', 'proxy', `project default GOPROXY=${projectProxy} (process only; no go env -w)`);
  const global = goCommand ? spawn(goCommand, ['env', 'GOPROXY']) : null;
  if (global && !global.error && global.status === 0) {
    const globalProxy = global.stdout.trim();
    if (globalProxy && globalProxy !== projectProxy) {
      report(
        'warn',
        'proxy',
        `go env GOPROXY is ${globalProxy}; just recipes use ${projectProxy}. ` +
          'Set VIVY_GOPROXY to keep an intentional override.',
      );
    }
  }
  const first = projectProxy
    .split(',')
    .map((entry) => entry.trim())
    .find((entry) => entry && entry !== 'direct' && entry !== 'off');
  if (!first) {
    report('info', 'proxy', 'no HTTP proxy in GOPROXY; direct downloads only');
    return;
  }
  let url;
  try {
    url = new URL(first);
  } catch {
    report('warn', 'proxy', `could not parse proxy entry ${first}`);
    return;
  }
  return checkProxyReachability(url);
}

async function checkProxyReachability(url) {
  try {
    const response = await fetch(url, { signal: AbortSignal.timeout(4000) });
    if (response.status >= 500) {
      report(
        'warn',
        'proxy',
        `${url.hostname} answered ${response.status}. Go's comma fallback handles only 404/410, ` +
          'not 5xx; if setup fails, retry with VIVY_GOPROXY=direct.',
      );
    } else {
      report('ok', 'proxy', `${url.hostname} reachable (HTTP ${response.status})`);
    }
  } catch (error) {
    report(
      'warn',
      'proxy',
      `${url.hostname} unreachable: ${error.message}. setup retries once with GOPROXY=direct.`,
    );
  }
}

async function main() {
  console.log(`${prefix}: repository ${repoRoot}`);
  console.log(`${prefix}: platform ${process.platform}/${process.arch}, node ${process.versions.node}`);

  const isWsl = process.platform === 'linux' && existsSync('/proc/version') &&
    /microsoft/i.test(readFileSync('/proc/version', 'utf8'));
  if (isWsl) {
    report('info', 'platform', 'WSL detected');
    if (repoRoot.replaceAll('\\', '/').startsWith('/mnt/')) {
      report(
        'warn',
        'platform',
        'the repository is on the Windows filesystem (/mnt/...); move the Linux dev checkout under ~/work ' +
          'to avoid cross-filesystem I/O and file-watch overhead',
      );
    }
  }

  section('tools');
  const git = spawn('git', ['--version']);
  if (git.error) {
    report('fail', 'git', 'not on PATH');
  } else {
    report('ok', 'git', git.stdout.trim());
  }

  const go = findGo();
  if (go.result.error) {
    report('fail', 'go', 'not on PATH; install Go 1.26+ and add it to PATH');
  } else {
    report('ok', 'go', go.result.stdout.trim());
    if (go.command !== 'go') {
      report('warn', 'go', `found at ${go.command} but not on PATH`);
    }
  }

  const mod = parseGoMod();
  if (mod && go.result.error === undefined) {
    const installed = parseVersion(go.result.stdout);
    const required = parseVersion(mod.goVersion);
    if (!installed || !required) {
      report('warn', 'go', `could not compare installed Go with go.mod ${mod.goVersion}`);
    } else if (compareVersions(installed, required) < 0) {
      report('fail', 'go', `installed Go is older than go.mod requires (${mod.goVersion})`);
    } else {
      report('ok', 'go', `satisfies go.mod ${mod.goVersion}`);
    }
  }

  const nodeRequired = [22, 0, 0];
  const nodeInstalled = parseVersion(process.versions.node);
  if (!nodeInstalled || compareVersions(nodeInstalled, nodeRequired) < 0) {
    report('fail', 'node', `installed ${process.versions.node}; Go/UI tooling requires Node 22+`);
  } else {
    report('ok', 'node', process.versions.node);
  }

  const pnpm = runProcess('pnpm', ['--version'], { timeout: 5000 });
  let pnpmVersion = null;
  if (pnpm.error) {
    report('warn', 'pnpm', 'not on PATH; install pnpm 10.33.0 for the UI lane');
  } else {
    pnpmVersion = pnpm.stdout.trim();
    report('ok', 'pnpm', pnpmVersion);
    const packageFile = join(repoRoot, 'ui/package.json');
    if (existsSync(packageFile)) {
      const declared = readJSON(packageFile).packageManager;
      const expected = String(declared || '').replace(/^pnpm@/, '');
      if (expected && pnpmVersion !== expected) {
        report(
          'warn',
          'pnpm',
          `installed ${pnpmVersion}; ui/package.json pins ${declared}. Run 'corepack prepare ${declared} --activate' ` +
            'or use corepack pnpm.',
        );
      } else if (expected) {
        report('ok', 'pnpm', `matches ui/package.json ${declared}`);
      }
    }
  }

  const just = spawn('just', ['--version']);
  if (just.error) {
    report('warn', 'just', 'not on PATH; recipes can be run manually');
  } else {
    report('ok', 'just', just.stdout.trim());
  }

  const docker = spawn('docker', ['version', '--format', '{{.Server.Version}}']);
  if (docker.error) {
    report('warn', 'docker', 'CLI not on PATH; Docker packaging/acceptance is unavailable');
  } else if (docker.status !== 0) {
    report('warn', 'docker', 'daemon unavailable; start Docker before packaging acceptance');
  } else {
    report('ok', 'docker', `daemon ${docker.stdout.trim()}`);
  }

  section('dependency pins');
  const lockPath = join(repoRoot, 'laputa-source.lock.json');
  if (!existsSync(lockPath)) {
    report('fail', 'laputa', 'laputa-source.lock.json is missing');
  } else {
    let lock;
    try {
      lock = readJSON(lockPath);
    } catch (error) {
      report('fail', 'laputa', `cannot parse laputa-source.lock.json: ${error.message}`);
    }
    if (lock) {
      if (!lock.repository || !/^[0-9a-f]{40}$/.test(lock.commit || '')) {
        report('fail', 'laputa', 'lock must declare a repository and a full 40-character commit');
      } else {
        report('ok', 'laputa', `pin ${lock.commit.slice(0, 12)}`);
        if (mod) {
          const revision = lock.commit.slice(0, 12);
          const mismatched = ['garden', 'mentle', 'laputa'].filter(
            (name) =>
              !new RegExp(
                `^\\s*github\\.com/ProjectViVy/laputa/${name}\\s+v0\\.0\\.0-\\d{14}-${revision}(?:\\s|$)`,
                'm',
              ).test(mod.text),
          );
          if (mismatched.length) {
            report('fail', 'laputa', `go.mod revision does not match the lock for ${mismatched.join(', ')}`);
          } else {
            report('ok', 'laputa', 'go.mod revisions match the lock');
          }
        }
        checkLaputa(mod?.goVersion || '', lock);
      }
    }
  }

  if (mod) {
    const missing = [];
    for (const replace of mod.replaces) {
      if (!replace.target.startsWith('./') && !replace.target.startsWith('../')) {
        continue;
      }
      if (!existsSync(resolve(repoRoot, replace.target))) {
        missing.push(`${replace.module} -> ${replace.target}`);
      }
    }
    if (missing.length) {
      report('fail', 'replace', `local replace targets missing: ${missing.join(', ')}`);
    } else {
      report('ok', 'replace', `${mod.replaces.length} local replace targets present`);
    }
  }

  section('runtime');
  const devConfig = join(repoRoot, 'scripts/dev.config.yaml');
  if (existsSync(devConfig)) {
    report('ok', 'config', 'scripts/dev.config.yaml present; dev entries load it instead of the daily config.yaml');
  } else {
    report('fail', 'config', 'scripts/dev.config.yaml is missing; dev entries would fall back to the repo config.yaml');
  }
  const savedConfig = (process.env.VIVY_CONFIG || '').trim();
  if (savedConfig) {
    report('warn', 'config', `VIVY_CONFIG=${savedConfig}; dev scripts ignore it unless VIVY_DEV_CONFIG is set`);
  }
  if (existsSync(join(repoRoot, 'config.yaml'))) {
    report('info', 'config', 'daily config.yaml exists (gitignored); direct vivy.exe runs read it, dev entries do not');
  }
  const dailyHome = join(homedir(), '.vivy');
  report('info', 'home', `daily Vivy root ${dailyHome}${existsSync(dailyHome) ? ' (present)' : ' (not created yet)'}`);
  if (process.env.VIVY_USER_HOME) {
    report('info', 'home', `VIVY_USER_HOME is set to ${process.env.VIVY_USER_HOME}; dev scripts override it`);
  }

  let defaults = { backendPort: 8788, uiPort: 3016, userHome: '.workspace/dev/home' };
  const defaultsPath = join(repoRoot, 'scripts/dev.defaults.json');
  if (!existsSync(defaultsPath)) {
    report('warn', 'dev', 'scripts/dev.defaults.json is missing; using built-in port defaults');
  } else {
    try {
      defaults = { ...defaults, ...readJSON(defaultsPath) };
      report('ok', 'dev', `backend ${defaults.backendPort}, UI ${defaults.uiPort}, home ${defaults.userHome}`);
    } catch (error) {
      report('fail', 'dev', `cannot parse scripts/dev.defaults.json: ${error.message}`);
    }
  }
  const devHome = resolve(repoRoot, defaults.userHome);
  report('info', 'home', `dev root ${devHome}${existsSync(devHome) ? ' (present)' : ' (will be created by just dev)'}`);

  section('ports');
  for (const [label, port] of [
    ['dev backend', Number(defaults.backendPort)],
    ['dev UI', Number(defaults.uiPort)],
    ['daily/embedded', 8787],
    ['default Vite', 3015],
  ]) {
    const state = await portState(port);
    if (state.free) {
      report('ok', 'port', `${label} 127.0.0.1:${port} free`);
    } else if (label.startsWith('dev')) {
      report('warn', 'port', `${label} 127.0.0.1:${port} is in use; a dev instance may already be running`);
    } else {
      report('info', 'port', `${label} 127.0.0.1:${port} is in use (dev uses ${defaults.backendPort}/${defaults.uiPort})`);
    }
  }

  section('write access');
  const rootWrite = canWrite(repoRoot);
  if (rootWrite.ok) {
    report('ok', 'write', `${repoRoot} writable`);
  } else {
    report('fail', 'write', `${repoRoot} is not writable: ${rootWrite.reason}`);
  }
  const workspace = join(repoRoot, '.workspace');
  if (!existsSync(workspace)) {
    report('info', 'write', '.workspace is missing; just dev creates it');
  } else {
    const state = canWrite(workspace);
    report(state.ok ? 'ok' : 'fail', 'write', state.ok ? '.workspace writable' : `.workspace is not writable: ${state.reason}`);
  }
  const dist = join(repoRoot, 'dist');
  if (existsSync(dist)) {
    const state = canWrite(dist);
    report(state.ok ? 'ok' : 'fail', 'write', state.ok ? 'dist writable' : `dist is not writable: ${state.reason}`);
  }

  section('ui');
  const uiModules = join(repoRoot, 'ui/node_modules');
  if (existsSync(uiModules)) {
    report('ok', 'ui', 'ui/node_modules present');
  } else {
    report('warn', 'ui', 'ui/node_modules missing; run pnpm --dir ui install --frozen-lockfile');
  }
  const uiDist = join(repoRoot, 'ui/dist/index.html');
  if (existsSync(uiDist)) {
    report('ok', 'ui', 'ui/dist present for go:embed builds');
  } else {
    report('warn', 'ui', 'ui/dist missing; Vite development works, but embedded builds need pnpm --dir ui build');
  }
  const lockfile = join(repoRoot, 'ui/pnpm-lock.yaml');
  if (existsSync(lockfile)) {
    const lockVersion = readFileSync(lockfile, 'utf8').match(/^lockfileVersion:\s*'?([^'\r\n]+)'?/m)?.[1];
    report('info', 'ui', `pnpm lockfileVersion ${lockVersion || 'unknown'}`);
  }

  return checkProxy(go.result.error ? '' : go.command);
}

await main();
const finalFailures = results.filter((result) => result.status === 'fail');
const finalWarnings = results.filter((result) => result.status === 'warn');
console.log(`\n${prefix}: ${results.length - finalFailures.length - finalWarnings.length} ok, ${finalWarnings.length} warnings, ${finalFailures.length} failures`);
if (finalFailures.length > 0) {
  console.error(`${prefix}: fix the failures above, then run 'just doctor' again`);
  process.exit(1);
}
if (finalWarnings.length > 0) {
  console.log(`${prefix}: warnings are non-fatal; review them before starting a full gate`);
}
