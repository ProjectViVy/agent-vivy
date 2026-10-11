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
        changes: [{ kind: 'hook', local_id: 'lint', change: 'changed', before_digest: '3'.repeat(64), after_digest: '4'.repeat(64), requires_fresh_consent: true }],
      },
    };
    const client = makeClient({ prepareSource: vi.fn().mockResolvedValue({ operation, candidate_digest: '2'.repeat(64), expected_catalog: catalog, releases: [] }) });
    await selectInstalled(client, container, root);
    const update = [...container.querySelectorAll('button')].find((button) => button.textContent?.includes('准备更新'));
    expect(update).toBeDefined();
    await act(async () => { update!.click(); await Promise.resolve(); });
    expect(client.prepareSource).toHaveBeenCalledWith('local', 'update', expect.objectContaining({ install_id: install.install_id, location: '/host/toolkit', expected_install_revision: 2 }));
    expect(container.textContent).toContain('hook/lint: changed');
    expect(container.textContent).toContain('需要重新确认授权');
    expect(container.textContent).toContain('发布变更');
  });

  it('prepares rollback only from a retained prior release', async () => {
    const client = makeClient({ prepare: vi.fn().mockResolvedValue({ operation: pendingOperation('rollback'), candidate_digest: '5'.repeat(64), expected_catalog: catalog, releases: [] }) });
    await selectInstalled(client, container, root);
    const rollback = [...container.querySelectorAll('button')].find((button) => button.textContent?.includes('准备回滚'));
    expect(rollback).toBeDefined();
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
    const update = [...container.querySelectorAll('button')].find((button) => button.textContent?.includes('准备更新'));
    await act(async () => { update!.click(); await Promise.resolve(); });
    const publish = [...container.querySelectorAll('button')].find((button) => button.textContent?.includes('发布变更'));
    await act(async () => { publish!.click(); await Promise.resolve(); await Promise.resolve(); });
    expect(client.operation).toHaveBeenCalledWith('operation');
    expect(client.inspect).toHaveBeenCalledTimes(2);
    expect(container.textContent).toContain('f'.repeat(12));
    expect(container.textContent).toContain('工具包目录已更新');
  });

  it('shows stale consent and saves only host environment variable references', async () => {
    const save = vi.fn().mockRejectedValue(new Error('revision conflict'));
    const client = makeClient({ setBindings: save, operation: vi.fn().mockRejectedValue(new Error('not found')) });
    await selectInstalled(client, container, root);
    expect(container.textContent).toContain('旧授权已失效，需重新确认');
    expect(container.textContent).toContain('重新确认授权');
    expect(container.textContent).toContain('缺少主机环境变量绑定');
    const input = container.querySelector<HTMLInputElement>('#binding-hook-lint-envFrom-token');
    expect(input).not.toBeNull();
    await act(async () => {
      const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set;
      setter?.call(input, 'VIVY_LINT_TOKEN');
      input!.dispatchEvent(new Event('input', { bubbles: true }));
    });
    const saveButton = [...container.querySelectorAll('button')].find((button) => button.textContent?.includes('保存绑定'));
    await act(async () => { saveButton!.click(); await Promise.resolve(); await Promise.resolve(); });
    expect(save).toHaveBeenCalledWith(expect.objectContaining({
      operation_id: expect.any(String), expected_config_revision: 3,
      bindings: [{ name: 'envFrom:token', reference: 'VIVY_LINT_TOKEN' }],
    }));
    expect(input!.value).toBe('VIVY_LINT_TOKEN');
    expect(container.textContent).not.toContain('DECLARED_DEFAULT');
    expect(container.textContent).not.toContain('DECLARED_AUTH');
    expect(container.textContent).toContain('表单内容已保留');
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
