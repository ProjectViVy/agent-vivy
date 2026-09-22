// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { hydrateLocale, resetLocaleForTests } from '@/i18n';
import { useVivyStore } from '@/lib/store';
import { CodeModeControl } from './CodeModeControl';

describe('CodeModeControl', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    hydrateLocale('en');
    useVivyStore.setState({
      activeSessionId: 'session-code',
      codeModeAvailable: true,
      codeMode: false,
    });
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    useVivyStore.setState({ activeSessionId: null, codeModeAvailable: false, codeMode: false });
    resetLocaleForTests();
  });

  async function render(): Promise<void> {
    await act(async () => root.render(<CodeModeControl />));
  }

  it('toggles code mode without depending on the mask selection', async () => {
    await render();
    const toggle = container.querySelector<HTMLButtonElement>('[data-code-mode-control]');
    expect(toggle).not.toBeNull();
    expect(toggle?.getAttribute('aria-pressed')).toBe('false');

    await act(async () => toggle?.click());
    expect(useVivyStore.getState().codeMode).toBe(true);
    expect(toggle?.getAttribute('aria-pressed')).toBe('true');

    // The mask catalog is a separate UI authority in the current shell. A
    // mask event must not turn code mode off once the user chose it.
    window.dispatchEvent(new Event('vivy.ui.activeMask.changed'));
    expect(useVivyStore.getState().codeMode).toBe(true);
  });

  it('does not render a toggle when the backend did not advertise code mode', async () => {
    useVivyStore.setState({ codeModeAvailable: false, codeMode: false });
    await render();
    expect(container.querySelector('[data-code-mode-control]')).toBeNull();
  });
});
