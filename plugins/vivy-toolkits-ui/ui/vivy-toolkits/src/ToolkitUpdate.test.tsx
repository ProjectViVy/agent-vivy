// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ToolkitPage } from './ToolkitPage';
import type { ToolkitClient, ToolkitInstallation, ToolkitInspection, ToolkitOperation, ToolkitPageResult } from './toolkit-client';

const install: ToolkitInstallation = {
  install_id: '10000000-0000-4000-8000-000000000001', bundle_id: 'example/toolkit', state: 'enabled',
  source: { kind: 'local', origin: '/host/toolkit' }, release_digest: 'a'.repeat(64), revision: 2, tombstone: false,
};
const catalog = { epoch: 4, config_revision: 3, effective_digest: 'c'.repeat(64), generation_id: 'generation' };
const page: ToolkitPageResult = { items: [install], next_after_id: '', catalog, capabilities: { owners: { skill: true, mask: true, rule: true, hook: true, mcp: true } } };
const baseInspection: ToolkitInspection = {
  installation: install,
  release: { install_id: install.install_id, bundle_id: install.bundle_id, release_digest: install.release_digest, release_sequence: 2 },
  releases: [
    { install_id: install.install_id, bundle_id: install.bundle_id, release_digest: 'a'.repeat(64), release_sequence: 2 },
    { install_id: install.install_id, bundle_id: install.bundle_id, release_digest: 'b'.repeat(64), release_sequence: 1 },
  ],
  manifest: { id: install.bundle_id, version: '2', name: 'Example Toolkit', components: {
    skills: [], masks: [], rules: [], hooks: [{ id: 'lint', event: 'PreToolUse', command: 'lint.sh', cwd: '.', timeoutMs: 5000, envFrom: { token: 'DECLARED_DEFAULT' } }],
    mcp: [{ id: 'docs', endpoint: 'https://example.invalid/mcp', args: [], authEnv: 'DECLARED_AUTH', envFrom: {}, resourceBridge: false }],
  }, resources: [] },
  consents: [{ kind: 'hook', local_id: 'lint', declaration_digest: 'd'.repeat(64), allowed: true, revision: 2, subject: 'human' }],
  executables: [
    { kind: 'hook', local_id: 'lint', release_digest: install.release_digest, declaration_digest: 'd'.repeat(64), consent_current: false, consent_allowed: true, consent_revision: 2, missing_bindings: ['envFrom:token'] },
    { kind: 'mcp', local_id: 'docs', release_digest: install.release_digest, declaration_digest: 'e'.repeat(64), consent_current: false, consent_allowed: false, consent_revision: 0, missing_bindings: ['authEnv'] },
  ],
  bindings: [],
};

describe('Toolkit updates and bindings', () => {
  let container: HTMLDivElement;
  let root: Root;
  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    container = document.createElement('div'); document.body.append(container); root = createRoot(container);
  });
  afterEach(async () => { await act(async () => root.unmount()); container.remove(); vi.restoreAllMocks(); });

  it('shows component digests and fresh-consent requirements before update publication', async () => {
    const operation: ToolkitOperation = {
      id: 'operation-update', kind: 'update', state: 'pending', revision: 1, expected_epoch: 4, committed_epoch: 0,
      retryable: false, blockers: [], prepared: {
        input_digest: '1'.repeat(64), expected_catalog: catalog, candidate_digest: '2'.repeat(64),
        releases: [], new_releases: [], installations: [], mutation_kind: 'update', mutation_install_id: install.install_id,
        changes: [
          { kind: 'hook', local_id: 'lint', change: 'changed', before_digest: '3'.repeat(64), after_digest: '4'.repeat(64), requires_fresh_consent: true },
          { kind: 'mask', local_id: 'pm', change: 'removed' },
        ],
      },
    };
    const client = makeClient({ prepareSource: vi.fn().mockResolvedValue({ operation, candidate_digest: '2'.repeat(64), expected_catalog: catalog, releases: [] }) });
    await selectInstalled(client, container, root);
    const update = container.querySelector<HTMLButtonElement>('[data-action="prepare-update"]');
    expect(update).not.toBeNull();
    await act(async () => { update!.click(); await Promise.resolve(); });
    expect(client.prepareSource).toHaveBeenCalledWith('local', 'update', expect.objectContaining({ install_id: install.install_id, location: '/host/toolkit', expected_install_revision: 2 }));
    expect(container.querySelector('[data-action="publish-candidate"]')).not.toBeNull();
    // The diff row keeps its machine verb in data and its catalog key as text,
    // so the change is translatable without a copy of the label in code.
    const change = container.querySelector('[data-testid="prepared-candidate"] [data-change]');
    expect(change?.getAttribute('data-change')).toBe('changed');
    expect(change?.textContent).toContain('hook/lint');
    expect(change?.textContent).toContain('plugin.vivy/toolkits-ui.changeChanged');
    expect(container.textContent).toContain('plugin.vivy/toolkits-ui.freshConsent');
    // A candidate that removes a Mask must point the user at the Masks page:
    // the backend reports the singular component kind, not the section name.
    const maskChange = container.querySelector('[data-testid="prepared-candidate"] [data-change="removed"]');
    expect(maskChange?.textContent).toContain('plugin.vivy/toolkits-ui.maskRemovalHint');
    expect(maskChange?.querySelector('a[href="/masks"]')).not.toBeNull();
  });

  it('prepares rollback only from a retained prior release', async () => {
    const client = makeClient({ prepare: vi.fn().mockResolvedValue({ operation: pendingOperation('rollback'), candidate_digest: '5'.repeat(64), expected_catalog: catalog, releases: [] }) });
    await selectInstalled(client, container, root);
    const rollback = container.querySelector<HTMLButtonElement>('[data-action="prepare-rollback"]');
    expect(rollback).not.toBeNull();
    expect(rollback?.disabled).toBe(false);
    await act(async () => { rollback!.click(); await Promise.resolve(); });
    expect(client.prepare).toHaveBeenCalledWith('rollback', expect.objectContaining({ release_digest: 'b'.repeat(64), install_id: install.install_id }));
  });

  it('recovers a committed update after a lost publish response and reloads its details', async () => {
    const operation = pendingOperation('update');
    const nextInstall = { ...install, release_digest: 'f'.repeat(64), revision: 3 };
    const nextInspection: ToolkitInspection = {
      ...baseInspection,
      installation: nextInstall,
      release: { ...baseInspection.release, release_digest: nextInstall.release_digest, release_sequence: 3 },
    };
    const client = makeClient({
      prepareSource: vi.fn().mockResolvedValue({ operation, candidate_digest: '5'.repeat(64), expected_catalog: catalog, releases: [] }),
      publish: vi.fn().mockRejectedValue(new Error('response lost')),
      operation: vi.fn().mockResolvedValue({ ...operation, state: 'committed', committed_epoch: 5 }),
      inspect: vi.fn().mockResolvedValueOnce(baseInspection).mockResolvedValue(nextInspection),
    });
    await selectInstalled(client, container, root);
    const update = container.querySelector<HTMLButtonElement>('[data-action="prepare-update"]');
    await act(async () => { update!.click(); await Promise.resolve(); });
    const publish = container.querySelector<HTMLButtonElement>('[data-action="publish-candidate"]');
    await act(async () => { publish!.click(); await Promise.resolve(); await Promise.resolve(); });
    expect(client.operation).toHaveBeenCalledWith('operation');
    expect(client.inspect).toHaveBeenCalledTimes(2);
    expect(container.textContent).toContain('f'.repeat(12));
    expect(container.textContent).toContain('plugin.vivy/toolkits-ui.catalogUpdated');
  });

  it('shows stale consent and saves only host environment variable references', async () => {
    const save = vi.fn().mockRejectedValue(new Error('revision conflict'));
    const client = makeClient({ setBindings: save, operation: vi.fn().mockRejectedValue(new Error('not found')) });
    await selectInstalled(client, container, root);
    expect(container.textContent).toContain('plugin.vivy/toolkits-ui.consentStale');
    expect(container.textContent).toContain('plugin.vivy/toolkits-ui.reauthorizeConsent');
    expect(container.textContent).toContain('plugin.vivy/toolkits-ui.missingBindings');
    const hookRow = container.querySelector('[data-testid="toolkit-executable"][data-component="hook/lint"]');
    const input = container.querySelector<HTMLInputElement>('#binding-hook-lint-envFrom-token');
    expect(input).not.toBeNull();
    await act(async () => {
      const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set;
      setter?.call(input, 'VIVY_LINT_TOKEN');
      input!.dispatchEvent(new Event('input', { bubbles: true }));
    });
    const saveButton = hookRow?.querySelector<HTMLButtonElement>('[data-action="save-bindings"]');
    await act(async () => { saveButton!.click(); await Promise.resolve(); await Promise.resolve(); });
    expect(save).toHaveBeenCalledWith(expect.objectContaining({
      operation_id: expect.any(String), expected_config_revision: 3,
      bindings: [{ name: 'envFrom:token', reference: 'VIVY_LINT_TOKEN' }],
    }));
    expect(input!.value).toBe('VIVY_LINT_TOKEN');
    expect(container.textContent).not.toContain('DECLARED_DEFAULT');
    expect(container.textContent).not.toContain('DECLARED_AUTH');
    expect(container.textContent).toContain('plugin.vivy/toolkits-ui.bindingsConflict');
  });
});

async function selectInstalled(client: ToolkitClient, container: HTMLDivElement, root: Root) {
  await act(async () => root.render(<ToolkitPage client={client} />));
  const button = container.querySelector<HTMLButtonElement>('ul button');
  await act(async () => { button?.click(); await Promise.resolve(); });
}

function makeClient(overrides: Partial<ToolkitClient> = {}): ToolkitClient {
  return {
    list: vi.fn().mockResolvedValue(page),
    inspect: vi.fn().mockResolvedValue(baseInspection),
    operation: vi.fn(),
    prepareSource: vi.fn(),
    prepare: vi.fn(),
    setBindings: vi.fn(),
    ...overrides,
  } as unknown as ToolkitClient;
}

function pendingOperation(kind: string): ToolkitOperation {
  return { id: 'operation', kind, state: 'pending', revision: 1, expected_epoch: 4, committed_epoch: 0, retryable: false, blockers: [] };
}
