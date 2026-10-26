// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { resetLocaleForTests, t } from '@/i18n';
import { FailureNotice } from './FailureNotice';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let root: Root | undefined;
let container: HTMLDivElement | undefined;

function mount(node: React.ReactNode) {
  resetLocaleForTests();
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  act(() => root!.render(node));
  return container;
}

afterEach(() => {
  act(() => root?.unmount());
  container?.remove();
  root = undefined;
  container = undefined;
});

describe('FailureNotice', () => {
  it('renders a quiet alert card for each variant', () => {
    for (const [variant, padding] of [
      ['full', 'p-4'],
      ['compact', 'p-3'],
      ['inline', 'px-2'],
    ] as const) {
      const host = mount(<FailureNotice variant={variant} detail="Could not load" />);
      const alert = host.querySelector('[role="alert"]')!;
      expect(alert.className).toContain('rounded-xl');
      expect(alert.className).toContain('border-l-danger');
      expect(alert.className).toContain(padding);
      expect(alert.textContent).toContain('Could not load');
      act(() => root?.unmount());
      host.remove();
      root = undefined;
    }
  });

  it('keeps raw machine text inside the folded details only', () => {
    const host = mount(<FailureNotice error={new Error('rpc error: code = Internal desc = boom')} />);
    const details = host.querySelector('details')!;
    expect(details.querySelector('pre')!.textContent).toBe('rpc error: code = Internal desc = boom');
    expect(details.querySelector('summary')!.textContent).toBe(t('feedback.details'));
    expect(host.querySelector('p')!.textContent).not.toContain('rpc error');
    expect(details.open).toBe(false);
  });

  it('uses the classifier hint only when no title overrides it', () => {
    const classifierHost = mount(<FailureNotice error={new Error('The model run could not be completed.')} />);
    expect(classifierHost.textContent).toContain(t('errors.runFailedHint'));
    act(() => root?.unmount());
    classifierHost.remove();

    const overridden = mount(<FailureNotice error={new Error('The model run could not be completed.')} title="Refresh failed" />);
    expect(overridden.textContent).toContain('Refresh failed');
    expect(overridden.textContent).not.toContain(t('errors.runFailedHint'));
  });

  it('exposes retry as an outline button and shows the busy label', () => {
    const onRetry = vi.fn();
    const host = mount(<FailureNotice detail="Could not load" onRetry={onRetry} retryLabel="Reload" />);
    const button = host.querySelector('button')!;
    expect(button.textContent).toBe('Reload');
    act(() => button.click());
    expect(onRetry).toHaveBeenCalledTimes(1);

    act(() => root?.render(<FailureNotice detail="Could not load" onRetry={onRetry} retryBusy />));
    expect(host.querySelector('button')!.textContent).toBe(t('errors.retrying'));
    expect(host.querySelector<HTMLButtonElement>('button')!.disabled).toBe(true);
  });

  it('renders nothing without a title or detail', () => {
    const host = mount(<FailureNotice />);
    expect(host.innerHTML).toBe('');
  });
});
