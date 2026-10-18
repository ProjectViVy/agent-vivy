#!/usr/bin/env node
// Build the default container image with the pinned Laputa checkout as a
// named build context. Fails closed when the checkout is not at the lock, or
// when the Dockerfile is missing an input for a local replace or UI file
// dependency.
import { spawnSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const prefix = 'docker-build';
const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const laputaRoot = resolve(dirname(repoRoot), 'laputa');
const lock = JSON.parse(readFileSync(join(repoRoot, 'laputa-source.lock.json'), 'utf8'));

function fail(message) {
  console.error(`${prefix}: ${message}`);
  process.exit(1);
}

function docker(args, options = {}) {
  return spawnSync('docker', args, {
    cwd: repoRoot,
    encoding: 'utf8',
    stdio: options.inherit ? 'inherit' : 'pipe',
    ...options,
  });
}

function usage() {
  return `usage: node scripts/docker-build.mjs [--tag <name>] [--no-cache] [--smoke]

Builds the embedded-UI image (default tag vivy:local). --smoke starts the
image without a host port and waits for its /healthz healthcheck, proving a
one-shot startup on an empty /data. VIVY_GOPROXY / VIVY_NPM_REGISTRY override
the build-time module and package registries.`;
}

function parseArgs(argv) {
  const options = { tag: 'vivy:local', noCache: false, smoke: false };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '--tag') {
      options.tag = argv[i + 1] ?? '';
      i += 1;
    } else if (arg.startsWith('--tag=')) {
      options.tag = arg.slice('--tag='.length);
    } else if (arg === '--no-cache') {
      options.noCache = true;
    } else if (arg === '--smoke') {
      options.smoke = true;
    } else if (arg === '--help' || arg === '-h') {
      console.log(usage());
      process.exit(0);
    } else {
      fail(`unknown argument: ${arg}`);
    }
  }
  if (!options.tag) fail('--tag needs a value');
  return options;
}

function verifyLaputaPin() {
  const head = spawnSync('git', ['-C', laputaRoot, 'rev-parse', 'HEAD'], { encoding: 'utf8' });
  if (head.error || head.status !== 0) {
    fail(`cannot inspect ${laputaRoot}; run 'just ensure-laputa'`);
  }
  if (head.stdout.trim() !== lock.commit) {
    fail(`${laputaRoot} is at ${head.stdout.trim().slice(0, 12)}; the lock pins ${lock.commit.slice(0, 12)}`);
  }
  const changes = spawnSync('git', ['-C', laputaRoot, 'status', '--porcelain'], { encoding: 'utf8' });
  if (changes.error || changes.status !== 0) {
    fail(`cannot inspect the worktree at ${laputaRoot}`);
  }
  if (changes.stdout.trim()) {
    fail(`${laputaRoot} has local changes; the image would not match the pinned source`);
  }
}

function verifyDockerfileManifests() {
  const goMod = readFileSync(join(repoRoot, 'go.mod'), 'utf8');
  const dockerfile = readFileSync(join(repoRoot, 'Dockerfile'), 'utf8');
  const localTargets = [...goMod.matchAll(/^replace\s+\S+(?:\s+v\S+)?\s+=>\s+(\.\/\S+)\s*$/gm)]
    .map((match) => match[1].slice(2).replace(/\/+$/, ''));
  if (localTargets.length === 0) {
    fail('go.mod has no local replace directives; the Dockerfile check is stale');
  }
  const missing = localTargets.filter((target) => !dockerfile.includes(`COPY ${target}/go.mod `));
  if (missing.length > 0) {
    fail(`Dockerfile does not copy manifests for local replace targets: ${missing.join(', ')}`);
  }
  const missingLaputa = ['garden', 'mentle', 'laputa'].filter(
    (name) => !dockerfile.includes(`COPY --from=laputa ${name}/go.mod `),
  );
  if (missingLaputa.length > 0) {
    fail(`Dockerfile does not copy the pinned Laputa manifests: ${missingLaputa.join(', ')}`);
  }
}

function verifyUiFileDependencies() {
  const pkg = JSON.parse(readFileSync(join(repoRoot, 'ui/package.json'), 'utf8'));
  const dockerfile = readFileSync(join(repoRoot, 'Dockerfile'), 'utf8');
  const targets = Object.values({ ...pkg.dependencies, ...pkg.devDependencies })
    .filter((spec) => typeof spec === 'string' && spec.startsWith('file:'))
    .map((spec) =>
      relative(repoRoot, resolve(repoRoot, 'ui', spec.slice('file:'.length))).replaceAll('\\', '/'),
    )
    .filter((target) => !target.startsWith('..'));
  if (targets.length === 0) {
    fail('ui/package.json has no local file dependencies; the Dockerfile check is stale');
  }
  const missing = targets.filter((target) => !dockerfile.includes(`COPY ${target} `));
  if (missing.length > 0) {
    fail(`Dockerfile does not copy UI file dependencies before install: ${missing.join(', ')}`);
  }
}

function verifyDocker() {
  const version = docker(['version', '--format', '{{.Server.Version}}']);
  if (version.error) {
    fail('docker CLI is not on PATH; run this from WSL/Linux where the daemon lives');
  }
  if (version.status !== 0) {
    fail(`Docker daemon unavailable: ${(version.stderr || '').trim() || 'start Docker and retry'}`);
  }
  console.log(`${prefix}: Docker ${version.stdout.trim()}`);
}

function printLogs(name) {
  const logs = docker(['logs', '--tail', '80', name]);
  const text = `${logs.stdout || ''}${logs.stderr || ''}`.trim();
  if (text) {
    console.error(`${prefix}: container logs (${name}):`);
    console.error(text);
  }
}

async function smoke(tag) {
  const name = `vivy-docker-smoke-${process.pid}`;
  docker(['rm', '-f', name]);
  const started = docker(['run', '-d', '--name', name, '--tmpfs', '/data', tag]);
  if (started.status !== 0) {
    fail(`docker run failed: ${(started.stderr || started.stdout || '').trim()}`);
  }
  console.log(`${prefix}: smoke container ${name} started; waiting for health`);
  let failure = '';
  try {
    const deadline = Date.now() + 120_000;
    while (Date.now() < deadline) {
      const inspect = docker([
        'inspect',
        '-f',
        '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}}',
        name,
      ]);
      if (inspect.status !== 0) {
        failure = `docker inspect failed: ${(inspect.stderr || '').trim()}`;
        break;
      }
      const [state, health] = inspect.stdout.trim().split(/\s+/);
      if (state === 'exited' || state === 'dead') {
        failure = `container exited during startup (${health || 'no health record'})`;
        break;
      }
      if (health === 'healthy') {
        console.log(`${prefix}: smoke passed; container healthy on an empty /data`);
        break;
      }
      if (health === 'unhealthy') {
        failure = 'container became unhealthy';
        break;
      }
      await new Promise((resolveSleep) => setTimeout(resolveSleep, 2000));
    }
    if (!failure && Date.now() >= deadline) {
      failure = 'container did not become healthy within 120s';
    }
  } finally {
    if (failure) printLogs(name);
    docker(['rm', '-f', name]);
  }
  if (failure) fail(failure);
}

async function main() {
  const options = parseArgs(process.argv.slice(2));
  verifyLaputaPin();
  verifyDockerfileManifests();
  verifyUiFileDependencies();
  verifyDocker();

  const args = ['build', '--build-context', `laputa=${laputaRoot}`, '-t', options.tag];
  const goproxy = process.env.VIVY_GOPROXY || process.env.GOPROXY;
  if (goproxy) args.push('--build-arg', `GOPROXY=${goproxy}`);
  const npmRegistry = process.env.VIVY_NPM_REGISTRY || process.env.NPM_REGISTRY;
  if (npmRegistry) args.push('--build-arg', `NPM_REGISTRY=${npmRegistry}`);
  if (options.noCache) args.push('--no-cache');
  args.push('.');
  console.log(`${prefix}: ${args.join(' ')}`);
  const built = docker(args, { inherit: true });
  if (built.error) fail(`could not run docker: ${built.error.message}`);
  if (built.status !== 0) fail(`docker build failed (exit ${built.status})`);

  if (options.smoke) await smoke(options.tag);
}

await main();
