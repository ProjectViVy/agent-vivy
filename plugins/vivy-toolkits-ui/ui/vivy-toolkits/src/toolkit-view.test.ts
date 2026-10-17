// @vitest-environment happy-dom
import { describe, expect, it } from 'vitest';
import type { UITranslator } from '@vivy/ui-sdk';
import type { PreparedCandidate, ToolkitInstallation, ToolkitInspection } from './toolkit-client';
import {
  COMPONENT_KINDS,
  candidateTarget,
  changeVerbLabel,
  componentCount,
  componentKindLabel,
  componentKindOf,
  emptyKindLabel,
  installationStateLabel,
  installationStateTone,
  mcpRuntimeLabel,
  mutationLabel,
  operationStateLabel,
  ownerKey,
  shortDigest,
  sourceKindLabel,
  sourceKindOf,
} from './toolkit-view';

const t: UITranslator = (key: string) => key;
const key = (name: string) => `plugin.vivy/toolkits-ui.${name}`;

const install: ToolkitInstallation = {
  install_id: '10000000-0000-4000-8000-000000000001',
  bundle_id: 'example/toolkit',
  state: 'enabled',
  source: { kind: 'git', origin: 'https://example.invalid/toolkit' },
  release_digest: 'a'.repeat(64),
  revision: 2,
  tombstone: false,
};

describe('toolkit-view', () => {
  it('translates every installation state and keeps an unknown token visible', () => {
    expect(installationStateLabel(t, 'enabled')).toBe(key('stateEnabled'));
    expect(installationStateLabel(t, 'disabled')).toBe(key('stateDisabled'));
    expect(installationStateLabel(t, 'orphaned')).toBe(key('stateOrphaned'));
    expect(installationStateLabel(t, 'mystery')).toBe(key('stateUnknown'));
    expect(installationStateTone('enabled')).toBe('default');
    expect(installationStateTone('orphaned')).toBe('destructive');
    expect(installationStateTone('disabled')).toBe('secondary');
    expect(installationStateTone('mystery')).toBe('outline');
  });

  it('translates operation states, change verbs and mutation kinds', () => {
    for (const [state, name] of [
      ['pending', 'operationStatePreparing'],
      ['committed', 'operationStatePublished'],
      ['cancelled', 'operationStateCancelled'],
      ['failed', 'operationStateFailed'],
    ] as const) {
      expect(operationStateLabel(t, state)).toBe(key(name));
    }
    expect(changeVerbLabel(t, 'added')).toBe(key('changeAdded'));
    expect(changeVerbLabel(t, 'removed')).toBe(key('changeRemoved'));
    expect(changeVerbLabel(t, 'changed')).toBe(key('changeChanged'));
    expect(changeVerbLabel(t, 'rotated')).toBe('rotated');
    for (const kind of ['install', 'enable', 'disable', 'uninstall', 'update', 'rebind', 'rollback']) {
      expect(mutationLabel(t, kind)).toBe(key(`mutation${kind[0].toUpperCase()}${kind.slice(1)}`));
    }
    expect(mutationLabel(t, 'unknown-kind')).toBe('unknown-kind');
  });

  it('translates source kinds and MCP runtime states', () => {
    expect(sourceKindOf('git')).toBe('git');
    expect(sourceKindOf('local')).toBe('local');
    expect(sourceKindOf('carrier-pigeon')).toBe('local');
    expect(sourceKindLabel(t, 'git')).toBe(key('gitSource'));
    expect(sourceKindLabel(t, 'local')).toBe(key('localSource'));
    for (const state of ['not-compiled', 'unconfigured', 'inactive', 'ready', 'unavailable', 'deferred']) {
      expect(mcpRuntimeLabel(t, state)).not.toBe(key('mcpUnknown'));
      expect(mcpRuntimeLabel(t, state)).toBe(key(`mcp${state.split('-').map((part) => part[0].toUpperCase() + part.slice(1)).join('')}`));
    }
    expect(mcpRuntimeLabel(t, 'mystery')).toBe(key('mcpUnknown'));
  });

  it('labels each component kind, its empty message and its owner capability key', () => {
    expect(COMPONENT_KINDS).toEqual(['skills', 'masks', 'rules', 'hooks', 'mcp']);
    expect(componentKindLabel(t, 'skills')).toBe(key('skills'));
    expect(componentKindLabel(t, 'masks')).toBe(key('masks'));
    expect(componentKindLabel(t, 'rules')).toBe(key('rules'));
    expect(componentKindLabel(t, 'hooks')).toBe(key('hooks'));
    expect(componentKindLabel(t, 'mcp')).toBe(key('mcp'));
    expect(emptyKindLabel(t, 'mcp')).toBe(key('emptyMcp'));
    expect(ownerKey('skills')).toBe('skill');
    expect(ownerKey('masks')).toBe('mask');
    expect(ownerKey('rules')).toBe('rule');
    expect(ownerKey('hooks')).toBe('hook');
    expect(ownerKey('mcp')).toBe('mcp');
    expect(componentKindOf('skill')).toBe('skills');
    expect(componentKindOf('mask')).toBe('masks');
    expect(componentKindOf('rule')).toBe('rules');
    expect(componentKindOf('hook')).toBe('hooks');
    expect(componentKindOf('mcp')).toBe('mcp');
    expect(componentKindOf('skills')).toBeNull();
    expect(
      componentCount(
        {
          id: 'example/toolkit',
          version: '1',
          name: 'Example Toolkit',
          components: { skills: [{ id: 'pm', path: 'skills/pm/SKILL.md' }], masks: [], rules: [], hooks: [], mcp: [] },
          resources: [],
        },
        'skills',
      ),
    ).toBe(1);
  });

  it('abbreviates digests for display without dropping short values', () => {
    expect(shortDigest('a'.repeat(64))).toBe('a'.repeat(12));
    expect(shortDigest('a'.repeat(64), 8)).toBe('a'.repeat(8));
    expect(shortDigest('abcdef')).toBe('abcdef');
  });

  it('names the candidate target from the selected inspection or the catalog', () => {
    const candidate = makeCandidate('update', install.install_id);
    const inspection = {
      ...makeInspection(),
      manifest: { ...makeInspection().manifest!, name: 'Example Toolkit' },
    } as ToolkitInspection;
    expect(candidateTarget(candidate, [install], inspection)).toEqual({
      mutationKind: 'update',
      installId: install.install_id,
      name: 'Example Toolkit',
    });
    expect(candidateTarget(candidate, [install], null)).toEqual({
      mutationKind: 'update',
      installId: install.install_id,
      name: 'example/toolkit',
    });
    const unknown = makeCandidate('enable', '30000000-0000-4000-8000-000000000003');
    expect(candidateTarget(unknown, [install], null)).toEqual({
      mutationKind: 'enable',
      installId: '30000000-0000-4000-8000-000000000003',
      name: '',
    });
  });
});

function makeCandidate(mutationKind: string, installId: string): PreparedCandidate {
  return {
    operation: {
      id: 'operation',
      kind: mutationKind,
      state: 'pending',
      revision: 1,
      expected_epoch: 1,
      committed_epoch: 0,
      retryable: false,
      blockers: [],
      prepared: {
        input_digest: '1'.repeat(64),
        expected_catalog: { epoch: 1, config_revision: 0, effective_digest: '2'.repeat(64), generation_id: 'generation' },
        candidate_digest: '3'.repeat(64),
        releases: [],
        new_releases: [],
        installations: [],
        mutation_kind: mutationKind,
        mutation_install_id: installId,
        changes: [],
      },
    },
    candidate_digest: '3'.repeat(64),
    expected_catalog: { epoch: 1, config_revision: 0, effective_digest: '2'.repeat(64), generation_id: 'generation' },
    releases: [],
  };
}

function makeInspection(): ToolkitInspection {
  return {
    installation: install,
    release: { install_id: install.install_id, bundle_id: install.bundle_id, release_digest: install.release_digest, release_sequence: 1 },
    releases: [],
    manifest: { id: install.bundle_id, version: '1', name: 'Example Toolkit', components: { skills: [], masks: [], rules: [], hooks: [], mcp: [] }, resources: [] },
    consents: [],
    executables: [],
    bindings: [],
  };
}
