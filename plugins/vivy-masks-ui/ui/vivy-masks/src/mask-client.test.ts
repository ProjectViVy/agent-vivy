import { describe, expect, it, vi } from 'vitest';
import {
  MASK_ACTIONS,
  MASK_MODULE_ID,
  MaskClient,
  type MaskActionTransport,
  type MaskDefinition,
  type MaskSelection,
} from './mask-client';

function transport(result: unknown = undefined): MaskActionTransport & { readonly invoke: ReturnType<typeof vi.fn> } {
  return { invoke: vi.fn().mockResolvedValue(result) };
}

describe('MaskClient', () => {
  it('maps list and lazy get calls to the fixed vivy/masks action owner', async () => {
    const rpc = transport({ items: [], next_after_id: '' });
    const client = new MaskClient(rpc);

    await client.list({ after_id: 'builtin/programmer', limit: 25 });
    await client.get('custom/one');

    expect(rpc.invoke).toHaveBeenNthCalledWith(1, {
      moduleId: MASK_MODULE_ID,
      actionId: MASK_ACTIONS.catalogList,
      input: { after_id: 'builtin/programmer', limit: 25 },
    });
    expect(rpc.invoke).toHaveBeenNthCalledWith(2, {
      moduleId: MASK_MODULE_ID,
      actionId: MASK_ACTIONS.catalogGet,
      input: { id: 'custom/one' },
    });
  });

  it('keeps create operation ids caller-owned for safe retry after an ambiguous response', async () => {
    const definition: MaskDefinition = {
      id: 'custom/one', name: 'One', description: '', body: 'body', revision: 1,
      digest: 'a'.repeat(64), built_in: false, generation_id: 'generation',
    };
    const rpc = transport(definition);
    const client = new MaskClient(rpc);
    const request = { operation_id: '00000000-0000-4000-8000-000000000001', name: 'One', description: '', body: 'body' };

    await client.create(request);
    await client.create(request);

    expect(rpc.invoke).toHaveBeenNthCalledWith(1, {
      moduleId: MASK_MODULE_ID,
      actionId: MASK_ACTIONS.catalogCreate,
      input: request,
    });
    expect(rpc.invoke).toHaveBeenNthCalledWith(2, {
      moduleId: MASK_MODULE_ID,
      actionId: MASK_ACTIONS.catalogCreate,
      input: request,
    });
  });

  it('sends explicit revisions for selection CAS and custom mutations', async () => {
    const selection: MaskSelection = {
      session_id: 'session-b', mask_id: 'builtin/writer', revision: 7, available: true, inactive_reason: '',
    };
    const rpc = transport(selection);
    const client = new MaskClient(rpc);

    await client.setSelection({ session_id: 'session-b', mask_id: 'custom/one', expected_revision: 7 });
    await client.update({ id: 'custom/one', expected_revision: 3, name: 'New', description: '', body: 'new body' });
    await client.remove({ id: 'custom/one', expected_revision: 4 });

    expect(rpc.invoke).toHaveBeenNthCalledWith(1, {
      moduleId: MASK_MODULE_ID,
      actionId: MASK_ACTIONS.selectionSet,
      input: { session_id: 'session-b', mask_id: 'custom/one', expected_revision: 7 },
    });
    expect(rpc.invoke).toHaveBeenNthCalledWith(2, {
      moduleId: MASK_MODULE_ID,
      actionId: MASK_ACTIONS.catalogUpdate,
      input: { id: 'custom/one', expected_revision: 3, name: 'New', description: '', body: 'new body' },
    });
    expect(rpc.invoke).toHaveBeenNthCalledWith(3, {
      moduleId: MASK_MODULE_ID,
      actionId: MASK_ACTIONS.catalogDelete,
      input: { id: 'custom/one', expected_revision: 4 },
    });
  });
});
