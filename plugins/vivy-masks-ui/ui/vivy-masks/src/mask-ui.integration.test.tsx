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
    expect(container.querySelector('textarea')).toHaveProperty('readOnly', true);
    const duplicate = [...container.querySelectorAll<HTMLButtonElement>('button')].find((item) => item.textContent?.includes('plugin.vivy/masks-ui.duplicate'));
    expect(duplicate).not.toBeUndefined();
    await act(async () => duplicate!.click());
    expect(container.querySelector('textarea')).toHaveProperty('readOnly', false);
    expect(container.querySelector('textarea')).toHaveProperty('value', 'Builtin instructions');
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
