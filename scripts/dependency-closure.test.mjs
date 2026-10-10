import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');

test('all Laputa Go revisions match the full source lock', () => {
  const lock = JSON.parse(readFileSync(resolve(root, 'laputa-source.lock.json'), 'utf8'));
  const mod = readFileSync(resolve(root, 'go.mod'), 'utf8');
  for (const name of ['garden', 'laputa', 'mentle']) {
    const version = mod.match(new RegExp(`github.com/ProjectViVy/laputa/${name} (v0\\.0\\.0-\\d{14}-[a-f0-9]{12})`))?.[1];
    assert.ok(version, `missing Laputa ${name} pseudo-version`);
    assert.equal(version.split('-').at(-1), lock.commit.slice(0, 12));
  }
});

test('default catalog imports optional metadata without concrete implementations', () => {
  const source = readFileSync(resolve(root, 'internal/modules/defaults/catalog.go'), 'utf8').split(')')[0];
  for (const name of ['diva-cognitive', 'memory', 'masks']) {
    assert.doesNotMatch(source, new RegExp(`"agent-vivy/internal/modules/${name}"`), name);
  }
});
