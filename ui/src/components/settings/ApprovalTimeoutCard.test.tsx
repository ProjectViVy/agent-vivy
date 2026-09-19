// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { hydrateLocale } from '@/i18n';
import { ApprovalTimeoutCard } from './ApprovalTimeoutCard';

const view = vi.hoisted(() => ({
  settings: {
    provider: 'openai', default_model: 'gpt-4o-mini', base_url: '', read_only: false,
    config_provider: '', config_model: '', locale: 'en' as const, generation_locale: 'en' as const,
    workspace_locale: '' as const, locale_read_only: false,
    sandbox: {
      default_preset: 'smart' as const,
      config_default_preset: 'smart' as const,
      deny_private_ips: true,
      allowed_domains: [] as string[],
      approval_timeout_seconds: 300,
      config_approval_timeout_seconds: 300,
      approval_expiration_seconds: 300,
    },
  },
  saveSettings: vi.fn().mockResolvedValue(undefined),
}));

vi.mock('@/lib/store', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/store')>();
  return {
    ...original,
    useVivyStore: (selector: (state: typeof view) => unknown) => selector(view),
  };
});

describe('ApprovalTimeoutCard', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    hydrateLocale('en');
    view.saveSettings.mockClear();
    view.settings.sandbox.approval_timeout_seconds = 300;
    view.settings.sandbox.approval_expiration_seconds = 300;
    view.settings.sandbox.approval_expiration_seconds = 300;
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    hydrateLocale('en');
    localStorage.clear();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  function button(label: string): HTMLButtonElement {
    const match = [...container.querySelectorAll('button')].find((node) => node.textContent === label);
    expect(match, `button: ${label}`).toBeDefined();
    return match!;
  }

  async function render() {
    await act(async () => root.render(<ApprovalTimeoutCard />));
  }

  it('saves the configured window with the untouched sandbox overlay', async () => {
    await render();
    await act(async () => button('Save approval timeout').click());
    expect(view.saveSettings).toHaveBeenCalledTimes(1);
    const payload = view.saveSettings.mock.calls[0][0];
    expect(payload.sandbox).toEqual({
      default_preset: 'smart',
      deny_private_ips: true,
      allowed_domains: [],
      approval_timeout_seconds: 300,
    });
    const feedback = container.querySelector('[aria-live="polite"]');
    expect(feedback?.textContent).toBe('Approval timeout saved; it applies to new approvals immediately.');
  });

  it('turning the switch off writes an explicit 0 (never auto-approve)', async () => {
    await render();
    const toggle = container.querySelector('[data-testid="approval-timeout-switch"]') as HTMLButtonElement;
    expect(toggle).toBeTruthy();
    await act(async () => toggle.click());
    await act(async () => button('Save approval timeout').click());
    expect(view.saveSettings.mock.calls[0][0].sandbox.approval_timeout_seconds).toBe(0);
  });

  it('caps the input at the approval expiration and rejects out-of-range values', async () => {
    await render();
    const input = container.querySelector('#approval-timeout-seconds') as HTMLInputElement;
    expect(input.max).toBe('300');
    await act(async () => {
      // React tracks the node value, so drive it through the native setter.
      const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')!.set!;
      setter.call(input, '900');
      input.dispatchEvent(new Event('input', { bubbles: true }));
    });
    expect(container.querySelector('[role="alert"]')?.textContent).toContain('300');
    expect(button('Save approval timeout').disabled).toBe(true);
  });

  it('shows the effective window and disables saving when settings are read-only', async () => {
    view.settings.read_only = true;
    await render();
    expect(container.querySelector('[data-testid="approval-timeout-effective"]')?.textContent)
      .toBe('In force now: auto-approved after 300 seconds.');
    expect(button('Save approval timeout').disabled).toBe(true);
    view.settings.read_only = false;
  });
});