// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { resetLocaleForTests } from '@/i18n';
import {
  dismissToast,
  notify,
  notifyFailure,
  notifySuccess,
  pauseToasts,
  resetToasts,
  resumeToasts,
  useToasts,
} from './toasts';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let root: Root;
let container: HTMLDivElement;

function Probe() {
  const toasts = useToasts();
  return (
    <ol>
      {toasts.map((toast) => (
        <li key={toast.id} data-count={toast.count} data-tone={toast.tone}>
          <span>{toast.title}</span>
          {toast.detail ? <span data-detail>{toast.detail}</span> : null}
          {toast.action ? <button type="button" data-retry onClick={toast.action.onClick}>{toast.action.label}</button> : null}
        </li>
      ))}
    </ol>
  );
}

function toasts() {
  return [...container.querySelectorAll('li')].map((item) => ({
    count: Number(item.getAttribute('data-count')),
    title: item.querySelector('span')?.textContent ?? '',
  }));
}

function advance(ms: number) {
  act(() => {
    vi.advanceTimersByTime(ms);
  });
}

beforeEach(() => {
  vi.useFakeTimers();
  resetLocaleForTests();
  resetToasts();
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  act(() => root.render(<Probe />));
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  resetToasts();
  vi.useRealTimers();
});

describe('toasts', () => {
  it('merges a repeat failure into one toast and bumps its count', () => {
    act(() => {
      notify({ tone: 'danger', title: 'Save failed', detail: 'disk full' });
      notify({ tone: 'danger', title: 'Save failed', detail: 'disk full' });
    });

    expect(toasts()).toEqual([{ count: 2, title: 'Save failed' }]);
  });

  it('auto-dismisses a success toast at 2500 ms', () => {
    act(() => {
      notifySuccess('Saved');
    });

    advance(2499);
    expect(toasts()).toHaveLength(1);
    advance(1);
    expect(toasts()).toEqual([]);
  });

  it('keeps a danger toast past ten seconds until dismissed', () => {
    let id = '';
    act(() => {
      id = notifyFailure(new Error('Failed to fetch'), { title: 'Connect failed' });
    });

    advance(10_000);
    expect(toasts()).toEqual([{ count: 1, title: 'Connect failed' }]);
    act(() => dismissToast(id));
    expect(toasts()).toEqual([]);
  });

  it('renders newest first and drops the oldest beyond three', () => {
    act(() => {
      notify({ tone: 'warn', title: 'one' });
      notify({ tone: 'warn', title: 'two' });
      notify({ tone: 'warn', title: 'three' });
      notify({ tone: 'warn', title: 'four' });
    });

    expect(toasts().map((toast) => toast.title)).toEqual(['four', 'three', 'two']);
  });

  it('pauses and resumes with the remaining time intact', () => {
    act(() => {
      notify({ tone: 'info', title: 'Syncing' });
    });

    advance(1000);
    act(() => pauseToasts());
    advance(5000);
    expect(toasts()).toHaveLength(1);

    act(() => resumeToasts());
    advance(2499);
    expect(toasts()).toHaveLength(1);
    advance(1);
    expect(toasts()).toEqual([]);
  });

  it('restarts the timer when a repeat extends the same toast', () => {
    act(() => {
      notify({ tone: 'ok', title: 'Saved' });
    });
    advance(2000);
    act(() => {
      notify({ tone: 'ok', title: 'Saved' });
    });
    advance(2000);

    expect(toasts()).toEqual([{ count: 2, title: 'Saved' }]);
  });

  it('gives the failure helper a retry action that calls back', () => {
    const onRetry = vi.fn();
    act(() => {
      notifyFailure(new Error('rpc error: code = Unavailable'), { title: 'Load failed', onRetry });
    });

    const detail = container.querySelector('[data-detail]')?.textContent ?? '';
    expect(container.textContent).toContain('Load failed');
    expect(detail).not.toContain('rpc error');

    act(() => container.querySelector<HTMLButtonElement>('[data-retry]')!.click());
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it('clears pending timers on reset', () => {
    act(() => {
      notify({ tone: 'ok', title: 'Saved' });
    });
    const pending = vi.getTimerCount();
    expect(pending).toBeGreaterThan(0);

    act(() => resetToasts());
    expect(vi.getTimerCount()).toBe(pending - 1);
    advance(5000);
    expect(toasts()).toEqual([]);
  });
});
