// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { PluginHostProvider, type FaceClientStore, type FaceStoreState, type FullUIHost } from '@vivy/ui-sdk';
import { PersonaMemoryView } from './view';

const IDENTITY = {
  kind: 'identity',
  file_name: 'IDENTITY.MD',
  exists: true,
  valid: true,
  content: '# Real identity',
  revision: 3,
  content_hash: 'sha256:identity',
  updated_at: '2026-10-06T10:00:00Z',
  pending_count: 1,
};

const REVIEW = {
  id: 'review-1',
  kind: 'identity',
  base_revision: 3,
  base_hash: 'sha256:identity',
  proposed_markdown: '# Proposed identity',
  actor: 'agent',
  reason: 'keep this current',
  created_at: '2026-10-06T10:00:00Z',
  state: 'pending',
  decided_at: null,
};

describe('PersonaMemoryView', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    vi.unstubAllGlobals();
  });

  it('offers the host session creator when no session is active', async () => {
    const createSession = vi.fn().mockResolvedValue({ id: 'session-2', title: '', created_at: 0 });
    const host = makeHost(async () => ({ status: 'ok' }), null, { createSession });

    await act(async () => root.render(<PluginHostProvider host={host}><PersonaMemoryView /></PluginHostProvider>));

    expect(container.textContent).toContain('plugin.vivy/persona.noSession');
    const create = [...container.querySelectorAll('button')].find((button) => button.textContent === 'plugin.vivy/persona.noSessionCreate');
    expect(create).not.toBeUndefined();
    await act(async () => (create as HTMLButtonElement).click());
    expect(createSession).toHaveBeenCalledTimes(1);
    expect(host.rpc.call).not.toHaveBeenCalled();
  });

  it('shows an explicit five-field initialization form for an uninitialized session', async () => {
    const host = makeHost(async (_method, params) => {
      expect(params).toEqual({
        module_id: 'vivy/diva-cognitive',
        action_id: 'diva.cognitive.status',
        input: { session_id: 'session-1' },
      });
      return { status: 'ok', value: { persona: { state: 'uninitialized', current_revisions: {} } } };
    }, 'session-1');

    await act(async () => root.render(<PluginHostProvider host={host}><PersonaMemoryView /></PluginHostProvider>));

    expect(container.textContent).toContain('plugin.vivy/persona.initialize.title');
    for (const key of ['identity', 'relationship', 'redline', 'user', 'world']) {
      expect(container.querySelector(`textarea[name="${key}"]`)).not.toBeNull();
    }
    expect(container.textContent).not.toContain('plugin.vivy/persona.history');
    expect(container.textContent).not.toContain('demo.personaDocs');
  });

  it('reads the real document and review list through the live action plane', async () => {
    const host = makeHost(async (_method, params) => {
      const actionId = (params as { action_id?: string }).action_id;
      if (actionId === 'diva.cognitive.status') return { status: 'ok', value: { persona: { state: 'ready', current_revisions: { identity: 3 } } } };
      if (actionId === 'diva.cognitive.persona.read') return { status: 'ok', value: IDENTITY };
      if (actionId === 'diva.cognitive.persona.reviews.list') return { status: 'ok', value: { items: [REVIEW] } };
      throw new Error(`unexpected action ${actionId}`);
    }, 'session-1');

    await act(async () => root.render(<PluginHostProvider host={host}><PersonaMemoryView /></PluginHostProvider>));

    expect(container.textContent).toContain('# Real identity');
    const pendingTab = [...container.querySelectorAll('button[role="tab"]')].find((button) => button.textContent?.includes('plugin.vivy/persona.pending'));
    await act(async () => {
      pendingTab!.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
      pendingTab!.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
      pendingTab!.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    expect(container.textContent).toContain('plugin.vivy/persona.review.reason');
    expect(container.textContent).not.toContain('No history');
    expect(host.rpc.call).toHaveBeenCalledWith('module.action.invoke', {
      module_id: 'vivy/diva-cognitive',
      action_id: 'diva.cognitive.persona.read',
      input: { session_id: 'session-1', kind: 'identity' },
    });
  });

  it('saves with the document revision and refreshes the authoritative document', async () => {
    let content = IDENTITY.content;
    const host = makeHost(async (_method, params) => {
      const actionId = (params as { action_id?: string }).action_id;
      const input = (params as { input?: Record<string, unknown> }).input ?? {};
      if (actionId === 'diva.cognitive.status') return { status: 'ok', value: { persona: { state: 'ready', current_revisions: { identity: 3 } } } };
      if (actionId === 'diva.cognitive.persona.read') return { status: 'ok', value: { ...IDENTITY, content, revision: content === IDENTITY.content ? 3 : 4 } };
      if (actionId === 'diva.cognitive.persona.reviews.list') return { status: 'ok', value: { items: [] } };
      if (actionId === 'diva.cognitive.persona.save') {
        expect(input).toMatchObject({ session_id: 'session-1', kind: 'identity', base_revision: 3 });
        content = String(input.content);
        return { status: 'ok', value: { document: { ...IDENTITY, content, revision: 4 } } };
      }
      throw new Error(`unexpected action ${actionId}`);
    }, 'session-1');

    await act(async () => root.render(<PluginHostProvider host={host}><PersonaMemoryView /></PluginHostProvider>));
    const textarea = container.querySelector('textarea[name="document"]') as HTMLTextAreaElement;
    const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!;
    await act(async () => {
      setter.call(textarea, 'updated identity');
      textarea.dispatchEvent(new Event('input', { bubbles: true }));
    });
    const save = [...container.querySelectorAll('button')].find((button) => button.textContent === 'common.save');
    expect(save).not.toBeUndefined();
    await act(async () => (save as HTMLButtonElement).click());

    expect(host.rpc.call).toHaveBeenCalledWith('module.action.invoke', {
      module_id: 'vivy/diva-cognitive',
      action_id: 'diva.cognitive.persona.save',
      input: {
        session_id: 'session-1',
        kind: 'identity',
        content: 'updated identity',
        base_revision: 3,
        reason: 'user edit',
      },
    });
    expect(container.textContent).toContain('updated identity');
  });

  it('decides a review through the live action plane', async () => {
    const host = makeHost(async (_method, params) => {
      const actionId = (params as { action_id?: string }).action_id;
      if (actionId === 'diva.cognitive.status') return { status: 'ok', value: { persona: { state: 'ready', current_revisions: { identity: 3 } } } };
      if (actionId === 'diva.cognitive.persona.read') return { status: 'ok', value: IDENTITY };
      if (actionId === 'diva.cognitive.persona.reviews.list') return { status: 'ok', value: { items: [REVIEW] } };
      if (actionId === 'diva.cognitive.persona.review.decide') return { status: 'ok', value: { changed: true, document: IDENTITY } };
      throw new Error(`unexpected action ${actionId}`);
    }, 'session-1');

    await act(async () => root.render(<PluginHostProvider host={host}><PersonaMemoryView /></PluginHostProvider>));
    const pending = [...container.querySelectorAll('button[role="tab"]')].find((button) => button.textContent?.includes('plugin.vivy/persona.pending'));
    await act(async () => {
      pending!.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
      pending!.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
      pending!.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    const accept = [...container.querySelectorAll('button')].find((button) => button.textContent === 'common.accept');
    await act(async () => (accept as HTMLButtonElement).click());

    expect(host.rpc.call).toHaveBeenCalledWith('module.action.invoke', {
      module_id: 'vivy/diva-cognitive',
      action_id: 'diva.cognitive.persona.review.decide',
      input: { session_id: 'session-1', review_id: 'review-1', decision: 'accept' },
    });
  });

  it('requires confirmation before a review decision can discard a dirty draft', async () => {
    const host = makeHost(async (_method, params) => {
      const actionId = (params as { action_id?: string }).action_id;
      if (actionId === 'diva.cognitive.status') return { status: 'ok', value: { persona: { state: 'ready', current_revisions: { identity: 3 } } } };
      if (actionId === 'diva.cognitive.persona.read') return { status: 'ok', value: IDENTITY };
      if (actionId === 'diva.cognitive.persona.reviews.list') return { status: 'ok', value: { items: [REVIEW] } };
      if (actionId === 'diva.cognitive.persona.review.decide') return { status: 'ok', value: { changed: true, document: IDENTITY } };
      throw new Error(`unexpected action ${actionId}`);
    }, 'session-1');
    const confirm = vi.fn(() => false);
    vi.stubGlobal('confirm', confirm);

    await act(async () => root.render(<PluginHostProvider host={host}><PersonaMemoryView /></PluginHostProvider>));
    const textarea = container.querySelector('textarea[name="document"]') as HTMLTextAreaElement;
    const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!;
    await act(async () => {
      setter.call(textarea, '# unsaved identity');
      textarea.dispatchEvent(new Event('input', { bubbles: true }));
    });
    const pending = [...container.querySelectorAll('button[role="tab"]')].find((button) => button.textContent?.includes('plugin.vivy/persona.pending'));
    await act(async () => {
      pending!.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
      pending!.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
      pending!.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    const accept = [...container.querySelectorAll('button')].find((button) => button.textContent === 'common.accept');
    await act(async () => (accept as HTMLButtonElement).click());

    expect(confirm).toHaveBeenCalledWith('plugin.vivy/persona.dirtyConfirm');
    expect(host.rpc.call).not.toHaveBeenCalledWith('module.action.invoke', expect.objectContaining({
      action_id: 'diva.cognitive.persona.review.decide',
    }));
  });

  it('does not let a stale document load overwrite the newly selected kind', async () => {
    const identityRead = deferred<unknown>();
    const relationship = { ...IDENTITY, kind: 'relationship', file_name: 'RELATIONSHIP.MD', content: '# Real relationship' };
    const host = makeHost(async (_method, params) => {
      const actionId = (params as { action_id?: string }).action_id;
      const input = (params as { input?: { kind?: string } }).input;
      if (actionId === 'diva.cognitive.status') return { status: 'ok', value: { persona: { state: 'ready', current_revisions: {} } } };
      if (actionId === 'diva.cognitive.persona.read') {
        if (input?.kind === 'identity') return identityRead.promise;
        return { status: 'ok', value: relationship };
      }
      if (actionId === 'diva.cognitive.persona.reviews.list') return { status: 'ok', value: { items: [] } };
      throw new Error(`unexpected action ${actionId}`);
    }, 'session-1');

    await act(async () => root.render(<PluginHostProvider host={host}><PersonaMemoryView /></PluginHostProvider>));
    const relationshipButton = [...container.querySelectorAll('button')].find((button) => button.textContent?.includes('RELATIONSHIP.MD'));
    await act(async () => (relationshipButton as HTMLButtonElement).click());

    expect((container.querySelector('textarea[name="document"]') as HTMLTextAreaElement).value).toBe(relationship.content);
    identityRead.resolve({ status: 'ok', value: { ...IDENTITY, content: '# stale identity' } });
    await act(async () => { await identityRead.promise; });
    expect((container.querySelector('textarea[name="document"]') as HTMLTextAreaElement).value).toBe(relationship.content);
    expect(container.textContent).not.toContain('# stale identity');
  });

  it('locks conflicting controls while a save is pending', async () => {
    const saveResult = deferred<unknown>();
    const relationship = { ...IDENTITY, kind: 'relationship', file_name: 'RELATIONSHIP.MD', content: '# Real relationship' };
    const host = makeHost(async (_method, params) => {
      const actionId = (params as { action_id?: string }).action_id;
      const input = (params as { input?: { kind?: string; content?: string } }).input;
      if (actionId === 'diva.cognitive.status') return { status: 'ok', value: { persona: { state: 'ready', current_revisions: {} } } };
      if (actionId === 'diva.cognitive.persona.read') return { status: 'ok', value: input?.kind === 'relationship' ? relationship : IDENTITY };
      if (actionId === 'diva.cognitive.persona.reviews.list') return { status: 'ok', value: { items: [] } };
      if (actionId === 'diva.cognitive.persona.save') return saveResult.promise;
      throw new Error(`unexpected action ${actionId}`);
    }, 'session-1');
    vi.stubGlobal('confirm', vi.fn(() => true));

    await act(async () => root.render(<PluginHostProvider host={host}><PersonaMemoryView /></PluginHostProvider>));
    const textarea = container.querySelector('textarea[name="document"]') as HTMLTextAreaElement;
    const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!;
    await act(async () => {
      setter.call(textarea, '# edited identity');
      textarea.dispatchEvent(new Event('input', { bubbles: true }));
    });
    const save = [...container.querySelectorAll('button')].find((button) => button.textContent === 'common.save');
    await act(async () => (save as HTMLButtonElement).click());

    const relationshipButton = [...container.querySelectorAll('button')].find((button) => button.textContent?.includes('RELATIONSHIP.MD'));
    const refresh = container.querySelector('button[aria-label="common.refresh"]') as HTMLButtonElement;
    expect((container.querySelector('textarea[name="document"]') as HTMLTextAreaElement).disabled).toBe(true);
    expect(refresh.disabled).toBe(true);
    expect((relationshipButton as HTMLButtonElement).disabled).toBe(true);
    expect((save as HTMLButtonElement).disabled).toBe(true);
    await act(async () => (relationshipButton as HTMLButtonElement).click());
    expect((container.querySelector('textarea[name="document"]') as HTMLTextAreaElement).value).toBe('# edited identity');

    saveResult.resolve({ status: 'ok', value: { changed: true, document: { ...IDENTITY, content: '# saved identity' } } });
    await act(async () => { await saveResult.promise; });
    expect((container.querySelector('textarea[name="document"]') as HTMLTextAreaElement).value).toBe('# saved identity');
    expect((container.querySelector('textarea[name="document"]') as HTMLTextAreaElement).disabled).toBe(false);
  });
});

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

function makeHost(
  action: (method: string, params?: unknown) => Promise<unknown>,
  activeSessionId: string | null,
  stateOverrides: Partial<FaceStoreState> = {},
): FullUIHost {
  let state = { activeSessionId, connection: 'connected', currentRun: null, ...stateOverrides } as unknown as FaceStoreState;
  const listeners = new Set<() => void>();
  const rpc = {
    capabilities: { protocol_version: 'vivy.rpc.v1', capabilities: ['module.action.invoke'] },
    call: vi.fn(action),
    onNotification: vi.fn(() => () => undefined),
    onClose: vi.fn(() => () => undefined),
    close: vi.fn(),
  } as unknown as FullUIHost['rpc'];
  const store = {
    getState: () => state,
    getInitialState: () => state,
    setState: vi.fn(),
    subscribe: (listener: () => void) => { listeners.add(listener); return () => listeners.delete(listener); },
  } as unknown as FaceClientStore<FaceStoreState>;
  return {
    api: {} as FullUIHost['api'],
    rpc,
    store,
    router: { navigate: vi.fn().mockResolvedValue(undefined), invalidate: vi.fn().mockResolvedValue(undefined) },
    composition: {} as FullUIHost['composition'],
    t: (key) => key,
    registerCleanup: () => ({ active: true, dispose: vi.fn() }),
  };
}
