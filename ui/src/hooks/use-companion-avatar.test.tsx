// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  COMPANION_AVATAR_SIZE,
  COMPANION_EMOJI_PRESETS,
  COMPANION_PHOTO_MAX_DATA_LENGTH,
  DEFAULT_COMPANION_AVATAR,
  isCompanionAvatar,
  readCompanionAvatar,
  resetCompanionAvatarForTests,
  setCompanionAvatar,
  useCompanionAvatar,
} from './use-companion-avatar';

function stubStorage(values: Map<string, string>) {
  vi.stubGlobal('window', {
    localStorage: {
      getItem: (key: string) => values.get(key) ?? null,
      setItem: (key: string, value: string) => values.set(key, value),
      removeItem: (key: string) => values.delete(key),
    },
  });
}

function storedPhoto(): string {
  return `data:image/webp;base64,${'A'.repeat(64)}`;
}

/** Renders the value the running app actually shows. */
function FaceProbe() {
  const { avatar } = useCompanionAvatar();
  return <span>{avatar.kind === 'emoji' ? avatar.value : 'photo'}</span>;
}

let reactRoot: Root | undefined;
let container: HTMLDivElement | undefined;

afterEach(() => {
  if (reactRoot) act(() => reactRoot?.unmount());
  container?.remove();
  reactRoot = undefined;
  container = undefined;
  resetCompanionAvatarForTests();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

function renderFace(): string {
  container = document.createElement('div');
  document.body.appendChild(container);
  reactRoot = createRoot(container);
  act(() => reactRoot?.render(<FaceProbe />));
  return container.textContent ?? '';
}

describe('companion avatar preference', () => {
  it('uses the default face when storage is empty', () => {
    stubStorage(new Map());
    expect(readCompanionAvatar()).toEqual(DEFAULT_COMPANION_AVATAR);
    expect(COMPANION_EMOJI_PRESETS).toContain(DEFAULT_COMPANION_AVATAR.value);
  });

  it('round-trips a chosen emoji and a chosen photo', () => {
    const values = new Map<string, string>();
    stubStorage(values);
    setCompanionAvatar({ kind: 'emoji', value: COMPANION_EMOJI_PRESETS[3] });
    expect(readCompanionAvatar()).toEqual({ kind: 'emoji', value: COMPANION_EMOJI_PRESETS[3] });

    setCompanionAvatar({ kind: 'photo', value: storedPhoto() });
    expect(readCompanionAvatar()).toEqual({ kind: 'photo', value: storedPhoto() });
    expect(values.get('vivy.companion.avatar')).toContain('"kind":"photo"');
  });

  it('falls back to the default for a value storage cannot be trusted to hold', () => {
    const values = new Map<string, string>();
    values.set('vivy.companion.avatar', '{not json');
    stubStorage(values);
    expect(readCompanionAvatar()).toEqual(DEFAULT_COMPANION_AVATAR);

    // An emoji outside the presets would let polluted storage inject text into
    // the header.
    values.set('vivy.companion.avatar', JSON.stringify({ kind: 'emoji', value: 'not-a-preset' }));
    expect(readCompanionAvatar()).toEqual(DEFAULT_COMPANION_AVATAR);

    // A photo must be an accepted image data URL within the storage budget.
    values.set('vivy.companion.avatar', JSON.stringify({ kind: 'photo', value: 'data:text/html;base64,QQ==' }));
    expect(readCompanionAvatar()).toEqual(DEFAULT_COMPANION_AVATAR);
    values.set('vivy.companion.avatar', JSON.stringify({ kind: 'photo', value: `data:image/png;base64,${'A'.repeat(COMPANION_PHOTO_MAX_DATA_LENGTH)}` }));
    expect(readCompanionAvatar()).toEqual(DEFAULT_COMPANION_AVATAR);
  });

  it('ignores an invalid update instead of storing it', () => {
    const values = new Map<string, string>();
    stubStorage(values);
    setCompanionAvatar({ kind: 'emoji', value: 'not-a-preset' });
    expect(values.has('vivy.companion.avatar')).toBe(false);
    expect(readCompanionAvatar()).toEqual(DEFAULT_COMPANION_AVATAR);
  });

  it('treats an inaccessible localStorage as the default without throwing', () => {
    vi.spyOn(window.localStorage, 'getItem').mockImplementation(() => { throw new Error('unavailable'); });
    vi.spyOn(window.localStorage, 'setItem').mockImplementation(() => { throw new Error('unavailable'); });
    expect(readCompanionAvatar()).toEqual(DEFAULT_COMPANION_AVATAR);
    expect(() => setCompanionAvatar({ kind: 'emoji', value: COMPANION_EMOJI_PRESETS[1] })).not.toThrow();
    // Persistence is best effort: the choice the user just made is the one the
    // app shows for the rest of the session.
    expect(renderFace()).toBe(COMPANION_EMOJI_PRESETS[1]);
  });

  it('notifies a rendered face when the choice changes', () => {
    stubStorage(new Map());
    expect(renderFace()).toBe(DEFAULT_COMPANION_AVATAR.value);
    act(() => setCompanionAvatar({ kind: 'emoji', value: COMPANION_EMOJI_PRESETS[2] }));
    expect(container?.textContent).toBe(COMPANION_EMOJI_PRESETS[2]);
  });

  it('keeps the face guard total', () => {
    expect(isCompanionAvatar(null)).toBe(false);
    expect(isCompanionAvatar('😊')).toBe(false);
    expect(isCompanionAvatar({ kind: 'emoji', value: DEFAULT_COMPANION_AVATAR.value })).toBe(true);
    expect(isCompanionAvatar({ kind: 'photo', value: storedPhoto() })).toBe(true);
    expect(COMPANION_AVATAR_SIZE).toBe(256);
  });
});
