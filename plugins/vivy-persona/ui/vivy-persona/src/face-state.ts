import { useCallback, useSyncExternalStore } from 'react';
import type { FaceStoreState, FullUIHost } from '@vivy/ui-sdk';

/** The host store is the only source of the active, authenticated session. */
export const EMPTY_FACE_STATE = {
  activeSessionId: null,
  connection: 'idle',
  currentRun: null,
} as unknown as FaceStoreState;

export function useFaceState(host?: FullUIHost): FaceStoreState {
  const subscribe = useCallback((listener: () => void) => host?.store.subscribe(listener) ?? (() => undefined), [host]);
  const getSnapshot = useCallback(() => host?.store.getState() ?? EMPTY_FACE_STATE, [host]);
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}
