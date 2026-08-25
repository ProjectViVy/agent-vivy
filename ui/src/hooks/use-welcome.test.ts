import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  completeWelcome,
  isWelcomeCompleted,
  markWelcomeCompleted,
  openWelcome,
  resetWelcomeForTests,
} from './use-welcome';

function stubStorage(values: Map<string, string>) {
  vi.stubGlobal('window', {
    localStorage: {
      getItem: (key: string) => values.get(key) ?? null,
      setItem: (key: string, value: string) => values.set(key, value),
      removeItem: (key: string) => values.delete(key),
    },
  });
}

afterEach(() => {
  resetWelcomeForTests();
  vi.unstubAllGlobals();
});

describe('use-welcome completion flag', () => {
  it('reports not completed when storage is empty', () => {
    stubStorage(new Map());
    expect(isWelcomeCompleted()).toBe(false);
  });

  it('reports completed after the flag is written', () => {
    const values = new Map<string, string>();
    stubStorage(values);
    markWelcomeCompleted();
    expect(isWelcomeCompleted()).toBe(true);
    expect(values.get('vivy.ui.welcome.completed')).toBe('1');
  });

  it('treats an inaccessible localStorage as not completed without throwing', () => {
    vi.stubGlobal('window', {
      localStorage: {
        getItem: () => {
          throw new Error('unavailable');
        },
        setItem: () => {
          throw new Error('unavailable');
        },
      },
    });
    expect(isWelcomeCompleted()).toBe(false);
    expect(() => markWelcomeCompleted()).not.toThrow();
  });
});

describe('use-welcome lifecycle', () => {
  it('completeWelcome persists the flag exactly once', () => {
    const values = new Map<string, string>();
    stubStorage(values);
    openWelcome();
    openWelcome();
    completeWelcome();
    expect(values.get('vivy.ui.welcome.completed')).toBe('1');
    expect(isWelcomeCompleted()).toBe(true);
  });

  it('completeWelcome writes the flag even when the wizard never opened', () => {
    const values = new Map<string, string>();
    stubStorage(values);
    completeWelcome();
    expect(values.get('vivy.ui.welcome.completed')).toBe('1');
  });
});
