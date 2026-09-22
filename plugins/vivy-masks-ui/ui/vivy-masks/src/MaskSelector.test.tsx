// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { MaskSelector } from './MaskSelector';

describe('MaskSelector', () => {
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

  it('keeps the selector disabled when there is no active session', async () => {
    await act(async () => root.render(
      <MaskSelector
        catalog={[{ id: 'builtin/writer', name: 'Writer', description: '', digest: 'a'.repeat(64), generation_id: 'generation', revision: 1, built_in: true }]}
        selectedMaskId=""
        activeSessionId={null}
        disabled={false}
        running={false}
        loading={false}
        onSelect={() => undefined}
        t={(key) => key}
      />,
    ));

    expect(container.querySelector('select')).toHaveProperty('disabled', true);
    expect(container.textContent).toContain('plugin.vivy/masks-ui.noActiveSession');
  });

  it('labels the selected choice as the next admission while a run is active', async () => {
    await act(async () => root.render(
      <MaskSelector
        catalog={[{ id: 'builtin/writer', name: 'Writer', description: '', digest: 'a'.repeat(64), generation_id: 'generation', revision: 1, built_in: true }]}
        selectedMaskId="builtin/writer"
        activeSessionId="session-1"
        disabled={false}
        running
        loading={false}
        onSelect={() => undefined}
        t={(key) => key}
      />,
    ));

    expect(container.textContent).toContain('plugin.vivy/masks-ui.nextRun');
  });
});
