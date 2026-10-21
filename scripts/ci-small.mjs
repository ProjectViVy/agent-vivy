#!/usr/bin/env node
// One path/Recipe association table, shared by local development and Actions.
import { spawnSync } from 'node:child_process';
import { appendFileSync, existsSync, mkdtempSync, readFileSync, readdirSync, realpathSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, isAbsolute, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { executedTests } from './verify-kernel.mjs';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const checkNames = new Set(['diff', 'scripts', 'go', 'wiring', 'kernel', 'protocol', 'race', 'postgres', 'ui', 'recipe', 'browser']);

export function command(program, args, options = {}) {
  const result = spawnSync(program, args, {
    cwd: root, encoding: 'utf8', windowsHide: true, maxBuffer: 128 * 1024 * 1024,
    ...options,
  });
  if (result.error || result.status !== 0) {
    throw new Error(`${program} ${args.join(' ')} failed: ${result.error?.message || result.stderr || result.stdout || result.status}`);
  }
  return result.stdout || '';
}

export function changedFiles(cwd, base) {
  // Reject option injection and verify the supplied baseline before diffing.
  const revision = command('git', ['rev-parse', '--verify', '--end-of-options', `${base}^{commit}`], { cwd }).trim();
  const names = command('git', ['diff', '--name-status', '-z', '--find-renames', revision, '--'], { cwd }).split('\0');
  const paths = [];
  for (let i = 0; i < names.length && names[i];) {
    const status = names[i++];
    paths.push(names[i++]);
    if (/^[RC]/.test(status)) paths.push(names[i++]);
  }
  paths.push(...command('git', ['ls-files', '--others', '--exclude-standard', '-z'], { cwd }).split('\0').filter(Boolean));
  return [...new Set(paths)].sort();
}

export function selectScopes(files, scopes, extra = []) {
  for (const id of extra) if (!scopes.some(s => s.id === id)) throw new Error(`Unknown scope: ${id}`);
  const selected = new Set(extra);
  for (const file of files) {
    const matching = scopes.filter(s => (!file.endsWith('.md') || s.id === 'docs') && s.paths.some(pattern => new RegExp(pattern).test(file)));
    if (!matching.length) throw new Error(`Unmapped input: ${file}; add its owner to .github/ci-scopes.json`);
    for (const scope of matching) selected.add(scope.id);
  }
  return scopes.filter(s => selected.has(s.id));
}

export function validateRecipe(cwd, recipe) {
  const target = resolve(cwd, recipe);
  if (isAbsolute(recipe) || !existsSync(target)) throw new Error(`Recipe missing or outside checkout: ${recipe}`);
  const rel = relative(realpathSync(cwd), realpathSync(target));
  if (rel.startsWith('..') || isAbsolute(rel) || !/^apiVersion:\s*vivy\.generation\/v1\s*$/m.test(readFileSync(target, 'utf8'))) {
    throw new Error(`Recipe must be a checkout-owned vivy.generation/v1 document: ${recipe}`);
  }
  return recipe;
}

export function fullRequested(eventName, event) {
  return eventName === 'pull_request'
    ? (event.pull_request?.labels || []).some(label => label.name === 'epic-final')
    : eventName === 'workflow_dispatch' && String(event.inputs?.full) === 'true';
}

export function aggregateResult(full, small, lanes) {
  return small === 'success' && (!full || (lanes.length > 0 && lanes.every(value => value === 'success')));
}

function loadScopes(cwd) {
  const scopes = JSON.parse(readFileSync(join(cwd, '.github/ci-scopes.json'), 'utf8')).scopes;
  if (!Array.isArray(scopes) || !scopes.length) throw new Error('CI scopes must be a nonempty array');
  const ids = new Set();
  for (const scope of scopes) {
    if (!/^[a-z][a-z0-9-]*$/.test(scope.id) || ids.has(scope.id)) throw new Error(`Invalid/duplicate scope: ${scope.id}`);
    ids.add(scope.id);
    if (!scope.paths?.length || !scope.checks?.length) throw new Error(`Empty scope: ${scope.id}`);
    scope.paths.forEach(pattern => new RegExp(pattern));
    for (const check of scope.checks) if (!checkNames.has(check)) throw new Error(`Unknown check: ${check}`);
    if (scope.recipe && scope.recipe !== '@changed') validateRecipe(cwd, scope.recipe);
    for (const pkg of scope.goPackages || []) {
      if (pkg !== '@changed' && (!pkg.startsWith('./') || pkg.includes('..\\') || /[\r\n]/.test(pkg))) throw new Error(`Invalid Go package: ${pkg}`);
    }
  }
  return scopes;
}

export function makePlan(cwd, base, extra = [], files = changedFiles(cwd, base)) {
  const selected = selectScopes(files, loadScopes(cwd), extra);
  const recipes = [...new Set(selected.flatMap(s => s.recipe === '@changed'
    ? files.filter(f => /^recipes\/.*\.vivy\.yml$/.test(f) && existsSync(join(cwd, f)))
    : s.recipe ? [s.recipe] : []))];
  recipes.forEach(recipe => validateRecipe(cwd, recipe));
  const plan = { base, candidate: command('git', ['rev-parse', 'HEAD'], { cwd }).trim(), files,
    scopes: selected.map(s => ({ ...s, reasons: files.filter(f => s.paths.some(p => new RegExp(p).test(f))) })),
    checks: [...new Set(selected.flatMap(s => s.checks))], recipes };
  const modes = { wiring: 'wiring', kernel: 'kernel', protocol: 'd02', race: 'kernel-race', postgres: 'postgres' };
  plan.commands = plan.checks.flatMap(check => modes[check] ? [`node scripts/verify-kernel.mjs ${modes[check]}`]
    : check === 'diff' ? [`git diff --check ${base} --`]
      : check === 'scripts' ? ['node --test scripts/*.test.mjs', 'node --check <changed .mjs files>']
        : check === 'go' ? [...goGroups(plan, cwd)].flatMap(([owner, packages]) => [`(cwd ${owner}) go vet -tags=vivy_headless ${[...packages].join(' ')}`, `(cwd ${owner}) go test -json -tags=vivy_headless -count=1 -timeout=35m ${[...packages].join(' ')}`])
          : recipeSessions(plan).filter(session => session.checks.includes(check)).map(({ recipe }) => `${check}: ${recipe} -> ${check === 'ui' ? 'stage-ui, tsc, selected/related vitest' : check === 'browser' ? 'pack, inspect-artifact, stage-ui, playwright.minimal.config.ts' : 'pack, inspect-artifact'}`));
  return plan;
}

export function goGroups(plan, cwd) {
  cwd = resolve(cwd);
  const groups = new Map();
  for (const scope of plan.scopes) for (const item of scope.goPackages || []) {
    const packages = item === '@changed' ? scope.reasons.filter(f => !f.endsWith('.md')).map(f => `./${dirname(f).replaceAll('\\', '/')}`) : [item];
    for (const pkg of packages) {
      const target = resolve(cwd, pkg.replace(/\/\.\.\.$/, ''));
      const boundary = relative(cwd, target);
      if (boundary.startsWith('..') || isAbsolute(boundary)) throw new Error(`Go package outside checkout: ${pkg}`);
      if (!existsSync(target)) continue;
      let owner = target;
      while (owner !== cwd && !existsSync(join(owner, 'go.mod'))) owner = dirname(owner);
      if (!existsSync(join(owner, 'go.mod'))) throw new Error(`No Go module for ${pkg}`);
      if (!groups.has(owner)) groups.set(owner, new Set());
      // Non-source inputs belong to the nearest package, or to the whole module.
      let packageRoot = target;
      while (packageRoot !== owner && !readdirSync(packageRoot).some(f => f.endsWith('.go'))) packageRoot = dirname(packageRoot);
      const local = relative(owner, packageRoot).replaceAll('\\', '/');
      const recursive = pkg.endsWith('/...') || !readdirSync(packageRoot).some(f => f.endsWith('.go'));
      groups.get(owner).add(`${local ? `./${local}` : '.'}${recursive ? '/...' : ''}`);
    }
  }
  if (plan.scopes.some(s => s.goPackages?.length) && !groups.size) throw new Error('No Go owner tests selected; map the changed owner');
  return groups;
}

export function recipeSessions(plan) {
  const sessions = new Map();
  for (const scope of plan.scopes) {
    const recipes = scope.recipe === '@changed'
      ? plan.files.filter(f => /^recipes\/.*\.vivy\.yml$/.test(f) && existsSync(join(root, f)))
      : scope.recipe ? [scope.recipe] : [];
    for (const recipe of recipes) {
      if (!sessions.has(recipe)) sessions.set(recipe, { recipe, checks: [], uiTests: [], uiSources: [] });
      const session = sessions.get(recipe);
      session.checks = [...new Set([...session.checks, ...scope.checks.filter(c => ['ui', 'recipe', 'browser'].includes(c))])];
      session.uiTests = [...new Set([...session.uiTests, ...(scope.uiTests || [])])];
      session.uiSources = [...new Set([...session.uiSources, ...(scope.reasons || [])
        .filter(f => /^(ui\/src\/|sdk\/ui\/).*\.tsx?$/.test(f) && existsSync(join(root, f)))
        .map(f => relative(join(root, 'ui'), join(root, f)).replaceAll('\\', '/'))])];
    }
  }
  return [...sessions.values()];
}

export function recipeNeedsUI(cwd, recipe) {
  // Conservative dependency preparation only; the SDK remains the authority for
  // full Recipe parsing, dependency closure and capability validation.
  const source = readFileSync(join(cwd, validateRecipe(cwd, recipe)), 'utf8');
  return !/^profile:\s*minimal\s*$/m.test(source) || /^ui\s*:/m.test(source);
}

export function executePlan(plan, cwd = root) {
  const run = (program, args, options = {}) => {
    console.log(`> ${program} ${args.join(' ')}`);
    const output = command(program, args, { cwd, ...options });
    if (output.length > 6000) {
      console.log('(successful command output condensed)');
      console.log(output.split('\n').slice(-20).map(line => line.slice(0, 300)).join('\n'));
    } else if (output) process.stdout.write(output);
    return output;
  };
  const needsGo = plan.checks.some(c => ['go', 'wiring', 'kernel', 'protocol', 'race', 'postgres', 'ui', 'recipe', 'browser'].includes(c));
  if (needsGo) run(process.execPath, ['scripts/ensure-laputa.mjs']);
  if (plan.checks.includes('diff')) run('git', ['diff', '--check', plan.base, '--']);
  if (plan.checks.includes('scripts')) {
    const tests = readdirSync(join(cwd, 'scripts')).filter(f => f.endsWith('.test.mjs')).map(f => `scripts/${f}`);
    if (!tests.length) throw new Error('No script tests found');
    run(process.execPath, ['--test', ...tests]);
    for (const file of plan.files.filter(f => f.endsWith('.mjs') && existsSync(join(cwd, f)))) run(process.execPath, ['--check', file]);
  }
  if (plan.checks.includes('go')) for (const [owner, packages] of goGroups(plan, cwd)) {
    run('go', ['vet', '-tags=vivy_headless', ...packages], { cwd: owner });
    const output = run('go', ['test', '-json', '-tags=vivy_headless', '-count=1', '-timeout=35m', ...packages], { cwd: owner });
    const evidence = executedTests(output);
    console.log(`${evidence.passed.length} Go tests passed; ${evidence.skipped.length} skipped`);
  }
  for (const [check, mode] of Object.entries({ wiring: 'wiring', kernel: 'kernel', protocol: 'd02', race: 'kernel-race', postgres: 'postgres' })) {
    if (plan.checks.includes(check)) run(process.execPath, ['scripts/verify-kernel.mjs', mode]);
  }
  if (plan.checks.some(c => ['ui', 'recipe', 'browser'].includes(c))) {
    const scratch = mkdtempSync(join(tmpdir(), 'vivy-ci-small-'));
    try {
      for (const session of recipeSessions(plan)) {
        const { recipe, checks, uiTests, uiSources } = session;
        const output = join(scratch, `artifact-${plan.recipes.indexOf(recipe)}`);
        if (checks.includes('recipe') || checks.includes('browser')) {
          run('go', ['run', '-tags=vivy_headless', './sdk', 'pack', '--recipe', recipe, '--output', output]);
          run('go', ['run', '-tags=vivy_headless', './sdk', 'inspect-artifact', output]);
        }
        if (checks.includes('ui')) {
          const env = { ...process.env, VIVY_UI_RECIPE: recipe };
          run(process.execPath, ['ui/scripts/stage-ui-assembly.mjs'], { env });
          // Direct executables avoid repeated npm pre-hooks within this session.
          run(process.execPath, ['ui/node_modules/typescript/bin/tsc', '--noEmit', '-p', 'ui/tsconfig.json'], { env });
          const tests = uiTests.filter(test => test !== '@related');
          if (!tests.length) throw new Error('UI scope must select behavior tests');
          run(process.execPath, ['node_modules/vitest/vitest.mjs', 'run', ...tests], { cwd: join(cwd, 'ui'), env });
          if (uiTests.includes('@related') && uiSources.length) {
            run(process.execPath, ['node_modules/vitest/vitest.mjs', 'related', '--run', ...uiSources], { cwd: join(cwd, 'ui'), env });
          }
        }
        if (checks.includes('browser') && recipe === 'recipes/minimal-chat.vivy.yml') {
          const projection = join(scratch, 'chat-ui');
          run('go', ['run', '-tags=vivy_headless', './sdk', 'stage-ui', '--recipe', recipe, '--out', projection]);
          run(process.execPath, ['node_modules/@playwright/test/cli.js', 'test', '--config', 'playwright.minimal.config.ts'], {
            cwd: join(cwd, 'ui'), env: { ...process.env, VIVY_MINIMAL_CHAT_ARTIFACT: output, VIVY_UI_ASSEMBLY_ENTRY: join(projection, 'assembly.ts') },
          });
        }
      }
    } finally { rmSync(scratch, { recursive: true, force: true }); }
  }
}

function main(args) {
  if (args[0] === '--aggregate') {
    const full = process.env.FULL === 'true';
    const lanes = [process.env.BACKEND_RESULT, process.env.UI_RESULT, process.env.BROWSER_RESULT];
    const ok = process.env.PLAN_RESULT === 'success' && aggregateResult(full, process.env.SMALL_RESULT, lanes);
    const summary = `Small CI: ${process.env.SMALL_RESULT}\nFull product CI: ${full ? lanes.join(', ') : 'not requested (not verified)'}\nCandidate: ${process.env.GITHUB_SHA}\n`;
    console.log(summary);
    if (process.env.GITHUB_STEP_SUMMARY) appendFileSync(process.env.GITHUB_STEP_SUMMARY, summary);
    if (!ok) throw new Error('Required CI lanes did not all succeed');
    return;
  }
  let base, planOnly = false, actions = false;
  const extra = (process.env.CI_SCOPES || '').split(/\s+/).filter(Boolean);
  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--base') base = args[++i];
    else if (args[i] === '--scope') extra.push(args[++i]);
    else if (args[i] === '--plan') planOnly = true;
    else if (args[i] === '--actions') actions = true;
    else throw new Error(`Unknown option: ${args[i]}`);
  }
  if (!base) throw new Error('usage: ci-small --base <ref> [--scope <id> ...] [--plan]');
  const plan = makePlan(root, base, extra);
  console.log(JSON.stringify(plan, null, 2));
  if (actions) {
    const event = JSON.parse(readFileSync(process.env.GITHUB_EVENT_PATH, 'utf8'));
    const needsGo = plan.checks.some(c => ['go', 'wiring', 'kernel', 'protocol', 'race', 'postgres', 'ui', 'recipe', 'browser'].includes(c));
    appendFileSync(process.env.GITHUB_OUTPUT, `full=${fullRequested(process.env.GITHUB_EVENT_NAME, event)}\nui=${plan.checks.some(c => ['ui', 'browser'].includes(c)) || plan.recipes.some(recipe => recipeNeedsUI(root, recipe))}\ngo=${needsGo}\npostgres=${plan.checks.includes('postgres')}\n`);
  }
  if (!planOnly) executePlan(plan);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { main(process.argv.slice(2)); }
  catch (error) { console.error(error.message); process.exitCode = 1; }
}
