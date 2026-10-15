// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { PluginHostProvider, type FaceClientStore, type FaceStoreState, type FullUIHost } from '@vivy/ui-sdk';
import { PersonaOnboardingGate } from './gate';
import { dismissPersonaOnboarding, resetPersonaOnboardingDismissalForTests } from './dismissal';

const DISMISSED_KEY = 'vivy.persona.onboarding.dismissed';

describe('PersonaOnboardingGate', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    window.localStorage.clear();
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    resetPersonaOnboardingDismissalForTests();
    vi.unstubAllGlobals();
  });

  it('opens the guided panel when the authority reports an uninitialized persona', async () => {
    const host = gateHost('session-1', async () => ({ status: 'ok', value: { persona: { state: 'uninitialized', current_revisions: {} } } }));

    await act(async () => root.render(<PluginHostProvider host={host}><PersonaOnboardingGate /></PluginHostProvider>));

    expect(container.querySelector('[role="dialog"]')).not.toBeNull();
    expect(container.textContent).toContain('plugin.vivy/persona.onboarding.philosophy.title');
    expect(container.textContent).toContain('plugin.vivy/persona.onboarding.skip');
  });

  it('stays out of the way once the persona exists', async () => {
    const host = gateHost('session-1', async () => ({ status: 'ok', value: { persona: { state: 'ready', current_revisions: { identity: 1 } } } }));

    await act(async () => root.render(<PluginHostProvider host={host}><PersonaOnboardingGate /></PluginHostProvider>));

    expect(container.querySelector('[role="dialog"]')).toBeNull();
    expect(container.textContent).toBe('');
  });

  it('stays out of the way without a session and when the authority is unavailable', async () => {
    const noSession = gateHost(null, async () => ({ status: 'ok', value: { persona: { state: 'uninitialized', current_revisions: {} } } }));
    await act(async () => root.render(<PluginHostProvider host={noSession}><PersonaOnboardingGate /></PluginHostProvider>));
    expect(noSession.rpc.call).not.toHaveBeenCalled();

    const broken = gateHost('session-1', async () => { throw new Error('authority unavailable'); });
    await act(async () => root.render(<PluginHostProvider host={broken}><PersonaOnboardingGate /></PluginHostProvider>));
    expect(container.querySelector('[role="dialog"]')).toBeNull();
  });

  it('keeps quiet after the user asks to be left alone, on this device', async () => {
    dismissPersonaOnboarding();
    const host = gateHost('session-1', async () => ({ status: 'ok', value: { persona: { state: 'uninitialized', current_revisions: {} } } }));

    await act(async () => root.render(<PluginHostProvider host={host}><PersonaOnboardingGate /></PluginHostProvider>));

    expect(container.querySelector('[role="dialog"]')).toBeNull();
    expect(host.rpc.call).not.toHaveBeenCalled();
  });

  it('records the request to be left alone when skip is pressed', async () => {
    const host = gateHost('session-1', async () => ({ status: 'ok', value: { persona: { state: 'uninitialized', current_revisions: {} } } }));
    await act(async () => root.render(<PluginHostProvider host={host}><PersonaOnboardingGate /></PluginHostProvider>));

    const skip = [...container.querySelectorAll('button')].find((button) => button.textContent === 'plugin.vivy/persona.onboarding.skip');
    expect(skip).not.toBeUndefined();
    await act(async () => (skip as HTMLButtonElement).click());

    expect(window.localStorage.getItem(DISMISSED_KEY)).toBe('1');
    expect(container.querySelector('[role="dialog"]')).toBeNull();
  });
});

function gateHost(activeSessionId: string | null, action: () => Promise<unknown>): FullUIHost {
  const state = { activeSessionId, connection: 'connected', currentRun: null } as unknown as FaceStoreState;
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
    subscribe: () => () => undefined,
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
