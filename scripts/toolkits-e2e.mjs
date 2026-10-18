#!/usr/bin/env node
// Reproducible Toolkit browser gate.
//
// One repository entry prepares a packed body with the SDK, inspects that
// artifact, then runs both tracked Toolkit Playwright suites against their own
// backend, Vite server and data root. The previous workflow needed a manually
// built `.workspace/toolkits-e2e/default/vivy` and fixed ports, so it could not
// run beside another lane.
import { spawn } from 'node:child_process';
import { createServer } from 'node:net';
import { existsSync, mkdirSync, rmSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { executable, fail, repoRoot, run, runInherit } from './lib/process.mjs';

const prefix = 'toolkits-e2e';
const DEFAULT_BACKEND_PORT = 18787;
const DEFAULT_UI_PORT = 3015;
const GATE_ROOT = join(repoRoot, '.workspace', 'toolkits-e2e');

const backendPort = readPort('VIVY_TOOLKITS_E2E_BACKEND_PORT', DEFAULT_BACKEND_PORT);
const uiPort = readPort('VIVY_TOOLKITS_E2E_UI_PORT', DEFAULT_UI_PORT);
if (backendPort === uiPort) {
  fail(prefix, `backend and UI ports must differ (both are ${backendPort})`);
}

const goEnv = buildGoEnv();
const artifactRoot = join(GATE_ROOT, 'gate');
const sdkBinary = join(artifactRoot, 'vivy-sdk');
const artifactDirectory = join(artifactRoot, 'default');

await assertPort('backend', backendPort);
await assertPort('UI', uiPort);

// A stale artifact is worse than no artifact: pack renames its staging tree
// into --output and refuses to overwrite, so the gate always starts clean.
rmSync(artifactRoot, { recursive: true, force: true });
mkdirSync(artifactRoot, { recursive: true });

console.log(`${prefix}: backend 127.0.0.1:${backendPort}, UI 127.0.0.1:${uiPort}`);
console.log(`${prefix}: artifact root ${artifactRoot}`);
if (goEnv.GOFLAGS) console.log(`${prefix}: GOFLAGS ${goEnv.GOFLAGS}`);

requireSuccess(
  runInherit('go', ['build', '-o', sdkBinary, './sdk'], { env: goEnv }),
  'build vivy-sdk',
  'go build ./sdk',
);
// Pack hides the repository UI dist into the artifact, so the embedded-UI
// build must exist before packing. Always rebuild: a stale dist must not stand
// in for the checked-out source.
requireSuccess(
  runInherit('pnpm', ['--dir', 'ui', 'build'], { env: goEnv }),
  'build embedded UI',
  'pnpm --dir ui build',
);

const override = process.env.VIVY_TOOLKITS_E2E_BINARY;
let artifact;
if (override) {
  if (!existsSync(override)) {
    fail(prefix, `VIVY_TOOLKITS_E2E_BINARY does not exist: ${override}`);
  }
  artifact = { binary: override, manifest: {} };
  console.log(`${prefix}: using explicit binary ${override}`);
} else {
  const packed = run(sdkBinary, [
    'pack',
    '--recipe',
    'recipes/default.vivy.yml',
    '--output',
    artifactDirectory,
  ], { env: goEnv, maxBuffer: 64 * 1024 * 1024 });
  requireSuccess(packed, 'pack default Recipe', `${sdkBinary} pack`);
  artifact = parseJSON(packed.stdout, 'pack');
  if (!artifact.binary || !artifact.manifest?.generationId) {
    fail(prefix, 'pack output did not declare a binary and generationId');
  }
}

const inspected = run(sdkBinary, ['inspect-artifact', dirname(artifact.binary)], {
  env: goEnv,
  maxBuffer: 64 * 1024 * 1024,
});
requireSuccess(inspected, 'inspect packed artifact', `${sdkBinary} inspect-artifact`);
const report = parseJSON(inspected.stdout, 'inspect-artifact');
if (!report.manifest?.generationId) {
  fail(prefix, 'inspect-artifact did not return a generation manifest');
}
if (artifact.manifest.generationId && report.manifest.generationId !== artifact.manifest.generationId) {
  fail(
    prefix,
    `inspect generation ${report.manifest.generationId} does not match packed generation ${artifact.manifest.generationId}`,
  );
}
console.log(`${prefix}: generation ${report.manifest.generationId}`);
console.log(`${prefix}: modules ${Array.isArray(report.manifest.modules) ? report.manifest.modules.length : 0}`);
console.log(`${prefix}: binary ${artifact.binary}`);

const playwrightEnv = {
  ...process.env,
  VIVY_TOOLKITS_E2E_BINARY: artifact.binary,
  VIVY_TOOLKITS_E2E_BACKEND_PORT: String(backendPort),
  VIVY_TOOLKITS_E2E_UI_PORT: String(uiPort),
};
// The gate always owns its backend; the manual external-backend mode stays a
// direct-Playwright option and is deliberately not a reproducible gate.
delete playwrightEnv.VIVY_TOOLKITS_E2E_EXTERNAL_BACKEND;
if (goEnv.GOFLAGS) playwrightEnv.GOFLAGS = goEnv.GOFLAGS;

const suites = [
  { label: 'lifecycle', config: 'playwright.toolkits.config.ts' },
  { label: 'accessibility', config: 'playwright.toolkits-accessibility.config.ts' },
];
for (const suite of suites) {
  console.log(`${prefix}: running ${suite.label} suite`);
  const status = await runPlaywright(suite.config, playwrightEnv);
  if (status !== 0) {
    fail(prefix, `${suite.label} suite failed (exit ${status}); artifact kept at ${artifactDirectory}`);
  }
}
console.log(`${prefix}: all Toolkit suites passed`);

function readPort(name, fallback) {
  const raw = process.env[name];
  if (raw === undefined || raw === '') return fallback;
  if (!/^\d+$/.test(raw)) {
    fail(prefix, `${name} must be an integer between 1 and 65535, got ${JSON.stringify(raw)}`);
  }
  const port = Number(raw);
  if (port < 1 || port > 65535) {
    fail(prefix, `${name} must be between 1 and 65535, got ${port}`);
  }
  return port;
}

function requireSuccess(result, label, command) {
  if (result.error) {
    fail(prefix, `${label} could not start: ${result.error.message}`);
  }
  if (result.status !== 0) {
    const detail = `${result.stderr || ''}${result.stdout || ''}`.trim().slice(-4000);
    fail(prefix, `${label} failed (exit ${result.status})${detail ? `:\n${detail}` : ''}`);
  }
  return result;
}

function parseJSON(raw, label) {
  try {
    return JSON.parse(String(raw).trim());
  } catch (error) {
    fail(prefix, `${label} did not print JSON: ${error.message}\n${String(raw).slice(-2000)}`);
  }
}

function assertPort(label, port) {
  return new Promise((resolve, reject) => {
    const server = createServer();
    server.unref();
    server.once('error', (error) => {
      reject(
        new Error(
          `${label} port 127.0.0.1:${port} is unavailable (${error.code || error.message}); ` +
            'choose VIVY_TOOLKITS_E2E_BACKEND_PORT / VIVY_TOOLKITS_E2E_UI_PORT',
        ),
      );
    });
    server.listen({ host: '127.0.0.1', port, exclusive: true }, () => {
      server.close(() => resolve());
    });
  }).catch((error) => fail(prefix, error.message));
}

// Host Go 1.27 removes the x/net HTTP/2 surface that the pinned grpc still
// calls unless x/net is built with `http2legacy`. CI uses go.mod's toolchain
// and does not need this; merge the tag only for a newer host toolchain.
function buildGoEnv() {
  const env = { ...process.env };
  const version = run('go', ['env', 'GOVERSION'], { env: { ...process.env } });
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
  for (let i = 0; i < tokens.length; i += 1) {
    if (tokens[i] === '-tags' && i + 1 < tokens.length) {
      tokens[i + 1] = mergeTags(tokens[i + 1], tag);
      return tokens.join(' ');
    }
    if (tokens[i].startsWith('-tags=')) {
      tokens[i] = `-tags=${mergeTags(tokens[i].slice('-tags='.length), tag)}`;
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

function runPlaywright(config, env) {
  return new Promise((resolve) => {
    const child = spawn(
      executable('pnpm'),
      ['exec', 'playwright', 'test', '--config', config],
      { cwd: join(repoRoot, 'ui'), env, stdio: 'inherit' },
    );
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
      console.error(`${prefix}: playwright could not start: ${error.message}`);
      resolve(1);
    });
    child.on('exit', (code) => {
      process.removeListener('SIGINT', onInterrupt);
      process.removeListener('SIGTERM', onTerminate);
      resolve(code ?? 1);
    });
  });
}
