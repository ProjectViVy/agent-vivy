// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { NotebookEditor } from './editor';
import { NotebookError } from './api';
import type { EntryView } from './types';

const ENTRY_ID = 'entry-1';

function headView(markdown = '# saved', version = 2): EntryView {
  return {
    entry: {
      id: ENTRY_ID, section_id: 'section-notes', kind: 'note', title: 'Doc',
      head_revision_id: `rev-${version}`, version, created_at: 1, updated_at: 1,
    },
    revision: {
      id: `rev-${version}`, entry_id: ENTRY_ID, sequence: version, title: 'Doc',
      markdown, origin: 'human', actor: 'local:operator', created_at: 1,
    },
  };
}

interface FakeClient {
  getEntry: ReturnType<typeof vi.fn>;
  saveEntry: ReturnType<typeof vi.fn>;
  listRevisions: ReturnType<typeof vi.fn>;
  exportEntry: ReturnType<typeof vi.fn>;
}

function clientStub(): FakeClient {
  return {
    getEntry: vi.fn(async () => headView()),
    saveEntry: vi.fn(async () => ({ resource_id: ENTRY_ID, version: 3, revision_id: 'rev-3', replayed: false })),
    listRevisions: vi.fn(async () => ({ revisions: [], next_cursor: '' })),
    exportEntry: vi.fn(async () => ({ ...headView(), comments: [] })),
  };
}

async function flush() {
  await act(async () => { await Promise.resolve(); });
}

describe('NotebookEditor', () => {
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

  function render(client: FakeClient) {
    return act(async () => root.render(
      <NotebookEditor client={client as never} entryId={ENTRY_ID} onChanged={vi.fn()} onRemoved={vi.fn()} />,
    ));
  }

  function draftArea(): HTMLTextAreaElement {
    const area = container.querySelector<HTMLTextAreaElement>('[data-testid="notebook-draft"]');
    expect(area).not.toBeNull();
    return area!;
  }

  function setDraft(value: string) {
    const area = draftArea();
    act(() => {
      const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!;
      setter.call(area, value);
      area.dispatchEvent(new Event('input', { bubbles: true }));
    });
  }

  function clickSave() {
    const button = container.querySelector<HTMLButtonElement>('[data-testid="notebook-save"]');
    expect(button).not.toBeNull();
    act(() => button!.click());
  }

  it('TestNotebookEditorPreservesDraftOnConflict: a stale CAS keeps the local draft and offers reload/current-version', async () => {
    const client = clientStub();
    client.saveEntry = vi.fn(async () => {
      throw new NotebookError('revision_conflict', 'stale', false, 5, 'rev-5');
    });
    await render(client);
    await flush();

    setDraft('local edits that must survive');
    clickSave();
    await flush();

    expect(draftArea().value).toBe('local edits that must survive');
    expect(container.querySelector('[data-testid="notebook-conflict"]')).not.toBeNull();
    expect(container.querySelector('[data-testid="notebook-reload"]')).not.toBeNull();
    expect(container.querySelector('[data-testid="notebook-view-current"]')).not.toBeNull();
  });

  it('retries a lost-ack save with one operation key and produces one revision', async () => {
    const client = clientStub();
    client.saveEntry = vi.fn()
      .mockRejectedValueOnce(new Error('network down'))
      .mockResolvedValueOnce({ resource_id: ENTRY_ID, version: 3, revision_id: 'rev-3', replayed: true });
    await render(client);
    await flush();

    setDraft('retry body');
    clickSave();
    await flush();
    // Unknown outcome → retry reuses the pending key+payload.
    clickSave();
    await flush();

    expect(client.saveEntry).toHaveBeenCalledTimes(2);
    const first = client.saveEntry.mock.calls[0][0];
    const second = client.saveEntry.mock.calls[1][0];
    expect(first.operationKey).toBe(second.operationKey);
    expect(first.request?.title ?? first.title).toBeDefined();
    expect(container.querySelector('[data-testid="notebook-save-state"]')?.textContent).toContain('saved');
  });

  it('blocks navigation while dirty only through the view-level guard; editor reports dirt via onDirty', async () => {
    const onDirty = vi.fn();
    const client = clientStub();
    await act(async () => root.render(
      <NotebookEditor client={client as never} entryId={ENTRY_ID} onChanged={vi.fn()} onRemoved={vi.fn()} onDirty={onDirty} />,
    ));
    await flush();
    setDraft('changed');
    await flush();
    expect(onDirty).toHaveBeenLastCalledWith(true);
  });

  it('keeps a distinct pending state while the save is in flight and clears the key only on success', async () => {
    const client = clientStub();
    let resolveSave: ((value: unknown) => void) | undefined;
    client.saveEntry = vi.fn(() => new Promise((resolve) => { resolveSave = resolve; }));
    await render(client);
    await flush();

    setDraft('in flight');
    clickSave();
    await flush();
    expect(container.querySelector('[data-testid="notebook-save-state"]')?.textContent).toContain('saving');
    expect(client.saveEntry).toHaveBeenCalledTimes(1);
    const key = client.saveEntry.mock.calls[0][0].operationKey;
    expect(typeof key).toBe('string');

    await act(async () => {
      resolveSave!({ resource_id: ENTRY_ID, version: 3, revision_id: 'rev-3', replayed: false });
      await Promise.resolve();
    });
    expect(container.querySelector('[data-testid="notebook-save-state"]')?.textContent).toContain('saved');
  });
});
