import assert from 'node:assert/strict';
import test from 'node:test';
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';
import { selectScopes, changedFiles, fullRequested, aggregateResult, validateRecipe, makePlan, command, goGroups, recipeSessions, recipeNeedsUI } from './ci-small.mjs';

const scopes = [
  { id: 'docs', paths: ['\\.md$'], checks: ['diff'] },
  { id: 'runtime', paths: ['^internal/runtime/.*\\.go$'], checks: ['kernel'] },
  { id: 'ui', paths: ['^ui/.*\\.tsx?$'], recipe: 'recipes/minimal-chat.vivy.yml', checks: ['ui'] },
];
test('selection unions scopes and explicit scopes can only add', () => {
  assert.deepEqual(selectScopes(['internal/runtime/a.go', 'ui/a.ts', 'docs/a.md'], scopes, ['ui']).map(s => s.id), ['docs', 'runtime', 'ui']);
  assert.deepEqual(selectScopes(['docs/a.md'], scopes, ['runtime']).map(s => s.id), ['docs', 'runtime']);
});
test('unknown executable paths and unknown explicit scopes fail', () => {
  assert.throws(() => selectScopes(['new-owner/code.go'], scopes), /Unmapped/);
  assert.throws(() => selectScopes([], scopes, ['typo']), /Unknown scope/);
});
test('unknown data/build inputs also fail rather than silently becoming docs', () => {
  for (const file of ['unknown/build.conf', 'unknown/Makefile', 'assets/new.bin']) {
    assert.throws(() => selectScopes([file], scopes), /Unmapped/);
  }
});
test('only instruction documents select docs', () => {
  assert.deepEqual(selectScopes(['AGENTS.md'], scopes).map(s => s.id), ['docs']);
  assert.deepEqual(selectScopes(['ui/README.md'], scopes).map(s => s.id), ['docs']);
});
test('command failure is propagated', () => {
  assert.throws(() => command(process.execPath, ['-e', 'process.exit(7)']), /failed/);
});
test('actual association table selects owners and Recipes without broadening docs', () => {
  const root = new URL('../', import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, '$1');
  const plan = files => makePlan(root, 'HEAD', [], files);
  assert.deepEqual(plan(['plugins/qq/README.md']).checks, ['diff']);
  assert.ok(plan(['internal/runtime/engine.go']).checks.includes('kernel'));
  assert.ok(plan(['internal/rpc/control.go']).checks.includes('protocol'));
  assert.ok(plan(['internal/storage/migrations/sqlite/039.sql']).checks.includes('postgres'));
  assert.ok(plan(['internal/runtime/tool_operation.go']).checks.includes('race'));
  assert.ok(plan(['go.mod']).checks.includes('wiring'));
  assert.ok(plan(['ui/src/lib/store.ts']).recipes.includes('recipes/minimal-chat.vivy.yml'));
  assert.ok(plan(['plugins/hello-fs/plugin.go']).recipes.includes('examples/minimal-plus-hello-fs/recipe.vivy.yml'));
  assert.ok(plan(['plugins/vivy-evolution/ui/vivy-evolution/src/view.tsx']).recipes.includes('recipes/default.vivy.yml'));
  assert.ok(goGroups(plan(['plugins/discord/go.mod']), root).size > 0);
  assert.ok(recipeNeedsUI(root, 'recipes/default.vivy.yml'));
  assert.equal(recipeNeedsUI(root, 'recipes/minimal.vivy.yml'), false);
  for (const recipe of ['headless', 'vivy-code', 'scx', 'lite']) assert.equal(recipeNeedsUI(root, `recipes/${recipe}.vivy.yml`), true);
  const persona = plan(['plugins/vivy-persona/ui/vivy-persona/src/view.tsx']);
  assert.ok(persona.scopes.flatMap(s => s.uiTests || []).some(t => t.startsWith('../plugins/vivy-persona/')));
  const settings = plan(['ui/src/components/settings/ModelSettingsCard.tsx']);
  assert.ok(recipeSessions(settings)[0].uiSources.includes('src/components/settings/ModelSettingsCard.tsx'));
  assert.ok(recipeSessions(settings)[0].uiTests.includes('@related'));
  const mixed = plan(['internal/runtime/engine.go', 'ui/src/lib/store.ts']);
  assert.ok(!mixed.commands.some(c => c.startsWith('ui: recipes/minimal.vivy.yml')));
});
test('scope Recipe sessions do not cross-product backend and UI checks', () => {
  const sessions = recipeSessions({ scopes: [
    { recipe: 'recipes/minimal.vivy.yml', checks: ['recipe'] },
    { recipe: 'recipes/minimal-chat.vivy.yml', checks: ['ui', 'browser'], uiTests: ['chat.test.ts'] },
  ], files: [] });
  assert.deepEqual(sessions[0].checks, ['recipe']);
  assert.deepEqual(sessions[0].uiTests, []);
  assert.deepEqual(sessions[1].checks, ['ui', 'browser']);
});
test('Go ownership rejects traversal and empty required package selection', () => {
  const root = new URL('../', import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, '$1');
  assert.throws(() => goGroups({ scopes: [{ goPackages: ['./../../../outside'], reasons: [] }] }, root), /outside|package/);
  assert.throws(() => goGroups({ scopes: [{ goPackages: ['@changed'], reasons: ['deleted-owner/no.go'] }] }, root), /No Go/);
});
test('full CI requires the final label or explicit manual request', () => {
  assert.equal(fullRequested('pull_request', { pull_request: { labels: [] } }), false);
  for (const action of ['labeled', 'synchronize', 'reopened']) {
    assert.equal(fullRequested('pull_request', { action, pull_request: { labels: [{ name: 'epic-final' }] } }), true);
  }
  assert.equal(fullRequested('pull_request', { action: 'unlabeled', pull_request: { labels: [] } }), false);
  assert.equal(fullRequested('push', {}), false);
  assert.equal(fullRequested('workflow_dispatch', { inputs: { full: 'true' } }), true);
  assert.equal(fullRequested('workflow_dispatch', { inputs: { full: 'false' } }), false);
});
test('aggregate checks every required lane and does not borrow skipped success', () => {
  assert.equal(aggregateResult(false, 'success', ['skipped', 'skipped']), true);
  assert.equal(aggregateResult(true, 'success', ['success', 'success']), true);
  for (const result of ['failure', 'cancelled', 'skipped']) {
    assert.equal(aggregateResult(true, 'success', ['success', result]), false);
    assert.equal(aggregateResult(false, result, []), false);
  }
});
test('actual aggregate CLI reports not requested and fails cancelled full lanes', () => {
  const cli = fileURLToPath(new URL('./ci-small.mjs', import.meta.url));
  const env = { ...process.env, PLAN_RESULT: 'success', SMALL_RESULT: 'success', FULL: 'false', GITHUB_STEP_SUMMARY: '' };
  assert.match(command(process.execPath, [cli, '--aggregate'], { env }), /not requested \(not verified\)/);
  assert.throws(() => command(process.execPath, [cli, '--aggregate'], { env: { ...env, FULL: 'true', BACKEND_RESULT: 'success', UI_RESULT: 'cancelled', BROWSER_RESULT: 'success' } }), /failed/);
});
test('git discovery includes deletes, both rename paths and untracked source', () => {
  const root = mkdtempSync(join(tmpdir(), 'vivy-ci-git-'));
  const git = (...args) => execFileSync('git', args, { cwd: root, encoding: 'utf8' });
  try {
    git('init'); git('config', 'user.name', 'CI Fixture'); git('config', 'user.email', 'ci@example.invalid');
    writeFileSync(join(root, 'old.go'), 'package fixture\n');
    writeFileSync(join(root, 'deleted.go'), 'package deleted\n');
    git('add', '.'); git('commit', '-m', 'fixture');
    git('mv', 'old.go', 'new.go'); rmSync(join(root, 'deleted.go'));
    writeFileSync(join(root, 'untracked.go'), 'package untracked\n');
    assert.deepEqual(new Set(changedFiles(root, 'HEAD')), new Set(['old.go', 'new.go', 'deleted.go', 'untracked.go']));
  } finally { rmSync(root, { recursive: true, force: true }); }
});
test('Recipe must exist inside the checkout and have the generation API', () => {
  const root = mkdtempSync(join(tmpdir(), 'vivy-ci-recipe-'));
  try {
    mkdirSync(join(root, 'recipes'));
    writeFileSync(join(root, 'recipes', 'valid.yml'), 'apiVersion: vivy.generation/v1\nmodules: []\n');
    assert.equal(validateRecipe(root, 'recipes/valid.yml'), 'recipes/valid.yml');
    assert.throws(() => validateRecipe(root, '../outside.yml'), /Recipe/);
    assert.throws(() => validateRecipe(root, 'recipes/missing.yml'), /Recipe/);
    writeFileSync(join(root, 'recipes', 'wrong.yml'), 'apiVersion: other/v1\n');
    assert.throws(() => validateRecipe(root, 'recipes/wrong.yml'), /Recipe/);
  } finally { rmSync(root, { recursive: true, force: true }); }
});
