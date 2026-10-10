import { describe, expect, it, vi } from 'vitest';
import type { FaceClientRPC } from '@vivy/ui-sdk';
import { PersonaClient } from './persona-client';

describe('PersonaClient', () => {
  it('binds every cognitive action to the supplied host session', async () => {
    const rpc = {
      call: vi.fn().mockResolvedValue({
        status: 'ok',
        value: {
          persona: { state: 'ready', current_revisions: { identity: 3 } },
        },
      }),
    } as unknown as FaceClientRPC;
    const client = PersonaClient.fromRPC(rpc);

    await client.status('session-1');

    expect(rpc.call).toHaveBeenCalledWith('module.action.invoke', {
      module_id: 'vivy/diva-cognitive',
      action_id: 'diva.cognitive.status',
      input: { session_id: 'session-1' },
    });
  });

  it('keeps business failures visible instead of fabricating a document', async () => {
    const rpc = {
      call: vi.fn().mockResolvedValue({
        status: 'failed',
        error: { code: 'revision_conflict', message: 'revision changed', retryable: false },
      }),
    } as unknown as FaceClientRPC;
    const client = PersonaClient.fromRPC(rpc);

    const outcome = await client.read('session-1', 'identity');

    expect(outcome).toEqual({
      status: 'failed',
      error: { code: 'revision_conflict', message: 'revision changed', retryable: false },
    });
  });

  it('sends required initialization fields and the document revision for CAS saves', async () => {
    const rpc = {
      call: vi.fn().mockResolvedValue({ status: 'ok', value: { document: { kind: 'identity', revision: 4 } } }),
    } as unknown as FaceClientRPC;
    const client = PersonaClient.fromRPC(rpc);

    await client.initialize('session-1', {
      identity: 'identity',
      relationship: 'relationship',
      redline: 'redline',
      user: 'user',
      world: 'world',
    });
    await client.save('session-1', {
      kind: 'identity',
      content: 'updated',
      base_revision: 3,
      reason: 'user edit',
    });

    expect(rpc.call).toHaveBeenNthCalledWith(1, 'module.action.invoke', {
      module_id: 'vivy/diva-cognitive',
      action_id: 'diva.cognitive.persona.initialize',
      input: {
        session_id: 'session-1',
        initialization: {
          identity: 'identity',
          relationship: 'relationship',
          redline: 'redline',
          user: 'user',
          world: 'world',
        },
      },
    });
    expect(rpc.call).toHaveBeenNthCalledWith(2, 'module.action.invoke', {
      module_id: 'vivy/diva-cognitive',
      action_id: 'diva.cognitive.persona.save',
      input: {
        session_id: 'session-1',
        kind: 'identity',
        content: 'updated',
        base_revision: 3,
        reason: 'user edit',
      },
    });
  });
});
