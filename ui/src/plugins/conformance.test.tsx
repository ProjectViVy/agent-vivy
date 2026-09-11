// @vitest-environment happy-dom
import { useEffect, useState, type ReactNode } from 'react';
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  composeUI,
  createModuleActionClient,
  defineUIRoot,
  MODULE_ACTION_METHOD,
  type FaceStoreState,
  type FullUIHost,
  type UICompositionHost,
  type UIExtension,
  type UIRoot,
} from '@vivy/ui-sdk';
import { UI_ASSEMBLY_MANIFEST } from '@vivy/generated-assembly';
import {
  PresentationHost,
  type PresentationProvenance,
} from './presentation-host';

type FixtureName = 'default' | 'extension' | 'replacement-root' | 'minimal';

interface FixtureGeneration {
  readonly name: FixtureName;
  readonly root?: UIRoot;
  readonly extensions: readonly UIExtension[];
  readonly provenance: PresentationProvenance;
  readonly selectedMarkers: readonly string[];
  readonly omittedMarkers: readonly string[];
}

function compositionHost(): UICompositionHost {
  const registry = () => ({
    register: vi.fn((id: string) => ({ id, active: true, dispose: vi.fn() })),
    unregister: vi.fn(),
  });
  return {
    routes: registry(),
    navigation: registry(),
    pages: registry(),
    components: registry(),
    styles: registry(),
    themes: registry(),
    shortcuts: registry(),
    commands: registry(),
    registerCleanup: () => ({ active: true, dispose: vi.fn() }),
  } as UICompositionHost;
}

function host(action: (method: string, params?: unknown) => Promise<unknown> = async () => undefined): FullUIHost {
  const state = { activeSessionId: 'fixture-session' } as unknown as FaceStoreState;
  const rpc = {
    capabilities: { protocol_version: 'vivy.rpc.v1', capabilities: ['module.action.invoke'] },
    call: vi.fn((method: string, params?: unknown) => action(method, params)),
    onNotification: vi.fn(() => () => undefined),
    onClose: vi.fn(() => () => undefined),
    close: vi.fn(),
  } as unknown as FullUIHost['rpc'];
  return {
    api: {} as FullUIHost['api'],
    rpc,
    store: {
      getState: () => state,
      getInitialState: () => state,
      setState: vi.fn(),
      subscribe: () => () => undefined,
    },
    router: {
      navigate: vi.fn().mockResolvedValue(undefined),
      invalidate: vi.fn().mockResolvedValue(undefined),
    },
    composition: compositionHost(),
    t: (key) => key,
    registerCleanup: () => ({ active: true, dispose: vi.fn() }),
  };
}

function root(id: string, marker: string, content?: ReactNode): UIRoot {
  return defineUIRoot({
    id,
    render: (receivedHost) => (
      <main data-testid={`${id}-root`}>
        <span data-testid="selected-session">{receivedHost.store.getState().activeSessionId}</span>
        <span data-testid={`${id}-marker`}>{marker}</span>
        {content}
      </main>
    ),
  });
}

function extension(id: string, marker: string, events: string[] = []): UIExtension {
  return {
    id,
    install: (receivedHost: FullUIHost) => {
      events.push(`install:${id}`);
      receivedHost.composition.navigation.register(`${id}:navigation`, { label: marker, to: `/${id}` });
      receivedHost.composition.routes.register(`${id}:route`, { path: `/${id}`, render: <article>{marker}</article> });
      receivedHost.composition.styles.register(`${id}:style`, `[data-testid="${id}-marker"] { color: red; }`);
      receivedHost.composition.themes.register(`${id}:theme`, { variables: { '--fixture-ui-generation': id } });
      return () => { events.push(`cleanup:${id}`); };
    },
  };
}

function fixtureGenerations(): readonly FixtureGeneration[] {
  const events: string[] = [];
  const defaultRoot = root('fixture/default-root', 'DEFAULT_GENERATION_ROOT');
  const extensionRoot = root('fixture/extension-root', 'EXTENSION_GENERATION_ROOT');
  const replacementRoot = root('fixture/replacement-root', 'REPLACEMENT_ROOT_GENERATION');
  const extensionModule = extension('fixture/extension', 'EXTENSION_GENERATION_MARKER', events);
  const replacementExtension = extension('fixture/replacement-extension', 'REPLACEMENT_EXTENSION_MARKER', events);
  return [
    {
      name: 'default',
      root: defaultRoot,
      extensions: [],
      provenance: { generationId: 'generation-default', rootId: defaultRoot.id, extensionIds: [] },
      selectedMarkers: ['DEFAULT_GENERATION_ROOT'],
      omittedMarkers: ['EXTENSION_GENERATION_MARKER', 'REPLACEMENT_ROOT_GENERATION', 'REPLACEMENT_EXTENSION_MARKER'],
    },
    {
      name: 'extension',
      root: extensionRoot,
      extensions: [extensionModule],
      provenance: { generationId: 'generation-extension', rootId: extensionRoot.id, extensionIds: [extensionModule.id], sourceHashes: { [extensionModule.id]: 'a'.repeat(64) }, dependencyLockHashes: { [extensionModule.id]: 'b'.repeat(64) }, assetHashes: { [extensionModule.id]: 'c'.repeat(64) } },
      selectedMarkers: ['EXTENSION_GENERATION_ROOT', 'EXTENSION_GENERATION_MARKER'],
      omittedMarkers: ['DEFAULT_GENERATION_ROOT', 'REPLACEMENT_ROOT_GENERATION', 'REPLACEMENT_EXTENSION_MARKER'],
    },
    {
      name: 'replacement-root',
      root: replacementRoot,
      extensions: [replacementExtension],
      provenance: { generationId: 'generation-replacement-root', rootId: replacementRoot.id, extensionIds: [replacementExtension.id], sourceHashes: { [replacementRoot.id]: 'd'.repeat(64) }, dependencyLockHashes: { [replacementRoot.id]: 'e'.repeat(64) }, assetHashes: { [replacementRoot.id]: 'f'.repeat(64) } },
      selectedMarkers: ['REPLACEMENT_ROOT_GENERATION', 'REPLACEMENT_EXTENSION_MARKER'],
      omittedMarkers: ['DEFAULT_GENERATION_ROOT', 'EXTENSION_GENERATION_ROOT', 'EXTENSION_GENERATION_MARKER'],
    },
    {
      name: 'minimal',
      extensions: [],
      provenance: { generationId: 'generation-minimal', rootId: 'default', extensionIds: [] },
      selectedMarkers: ['MINIMAL_ROUTE_TREE'],
      omittedMarkers: ['DEFAULT_GENERATION_ROOT', 'EXTENSION_GENERATION_ROOT', 'EXTENSION_GENERATION_MARKER', 'REPLACEMENT_ROOT_GENERATION', 'REPLACEMENT_EXTENSION_MARKER'],
    },
  ];
}

async function mountFixture(
  fixture: FixtureGeneration,
  selectedHost = host(),
): Promise<{ readonly container: HTMLDivElement; readonly reactRoot: Root }> {
  const container = document.createElement('div');
  document.body.append(container);
  const reactRoot = createRoot(container);
  await act(async () => {
    reactRoot.render(
      <PresentationHost
        host={selectedHost}
        root={fixture.root}
        extensions={fixture.extensions}
        provenance={fixture.provenance}
        children={<article data-testid="default-route">MINIMAL_ROUTE_TREE</article>}
      />,
    );
  });
  return { container, reactRoot };
}

describe('full UI module conformance', () => {
  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    window.history.replaceState({}, '', '/');
  });

  afterEach(() => {
    document.body.querySelectorAll('.vivy-presentation-host').forEach((node) => node.remove());
    document.head.querySelectorAll('[data-vivy-ui-style^="fixture/"]').forEach((style) => style.remove());
    document.documentElement.style.removeProperty('--fixture-ui-generation');
    vi.restoreAllMocks();
  });

  it('builds and presents default, extension, replacement-root, and minimal generations as isolated fixtures', async () => {
    for (const fixture of fixtureGenerations()) {
      const { container, reactRoot } = await mountFixture(fixture);
      const tree = container.querySelector('[data-vivy-presentation-tree]');
      expect(tree, `${fixture.name} should mount one PresentationHost tree`).not.toBeNull();
      const provenance = JSON.parse(tree?.getAttribute('data-vivy-presentation-provenance') ?? '{}') as PresentationProvenance;
      expect(provenance.generationId).toBe(fixture.provenance.generationId);
      expect(provenance.rootId).toBe(fixture.provenance.rootId);
      expect(provenance.extensionIds).toEqual(fixture.provenance.extensionIds);
      for (const marker of fixture.selectedMarkers) expect(container.textContent).toContain(marker);
      for (const marker of fixture.omittedMarkers) expect(container.textContent).not.toContain(marker);
      if (fixture.extensions.length === 0) {
        if (fixture.name === 'minimal') {
          expect(container.querySelector('[data-testid="default-route"]')).not.toBeNull();
        } else {
          expect(container.querySelector('[data-testid="default-route"]')).toBeNull();
        }
        expect(container.querySelector('[data-vivy-presentation-navigation]')).toBeNull();
      } else {
        expect(container.querySelector('[data-vivy-presentation-navigation]')).not.toBeNull();
      }
      await act(async () => reactRoot.unmount());
      expect(document.querySelector(`[data-vivy-ui-style^="${fixture.name === 'extension' ? 'fixture/extension' : fixture.name === 'replacement-root' ? 'fixture/replacement-extension' : 'fixture/'}"]`)).toBeNull();
      container.remove();
    }
  });

  it('rejects missing and duplicate Providers and contradictory extension order before execution', () => {
    const selectedRoot = root('fixture/root', 'root');
    expect(() => composeUI({ roots: [selectedRoot, root('fixture/second-root', 'second')] })).toThrowError(expect.objectContaining({ code: 'duplicate_root' }));
    expect(() => composeUI({ extensions: [{ install: () => undefined } as unknown as UIExtension] })).toThrowError(expect.objectContaining({ code: 'missing_id' }));
    const duplicate = extension('fixture/duplicate', 'duplicate');
    expect(() => composeUI({ extensions: [duplicate, duplicate] })).toThrowError(expect.objectContaining({ code: 'duplicate_id' }));
    const first = extension('fixture/first', 'first');
    const second = extension('fixture/second', 'second');
    const firstWithEdge = { ...first, before: [second.id] };
    const secondWithEdge = { ...second, before: [first.id] };
    expect(() => composeUI({ extensions: [firstWithEdge, secondWithEdge] })).toThrowError(expect.objectContaining({ code: 'composition_cycle' }));
    expect(() => composeUI({ extensions: [{ extension: first, port: 'std/ui-root@v1' } as never] })).toThrowError(expect.objectContaining({ code: 'port_mismatch' }));
  });

  it('rolls back a runtime install failure and disposes successful extensions in reverse order', async () => {
    const events: string[] = [];
    const successful = extension('fixture/successful', 'SUCCESSFUL', events);
    const failing: UIExtension = {
      id: 'fixture/failing',
      install: (receivedHost) => {
        events.push('install:fixture/failing');
        receivedHost.composition.styles.register('fixture/failing:style', 'body { --fixture-ui-failure: leaked; }');
        throw new Error('runtime install failure');
      },
    };
    const never = extension('fixture/never', 'NEVER', events);
    const { container, reactRoot } = await mountFixture({
      name: 'extension',
      root: root('fixture/failure-root', 'FAILURE_ROOT'),
      extensions: [successful, failing, never],
      provenance: { generationId: 'generation-runtime-failure', rootId: 'fixture/failure-root', extensionIds: [successful.id, failing.id, never.id] },
      selectedMarkers: [],
      omittedMarkers: [],
    });
    expect(container.querySelector('[data-vivy-presentation-error="extension-install"]')).not.toBeNull();
    expect(container.textContent).toContain('runtime install failure');
    expect(events).toEqual(['install:fixture/successful', 'install:fixture/failing', 'cleanup:fixture/successful']);
    expect(document.head.querySelector('[data-vivy-ui-style="fixture/failing:style"]')).toBeNull();
    expect(document.head.querySelector('[data-vivy-ui-style="fixture/successful:style"]')).toBeNull();
    expect(document.documentElement.style.getPropertyValue('--fixture-ui-failure')).toBe('');
    await act(async () => reactRoot.unmount());
    container.remove();
  });

  it('keeps cleanup idempotent and records exact reverse order after a successful generation', async () => {
    const events: string[] = [];
    const extensions = ['first', 'second', 'third'].map((id) => extension(`fixture/${id}`, id.toUpperCase(), events));
    const { container, reactRoot } = await mountFixture({
      name: 'extension',
      root: root('fixture/order-root', 'ORDER_ROOT'),
      extensions,
      provenance: { generationId: 'generation-order', rootId: 'fixture/order-root', extensionIds: extensions.map((item) => item.id) },
      selectedMarkers: [],
      omittedMarkers: [],
    });
    expect(events).toEqual(['install:fixture/first', 'install:fixture/second', 'install:fixture/third']);
    await act(async () => reactRoot.unmount());
    expect(events).toEqual([
      'install:fixture/first', 'install:fixture/second', 'install:fixture/third',
      'cleanup:fixture/third', 'cleanup:fixture/second', 'cleanup:fixture/first',
    ]);
    await act(async () => reactRoot.unmount());
    expect(events).toHaveLength(6);
    container.remove();
  });

  it('surfaces Action failure through the fixed typed client without adding a UI permission path', async () => {
    const calls: Array<{ readonly method: string; readonly params: unknown }> = [];
    const selectedHost = host(async (method, params) => {
      calls.push({ method, params });
      throw new Error('ActionHost rejected fixture action');
    });
    function ActionSurface({ receivedHost }: { readonly receivedHost: FullUIHost }) {
      const [status, setStatus] = useState('pending');
      useEffect(() => {
        let active = true;
        const client = createModuleActionClient(receivedHost.rpc);
        void client.invoke({ moduleId: 'fixture/full-ui', actionId: 'fixture.full-ui.echo', input: { text: 'fixture' } })
          .then(() => { if (active) setStatus('unexpected-success'); })
          .catch((error: unknown) => { if (active) setStatus(error instanceof Error ? error.message : String(error)); });
        return () => { active = false; };
      }, [receivedHost]);
      return <main data-testid="action-failure">{status}</main>;
    }
    const actionRoot = defineUIRoot({ id: 'fixture/action-root', render: (receivedHost) => <ActionSurface receivedHost={receivedHost} /> });
    const { container, reactRoot } = await mountFixture({
      name: 'default',
      root: actionRoot,
      extensions: [],
      provenance: { generationId: 'generation-action', rootId: actionRoot.id, extensionIds: [] },
      selectedMarkers: [],
      omittedMarkers: [],
    }, selectedHost);
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(calls).toEqual([{ method: MODULE_ACTION_METHOD, params: { module_id: 'fixture/full-ui', action_id: 'fixture.full-ui.echo', input: { text: 'fixture' } } }]);
    expect(container.querySelector('[data-testid="action-failure"]')?.textContent).toContain('ActionHost rejected fixture action');
    expect(container.textContent?.toLowerCase()).not.toContain('requestpermission');
    await act(async () => reactRoot.unmount());
    container.remove();
  });

  it('proves minimal output removes old v0 UI, route, and permission paths from the selected presentation', async () => {
    const { container, reactRoot } = await mountFixture({
      name: 'minimal',
      extensions: [],
      provenance: { generationId: 'generation-minimal-removal', rootId: 'default', extensionIds: [] },
      selectedMarkers: ['MINIMAL_ROUTE_TREE'],
      omittedMarkers: ['DEFAULT_GENERATION_ROOT', 'EXTENSION_GENERATION_MARKER', 'REPLACEMENT_ROOT_GENERATION'],
    });
    const rendered = `${container.textContent}\n${container.innerHTML}\n${JSON.stringify(UI_ASSEMBLY_MANIFEST)}`;
    for (const legacyPath of ['vivy.plugin/v0', 'vivy.generation/v0', 'ui.full', 'requestPermission', 'permission-dialog', 'legacy-plugin-route']) {
      expect(rendered).not.toContain(legacyPath);
    }
    expect(container.querySelector('[data-vivy-presentation-navigation]')).toBeNull();
    expect(container.querySelector('[data-vivy-presentation-error]')).toBeNull();
    await act(async () => reactRoot.unmount());
    container.remove();
  });
});
