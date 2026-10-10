import { describe, expect, it, vi } from 'vitest';
import { NOTEBOOK_ACTIONS, NOTEBOOK_MODULE_ID } from './types';
import { NotebookClient, NotebookError, newOperationKey, type NotebookActionTransport } from './api';

function transport(result: unknown = { status: 'ok', data: {} }): NotebookActionTransport & { readonly invoke: ReturnType<typeof vi.fn> } {
  return { invoke: vi.fn().mockResolvedValue(result) };
}

describe('NotebookClient wire shape', () => {
  it('routes reads to the sealed vivy/notebook-core actions', async () => {
    const rpc = transport({ status: 'ok', data: { sections: [], next_cursor: '' } });
    const client = new NotebookClient(rpc);

    await client.listSections({ limit: 100 });
    await client.getEntry({ id: 'entry-1' });
    await client.listRevisions({ entry_id: 'entry-1' });
    await client.listComments({ entry_id: 'entry-1', status: 'active' });
    await client.exportEntry({ entry_id: 'entry-1', revision_id: 'rev-9' });

    expect(rpc.invoke).toHaveBeenNthCalledWith(1, {
      moduleId: NOTEBOOK_MODULE_ID,
      actionId: NOTEBOOK_ACTIONS.sectionsList,
      input: { limit: 100 },
    });
    expect(rpc.invoke).toHaveBeenNthCalledWith(2, {
      moduleId: NOTEBOOK_MODULE_ID,
      actionId: NOTEBOOK_ACTIONS.entriesGet,
      input: { id: 'entry-1' },
    });
    expect(rpc.invoke).toHaveBeenNthCalledWith(3, {
      moduleId: NOTEBOOK_MODULE_ID,
      actionId: NOTEBOOK_ACTIONS.revisionsList,
      input: { entry_id: 'entry-1' },
    });
    expect(rpc.invoke).toHaveBeenNthCalledWith(4, {
      moduleId: NOTEBOOK_MODULE_ID,
      actionId: NOTEBOOK_ACTIONS.commentsList,
      input: { entry_id: 'entry-1', status: 'active' },
    });
    expect(rpc.invoke).toHaveBeenNthCalledWith(5, {
      moduleId: NOTEBOOK_MODULE_ID,
      actionId: NOTEBOOK_ACTIONS.export,
      input: { entry_id: 'entry-1', revision_id: 'rev-9' },
    });
  });

  it('wraps mutations in the {operation_key, request} envelope and sends caller-owned keys', async () => {
    const rpc = transport({ status: 'ok', data: { resource_id: 'entry-1', version: 3, revision_id: 'rev-3', replayed: false } });
    const client = new NotebookClient(rpc);
    const key = newOperationKey();

    await client.createSection({ operationKey: key, title: 'Field notes' });
    await client.saveEntry({
      operationKey: 'op-save-1',
      entry_id: 'entry-1',
      expected_version: 2,
      base_revision_id: 'rev-2',
      title: 'Doc',
      markdown: '# body',
    });

    expect(rpc.invoke).toHaveBeenNthCalledWith(1, {
      moduleId: NOTEBOOK_MODULE_ID,
      actionId: NOTEBOOK_ACTIONS.sectionsCreate,
      input: { operation_key: key, request: { title: 'Field notes' } },
    });
    expect(rpc.invoke).toHaveBeenNthCalledWith(2, {
      moduleId: NOTEBOOK_MODULE_ID,
      actionId: NOTEBOOK_ACTIONS.entriesSave,
      input: {
        operation_key: 'op-save-1',
        request: {
          entry_id: 'entry-1',
          expected_version: 2,
          base_revision_id: 'rev-2',
          title: 'Doc',
          markdown: '# body',
        },
      },
    });
  });

  it('decodes a revision conflict into a typed error carrying current-version metadata', async () => {
    const rpc = transport({
      status: 'error',
      error: { code: 'revision_conflict', message: 'stale', retryable: false, current_version: 5, current_revision_id: 'rev-5' },
    });
    const client = new NotebookClient(rpc);

    const failure = await client.saveEntry({
      operationKey: 'op-save-1', entry_id: 'entry-1', expected_version: 2,
      base_revision_id: 'rev-2', title: 'Doc', markdown: '# body',
    }).then(
      () => { throw new Error('must reject'); },
      (cause) => cause,
    );

    expect(failure).toBeInstanceOf(NotebookError);
    expect(failure.code).toBe('revision_conflict');
    expect(failure.retryable).toBe(false);
    expect(failure.currentVersion).toBe(5);
    expect(failure.currentRevisionId).toBe('rev-5');
  });

  it('carries no client-supplied scope, actor, origin or provenance fields on the wire', async () => {
    const rpc = transport({ status: 'ok', data: { resource_id: 'x', version: 1, replayed: false } });
    const client = new NotebookClient(rpc);

    await client.createEntry({ operationKey: 'op-1', section_id: 'section-notes', title: 't', markdown: 'm' });
    await client.updateComment({ operationKey: 'op-2', comment_id: 'c-1', expected_version: 1, status: 'resolved' });
    await client.adoptRevision({ operationKey: 'op-3', entry_id: 'e', revision_id: 'r', expected_version: 4 });

    const forbidden = /scope|actor|origin|generated|provenance|source_id/i;
    for (const [request] of rpc.invoke.mock.calls as Array<[never]>) {
      const serialized = JSON.stringify(request);
      expect(serialized.match(forbidden)).toBeNull();
    }
  });

  it('returns receipt data unchanged, including the replayed flag on same-key retries', async () => {
    const rpc = transport({ status: 'ok', data: { resource_id: 'entry-1', version: 3, revision_id: 'rev-3', replayed: true } });
    const client = new NotebookClient(rpc);

    const receipt = await client.deleteEntry({ operationKey: 'op-del', entry_id: 'entry-1', expected_version: 3 });
    expect(receipt).toEqual({ resource_id: 'entry-1', version: 3, revision_id: 'rev-3', replayed: true });
  });

  it('rejects transport failures and malformed envelopes as unknown outcomes, not data', async () => {
    const rpc = transport({ unexpected: true });
    const client = new NotebookClient(rpc);
    await expect(client.listSections()).rejects.toBeInstanceOf(NotebookError);
    await expect(client.listSections()).rejects.toMatchObject({ code: 'outcome_unknown' });
  });
});
