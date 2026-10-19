// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { PluginHostProvider, type FullUIHost } from '@vivy/ui-sdk';
import { MemoryPage } from './page';

vi.mock('./recap', () => ({ RecapWorkspace: () => <div data-testid="recap-workspace" /> }));
vi.mock('./view', () => ({ MemoryView: () => <div data-testid="memory-view" /> }));

function hostWith(capabilities: readonly string[]): FullUIHost {
  return { rpc: { capabilities: { protocol_version: 'vivy.rpc.v1', capabilities } } } as unknown as FullUIHost;
}

// Recap is offered only when the backend declares it; a default build without a
// trusted resolver keeps the established memory view instead of a denied page.
describe('MemoryPage', () => {
  let container: HTMLDivElement;
  let root: Root;
  beforeEach(() => { vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true); container = document.createElement('div'); document.body.append(container); root = createRoot(container); });
  afterEach(async () => { await act(async () => root.unmount()); container.remove(); vi.unstubAllGlobals(); });
  const render = async (capabilities: readonly string[]) => act(async () => root.render(<PluginHostProvider host={hostWith(capabilities)}><MemoryPage /></PluginHostProvider>));

  it('shows the memory view when the backend does not declare recap', async () => {
    await render(['module.action.invoke']);
    expect(container.querySelector('[data-testid="memory-view"]')).not.toBeNull();
    expect(container.querySelector('[data-testid="recap-workspace"]')).toBeNull();
  });
  it('shows the Recap workspace when the backend declares recap', async () => {
    await render(['module.action.invoke', 'recap']);
    expect(container.querySelector('[data-testid="recap-workspace"]')).not.toBeNull();
  });
});
