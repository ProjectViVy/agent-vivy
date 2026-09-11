// @vitest-environment happy-dom
import { createHash } from 'node:crypto';
import { StrictMode, act, type ReactElement } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Navigate,
  Outlet,
  RouterProvider,
  useRouterState,
} from '@tanstack/react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type {
  FaceStoreState,
  FullUIHost,
  UIExtension,
  UICompositionHost,
  UIRoot,
} from '@vivy/ui-sdk';
import { useVivyStore } from '@/lib/store';
import {
  createWebFaceHost,
  PRESENTATION_COMMAND_EVENT,
  PresentationHost,
  type PresentationProvenance,
} from './presentation-host';

function compositionHost(): UICompositionHost {
  const registries = Object.fromEntries(
    ['routes', 'navigation', 'pages', 'components', 'styles', 'themes', 'shortcuts', 'commands']
      .map((name) => [name, {
        register: (id: string) => ({ id, active: true, dispose: vi.fn() }),
        unregister: vi.fn(),
      }]),
  ) as unknown as Pick<UICompositionHost, 'routes' | 'navigation' | 'pages' | 'components' | 'styles' | 'themes' | 'shortcuts' | 'commands'>;
  return {
    ...registries,
    registerCleanup: () => ({ active: true, dispose: vi.fn() }),
  };
}

function host(state: Partial<FaceStoreState> = {}, capabilities: string[] = []): FullUIHost {
  const current = state as FaceStoreState;
  return {
    api: {} as FullUIHost['api'],
    rpc: { capabilities: { protocol_version: 'vivy.rpc.v1', capabilities }, call: vi.fn(), onNotification: vi.fn(() => () => undefined), onClose: vi.fn(() => () => undefined), close: vi.fn() },
    store: {
      getState: () => current,
      getInitialState: () => current,
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

function extension(id: string, install: UIExtension['install']): UIExtension {
  return { id, install };
}

function root(render: UIRoot['render']): UIRoot {
  return { id: 'fixture/root', render };
}

const provenance: PresentationProvenance = {
  generationId: 'generation-fixture',
  rootId: 'fixture/root',
  extensionIds: ['fixture/extension'],
};

describe('PresentationHost', () => {
  let container: HTMLDivElement;
  let reactRoot: Root;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    container = document.createElement('div');
    document.body.append(container);
    reactRoot = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => reactRoot.unmount());
    container.remove();
    window.history.replaceState({}, '', '/');
    document.documentElement.style.removeProperty('--fixture-collision');
    document.documentElement.style.removeProperty('--fixture-theme');
    document.documentElement.classList.remove('fixture-theme-class');
    document.documentElement.removeAttribute('data-fixture-theme');
    document.head.querySelectorAll('[data-vivy-ui-style^="fixture-"]').forEach((style) => style.remove());
    vi.restoreAllMocks();
  });

  it('gives selected code full Face control and renders one replacement tree without a UI permission request', async () => {
    const selectedHost = host({ activeSessionId: 'session-current' });
    const registrations: string[] = [];
    const selectedExtension = extension('fixture/extension', (receivedHost) => {
      receivedHost.composition.navigation.register('primary', { href: '/fixture' });
      receivedHost.composition.routes.register('fixture/route', { render: () => null });
      receivedHost.composition.styles.register('fixture/global-style', 'body { outline: 1px solid red; }');
      registrations.push(receivedHost.store.getState().activeSessionId ?? 'missing');
      document.documentElement.dataset.fixtureStyle = 'enabled';
      return () => { delete document.documentElement.dataset.fixtureStyle; };
    });
    const selectedRoot = root((receivedHost) => (
      <main data-testid="replacement-root">
        {receivedHost.store.getState().activeSessionId}
      </main>
    ));

    await act(async () => {
      reactRoot.render(
        <PresentationHost
          host={selectedHost}
          root={selectedRoot}
          extensions={[selectedExtension]}
          provenance={provenance}
        />,
      );
    });

    expect(container.querySelector('[data-testid="replacement-root"]')?.textContent).toBe('session-current');
    expect(registrations).toEqual(['session-current']);
    expect(document.documentElement.dataset.fixtureStyle).toBe('enabled');
    expect(container.querySelector('[data-vivy-presentation-provenance]')).not.toBeNull();
    expect(container.textContent).not.toContain('permission');
  });

  it('installs extensions in exact order and cleans every registration in reverse order once', async () => {
    const events: string[] = [];
    const selectedHost = host();
    const extensions = ['first', 'second', 'third'].map((id) => extension(`fixture/${id}`, () => {
      events.push(`install:${id}`);
      return () => events.push(`cleanup:${id}`);
    }));

    await act(async () => {
      reactRoot.render(<PresentationHost host={selectedHost} extensions={extensions} />);
    });
    expect(events).toEqual(['install:first', 'install:second', 'install:third']);

    await act(async () => reactRoot.unmount());
    expect(events).toEqual([
      'install:first', 'install:second', 'install:third',
      'cleanup:third', 'cleanup:second', 'cleanup:first',
    ]);
    await act(async () => reactRoot.unmount());
    expect(events).toHaveLength(6);
  });

  it('rolls back installed extensions in reverse order when a later install fails', async () => {
    const events: string[] = [];
    const selectedHost = host();
    const extensions = [
      extension('fixture/first', () => { events.push('install:first'); return () => events.push('cleanup:first'); }),
      extension('fixture/failing', () => { events.push('install:failing'); throw new Error('fixture install failed'); }),
      extension('fixture/never', () => { events.push('install:never'); return () => events.push('cleanup:never'); }),
    ];

    await act(async () => {
      reactRoot.render(<PresentationHost host={selectedHost} extensions={extensions} />);
    });

    expect(events).toEqual(['install:first', 'install:failing', 'cleanup:first']);
    expect(container.querySelector('[role="alert"]')?.textContent).toContain('fixture/failing');
    expect(container.querySelector('[role="alert"]')?.textContent).toContain('fixture install failed');
  });

  it('contains root render failures with provenance diagnostics and does not mount a second tree', async () => {
    const renderRoot = vi.fn(() => { throw new Error('fixture root failed'); });
    const selectedHost = host();

    await act(async () => {
      reactRoot.render(
        <PresentationHost
          host={selectedHost}
          root={root(renderRoot)}
          provenance={provenance}
        />,
      );
    });
    expect(renderRoot).toHaveBeenCalledTimes(1);
    expect(container.querySelector('[role="alert"]')?.textContent).toContain('fixture root failed');
    expect(container.querySelectorAll('[data-vivy-presentation-tree]').length).toBe(1);
  });

  it('does not reinstall or rerender the selected root on parent rerenders', async () => {
    const renderRoot = vi.fn((receivedHost: FullUIHost) => (
      <div data-vivy-presentation-tree="selected-root">
        {receivedHost.store.getState().activeSessionId ?? 'none'}
      </div>
    ));
    const install = vi.fn(() => undefined);
    const selectedHost = host({ activeSessionId: 'stable' });
    const selectedRoot = root(renderRoot);
    const selectedExtension = extension('fixture/extension', install);

    await act(async () => {
      reactRoot.render(
        <PresentationHost host={selectedHost} root={selectedRoot} extensions={[selectedExtension]} />,
      );
    });
    await act(async () => {
      reactRoot.render(
        <PresentationHost host={selectedHost} root={selectedRoot} extensions={[selectedExtension]} />,
      );
    });

    expect(install).toHaveBeenCalledTimes(1);
    expect(renderRoot).toHaveBeenCalledTimes(1);
    expect(container.querySelector('[data-vivy-presentation-tree="selected-root"]')?.textContent).toBe('stable');
  });

  it('keeps installation single-shot when React StrictMode replays effects', async () => {
    const events: string[] = [];
    const selectedHost = host();
    const selectedExtension = extension('fixture/strict', () => {
      events.push('install');
      return () => events.push('cleanup');
    });

    await act(async () => {
      reactRoot.render(
        <StrictMode>
          <PresentationHost host={selectedHost} extensions={[selectedExtension]} />
        </StrictMode>,
      );
    });
    expect(events).toEqual(['install']);

    await act(async () => reactRoot.unmount());
    expect(events).toEqual(['install', 'cleanup']);
  });

  it('renders registered navigation and routes, routes clicks through the Face router, and owns global styles', async () => {
    const selectedHost = host();
    const route = <article data-testid="fixture-route">Fixture route</article>;
    const page = <article data-testid="fixture-page">Fixture page</article>;
    const selectedExtension = extension('fixture/live', (receivedHost) => {
      receivedHost.composition.navigation.register('fixture-nav', { label: 'Fixture', to: '/fixture' });
      receivedHost.composition.navigation.register('fixture-page-nav', { label: 'Page', to: '/page' });
      receivedHost.composition.routes.register('fixture-route', { path: '/fixture', render: route });
      receivedHost.composition.pages.register('fixture-page', { path: '/page', render: page });
      receivedHost.composition.styles.register('fixture-style', 'body { --fixture-live: red; }');
      receivedHost.composition.themes.register('fixture-theme', { variables: { '--fixture-theme': 'blue' } });
    });

    await act(async () => {
      reactRoot.render(<PresentationHost host={selectedHost} root={root(() => <main data-testid="base">Base</main>)} extensions={[selectedExtension]} />);
    });

    const navigation = container.querySelector('[data-vivy-presentation-navigation]');
    expect(navigation?.textContent).toContain('Fixture');
    expect(document.head.querySelector('[data-vivy-ui-style="fixture-style"]')?.textContent).toContain('--fixture-live');
    expect(document.documentElement.style.getPropertyValue('--fixture-theme')).toBe('blue');

    await act(async () => {
      (navigation?.querySelector('a') as HTMLAnchorElement).click();
    });
    expect(selectedHost.router.navigate).toHaveBeenCalledWith(expect.objectContaining({ to: '/fixture' }));
    await act(async () => {
      window.history.pushState({}, '', '/fixture');
      window.dispatchEvent(new PopStateEvent('popstate'));
    });
    expect(window.location.pathname).toBe('/fixture');
    expect(container.querySelector('[data-testid="fixture-route"]')?.textContent).toBe('Fixture route');

    await act(async () => {
      (navigation?.querySelector('a[href="/page"]') as HTMLAnchorElement).click();
    });
    expect(selectedHost.router.navigate).toHaveBeenCalledWith(expect.objectContaining({ to: '/page' }));
    await act(async () => {
      window.history.pushState({}, '', '/page');
      window.dispatchEvent(new PopStateEvent('popstate'));
    });
    expect(window.location.pathname).toBe('/page');
    expect(container.querySelector('[data-testid="fixture-page"]')?.textContent).toBe('Fixture page');

    await act(async () => reactRoot.unmount());
    expect(document.head.querySelector('[data-vivy-ui-style="fixture-style"]')).toBeNull();
    expect(document.documentElement.style.getPropertyValue('--fixture-theme')).toBe('');
  });

  it('delegates registered navigation to the real Face router when no module route claims the target', async () => {
    const selectedHost = host();
    const selectedExtension = extension('fixture/real-navigation', (receivedHost) => {
      receivedHost.composition.navigation.register('settings', { label: 'Settings', to: '/settings' });
    });

    await act(async () => {
      reactRoot.render(<PresentationHost host={selectedHost} extensions={[selectedExtension]} />);
    });

    await act(async () => {
      (container.querySelector('a[href="/settings"]') as HTMLAnchorElement).click();
    });
    expect(selectedHost.router.navigate).toHaveBeenCalledWith(expect.objectContaining({ to: '/settings' }));
  });

  it('keeps registration ownership isolated across collisions and late async registration', async () => {
    const selectedHost = host();
    document.documentElement.style.setProperty('--fixture-collision', 'host');
    let registerLate: (() => void) | undefined;
    let lateError: unknown;
    const first = extension('fixture/first-owner', (receivedHost) => {
      receivedHost.composition.themes.register('shared-theme', { variables: { '--fixture-collision': 'first' } });
    });
    const second = extension('fixture/second-owner', (receivedHost) => {
      receivedHost.composition.themes.register('shared-theme', { variables: { '--fixture-collision': 'second' } });
      registerLate = () => {
        try {
          receivedHost.composition.styles.register('late-style', 'body { --fixture-late: leaked; }');
        } catch (error) {
          lateError = error;
        }
      };
    });

    await act(async () => {
      reactRoot.render(<PresentationHost host={selectedHost} extensions={[first, second]} />);
    });
    expect(document.documentElement.style.getPropertyValue('--fixture-collision')).toBe('second');
    await act(async () => reactRoot.unmount());
    registerLate?.();
    expect(document.documentElement.style.getPropertyValue('--fixture-collision')).toBe('host');
    expect(document.head.querySelector('[data-vivy-ui-style="late-style"]')).toBeNull();
    expect(lateError).toBeInstanceOf(Error);
  });

  it('rejects cleanup-time registration before it can reach the host registry', async () => {
    const selectedHost = host();
    let cleanupError: unknown;
    const selectedExtension = extension('fixture/cleanup-registration', (receivedHost) => () => {
      try {
        receivedHost.composition.styles.register('cleanup-style', 'body { --fixture-late: leaked; }');
      } catch (error) {
        cleanupError = error;
      }
    });

    await act(async () => {
      reactRoot.render(<PresentationHost host={selectedHost} extensions={[selectedExtension]} />);
    });
    await act(async () => reactRoot.unmount());

    expect(cleanupError).toBeInstanceOf(Error);
    expect(document.head.querySelector('[data-vivy-ui-style="cleanup-style"]')).toBeNull();
  });

  it('rejects cleanup-time composition cleanup registration before it reaches the source host', async () => {
    const selectedHost = host();
    let cleanupError: unknown;
    const selectedExtension = extension('fixture/cleanup-handle-registration', (receivedHost) => () => {
      try {
        receivedHost.composition.registerCleanup(() => undefined);
      } catch (error) {
        cleanupError = error;
      }
    });

    await act(async () => {
      reactRoot.render(<PresentationHost host={selectedHost} extensions={[selectedExtension]} />);
    });
    await act(async () => reactRoot.unmount());

    expect(cleanupError).toBeInstanceOf(Error);
    expect(String((cleanupError as Error).message)).toContain('fixture/cleanup-handle-registration');
  });

  it('lets each module unregister only its own colliding registration', async () => {
    const selectedHost = host();
    document.documentElement.style.setProperty('--fixture-collision', 'host');
    let unregisterFirst: (() => void) | undefined;
    const first = extension('fixture/first-owner', (receivedHost) => {
      receivedHost.composition.themes.register('shared-theme', { variables: { '--fixture-collision': 'first' } });
      unregisterFirst = () => receivedHost.composition.themes.unregister('shared-theme');
    });
    const second = extension('fixture/second-owner', (receivedHost) => {
      receivedHost.composition.themes.register('shared-theme', { variables: { '--fixture-collision': 'second' } });
      unregisterFirst?.();
    });

    await act(async () => {
      reactRoot.render(<PresentationHost host={selectedHost} extensions={[first, second]} />);
    });
    expect(document.documentElement.style.getPropertyValue('--fixture-collision')).toBe('second');

    await act(async () => reactRoot.unmount());
    expect(document.documentElement.style.getPropertyValue('--fixture-collision')).toBe('host');
  });

  it('logs ordinary-unmount cleanup failures with module and generation provenance', async () => {
    const error = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    const selectedHost = host();
    await act(async () => {
      reactRoot.render(
        <PresentationHost
          host={selectedHost}
          extensions={[extension('fixture/broken-cleanup', () => () => { throw new Error('cleanup exploded'); })]}
          provenance={{ ...provenance, extensionIds: ['fixture/broken-cleanup'] }}
        />,
      );
    });
    await act(async () => reactRoot.unmount());
    expect(error).toHaveBeenCalledWith(
      expect.stringContaining('cleanup failed'),
      expect.objectContaining({ provenance: expect.objectContaining({ generationId: 'generation-fixture' }) }),
    );
  });

  it('rolls back root-component failures and reports cleanup failures with provenance', async () => {
    const error = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    const events: string[] = [];
    function BrokenRootComponent(): ReactElement {
      throw new Error('fixture component failed');
    }
    await act(async () => {
      reactRoot.render(
        <PresentationHost
          host={host()}
          root={root(() => <BrokenRootComponent />)}
          extensions={[extension('fixture/broken-component', () => () => { events.push('cleanup'); throw new Error('component cleanup failed'); })]}
          provenance={{ ...provenance, artifactSha256: 'artifact-fixture', uiArtifactSha256: 'ui-artifact-fixture' }}
        />,
      );
    });
    expect(events).toEqual(['cleanup']);
    const diagnostic = container.querySelector('[role="alert"]');
    expect(diagnostic?.textContent).toContain('fixture component failed');
    expect(diagnostic?.textContent).toContain('component cleanup failed');
    expect(diagnostic?.getAttribute('data-vivy-presentation-provenance')).toContain('artifact-fixture');
    expect(error).toHaveBeenCalledWith(
      expect.stringContaining('cleanup failed'),
      expect.objectContaining({ provenance: expect.objectContaining({ uiArtifactSha256: 'ui-artifact-fixture' }) }),
    );
  });

  it('connects shortcuts and commands to the host dispatch surface', async () => {
    const command = vi.fn();
    const selectedHost = host();
    const selectedExtension = extension('fixture/commands', (receivedHost) => {
      receivedHost.composition.commands.register('save', command);
      receivedHost.composition.shortcuts.register('save-shortcut', { shortcut: 'ctrl+s', command: 'save' });
    });
    await act(async () => {
      reactRoot.render(<PresentationHost host={selectedHost} extensions={[selectedExtension]} />);
    });

    await act(async () => {
      window.dispatchEvent(new CustomEvent(PRESENTATION_COMMAND_EVENT, { detail: { id: 'save', payload: 'event-payload' } }));
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 's', ctrlKey: true, cancelable: true }));
      await Promise.resolve();
    });
    expect(command).toHaveBeenCalledWith('event-payload');
    expect(command).toHaveBeenCalledTimes(2);
  });

  it('contains command failures, identifies their owning extension, and removes its live surface', async () => {
    const selectedHost = host();
    await act(async () => {
      reactRoot.render(
        <PresentationHost
          host={selectedHost}
          extensions={[extension('fixture/failing-command', (receivedHost) => {
            receivedHost.composition.commands.register('explode', () => { throw new Error('command exploded'); });
            receivedHost.composition.navigation.register('explode-nav', { label: 'Explode', to: '/explode' });
          })]}
        />,
      );
    });
    expect(container.querySelector('a[href="/explode"]')).not.toBeNull();

    await act(async () => {
      window.dispatchEvent(new CustomEvent(PRESENTATION_COMMAND_EVENT, { detail: { id: 'explode' } }));
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    const diagnostic = container.querySelector('[role="alert"]');
    expect(diagnostic?.textContent).toContain('fixture/failing-command');
    expect(diagnostic?.textContent).toContain('command exploded');
    expect(container.querySelector('a[href="/explode"]')).toBeNull();
  });

  it('lets an extension observe initialized RPC capabilities during installation', async () => {
    const observed: string[] = [];
    const previousCapabilities = useVivyStore.getState().capabilities;
    useVivyStore.setState({ capabilities: ['session.read', 'module.action.invoke'] });
    try {
      const selectedHost = createWebFaceHost({ navigate: vi.fn().mockResolvedValue(undefined), invalidate: vi.fn().mockResolvedValue(undefined) });
      await act(async () => {
        reactRoot.render(<PresentationHost host={selectedHost} extensions={[extension('fixture/capabilities', (receivedHost) => { observed.push(...receivedHost.rpc.capabilities.capabilities); })]} />);
      });
      expect(observed).toEqual(['session.read', 'module.action.invoke']);
    } finally {
      useVivyStore.setState({ capabilities: previousCapabilities });
    }
  });

  it('resolves explicit plugin catalogs with form and locale fallback without a second locale store', async () => {
    const catalog = {
      apiVersion: 'vivy.i18n/v1',
      schemaVersion: 'vivy.i18n/v1',
      path: 'i18n/catalog.json',
      defaultLocale: 'en',
      locales: ['en', 'zh'],
      module: 'example/search-tools',
      units: {
        'plugin.example/search-tools.results': {
          description: 'Number of search results',
          placeholders: ['count'],
          messages: { en: '{{count}} results', zh: '{{count}} 个结果' },
          short: { en: '{{count}} results' },
        },
      },
    } as const;
    const catalogs = [{ ...catalog, digest: catalogProjectionDigest(catalog) }] as const;
    const selectedHost = createWebFaceHost(
      { navigate: vi.fn().mockResolvedValue(undefined), invalidate: vi.fn().mockResolvedValue(undefined) },
      { catalogs },
    );
    const observed: string[] = [];
    const selectedExtension = extension('example/search-tools', (receivedHost) => {
      observed.push(receivedHost.t('plugin.example/search-tools.results', { count: 3 }, 'short'));
      observed.push(receivedHost.t('plugin.example/search-tools.results', { count: 3 }, 'long'));
      observed.push(receivedHost.t('plugin.example/search-tools.missing'));
    });
    await act(async () => {
      reactRoot.render(<PresentationHost host={selectedHost} extensions={[selectedExtension]} />);
    });
    expect(observed[0]).toContain('3 results');
    expect(observed[1]).toContain('3 results');
    expect(observed[2]).toContain('plugin.example/search-tools.missing');
  });

  it('accepts a valid catalog when unit insertion order differs from its canonical digest order', () => {
    const firstKey = 'plugin.example/search-tools.alpha';
    const secondKey = 'plugin.example/search-tools.beta';
    const canonicalCatalog = {
      apiVersion: 'vivy.i18n/v1',
      schemaVersion: 'vivy.i18n/v1',
      path: 'i18n/catalog.json',
      defaultLocale: 'en',
      locales: ['en'],
      module: 'example/search-tools',
      units: {
        [firstKey]: { description: 'First result', placeholders: [], messages: { en: 'first' } },
        [secondKey]: { description: 'Second result', placeholders: [], messages: { en: 'second' } },
      },
    } as const;
    const reorderedCatalog = {
      ...canonicalCatalog,
      units: {
        [secondKey]: canonicalCatalog.units[secondKey],
        [firstKey]: canonicalCatalog.units[firstKey],
      },
      digest: catalogProjectionDigest(canonicalCatalog),
    } as const;
    const selectedHost = createWebFaceHost(
      { navigate: vi.fn().mockResolvedValue(undefined), invalidate: vi.fn().mockResolvedValue(undefined) },
      { catalogs: [reorderedCatalog] },
    );

    expect(selectedHost.t(firstKey)).toBe('first');
    expect(selectedHost.t(secondKey)).toBe('second');
  });

  it('seeds the production Face adapter from initialized client/store capabilities', () => {
    const faceHost = createWebFaceHost(
      { navigate: vi.fn().mockResolvedValue(undefined), invalidate: vi.fn().mockResolvedValue(undefined) },
      { capabilities: { protocol_version: 'vivy.rpc.v1', capabilities: ['session.read'] } },
    );
    expect(faceHost.rpc.capabilities.protocol_version).toBe('vivy.rpc.v1');
    expect(faceHost.rpc.capabilities.capabilities).toEqual(['session.read']);
  });

  it('fails closed when a generated catalog projection has an ambiguous key', () => {
    const duplicateKey = 'plugin.example/search-tools.results';
    const projection = [
      { module: 'example/search-tools', units: { [duplicateKey]: { messages: { en: 'first' } } } },
      { module: 'example/other-tools', units: { [duplicateKey]: { messages: { en: 'second' } } } },
    ] as const;
    const selectedHost = createWebFaceHost(
      { navigate: vi.fn().mockResolvedValue(undefined), invalidate: vi.fn().mockResolvedValue(undefined) },
      { catalogs: projection },
    );
    expect(selectedHost.t(duplicateKey)).toContain('[missing translation:');
  });

  it('fails closed for duplicate catalog owners and malformed sealed digests', () => {
    const key = 'plugin.example/search-tools.results';
    const duplicateOwner = [
      { module: 'example/search-tools', units: { [key]: { messages: { en: 'first' } } } },
      { module: 'example/search-tools', units: { 'plugin.example/search-tools.other': { messages: { en: 'other' } } } },
    ] as const;
    const malformedDigest = [{ module: 'example/search-tools', digest: 'not-a-sha256', units: { [key]: { messages: { en: 'first' } } } }] as const;
    const router = { navigate: vi.fn().mockResolvedValue(undefined), invalidate: vi.fn().mockResolvedValue(undefined) };
    expect(createWebFaceHost(router, { catalogs: duplicateOwner }).t(key)).toContain('[missing translation:');
    expect(createWebFaceHost(router, { catalogs: malformedDigest }).t(key)).toContain('[missing translation:');
  });

  it('fails closed for a valid-format catalog digest that does not match its body', () => {
    const key = 'plugin.example/search-tools.results';
    const wrongDigest = [{
      apiVersion: 'vivy.i18n/v1',
      schemaVersion: 'vivy.i18n/v1',
      path: 'i18n/catalog.json',
      defaultLocale: 'en',
      locales: ['en'],
      module: 'example/search-tools',
      digest: 'f'.repeat(64),
      units: { [key]: { description: 'Result count', placeholders: [], messages: { en: 'first' } } },
    }] as const;
    const router = { navigate: vi.fn().mockResolvedValue(undefined), invalidate: vi.fn().mockResolvedValue(undefined) };
    expect(createWebFaceHost(router, { catalogs: wrongDigest }).t(key)).toContain('[missing translation:');
  });

  it('fails closed for catalog schema, locale, and placeholder-parity violations', () => {
    const key = 'plugin.example/search-tools.results';
    const base = {
      apiVersion: 'vivy.i18n/v1',
      schemaVersion: 'vivy.i18n/v1',
      path: 'i18n/catalog.json',
      defaultLocale: 'en',
      locales: ['en', 'zh'],
      module: 'example/search-tools',
      digest: 'f'.repeat(64),
      units: { [key]: { description: 'Result count', placeholders: ['count'], messages: { en: '{{count}} results' } } },
    } as const;
    const cases = [
      { name: 'schema', catalog: { ...base, apiVersion: 'vivy.i18n/v2' } },
      { name: 'default locale', catalog: { ...base, defaultLocale: 'zh' } },
      { name: 'locale set', catalog: { ...base, locales: ['zh'] } },
      { name: 'placeholder parity', catalog: { ...base, units: { [key]: { ...base.units[key], messages: { en: 'results' } } } } },
    ];
    const router = { navigate: vi.fn().mockResolvedValue(undefined), invalidate: vi.fn().mockResolvedValue(undefined) };
    for (const test of cases) {
      expect(createWebFaceHost(router, { catalogs: [test.catalog] }).t(key), test.name).toContain('[missing translation:');
    }
  });

  it('uses the actual TanStack router location for registered plugin navigation', async () => {
    let actualRouter: ReturnType<typeof createRouter> | undefined;
    const selectedExtension = extension('fixture/tanstack-navigation', (receivedHost) => {
      receivedHost.composition.navigation.register('fixture-nav', { label: 'Fixture', to: '/fixture' });
      receivedHost.composition.routes.register('fixture-route', { path: '/fixture', render: <article data-testid="tanstack-route">TanStack route</article> });
    });
    const selectedHost = createWebFaceHost({
      navigate: (options) => actualRouter!.navigate(options as never),
      invalidate: () => actualRouter!.invalidate(),
    });
    const rootRoute = createRootRoute({
      notFoundComponent: () => null,
      component: () => {
        const pathname = useRouterState({ select: (state) => state.location.pathname });
        return <PresentationHost host={selectedHost} extensions={[selectedExtension]} path={pathname}><Outlet /></PresentationHost>;
      },
    });
    const indexRoute = createRoute({ getParentRoute: () => rootRoute, path: '/', component: () => null });
    actualRouter = createRouter({
      routeTree: rootRoute.addChildren([indexRoute]),
      history: createMemoryHistory({ initialEntries: ['/'] }),
    });

    await act(async () => {
      reactRoot.render(<RouterProvider router={actualRouter!} />);
    });
    const navigation = container.querySelector('[data-vivy-presentation-navigation]');
    expect(navigation?.textContent).toContain('Fixture');
    await act(async () => {
      (navigation?.querySelector('a[href="/fixture"]') as HTMLAnchorElement).click();
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    expect(actualRouter!.state.location.pathname).toBe('/fixture');
    expect(container.querySelector('[data-testid="tanstack-route"]')?.textContent).toBe('TanStack route');
  });

  it('keeps a direct plugin deep link in the real router and restores NotFound after cleanup', async () => {
    let actualRouter: ReturnType<typeof createRouter> | undefined;
    let disposeRoute: (() => void) | undefined;
    const selectedExtension = extension('fixture/tanstack-deep-link', (receivedHost) => {
      const handle = receivedHost.composition.routes.register('fixture-deep-route', {
        path: '/deep-link',
        render: <article data-testid="deep-link-route">Deep link route</article>,
      });
      disposeRoute = () => handle.dispose();
    });
    const selectedHost = createWebFaceHost({
      navigate: (options) => actualRouter!.navigate(options as never),
      invalidate: () => actualRouter!.invalidate(),
    });
    const rootRoute = createRootRoute({
      notFoundComponent: () => <Navigate to="/" replace />,
      component: () => {
        const pathname = useRouterState({ select: (state) => state.location.pathname });
        return <PresentationHost host={selectedHost} extensions={[selectedExtension]} path={pathname}><Outlet /></PresentationHost>;
      },
    });
    const indexRoute = createRoute({ getParentRoute: () => rootRoute, path: '/', component: () => null });
    actualRouter = createRouter({
      routeTree: rootRoute.addChildren([indexRoute]),
      history: createMemoryHistory({ initialEntries: ['/deep-link'] }),
      notFoundMode: 'fuzzy',
    });

    await act(async () => {
      reactRoot.render(<RouterProvider router={actualRouter!} />);
      await actualRouter!.load();
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    expect(actualRouter!.state.location.pathname).toBe('/deep-link');
    expect(container.querySelector('[data-testid="deep-link-route"]')?.textContent).toBe('Deep link route');

    await act(async () => {
      disposeRoute?.();
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    expect(actualRouter!.state.location.pathname).toBe('/');
    expect(container.querySelector('[data-testid="deep-link-route"]')).toBeNull();
  });
});

function catalogProjectionDigest(catalog: {
  readonly apiVersion: string;
  readonly units: Readonly<Record<string, {
    readonly description: string;
    readonly placeholders: readonly string[];
    readonly messages: Readonly<Record<string, string>>;
    readonly short?: Readonly<Record<string, string>>;
    readonly long?: Readonly<Record<string, string>>;
  }>>;
}): string {
  const units = Object.fromEntries(Object.entries(catalog.units).sort(([left], [right]) => left.localeCompare(right)).map(([key, unit]) => [key, {
    description: unit.description,
    placeholders: [...unit.placeholders],
    messages: Object.fromEntries(Object.entries(unit.messages).sort(([left], [right]) => left.localeCompare(right))),
    ...(unit.short ? { short: Object.fromEntries(Object.entries(unit.short).sort(([left], [right]) => left.localeCompare(right))) } : {}),
    ...(unit.long ? { long: Object.fromEntries(Object.entries(unit.long).sort(([left], [right]) => left.localeCompare(right))) } : {}),
  }]));
  return createHash('sha256').update(JSON.stringify({ apiVersion: catalog.apiVersion, units })).digest('hex');
}
