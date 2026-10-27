// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { resetLocaleForTests, t } from '@/i18n';
import { notify, notifySuccess, resetToasts } from '@/lib/toasts';
import { ToastViewport } from './ToastViewport';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let root: Root;
let container: HTMLDivElement;

function stack() {
  return document.body.querySelector<HTMLElement>('[class*="z-[300]"]')!;
}

function cards() {
  return [...document.body.querySelectorAll<HTMLElement>('[role="alert"], [role="status"]')];
}

beforeEach(() => {
  vi.useFakeTimers();
  resetLocaleForTests();
  resetToasts();
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  act(() => root.render(<ToastViewport />));
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  resetToasts();
  vi.useRealTimers();
});

describe('ToastViewport', () => {
  it('renders the stack newest first with an alert/status role split', () => {
    act(() => {
      notify({ tone: 'danger', title: 'Save failed' });
      notifySuccess('Saved');
    });

    const roles = cards().map((card) => [card.getAttribute('role'), card.textContent?.includes('Save failed') ? 'danger' : 'ok']);
    expect(roles).toEqual([
      ['status', 'ok'],
      ['alert', 'danger'],
    ]);
  });

  it('marks repeated failures with a count badge', () => {
    act(() => {
      notify({ tone: 'warn', title: 'Still syncing', detail: 'slow' });
      notify({ tone: 'warn', title: 'Still syncing', detail: 'slow' });
    });

    const card = cards()[0];
    expect(card.getAttribute('role')).toBe('alert');
    expect(card.textContent).toContain('×2');
  });

  it('dismisses a single toast from its close button', () => {
    act(() => {
      notify({ tone: 'danger', title: 'Save failed' });
      notifySuccess('Saved');
    });

    const close = cards().find((card) => card.textContent?.includes('Save failed'))!.querySelector<HTMLButtonElement>(
      `[aria-label="${t('feedback.dismiss')}"]`,
    )!;
    act(() => close.click());

    expect(cards()).toHaveLength(1);
    expect(cards()[0].textContent).toContain('Saved');
  });

  it('dismisses the top toast on Escape', () => {
    act(() => {
      notify({ tone: 'danger', title: 'older' });
      notify({ tone: 'danger', title: 'newest' });
    });

    act(() => {
      stack().dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    });

    expect(cards()).toHaveLength(1);
    expect(cards()[0].textContent).toContain('older');
  });

  it('pauses auto-dismiss while hovered and resumes on leave', () => {
    act(() => {
      notifySuccess('Saved');
    });

    act(() => {
      stack().dispatchEvent(new MouseEvent('mouseover', { bubbles: true }));
    });
    act(() => {
      vi.advanceTimersByTime(5000);
    });
    expect(cards()).toHaveLength(1);

    act(() => {
      stack().dispatchEvent(new MouseEvent('mouseout', { bubbles: true }));
    });
    act(() => {
      vi.advanceTimersByTime(2500);
    });
    expect(cards()).toHaveLength(0);
  });
});
