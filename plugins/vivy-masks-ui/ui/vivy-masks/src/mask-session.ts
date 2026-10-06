import type { FullUIHost } from '@vivy/ui-sdk';
import { useEffect, useMemo, useSyncExternalStore } from 'react';
import { MaskClient, type MaskMetadata, type MaskSelection } from './mask-client';
import type { MaskUIError } from './mask-state';

interface SessionState {
  readonly sessionId: string | null;
  readonly catalog: readonly MaskMetadata[];
  readonly catalogPending: boolean;
  readonly selection: MaskSelection | null;
  readonly selectionPending: boolean;
  readonly error: MaskUIError | null;
}
const EMPTY: SessionState = { sessionId: null, catalog: [], catalogPending: false, selection: null, selectionPending: false, error: null };

/** One extension-owned projection of backend state for the page and quick menu. */
export class MaskSession {
  readonly client: MaskClient;
  private state: SessionState = EMPTY;
  private readonly listeners = new Set<() => void>();
  private epoch = 0;
  private catalogEpoch = 0;
  private consumers = 0;
  private write?: { epoch: number; sessionId: string; refreshRequested: boolean };
  private disconnect?: () => void;
  constructor(private readonly host: FullUIHost) { this.client = MaskClient.fromRPC(host.rpc); }
  getSnapshot = () => this.state;
  subscribe = (listener: () => void) => { this.listeners.add(listener); return () => { this.listeners.delete(listener); }; };
  private update(patch: Partial<SessionState>) {
    this.state = { ...this.state, ...patch };
    for (const listener of this.listeners) listener();
  }
  connect = () => {
    if (this.consumers++ === 0) {
      let connection = this.host.store.getState().connection;
      void this.refreshCatalog();
      void this.refreshSelection(this.host.store.getState().activeSessionId);
      const unsubscribe = this.host.store.subscribe(() => {
        const next = this.host.store.getState();
        if (next.activeSessionId !== this.state.sessionId || (next.connection === 'connected' && connection !== 'connected')) {
          void this.refreshSelection(next.activeSessionId);
          if (next.connection === 'connected' && connection !== 'connected') void this.refreshCatalog();
        }
        connection = next.connection;
      });
      const focus = () => { void this.refreshCatalog(); void this.refreshSelection(this.host.store.getState().activeSessionId); };
      window.addEventListener('focus', focus);
      this.disconnect = () => { unsubscribe(); window.removeEventListener('focus', focus); };
    }
    return () => {
      if (--this.consumers === 0) { this.disconnect?.(); this.epoch++; this.catalogEpoch++; }
    };
  };
  async refreshCatalog() {
    const epoch = ++this.catalogEpoch;
    this.update({ catalogPending: true });
    try {
      const items: MaskMetadata[] = [];
      let afterId = '';
      for (let pageIndex = 0; pageIndex < 100; pageIndex++) {
        const page = await this.client.list({ after_id: afterId, limit: 100 });
        items.push(...page.items);
        if (!page.next_after_id || page.next_after_id === afterId) break;
        afterId = page.next_after_id;
      }
      if (epoch === this.catalogEpoch) this.update({ catalog: items, catalogPending: false });
    } catch (cause) {
      if (epoch === this.catalogEpoch) this.update({ catalogPending: false, error: toMaskError(cause) });
    }
  }
  async refreshSelection(sessionId = this.host.store.getState().activeSessionId, preserveError = false) {
    // Reopening the menu or focusing the window must not race a pending SET
    // with a pre-commit GET and discard the write's authoritative response.
    if (this.write?.epoch === this.epoch && this.write.sessionId === sessionId) {
      this.write.refreshRequested = true;
      return;
    }
    const epoch = ++this.epoch;
    this.update({ sessionId, selection: this.state.sessionId === sessionId ? this.state.selection : null, selectionPending: Boolean(sessionId), ...(!preserveError ? { error: null } : {}) });
    if (!sessionId) return;
    try {
      const next = await this.client.getSelection(sessionId);
      if (epoch === this.epoch && next.session_id === this.state.sessionId) this.update({ selection: next, selectionPending: false });
    } catch (cause) {
      if (epoch === this.epoch) this.update({ selectionPending: false, error: toMaskError(cause) });
    }
  }
  async select(maskId: string): Promise<boolean> {
    const { sessionId, selection, selectionPending } = this.state;
    if (!sessionId || selectionPending || !selection?.available || selection.session_id !== sessionId) return false;
    const epoch = this.epoch;
    const write = { epoch, sessionId, refreshRequested: false };
    this.write = write;
    this.update({ selectionPending: true, error: null });
    try {
      const next = await this.client.setSelection({ session_id: sessionId, mask_id: maskId, expected_revision: selection.revision });
      if (epoch !== this.epoch || next.session_id !== this.state.sessionId) return false;
      this.update({ selection: next, selectionPending: false });
      return true;
    } catch (cause) {
      if (epoch === this.epoch && sessionId === this.state.sessionId) {
        this.update({ selectionPending: false, error: toMaskError(cause) });
        // A lost response can follow a committed write. Recover the server's
        // revision while retaining a visible reason for the failed operation.
        this.write = undefined;
        await this.refreshSelection(sessionId, true);
      }
      return false;
    } finally {
      if (this.write === write) {
        this.write = undefined;
        if (write.refreshRequested && epoch === this.epoch) await this.refreshSelection(sessionId, true);
      }
    }
  }
}
export function useMaskSession(host: FullUIHost | undefined, supplied?: MaskSession) {
  const session = useMemo(() => supplied ?? (host ? new MaskSession(host) : undefined), [host, supplied]);
  useEffect(() => session?.connect(), [session]);
  const state = useSyncExternalStore(session?.subscribe ?? noopSubscribe, session?.getSnapshot ?? emptySnapshot, session?.getSnapshot ?? emptySnapshot);
  return { session, state };
}
const noopSubscribe = () => () => {};
const emptySnapshot = () => EMPTY;

export function toMaskError(cause: unknown): MaskUIError {
  if (isRecord(cause)) {
    // JSON-RPC transports expose the mask error code under error.data.code.
    const data = isRecord(cause.data) ? cause.data : undefined;
    const code = typeof data?.code === 'string' || typeof data?.code === 'number'
      ? data.code
      : typeof cause.code === 'string' || typeof cause.code === 'number' ? cause.code : undefined;
    const currentRevision = typeof data?.current_revision === 'number' ? data.current_revision
      : typeof cause.current_revision === 'number' ? cause.current_revision : undefined;
    const referenceCount = typeof data?.reference_count === 'number' ? data.reference_count
      : typeof cause.reference_count === 'number' ? cause.reference_count : undefined;
    const message = typeof cause.message === 'string' ? cause.message : String(cause);
    return { code, currentRevision, referenceCount, message };
  }
  return { message: cause instanceof Error ? cause.message : String(cause) };
}

export function maskErrorText(t: FullUIHost['t'], error: MaskUIError): string {
  if (error.code === 'revision_conflict') return t('plugin.vivy/masks-ui.errors.selectionConflict');
  if (error.code === 'authorization_denied') return t('plugin.vivy/masks-ui.errors.authorizationDenied');
  if (error.code === 'mask_unavailable') return t('plugin.vivy/masks-ui.errors.unavailable');
  return error.message;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}
