#!/usr/bin/env node
// Reproducible Toolkit browser gate.
//
// One repository entry prepares a packed body with the SDK, inspects that
// artifact, then runs the lifecycle and accessibility Playwright suites on
// their own backend, Vite server and data root. The gate fails closed on a
// missing pin, invalid/occupied ports, a stale artifact or a non-zero suite
// exit; the specs themselves stay unchanged.
import { spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, rmSync, statSync } from 'node:fs';
import { createServer } from 'node:net';
import { join, resolve } from 'node:path';
import { fail, repoRoot, run, runInherit, requireSuccess } from './lib/process.mjs';
import { resolveGateEndpoints } from '../ui/e2e/e2e-ports.mjs';

const prefix = 'toolkits-e2e';
const GATE_ROOT = join(repoRoot, '.workspace', 'toolkits-e2e');
const OUTPUT = join(GATE_ROOT, 'default');
const BINARY_NAME = process.platform === 'win32' ? 'vivy.exe' : 'vivy';
const SUITES = [
  { label: 'lifecycle', config: 'playwright.toolkits.config.ts' },
  { label: 'accessibility', config: 'playwright.toolkits-accessibility.config.ts' },
];

if (process.env.VIVY_TOOLKITS_E2E_EXTERNAL_BACKEND === '1') {
  fail(
    prefix,
    'VIVY_TOOLKITS_E2E_EXTERNAL_BACKEND is a manual Playwright mode, not the reproducible gate; ' +
      'unset it and run `just toolkits-e2e` so the gate owns its backend.',
  );
}

const endpoints = resolveEndpoints();
const goEnv = withLegacyHTTP2({ ...process.env });
await assertFree('backend', endpoints.backendPort);
await assertFree('UI', endpoints.uiPort);

console.log(`${prefix}: backend ${endpoints.backend}, UI ${endpoints.ui}, data root is created by the config`);
console.log(`${prefix}: artifact root ${OUTPUT}`);
if (goEnv.GOFLAGS) console.log(`${prefix}: GOFLAGS ${goEnv.GOFLAGS}`);

const override = process.env.VIVY_TOOLKITS_E2E_BINARY?.trim();
const artifactDir = process.env.VIVY_TOOLKITS_E2E_ARTIFACT_DIR?.trim();
if (override && !artifactDir) {
  fail(
    prefix,
    'VIVY_TOOLKITS_E2E_BINARY needs VIVY_TOOLKITS_E2E_ARTIFACT_DIR pointing at the matching ' +
      'pack output; an uninspected binary is not identifiable evidence.',
  );
}

let binary;
let inspectionDir;
if (override) {
  binary = resolve(override);
  if (!existsSync(binary) || !statSync(binary).isFile()) {
    fail(prefix, `VIVY_TOOLKITS_E2E_BINARY does not exist or is not a file: ${binary}`);
  }
  inspectionDir = resolve(artifactDir);
  const artifactBinary = join(inspectionDir, BINARY_NAME);
  if (!existsSync(join(inspectionDir, 'generation.json'))) {
    fail(prefix, `VIVY_TOOLKITS_E2E_ARTIFACT_DIR is not a pack output (no generation.json): ${inspectionDir}`);
  }
  if (!existsSync(artifactBinary)) {
    fail(prefix, `artifact does not carry ${BINARY_NAME}: ${artifactBinary}`);
  }
  if (sha256(binary) !== sha256(artifactBinary)) {
    fail(prefix, `VIVY_TOOLKITS_E2E_BINARY does not match the artifact binary at ${artifactBinary}`);
  }
  console.log(`${prefix}: explicit binary ${binary} bound to ${inspectionDir}`);
} else {
  // Pack publishes by renaming a staging tree onto --output, so the target
  // must not exist. Recreate this gate's own root to drop a stale artifact or
  // an interrupted pack staging directory.
  rmSync(GATE_ROOT, { recursive: true, force: true });
  mkdirSync(GATE_ROOT, { recursive: true });
  // Pack hides the repository UI dist into the artifact; a missing/stale dist
  // would either abort or stand in for the checked-out source.
  const built = runInherit('pnpm', ['--dir', 'ui', 'build'], { env: goEnv });
  requireSuccess(prefix, built, 'embedded UI build');
  const packed = run(
    'go',
    ['run', './sdk', 'pack', '--recipe', 'recipes/default.vivy.yml', '--output', OUTPUT],
    { env: goEnv, maxBuffer: 64 * 1024 * 1024 },
  );
  requireSuccess(prefix, packed, 'SDK pack');
  binary = join(OUTPUT, BINARY_NAME);
  inspectionDir = OUTPUT;
}

if (!existsSync(binary)) {
  fail(prefix, `packed binary is missing: ${binary}`);
}
const inspected = run('go', ['run', './sdk', 'inspect-artifact', inspectionDir], {
  env: goEnv,
  maxBuffer: 64 * 1024 * 1024,
});
requireSuccess(prefix, inspected, 'SDK inspect-artifact');
const artifact = parseJSON(inspected.stdout, 'inspect-artifact');
const generationId = artifact?.manifest?.generationId;
if (!generationId) {
  fail(prefix, 'inspect-artifact did not return a Generation Manifest');
}
console.log(`${prefix}: generation ${generationId}`);
console.log(`${prefix}: recipe ${artifact.manifest.recipeDigest ?? '-'}`);
console.log(`${prefix}: UI dist ${artifact.manifest.uiArtifacts?.['ui/dist'] ?? '-'}`);
console.log(`${prefix}: binary ${binary} sha256 ${sha256(binary)}`);

const playwrightEnv = {
  ...process.env,
  ...(goEnv.GOFLAGS ? { GOFLAGS: goEnv.GOFLAGS } : {}),
  VIVY_TOOLKITS_E2E_BINARY: binary,
  VIVY_TOOLKITS_E2E_BACKEND_PORT: String(endpoints.backendPort),
  VIVY_TOOLKITS_E2E_UI_PORT: String(endpoints.uiPort),
};
delete playwrightEnv.VIVY_TOOLKITS_E2E_EXTERNAL_BACKEND;

for (const suite of SUITES) {
  console.log(`${prefix}: running ${suite.label} suite`);
  const status = await runPlaywright(suite.config, playwrightEnv);
  if (status !== 0) {
    fail(prefix, `${suite.label} suite failed (exit ${status}); artifact kept at ${inspectionDir}`);
  }
}
console.log(`${prefix}: all Toolkit suites passed`);

function resolveEndpoints() {
  try {
    return resolveGateEndpoints({
      backendVariable: 'VIVY_TOOLKITS_E2E_BACKEND_PORT',
      backendDefault: 18787,
      uiVariable: 'VIVY_TOOLKITS_E2E_UI_PORT',
      uiDefault: 3015,
    });
  } catch (error) {
    fail(prefix, error.message);
  }
}

async function assertFree(label, port) {
  await new Promise((resolvePromise) => {
    const server = createServer();
    server.unref();
    server.once('error', (error) => {
      fail(
        prefix,
        `${label} port 127.0.0.1:${port} is unavailable (${error.code || error.message}); ` +
          'set VIVY_TOOLKITS_E2E_BACKEND_PORT / VIVY_TOOLKITS_E2E_UI_PORT to distinct free ports.',
      );
    });
    server.listen({ host: '127.0.0.1', port, exclusive: true }, () => {
      server.close(() => resolvePromise());
    });
  });
}

// Host Go 1.27 removes the x/net HTTP/2 surface that the pinned grpc still
// calls unless x/net is built with `http2legacy`. CI uses go.mod's toolchain
// and does not need this; merge the tag only for a newer host toolchain.
function withLegacyHTTP2(env) {
  const version = run('go', ['env', 'GOVERSION'], { env });
  if (version.status !== 0) {
    fail(prefix, 'go is unavailable; install the pinned toolchain before running this gate');
  }
  const match = /^go1\.(\d+)/.exec(String(version.stdout).trim());
  if (!match || Number(match[1]) < 27) return env;
  env.GOFLAGS = addBuildTag(env.GOFLAGS || '', 'http2legacy');
  console.log(`${prefix}: host ${String(version.stdout).trim()} needs -tags=http2legacy for the pinned x/net`);
  return env;
}

function addBuildTag(goflags, tag) {
  const tokens = goflags.split(/\s+/).filter(Boolean);
  for (let index = 0; index < tokens.length; index += 1) {
    if (tokens[index] === '-tags' && index + 1 < tokens.length) {
      tokens[index + 1] = mergeTags(tokens[index + 1], tag);
      return tokens.join(' ');
    }
    if (tokens[index].startsWith('-tags=')) {
      tokens[index] = `-tags=${mergeTags(tokens[index].slice('-tags='.length), tag)}`;
      return tokens.join(' ');
    }
  }
  tokens.push(`-tags=${tag}`);
  return tokens.join(' ');
}

function mergeTags(value, tag) {
  const tags = value.split(',').filter(Boolean);
  if (!tags.includes(tag)) tags.push(tag);
  return tags.join(',');
}

function sha256(file) {
  return createHash('sha256').update(readFileSync(file)).digest('hex');
}

function parseJSON(raw, label) {
  try {
    return JSON.parse(String(raw).trim());
  } catch (error) {
    fail(prefix, `${label} did not print JSON: ${error.message}`);
  }
}

// Invoke the tracked Playwright CLI through Node: no shell quoting, no
// platform-specific `pnpm.cmd` shim, and the config path is a real argv slot.
function runPlaywright(config, env) {
  const cli = join(repoRoot, 'ui', 'node_modules', '@playwright', 'test', 'cli.js');
  if (!existsSync(cli)) {
    fail(prefix, `Playwright is not installed at ${cli}; run pnpm --dir ui install first.`);
  }
  return new Promise((resolvePromise) => {
    const child = spawn(process.execPath, [cli, 'test', '--config', config], {
      cwd: join(repoRoot, 'ui'),
      env,
      stdio: 'inherit',
    });
    const forward = (signal) => {
      if (!child.killed) child.kill(signal);
    };
    const onInterrupt = () => forward('SIGINT');
    const onTerminate = () => forward('SIGTERM');
    process.once('SIGINT', onInterrupt);
    process.once('SIGTERM', onTerminate);
    child.on('error', (error) => {
      process.removeListener('SIGINT', onInterrupt);
      process.removeListener('SIGTERM', onTerminate);
      console.error(`${prefix}: Playwright could not start: ${error.message}`);
      resolvePromise(1);
    });
    child.on('exit', (code, signal) => {
      process.removeListener('SIGINT', onInterrupt);
      process.removeListener('SIGTERM', onTerminate);
      if (signal) {
        console.error(`${prefix}: Playwright exited on ${signal}`);
        resolvePromise(1);
        return;
      }
      resolvePromise(code ?? 1);
    });
  });
}
