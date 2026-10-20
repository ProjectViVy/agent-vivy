#!/usr/bin/env node
// Scoped verification of existing owners and the production minimal Recipe.
import { spawnSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const kernelOwners = [
  './internal/domain', './internal/events', './internal/config',
  './internal/storage', './internal/storage/migrations', './internal/storage/sqlite',
  './internal/provider/...', './internal/runtime/...', './internal/tools', './internal/toolhost',
  './internal/modules/loop', './internal/modules/model', './internal/modules/storage',
  './internal/modules/checkpoint', './internal/modules/credential', './internal/modules/sandbox',
  './internal/modules/optional', './sdk/module', './sdk/port/...', './sdk/generation',
];

export function executedTests(output, required = []) {
  const passed = new Set();
  const skipped = new Set();
  for (const line of output.split(/\r?\n/).filter(Boolean)) {
    const event = JSON.parse(line);
    if (event.Action === 'fail') throw new Error(`failed: ${event.Test || event.Package}`);
    if (event.Test && event.Action === 'pass') passed.add(event.Test);
    if (event.Test && event.Action === 'skip') skipped.add(event.Test);
  }
  if (!passed.size) throw new Error('no tests executed');
  for (const name of required) {
    if (!passed.has(name)) throw new Error(`required test did not execute: ${name}`);
  }
  return { passed: [...passed], skipped: [...skipped] };
}

function child(command, args, { cwd = root, env = process.env, capture = false } = {}) {
  console.log(`> ${command} ${args.join(' ')}`);
  const result = spawnSync(command, args, {
    cwd, env, encoding: 'utf8', windowsHide: true,
    stdio: capture ? 'pipe' : 'inherit', maxBuffer: 128 * 1024 * 1024,
  });
  if (result.error || result.status !== 0) {
    if (capture) process.stderr.write((result.stdout || '') + (result.stderr || ''));
    throw new Error(`${command} failed: ${result.error?.message || result.status}`);
  }
  return result.stdout?.trim() || '';
}

export function doctor() {
  if (Number(process.versions.node.split('.')[0]) < 22) throw new Error('Node 22+ is required');
  const lock = JSON.parse(readFileSync(join(root, 'laputa-source.lock.json'), 'utf8'));
  const dependency = resolve(root, '..', 'laputa');
  const head = child('git', ['-C', dependency, 'rev-parse', 'HEAD'], { capture: true });
  if (head !== lock.commit) throw new Error('Laputa does not match laputa-source.lock.json');
  if (child('git', ['-C', dependency, 'status', '--porcelain'], { capture: true })) {
    throw new Error('Laputa has local changes');
  }
  child('go', ['version']);
  console.log(`Node ${process.version}; Laputa ${head}; checkout ${root}`);
  console.log('doctor: 4 ok, 0 warnings, 0 failures');
}

function goTests(packages, { run, required, cwd, env, race = false } = {}) {
  const args = ['test', '-json', '-tags=vivy_headless', '-count=1', '-timeout=35m'];
  if (run) args.push('-run', run);
  if (race) args.push('-race');
  const output = child('go', [...args, ...packages], { cwd, env, capture: true });
  let evidence;
  try { evidence = executedTests(output, required); }
  catch (error) { process.stderr.write(output); throw error; }
  console.log(`${evidence.passed.length} executed tests passed; ${evidence.skipped.length} skipped`);
  for (const name of evidence.skipped) console.log(`SKIPPED: ${name}`);
}

function main(scope) {
  if (scope === 'doctor') return doctor();
  if (!['wiring', 'kernel', 'kernel-race', 'd02', 'd02-web', 'postgres'].includes(scope)) {
    throw new Error('usage: node scripts/verify-kernel.mjs <doctor|wiring|kernel|kernel-race|d02|d02-web|postgres>');
  }
  if (process.platform === 'win32') throw new Error('Use the independent WSL Linux-filesystem checkout for scoped CI; run Windows owner acceptance separately');
  if (root.startsWith('/mnt/')) throw new Error('Move the verification checkout to the Linux filesystem');
  if (scope === 'postgres' && !process.env.VIVY_POSTGRES_TEST_DSN) {
    throw new Error('VIVY_POSTGRES_TEST_DSN is required; skipping is not a pass');
  }
  doctor();
  console.log(`candidate ${child('git', ['rev-parse', 'HEAD'], { capture: true })}`);
  console.log(`worktree changes: ${child('git', ['status', '--porcelain'], { capture: true }).split('\n').filter(Boolean).length}`);
  const home = mkdtempSync(join(tmpdir(), 'vivy-verification-'));
  const env = { ...process.env, VIVY_USER_HOME: home, GOPROXY: process.env.VIVY_GOPROXY || 'direct' };
  try {
    if (scope === 'wiring') {
      child(process.execPath, ['--test', 'scripts/ensure-laputa.test.mjs', 'scripts/verify-kernel.test.mjs'], { env });
      goTests(['./sdk/internal/assembly'], {
        env, run: '^Test(MinimalRecipeOmitsOptionalCapabilities|GenerateRuntimeAssembly.*|Compile.*|SourceCatalog.*|P1P2PortConformanceSuite)$',
        required: ['TestMinimalRecipeOmitsOptionalCapabilities'],
      });
    } else if (scope === 'kernel' || scope === 'kernel-race') {
      child('go', ['vet', '-tags=vivy_headless', ...kernelOwners], { env });
      goTests(kernelOwners, { env, race: scope === 'kernel-race' });
      if (scope === 'kernel') goTests(['./sdk/internal'], {
        env, run: '^TestMinimalKernelArtifactBootAndInspect$', required: ['TestMinimalKernelArtifactBootAndInspect'],
      });
    } else if (scope === 'd02-web') {
      child('go', ['run', '-tags=vivy_headless', './sdk', 'stage-ui',
        '--recipe', 'recipes/default.vivy.yml', '--out', 'ui/src/generated'], { env });
      child('pnpm', ['--dir', 'ui', 'exec', 'tsc', '--noEmit'], { env });
      child('pnpm', ['--dir', 'ui', 'exec', 'vitest', 'run',
        'src/lib/model-completion.test.ts', 'src/lib/store.test.ts',
        'src/lib/run-subscription.test.ts', 'src/lib/run-rows.test.ts',
        'src/components/chat/ChatView.test.tsx'], { env });
      goTests(['./sdk/internal'], { env, run: '^Test(PackBuildsSelectedUIIntoFinalArtifact|LiteArtifactRetainsBaseWebShell)$',
        required: ['TestPackBuildsSelectedUIIntoFinalArtifact', 'TestLiteArtifactRetainsBaseWebShell'] });
    } else if (scope === 'postgres') {
      goTests(['./internal/storage/postgres'], { env, required: ['TestBackendConformance', 'TestMigrateUpgradesV14InPlace'] });
    } else {
      goTests(['./internal/runtime', './internal/rpc'], {
        env, run: 'TestModelCompletedV3|TestCompletion|TestReplayNormalizes|TestMapper|TestMessageProjector|TestTrajectory|TestModelCallPayloadSchemas|TestObservedCall|TestBudget|TestToolHook|TestRecoverAcrossModelGeneration|TestDescribeModelRequest|TestRunSubscription|TestSubscriptionNormalizesHistoricalCompletion|TestLegacyCompletionOnlyReadsPersistedMessages',
        required: ['TestModelCompletedV3AcrossSteps', 'TestSubscriptionNormalizesHistoricalCompletion'],
      });
      goTests(['./sdk/facerun', './sdk/tui/stream', './sdk/tui/live', './sdk/codeclient'], { env });
      goTests(['./internal/app'], { env, run: '^TestAppRunsWithoutDigestAudit$', required: ['TestAppRunsWithoutDigestAudit'] });
      goTests(['./internal/app'], { env, run: 'TestApp.*(Shutdown|Close|Listener)|TestCloseContext|TestHeadless' });
      goTests(['./...'], { env, cwd: join(root, 'faces', 'headless') });
    }
  } finally { rmSync(home, { recursive: true, force: true }); }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { main(process.argv[2]); }
  catch (error) { console.error(error.message); process.exitCode = 1; }
}
