// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { PluginHostProvider, type FaceStoreState, type FullUIHost, type ChatHeaderContribution, type UIRouteItem } from '@vivy/ui-sdk';
import { extension } from './index';

const catalog = ['Programmer', 'Researcher', 'Writer'].map((name) => ({
  id: `builtin/${name.toLowerCase()}`, name, description: `${name} description`,
  revision: 1, built_in: true, digest: 'a'.repeat(64), generation_id: 'generation',
}));
const selection = (session_id: string, mask_id: string, revision: number) => ({
  session_id, mask_id, revision, available: true, inactive_reason: '',
});
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}

describe('assembled mask UI', () => {
  let container: HTMLDivElement;
  let root: Root;
  let dispose: (() => void) | undefined;
  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  });
  afterEach(async () => {
    await act(async () => root.unmount());
    dispose?.();
    container.remove();
    vi.restoreAllMocks();
  });

  async function assemble(action: (params: any) => Promise<unknown>, showPage = true) {
    let state = { activeSessionId: 'A', currentRun: null, connection: 'connected' } as unknown as FaceStoreState;
    const listeners = new Set<() => void>();
    let route!: UIRouteItem;
    let header!: ChatHeaderContribution;
    const register = (_id: string, _value: unknown) => ({ dispose() {} });
    const host = {
      rpc: { call: vi.fn((_method, params) => action(params)) },
      t: (key: string, args?: Record<string, string>) => args?.name ? `${key} ${args.name}` : key,
      router: { navigate: vi.fn().mockResolvedValue(undefined) },
      store: {
        getState: () => state, getInitialState: () => state,
        subscribe: (listener: () => void) => { listeners.add(listener); return () => { listeners.delete(listener); }; },
      },
      composition: {
        routes: { register: (_id: string, value: UIRouteItem) => { route = value; return register(_id, value); } },
        navigation: { register },
        components: { register: (_id: string, value: ChatHeaderContribution) => { header = value; return register(_id, value); } },
      },
    } as unknown as FullUIHost;
    dispose = extension.install(host) as () => void;
    const render = async () => act(async () => root.render(
      <PluginHostProvider host={host}>
        <div data-test-header>{header.render({ sessionId: state.activeSessionId, running: false })}</div>
        {showPage ? route.render() : null}
      </PluginHostProvider>,
    ));
    await render();
    return {
      host,
      setSession: async (id: string | null) => {
        await act(async () => { state = { ...state, activeSessionId: id }; for (const listener of listeners) listener(); });
        await render();
      },
    };
  }

  async function chooseHeader(name: string) {
    const select = container.querySelector<HTMLSelectElement>('[data-test-header] select');
    if (select) {
      await act(async () => { select.value = `builtin/${name.toLowerCase()}`; select.dispatchEvent(new Event('change', { bubbles: true })); });
      return;
    }
    const trigger = container.querySelector<HTMLButtonElement>('[data-test-header] button');
    expect(trigger).not.toBeNull();
    if (!document.querySelector('[role="menu"]')) await act(async () => trigger!.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0 })));
    const option = [...document.querySelectorAll<HTMLElement>('[role="menuitem"], [role="menuitemradio"]')].find((item) => item.textContent?.includes(name));
    expect(option).not.toBeUndefined();
    await act(async () => option!.click());
  }

  it('renders the default identity without creating or reading a catalog definition', async () => {
    const ui = await assemble(async (p) => {
      if (p.action_id.endsWith('catalog.list')) return { items: [], next_after_id: '' };
      if (p.action_id.endsWith('selection.get')) return selection('A', '', 1);
      throw new Error('Unexpected catalog definition action');
    });
    expect(container.querySelector('[data-mask-id=""]')).not.toBeNull();
    expect(container.querySelector('[data-test-header]')?.textContent).toContain('plugin.vivy/masks-ui.unmasked');
    expect(container.querySelector('[data-mask-instructions]')?.textContent).toContain('plugin.vivy/masks-ui.defaultBody');
    expect((ui.host.rpc.call as ReturnType<typeof vi.fn>).mock.calls.some(([, p]) => p.action_id.endsWith('catalog.get'))).toBe(false);
  });

  it('previews a role without changing the session identity', async () => {
    const ui = await assemble(async (p) => {
      if (p.action_id.endsWith('catalog.list')) return { items: catalog, next_after_id: '' };
      if (p.action_id.endsWith('selection.get')) return selection('A', '', 1);
      if (p.action_id.endsWith('catalog.get')) return { ...catalog[0], body: 'Programmer instructions' };
    });
    await act(async () => container.querySelector<HTMLButtonElement>('[data-mask-id="builtin/programmer"] button')!.click());
    expect(container.querySelector('[data-mask-instructions]')?.textContent).toBe('Programmer instructions');
    expect(container.querySelector('[data-active-mask]')?.getAttribute('data-active-mask')).toBe('');
    expect((ui.host.rpc.call as ReturnType<typeof vi.fn>).mock.calls.some(([, p]) => p.action_id.endsWith('selection.set'))).toBe(false);
  });

  it('creates a mask and applies the committed definition when Save and use is chosen', async () => {
    let items = [...catalog];
    let committed = selection('A', '', 1);
    const ui = await assemble(async (p) => {
      if (p.action_id.endsWith('catalog.list')) return { items, next_after_id: '' };
      if (p.action_id.endsWith('selection.get')) return committed;
      if (p.action_id.endsWith('catalog.create')) {
        const mask = { ...catalog[0], ...p.input, id: 'custom/new', built_in: false };
        items = [...items, mask];
        return mask;
      }
      if (p.action_id.endsWith('selection.set')) { committed = selection('A', p.input.mask_id, 2); return committed; }
    });
    await act(async () => container.querySelector<HTMLButtonElement>('[data-mask-action="new"]')!.click());
    const editor = document.querySelector('[data-mask-editor]')!;
    await fill(editor.querySelector('input')!, 'Frontend partner');
    await fill(editor.querySelector('textarea')!, 'Check responsive layouts.');
    const save = [...document.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')].find((b) => b.textContent === 'plugin.vivy/masks-ui.saveAndUse')!;
    await act(async () => save.click());
    expect(document.querySelector('[data-mask-editor]')).toBeNull();
    expect(container.querySelector('[data-test-header]')?.textContent).toContain('Frontend partner');
    expect(container.querySelector('[data-active-mask]')?.getAttribute('data-active-mask')).toBe('custom/new');
    const calls = (ui.host.rpc.call as ReturnType<typeof vi.fn>).mock.calls.map(([, p]) => p);
    expect(calls.find((p) => p.action_id.endsWith('catalog.create')).input.operation_id).toMatch(/^[a-f0-9-]{36}$/);
    expect(calls.find((p) => p.action_id.endsWith('selection.set')).input.expected_revision).toBe(1);
  });

  it('keeps a dirty draft until discard is explicitly confirmed', async () => {
    await assemble(async (p) => p.action_id.endsWith('selection.get') ? selection('A', '', 1) : { items: catalog, next_after_id: '' });
    await act(async () => container.querySelector<HTMLButtonElement>('[data-mask-action="new"]')!.click());
    await fill(document.querySelector('[data-mask-editor] input')!, 'Unsaved');
    const cancel = [...document.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')].find((b) => b.textContent === 'plugin.vivy/masks-ui.cancel')!;
    await act(async () => cancel.click());
    expect(document.querySelector('[data-mask-editor] input')).toHaveProperty('value', 'Unsaved');
    const discard = [...document.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')].find((b) => b.textContent === 'plugin.vivy/masks-ui.discard')!;
    await act(async () => discard.click());
    expect(document.querySelector('[data-mask-editor]')).toBeNull();
  });

  it('closes a committed editor before a slow catalog refresh can accept more edits', async () => {
    const refresh = deferred<unknown>();
    let committed = false;
    let creates = 0;
    await assemble(async (p) => {
      if (p.action_id.endsWith('catalog.list')) return committed ? refresh.promise : { items: catalog, next_after_id: '' };
      if (p.action_id.endsWith('selection.get')) return selection('A', '', 1);
      if (p.action_id.endsWith('catalog.create')) {
        committed = true;
        creates++;
        return { ...catalog[0], ...p.input, id: `custom/saved-${creates}`, built_in: false };
      }
    });
    await act(async () => container.querySelector<HTMLButtonElement>('[data-mask-action="new"]')!.click());
    await fill(document.querySelector('[data-mask-editor] input')!, 'Saved');
    await fill(document.querySelector('[data-mask-editor] textarea')!, 'Committed instructions');
    const save = [...document.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')].find((b) => b.textContent === 'plugin.vivy/masks-ui.save')!;
    await act(async () => save.click());
    expect(document.querySelector('[data-mask-editor]')).toBeNull();
    await act(async () => container.querySelector<HTMLButtonElement>('[data-mask-action="new"]')!.click());
    const cancel = [...document.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')].find((b) => b.textContent === 'plugin.vivy/masks-ui.cancel')!;
    await act(async () => cancel.click());
    expect(document.querySelector('[data-mask-editor]')).toBeNull();
    await act(async () => container.querySelector<HTMLButtonElement>('[data-mask-action="new"]')!.click());
    await fill(document.querySelector('[data-mask-editor] input')!, 'Another saved mask');
    await fill(document.querySelector('[data-mask-editor] textarea')!, 'More instructions');
    const nextSave = [...document.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')].find((b) => b.textContent === 'plugin.vivy/masks-ui.save')!;
    await act(async () => nextSave.click());
    expect(creates).toBe(2);
    expect(document.querySelector('[data-mask-editor]')).toBeNull();
    await act(async () => refresh.resolve({ items: catalog, next_after_id: '' }));
  });

  it('preserves conflict recovery after typing and retries a failed authoritative reload', async () => {
    const custom = { ...catalog[0], id: 'custom/stale', name: 'Stale', built_in: false, body: 'Old instructions' };
    let reads = 0;
    await assemble(async (p) => {
      if (p.action_id.endsWith('catalog.list')) return { items: [...catalog, custom], next_after_id: '' };
      if (p.action_id.endsWith('selection.get')) return selection('A', '', 1);
      if (p.action_id.endsWith('catalog.get')) {
        reads++;
        if (reads === 2) throw new Error('Connection lost');
        return reads === 1 ? custom : { ...custom, revision: 2, body: 'Latest instructions' };
      }
      if (p.action_id.endsWith('catalog.update')) throw { message: 'Conflict', data: { code: 'revision_conflict', current_revision: 2 } };
    });
    const click = async (text: string) => act(async () => {
      [...document.querySelectorAll<HTMLButtonElement>('button')].find((b) => b.textContent === `plugin.vivy/masks-ui.${text}`)!.click();
    });
    await act(async () => container.querySelector<HTMLButtonElement>('[data-mask-id="custom/stale"] button')!.click());
    await click('edit');
    await fill(document.querySelector('[data-mask-editor] textarea')!, 'Local draft');
    await click('save');
    await fill(document.querySelector('[data-mask-editor] textarea')!, 'Continued local draft');
    await click('reloadLatest');
    await click('discard');
    expect(document.querySelector('[data-mask-editor] textarea')).toHaveProperty('value', 'Continued local draft');
    expect(document.querySelector('[data-mask-editor] [role="alert"]')?.textContent).toContain('errors.reloadFailed');
    await click('reloadLatest');
    await click('discard');
    expect(document.querySelector('[data-mask-editor] textarea')).toHaveProperty('value', 'Latest instructions');
    expect(reads).toBe(3);
  });

  it('ignores a late selection write from A after B is active', async () => {
    const late = deferred<unknown>();
    const ui = await assemble(async (p) => {
      if (p.action_id.endsWith('catalog.list')) return { items: catalog, next_after_id: '' };
      if (p.action_id.endsWith('selection.get')) return p.input.session_id === 'A' ? selection('A', '', 1) : selection('B', 'builtin/writer', 7);
      if (p.action_id.endsWith('selection.set')) return late.promise;
    }, false);
    await chooseHeader('Programmer');
    await ui.setSession('B');
    await act(async () => late.resolve(selection('A', 'builtin/programmer', 2)));
    const header = container.querySelector('[data-test-header]')!;
    const selected = header.querySelector<HTMLSelectElement>('select');
    if (selected) expect(selected.value).toBe('builtin/writer');
    else expect(header.textContent).toContain('Writer');
  });

  it('shares committed selection between the management page and quick menu', async () => {
    let committed = selection('A', '', 1);
    await assemble(async (p) => {
      if (p.action_id.endsWith('catalog.list')) return { items: catalog, next_after_id: '' };
      if (p.action_id.endsWith('selection.get')) return { ...committed };
      if (p.action_id.endsWith('selection.set')) {
        committed = selection('A', p.input.mask_id, committed.revision + 1);
        return { ...committed };
      }
    });
    const page = container.querySelector('[data-testid="mask-page"]')!;
    const select = page.querySelector<HTMLSelectElement>('select');
    if (select) await act(async () => { select.value = 'builtin/writer'; select.dispatchEvent(new Event('change', { bubbles: true })); });
    else await act(async () => page.querySelector<HTMLButtonElement>('[data-mask-id="builtin/writer"] [data-mask-use]')!.click());
    const header = container.querySelector('[data-test-header]')!;
    const selected = header.querySelector<HTMLSelectElement>('select');
    if (selected) expect(selected.value).toBe('builtin/writer');
    else expect(header.textContent).toContain('Writer');
    await chooseHeader('Researcher');
    expect(page.textContent).toContain('Researcher');
    expect(page.querySelector('[data-active-mask]')?.getAttribute('data-active-mask')).toBe('builtin/researcher');
  });

  it('opens builtin instructions read-only and duplicates them as a custom draft', async () => {
    await assemble(async (p) => {
      if (p.action_id.endsWith('catalog.list')) return { items: catalog, next_after_id: '' };
      if (p.action_id.endsWith('selection.get')) return selection('A', '', 1);
      if (p.action_id.endsWith('catalog.get')) return { ...catalog[0], body: 'Builtin instructions' };
    });
    const button = [...container.querySelectorAll<HTMLButtonElement>('[data-testid="mask-page"] button')].find((item) => item.textContent?.includes('Programmer'))!;
    await act(async () => button.click());
    expect(container.querySelector('[data-mask-instructions]')?.textContent).toBe('Builtin instructions');
    expect(document.querySelector('textarea')).toBeNull();
    const duplicate = [...container.querySelectorAll<HTMLButtonElement>('button')].find((item) => item.textContent?.includes('plugin.vivy/masks-ui.duplicate'));
    expect(duplicate).not.toBeUndefined();
    await act(async () => duplicate!.click());
    expect(document.querySelector('textarea')).toHaveProperty('readOnly', false);
    expect(document.querySelector('textarea')).toHaveProperty('value', 'Builtin instructions');
  });

  it('shows a selection conflict, reloads the committed choice, and retries with its revision', async () => {
    let committed = selection('A', '', 1);
    let conflict = true;
    const ui = await assemble(async (p) => {
      if (p.action_id.endsWith('catalog.list')) return { items: catalog, next_after_id: '' };
      if (p.action_id.endsWith('selection.get')) return { ...committed };
      if (p.action_id.endsWith('selection.set')) {
        if (conflict) {
          conflict = false;
          committed = selection('A', 'builtin/writer', 7);
          throw { message: 'Conflict', data: { code: 'revision_conflict' } };
        }
        committed = selection('A', p.input.mask_id, p.input.expected_revision + 1);
        return committed;
      }
    }, false);
    await chooseHeader('Programmer');
    expect(document.querySelector('[role="alert"]')?.textContent).toContain('plugin.vivy/masks-ui.errors.selectionConflict');
    expect(container.querySelector('[data-test-header]')?.textContent).toContain('Writer');
    await chooseHeader('Researcher');
    const writes = (ui.host.rpc.call as ReturnType<typeof vi.fn>).mock.calls.map(([, p]) => p).filter((p) => p.action_id.endsWith('selection.set'));
    expect(writes[1].input.expected_revision).toBe(7);
    expect(container.querySelector('[data-test-header]')?.textContent).toContain('Researcher');
  });

  it('preserves an unavailable persisted mask identity and its warning', async () => {
    await assemble(async (p) => {
      if (p.action_id.endsWith('catalog.list')) return { items: [], next_after_id: '' };
      if (p.action_id.endsWith('selection.get')) return { ...selection('A', 'custom/retired', 4), available: false, inactive_reason: 'not_compiled' };
    });
    const header = container.querySelector('[data-test-header]')!;
    expect(header.textContent).toContain('custom/retired');
    expect(header.querySelector('button')?.title).toContain('plugin.vivy/masks-ui.errors.unavailable');
    const page = container.querySelector('[data-testid="mask-page"]')!;
    expect(page.querySelector('[data-active-mask]')?.textContent).toContain('custom/retired');
    expect(page.textContent).toContain('plugin.vivy/masks-ui.errors.unavailable');
  });

  it('refreshes both surfaces after focus and loads catalog pages beyond the first hundred', async () => {
    let committed = selection('A', '', 1);
    const custom = { ...catalog[0], id: 'custom/extra', name: 'Extra', built_in: false };
    await assemble(async (p) => {
      if (p.action_id.endsWith('catalog.list')) return p.input.after_id ? { items: [custom], next_after_id: '' } : { items: catalog, next_after_id: 'builtin/writer' };
      if (p.action_id.endsWith('selection.get')) return { ...committed };
    });
    expect(container.querySelector('[data-mask-id="custom/extra"]')).not.toBeNull();
    committed = selection('A', 'custom/extra', 5);
    await act(async () => window.dispatchEvent(new Event('focus')));
    expect(container.querySelector('[data-test-header]')?.textContent).toContain('Extra');
    expect(container.querySelector('[data-active-mask]')?.getAttribute('data-active-mask')).toBe('custom/extra');
  });
});

async function fill(element: Element, value: string) {
  const prototype = element.tagName === 'TEXTAREA' ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
  await act(async () => {
    Object.getOwnPropertyDescriptor(prototype, 'value')!.set!.call(element, value);
    element.dispatchEvent(new Event('input', { bubbles: true }));
  });
}
