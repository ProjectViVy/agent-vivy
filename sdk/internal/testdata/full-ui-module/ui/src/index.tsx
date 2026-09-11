import {
  createModuleActionClient,
  defineUIExtension,
  defineUIRoot,
  type FaceStoreState,
  type FullUIHost,
} from '@vivy/ui-sdk';
import { useCallback, useState, useSyncExternalStore } from 'react';

const MODULE_ID = 'fixture/full-ui';
const ACTION_ID = 'fixture.full-ui.echo';

type FixtureActionInput = { readonly message: string };
type FixtureActionResult = { readonly accepted: boolean; readonly message: string };

function useCurrentFaceState(host: FullUIHost): FaceStoreState {
  const subscribe = useCallback(
    (listener: () => void) => host.store.subscribe(() => listener()),
    [host],
  );
  const getSnapshot = useCallback(() => host.store.getState(), [host]);
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

function FixtureRootView({ host }: { readonly host: FullUIHost }) {
  const state = useCurrentFaceState(host);
  const [actionResult, setActionResult] = useState('idle');
  const actionClient = useCallback(() => createModuleActionClient(host.rpc), [host]);
  const invokeAction = async () => {
    setActionResult('pending');
    try {
      const result = await actionClient().invoke<FixtureActionInput, FixtureActionResult>({
        moduleId: MODULE_ID,
        actionId: ACTION_ID,
        input: { message: 'from selected UI' },
      });
      setActionResult(result.accepted ? `accepted: ${result.message}` : 'rejected');
    } catch {
      setActionResult('failed');
    }
  };

  return (
    <main className="fixture-full-ui-root" data-testid="full-ui-root">
      <p className="fixture-full-ui-eyebrow">Selected T2 full-code Module</p>
      <h1>Full UI fixture replacement</h1>
      <p data-testid="full-ui-copy">{host.t('plugin.fixture/full-ui.selected')}</p>
      <dl>
        <dt>Live client connection</dt>
        <dd data-testid="full-ui-connection">{state.connection}</dd>
        <dt>Live active session</dt>
        <dd data-testid="full-ui-active-session">{state.activeSessionId ?? 'none'}</dd>
        <dt>Live session count</dt>
        <dd data-testid="full-ui-session-count">{state.sessions.length}</dd>
      </dl>
      <button type="button" onClick={() => void invokeAction()}>Invoke fixture action</button>
      <p data-testid="full-ui-action-method">module.action.invoke</p>
      <p data-testid="full-ui-action-result">{actionResult}</p>
    </main>
  );
}

function FixtureRoute() {
  return (
    <main data-testid="full-ui-route" className="fixture-full-ui-route">
      <h1>Selected full UI route</h1>
      <p>The route is provided by the selected extension.</p>
    </main>
  );
}

export const root = defineUIRoot({
  id: 'fixture.full-ui.root',
  render: (host) => <FixtureRootView host={host} />,
});

export const extension = defineUIExtension({
  id: 'fixture.full-ui.extension',
  install: (host) => {
    const registrations = [
      host.composition.routes.register('/dashboard', {
        path: '/dashboard',
        render: () => <FixtureRoute />,
      }),
      host.composition.navigation.register('fixture-full-ui', {
        label: 'Full UI fixture',
        to: '/dashboard',
      }),
      host.composition.styles.register('fixture-full-ui-style', {
        css: [
          ':root { --fixture-full-ui-accent: rgb(124 58 237); }',
          'body { background-color: rgb(248 245 255) !important; }',
          '.fixture-full-ui-root { border-top: 4px solid var(--fixture-full-ui-accent); }',
          '.fixture-full-ui-eyebrow { color: var(--fixture-full-ui-accent); }',
        ].join('\n'),
      }),
      host.composition.themes.register('fixture-full-ui-theme', {
        variables: { '--fixture-full-ui-accent': 'rgb(124 58 237)' },
        classes: ['fixture-full-ui-theme'],
        attributes: { 'data-vivy-full-ui-theme': 'active' },
      }),
    ];
    return () => {
      for (const registration of registrations.reverse()) registration.dispose();
    };
  },
});
