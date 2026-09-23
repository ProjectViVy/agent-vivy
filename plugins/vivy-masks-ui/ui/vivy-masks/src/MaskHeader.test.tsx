// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { PluginHostProvider, type FullUIHost } from '@vivy/ui-sdk';
import { MaskHeader } from './MaskHeader';

describe('MaskHeader', () => {
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

  it('renders a disabled selector without an active session', async () => {
    const host = {
      rpc: { call: vi.fn().mockResolvedValue({ items: [], next_after_id: '' }) },
      store: {
        getState: () => ({ activeSessionId: null, currentRun: null, connection: 'connected' }),
        getInitialState: () => ({ activeSessionId: null, currentRun: null, connection: 'connected' }),
        subscribe: () => () => undefined,
      },
      t: (key: string) => key,
    } as unknown as FullUIHost;
    await act(async () => root.render(
      <PluginHostProvider host={host}>
        <MaskHeader context={{ sessionId: null, running: false }} />
      </PluginHostProvider>,
    ));
    expect(container.querySelector('select')).toHaveProperty('disabled', true);
    expect(container.textContent).toContain('plugin.vivy/masks-ui.noActiveSession');
  });
});
