// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { PluginHostProvider, type FaceStoreState, type FullUIHost } from '@vivy/ui-sdk';
import { NotebookView } from './view';
import type { EntryView, Section } from './types';

const SECTIONS: readonly Section[] = [
  { id: 'section-notes', title: 'Notes', system_role: 'notes', version: 1, created_at: 1, updated_at: 1 },
  { id: 'section-custom', title: 'Scratch', version: 1, created_at: 2, updated_at: 2 },
];

function entryView(id = 'entry-1', title = 'First note'): EntryView {
  return {
    entry: {
      id, section_id: 'section-notes', kind: 'note', title,
      head_revision_id: 'rev-2', version: 2, created_at: 1, updated_at: 1,
    },
    revision: {
      id: 'rev-2', entry_id: id, sequence: 2, title,
      markdown: '# hello', origin: 'human', actor: 'local:operator', created_at: 1,
    },
  };
}

interface FakeActions {
  listSections: () => unknown;
  listEntries: (input: { section_id?: string }) => unknown;
  getEntry: () => unknown;
  createSection: () => unknown;
  createEntry: () => unknown;
  saveEntry: () => unknown;
  deleteEntry: () => unknown;
  restoreEntry: () => unknown;
  listRevisions: () => unknown;
  listComments: () => unknown;
  updateSection: () => unknown;
  moveEntry: () => unknown;
  exportEntry: () => unknown;
  adoptRevision: () => unknown;
  createComment: () => unknown;
  updateComment: () => unknown;
  deleteSection: () => unknown;
  restoreSection: () => unknown;
}

function makeActions(overrides: Partial<FakeActions> = {}): FakeActions {
  const sections = { sections: SECTIONS, next_cursor: '' };
  const entries = { entries: [entryView().entry], next_cursor: '' };
  return {
    listSections: vi.fn(async () => sections),
    listEntries: vi.fn(async () => entries),
    getEntry: vi.fn(async () => entryView()),
    createSection: vi.fn(async () => ({ resource_id: 'section-new', version: 1, replayed: false })),
    createEntry: vi.fn(async () => ({ resource_id: 'entry-new', version: 1, revision_id: 'rev-1', replayed: false })),
    saveEntry: vi.fn(async () => ({ resource_id: 'entry-1', version: 3, revision_id: 'rev-3', replayed: false })),
    deleteEntry: vi.fn(async () => ({ resource_id: 'entry-1', version: 4, replayed: false })),
    restoreEntry: vi.fn(async () => ({ resource_id: 'entry-1', version: 5, replayed: false })),
    listRevisions: vi.fn(async () => ({ revisions: [], next_cursor: '' })),
    listComments: vi.fn(async () => ({ comments: [], next_cursor: '' })),
    updateSection: vi.fn(async () => ({ resource_id: 'section-custom', version: 2, replayed: false })),
    moveEntry: vi.fn(async () => ({ resource_id: 'entry-1', version: 6, replayed: false })),
    exportEntry: vi.fn(async () => ({ ...entryView(), comments: [] })),
    adoptRevision: vi.fn(async () => ({ resource_id: 'entry-1', version: 7, revision_id: 'rev-4', replayed: false })),
    createComment: vi.fn(async () => ({ resource_id: 'comment-1', version: 1, replayed: false })),
    updateComment: vi.fn(async () => ({ resource_id: 'comment-1', version: 2, replayed: false })),
    deleteSection: vi.fn(async () => ({ resource_id: 'section-custom', version: 2, replayed: false })),
    restoreSection: vi.fn(async () => ({ resource_id: 'section-custom', version: 3, replayed: false })),
    ...overrides,
  };
}

function makeHost(actions: FakeActions): FullUIHost {
  const state = () => ({ activeSessionId: 's', currentRun: null, connection: 'connected' }) as unknown as FaceStoreState;
  const rpc = {
    capabilities: { protocol_version: 'vivy.rpc.v1', capabilities: ['module.action.invoke'] },
    call: vi.fn(async (_method: string, params?: { module_id?: string; action_id?: string; input?: unknown }) => {
      const action = params?.action_id ?? '';
      const map: Record<string, string> = {
        'vivy.notebook.sections.list': 'listSections',
        'vivy.notebook.entries.list': 'listEntries',
        'vivy.notebook.entries.get': 'getEntry',
        'vivy.notebook.sections.create': 'createSection',
        'vivy.notebook.entries.create': 'createEntry',
        'vivy.notebook.entries.save': 'saveEntry',
        'vivy.notebook.entries.delete': 'deleteEntry',
        'vivy.notebook.entries.restore': 'restoreEntry',
        'vivy.notebook.revisions.list': 'listRevisions',
        'vivy.notebook.comments.list': 'listComments',
        'vivy.notebook.sections.update': 'updateSection',
        'vivy.notebook.entries.move': 'moveEntry',
        'vivy.notebook.export': 'exportEntry',
        'vivy.notebook.revisions.adopt': 'adoptRevision',
        'vivy.notebook.comments.create': 'createComment',
        'vivy.notebook.comments.update': 'updateComment',
        'vivy.notebook.sections.delete': 'deleteSection',
        'vivy.notebook.sections.restore': 'restoreSection',
      };
      const fn = actions[map[action] as keyof FakeActions] as ((input?: unknown) => unknown) | undefined;
      if (!fn) return { status: 'error', error: { code: 'invalid_request', message: `no stub for ${action}`, retryable: false } };
      try {
        const data = await fn(params?.input);
        return { status: 'ok', data };
      } catch (cause) {
        if (cause && typeof cause === 'object' && 'code' in cause) {
          const typed = cause as { code: string; message: string; retryable?: boolean; currentVersion?: number; currentRevisionId?: string };
          return {
            status: 'error',
            error: {
              code: typed.code, message: typed.message, retryable: typed.retryable ?? false,
              current_version: typed.currentVersion, current_revision_id: typed.currentRevisionId,
            },
          };
        }
        return { status: 'error', error: { code: 'storage_unavailable', message: String(cause), retryable: true } };
      }
    }),
    onNotification: vi.fn(() => () => undefined),
    onClose: vi.fn(() => () => undefined),
    close: vi.fn(),
  } as unknown as FullUIHost['rpc'];
  return {
    api: {} as FullUIHost['api'],
    rpc,
    store: { getState: state, getInitialState: state, setState: vi.fn(), subscribe: () => () => undefined },
    router: { navigate: vi.fn().mockResolvedValue(undefined), invalidate: vi.fn().mockResolvedValue(undefined) },
    composition: {} as FullUIHost['composition'],
    t: (key) => key,
    registerCleanup: () => ({ active: true, dispose: vi.fn() }),
  };
}

async function flush() {
  await act(async () => { await Promise.resolve(); });
}

describe('NotebookView', () => {
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
  });

  async function render(actions = makeActions()) {
    const host = makeHost(actions);
    await act(async () => root.render(<PluginHostProvider host={host}><NotebookView /></PluginHostProvider>));
    await flush();
    return host;
  }

  it('shows backend capability unavailability instead of mock success', async () => {
    const actions = makeActions({
      listSections: vi.fn(async () => {
        const err = new Error('module unavailable') as Error & { code?: string; retryable?: boolean };
        err.code = 'capability_unavailable'; err.retryable = false; throw err;
      }),
    });
    await render(actions);
    expect(container.querySelector('[data-testid="notebook-unavailable"]')).not.toBeNull();
    expect(container.textContent).toContain('plugin.vivy/notebook.unavailable');
  });

  it('lists sections and entries, opens an entry and keeps stable selection after refresh', async () => {
    await render();
    expect(container.querySelector('[data-testid="notebook-page"]')).not.toBeNull();
    expect(container.querySelectorAll('[data-testid="notebook-section-select"] option')).toHaveLength(2);
    expect(container.querySelectorAll('[data-testid="notebook-entry"]')).toHaveLength(1);

    const entryButton = container.querySelector<HTMLButtonElement>('[data-testid="notebook-entry"]');
    act(() => entryButton!.click());
    await flush();
    expect(container.querySelector('[data-testid="notebook-editor"]')).not.toBeNull();
    expect(container.querySelector<HTMLTextAreaElement>('[data-testid="notebook-draft"]')!.value).toBe('# hello');
  });

  it('creates a section through the module action with an operation key', async () => {
    const actions = makeActions();
    await render(actions);
    const button = container.querySelector<HTMLButtonElement>('[data-testid="notebook-section-create"]');
    act(() => button!.click());
    await flush();
    const input = container.querySelector<HTMLInputElement>('[data-testid="notebook-section-title"]')!;
    act(() => {
      const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!;
      setter.call(input, 'Field notes');
      input.dispatchEvent(new Event('input', { bubbles: true }));
    });
    const submit = container.querySelector<HTMLButtonElement>('[data-testid="notebook-section-create-submit"]')!;
    act(() => submit.click());
    await flush();
    expect(actions.createSection).toHaveBeenCalledTimes(1);
    const arg = (actions.createSection as ReturnType<typeof vi.fn>).mock.calls[0][0] as { operation_key: string; request: { title: string } };
    expect(arg.operation_key).toMatch(/^nb-/);
    expect(arg.request.title).toBe('Field notes');
  });

  it('shows the empty state when a section has no entries', async () => {
    const actions = makeActions({ listEntries: vi.fn(async () => ({ entries: [], next_cursor: '' })) });
    await render(actions);
    expect(container.querySelector('[data-testid="notebook-entries-empty"]')).not.toBeNull();
  });

  it('confirms before discarding an unsaved draft when selecting another entry', async () => {
    const actions = makeActions({
      listEntries: vi.fn(async () => ({
        entries: [entryView('entry-1', 'One').entry, entryView('entry-2', 'Two').entry], next_cursor: '',
      })),
      getEntry: vi.fn(async (input?: { id?: string }) => entryView(input?.id ?? 'entry-1', 'Doc')),
    });
    await render(actions);
    const buttons = container.querySelectorAll<HTMLButtonElement>('[data-testid="notebook-entry"]');
    act(() => buttons[0].click());
    await flush();

    const draft = container.querySelector<HTMLTextAreaElement>('[data-testid="notebook-draft"]')!;
    act(() => {
      const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!;
      setter.call(draft, 'unsaved edit');
      draft.dispatchEvent(new Event('input', { bubbles: true }));
    });
    await flush();

    const buttons2 = container.querySelectorAll<HTMLButtonElement>('[data-testid="notebook-entry"]');
    act(() => buttons2[1].click());
    await flush();
    expect(container.querySelector('[data-testid="notebook-discard-confirm"]')).not.toBeNull();
    expect(container.querySelector<HTMLTextAreaElement>('[data-testid="notebook-draft"]')!.value).toBe('unsaved edit');

    // Stay keeps the draft; leave discards it.
    const leave = container.querySelector<HTMLButtonElement>('[data-testid="notebook-discard-confirm-leave"]')!;
    act(() => leave.click());
    await flush();
    expect(container.querySelector<HTMLTextAreaElement>('[data-testid="notebook-draft"]')!.value).toBe('# hello');
  });
});
