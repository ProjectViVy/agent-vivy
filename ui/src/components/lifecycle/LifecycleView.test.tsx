// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { useVivyStore } from '@/lib/store';
import { LifecycleView } from './LifecycleView';

afterEach(() => useVivyStore.setState(useVivyStore.getInitialState(), true));

describe('LifecycleView inspection identity', () => {
  it('renders the policy profile and opaque generation without retired digest projections', async () => {
    (globalThis as Record<string, unknown>).IS_REACT_ACT_ENVIRONMENT = true;
    useVivyStore.setState({
      lifecyclePhase: 'ready', lifecycleError: null, loadLifecycle: vi.fn(async () => undefined),
      species: JSON.parse(JSON.stringify({
        protocol_version: 'vivy.rpc.v1', binary_id: 'native-binary', generation_id: 'generation-opaque',
        recipe: {}, policy_profile: 'coder', tools: [], grants: [],
        policy_hash: 'retired-policy-digest', artifact_sha256: 'retired-artifact-digest',
      })),
      generations: [], evals: [], promotions: [],
    });
    const container = document.createElement('div');
    const root = createRoot(container);
    try {
      await act(async () => root.render(<LifecycleView />));
      expect(container.textContent).toContain('native-binary');
      expect(container.textContent).toContain('generation-opaque');
      expect(container.textContent).toContain('coder');
      expect(container.textContent).not.toContain('retired-policy-digest');
      expect(container.textContent).not.toContain('retired-artifact-digest');
    } finally {
      act(() => root.unmount());
    }
  });
});
