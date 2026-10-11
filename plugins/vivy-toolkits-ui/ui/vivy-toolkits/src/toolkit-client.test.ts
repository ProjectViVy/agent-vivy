import { describe, expect, it, vi } from 'vitest';
import { TOOLKIT_ACTIONS, TOOLKIT_MODULE_ID, ToolkitClient } from './toolkit-client';

describe('ToolkitClient', () => {
  it('uses the fixed authenticated Toolkits owner and lowercase source-specific actions', async () => {
    const invoke = vi.fn().mockResolvedValue({});
    const client = new ToolkitClient({ invoke });
    const catalog = { epoch: 3, config_revision: 0, effective_digest: 'a'.repeat(64), generation_id: 'gen' };
    await client.preview('local', { operation_id: 'op', expected_catalog: catalog, location: '/host/toolkit' });
    await client.preview('git', { operation_id: 'op-git', expected_catalog: catalog, location: 'https://example.invalid/repo', ref: 'main' });
    expect(invoke.mock.calls.map(([request]) => request)).toEqual([
      { moduleId: TOOLKIT_MODULE_ID, actionId: TOOLKIT_ACTIONS.preview, input: { operation_id: 'op', expected_catalog: catalog, location: '/host/toolkit' } },
      { moduleId: TOOLKIT_MODULE_ID, actionId: TOOLKIT_ACTIONS.previewGit, input: { operation_id: 'op-git', expected_catalog: catalog, location: 'https://example.invalid/repo', ref: 'main' } },
    ]);
    expect(TOOLKIT_ACTIONS.previewGit).toBe('vivy.bundles.preview-git');
  });

  it('replays a write with its caller-provided operation ID unchanged', async () => {
    const invoke = vi.fn().mockResolvedValue({ id: 'stable-op', state: 'committed' });
    const client = new ToolkitClient({ invoke });
    const request = { operation_id: 'stable-op', expected_catalog: { epoch: 0, config_revision: 0, effective_digest: '', generation_id: '' }, preview_operation_id: 'preview-op', expected_release_digest: 'a'.repeat(64) };
    await client.install(request);
    await client.install(request);
    expect(invoke).toHaveBeenCalledTimes(2);
    expect(invoke.mock.calls[0]?.[0].input).toEqual(invoke.mock.calls[1]?.[0].input);
    expect(invoke.mock.calls[1]?.[0].input).toHaveProperty('operation_id', 'stable-op');
  });
});
