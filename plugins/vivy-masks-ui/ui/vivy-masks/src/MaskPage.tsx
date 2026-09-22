import {
  type FaceStoreState,
  type FullUIHost,
  type UITranslator,
} from '@vivy/ui-sdk';
import { usePluginHost, usePluginTranslation } from '@vivy/ui-sdk';
import { useCallback, useEffect, useMemo, useReducer, useRef, useSyncExternalStore } from 'react';
import { MaskSelector } from './MaskSelector';
import {
  MaskClient,
  type MaskMetadata,
} from './mask-client';
import {
  initialMaskState,
  maskReducer,
  type MaskDraft,
  type MaskState,
  type MaskUIError,
} from './mask-state';

const EMPTY_FACE_STATE = { activeSessionId: null, currentRun: null, connection: 'idle' } as unknown as FaceStoreState;

export function MaskPage() {
  const host = usePluginHost();
  const { t } = usePluginTranslation();
  const faceState = useFaceState(host);
  const client = useMemo(() => (host ? MaskClient.fromRPC(host.rpc) : null), [host]);
  const [state, dispatch] = useReducer(maskReducer, initialMaskState);
  const stateRef = useRef<MaskState>(state);
  const epochRef = useRef(0);
  const previousConnectionRef = useRef(faceState.connection);
  stateRef.current = state;

  const loadCatalog = useCallback(async () => {
    if (!client) return;
    dispatch({ type: 'catalog/load-start' });
    try {
      const items: MaskMetadata[] = [];
      let afterId = '';
      for (let pageCount = 0; pageCount < 100; pageCount += 1) {
        const page = await client.list({ after_id: afterId, limit: 100 });
        items.push(...page.items);
        if (!page.next_after_id || page.next_after_id === afterId) break;
        afterId = page.next_after_id;
      }
      dispatch({ type: 'catalog/load-success', items });
    } catch (cause) {
      dispatch({ type: 'catalog/load-error', error: toMaskError(cause) });
    }
  }, [client]);

  const loadSelection = useCallback(async (sessionId: string, epoch: number) => {
    if (!client) return;
    dispatch({ type: 'session/load-start', epoch, sessionId });
    try {
      const selection = await client.getSelection(sessionId);
      dispatch({ type: 'session/load-success', epoch, selection });
    } catch (cause) {
      dispatch({ type: 'session/load-error', epoch, sessionId, error: toMaskError(cause) });
    }
  }, [client]);

  const refreshSelection = useCallback((sessionId: string | null) => {
    const epoch = ++epochRef.current;
    dispatch({ type: 'session/change', epoch, sessionId });
    if (sessionId) void loadSelection(sessionId, epoch);
  }, [loadSelection]);

  useEffect(() => {
    void loadCatalog();
  }, [loadCatalog]);

  useEffect(() => {
    refreshSelection(faceState.activeSessionId);
  }, [faceState.activeSessionId, refreshSelection]);

  // A reconnect produces a new connected state in the Face store. Refreshing
  // through the same epoch seam also invalidates responses from the old socket.
  useEffect(() => {
    const previous = previousConnectionRef.current;
    previousConnectionRef.current = faceState.connection;
    if (previous !== faceState.connection && faceState.connection === 'connected') {
      refreshSelection(faceState.activeSessionId);
    }
  }, [faceState.connection, refreshSelection]);

  useEffect(() => {
    if (typeof window === 'undefined') return undefined;
    const onFocus = () => {
      void loadCatalog();
      refreshSelection(stateRef.current.activeSessionId);
    };
    window.addEventListener('focus', onFocus);
    return () => window.removeEventListener('focus', onFocus);
  }, [loadCatalog, refreshSelection]);

  const selectMask = useCallback(async (maskId: string) => {
    if (!client) return;
    const current = stateRef.current;
    const sessionId = current.activeSessionId;
    if (!sessionId || current.selectionPending) return;
    const epoch = current.epoch;
    const expectedRevision = current.selection?.revision ?? 0;
    dispatch({ type: 'selection/save-start', epoch, sessionId });
    try {
      const selection = await client.setSelection({
        session_id: sessionId,
        mask_id: maskId,
        expected_revision: expectedRevision,
      });
      dispatch({ type: 'selection/save-success', epoch, sessionId, selection });
    } catch (cause) {
      dispatch({ type: 'selection/save-error', epoch, sessionId, error: toMaskError(cause) });
      // A transport can fail after the server committed the CAS. Read the
      // authoritative session state through a fresh epoch before showing it.
      refreshSelection(sessionId);
    }
  }, [client, refreshSelection]);

  const openDefinition = useCallback(async (item: MaskMetadata) => {
    const current = stateRef.current;
    const cached = current.definitions[item.id];
    if (cached) {
      dispatch({ type: 'definition/open', definition: cached });
      return;
    }
    if (!client) return;
    dispatch({ type: 'definition/load-start', id: item.id });
    try {
      const definition = await client.get(item.id);
      dispatch({ type: 'definition/open', definition });
    } catch (cause) {
      dispatch({ type: 'definition/load-error', id: item.id, error: toMaskError(cause) });
    }
  }, [client]);

  const saveDraft = useCallback(async () => {
    if (!client) return;
    const draft = stateRef.current.draft;
    if (!draft || stateRef.current.draftSaving) return;
    const operationId = draft.operationId ?? createOperationID();
    dispatch({ type: 'draft/save-start', operationId });
    try {
      const definition = draft.id
        ? await client.update({
          id: draft.id,
          expected_revision: draft.expectedRevision ?? 0,
          name: draft.name,
          description: draft.description,
          body: draft.body,
        })
        : await client.create({
          operation_id: operationId,
          name: draft.name,
          description: draft.description,
          body: draft.body,
        });
      dispatch({ type: 'draft/save-success', definition });
      await loadCatalog();
    } catch (cause) {
      const error = toMaskError(cause);
      dispatch({ type: 'draft/save-error', error });
      if (draft.id && error.code === 'revision_conflict') {
        try {
          const committed = await client.get(draft.id);
          dispatch({ type: 'definition/loaded', definition: committed });
        } catch (rereadCause) {
          dispatch({ type: 'draft/save-error', error: toMaskError(rereadCause) });
        }
      }
      // A create/update response may be ambiguous after the backend commits.
      // Catalog refresh never mutates a dirty draft and exposes the committed row.
      await loadCatalog();
    }
  }, [client, loadCatalog]);

  const deleteDraft = useCallback(async () => {
    if (!client) return;
    const draft = stateRef.current.draft;
    if (!draft?.id || draft.expectedRevision === undefined || stateRef.current.deletingId) return;
    dispatch({ type: 'delete/start', id: draft.id });
    try {
      await client.remove({ id: draft.id, expected_revision: draft.expectedRevision });
      dispatch({ type: 'delete/success', id: draft.id });
      await loadCatalog();
    } catch (cause) {
      dispatch({ type: 'delete/error', error: toMaskError(cause) });
      await loadCatalog();
    }
  }, [client, loadCatalog]);

  if (!host) {
    return <div className="p-6 text-sm text-muted-foreground">{t('plugin.vivy/masks-ui.unavailable')}</div>;
  }

  const activeRun = faceState.currentRun;
  const running = activeRun?.session_id === faceState.activeSessionId && (activeRun.status === 'accepted' || activeRun.status === 'queued' || activeRun.status === 'active');
  const selectedMaskId = state.selection?.mask_id ?? '';
  const errorText = state.error ? formatError(t, state.error) : null;
  const selectedDefinition = state.draft;

  return (
    <main className="flex min-h-0 flex-col gap-4 overflow-auto p-4 sm:p-6" data-testid="mask-page">
      {errorText ? <div className="rounded-md border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive" role="alert">{errorText}</div> : null}

      <MaskSelector
        catalog={state.catalog}
        selectedMaskId={selectedMaskId}
        activeSessionId={faceState.activeSessionId}
        inactiveReason={state.selection?.available ? undefined : state.selection?.inactive_reason}
        disabled={!state.selection?.available || state.selectionPending}
        running={Boolean(running)}
        loading={state.selectionPending}
        onSelect={(maskId) => void selectMask(maskId)}
        t={t}
      />

      <div className="grid min-h-0 gap-4 lg:grid-cols-[minmax(15rem,0.8fr)_minmax(20rem,1.2fr)]">
        <section className="min-h-0 rounded-lg border bg-card p-4" aria-label={t('plugin.vivy/masks-ui.catalogLabel')}>
          <div className="flex items-center justify-between gap-2">
            <h2 className="text-sm font-semibold">{t('plugin.vivy/masks-ui.catalogTitle')}</h2>
            <button type="button" data-mask-action="new" className="rounded-md border px-2 py-1 text-xs hover:bg-muted" onClick={() => dispatch({ type: 'draft/new' })}>
              {t('plugin.vivy/masks-ui.new')}
            </button>
          </div>
          {state.catalogPending && state.catalog.length === 0 ? <p className="mt-4 text-sm text-muted-foreground">{t('plugin.vivy/masks-ui.loading')}</p> : null}
          {!state.catalogPending && state.catalog.length === 0 ? <p className="mt-4 text-sm text-muted-foreground">{t('plugin.vivy/masks-ui.empty')}</p> : null}
          <div className="mt-3 space-y-1">
            {state.catalog.map((item) => (
              <button
                type="button"
                key={item.id}
                className={`w-full rounded-md p-2 text-left text-sm hover:bg-muted ${selectedDefinition?.id === item.id ? 'bg-muted' : ''}`}
                onClick={() => void openDefinition(item)}
              >
                <span className="block truncate font-medium">{item.name}</span>
                <span className="block truncate text-xs text-muted-foreground">{item.built_in ? t('plugin.vivy/masks-ui.builtIn') : t('plugin.vivy/masks-ui.custom')}</span>
              </button>
            ))}
          </div>
        </section>

        <MaskEditor
          draft={selectedDefinition}
          saving={state.draftSaving}
          deleting={state.deletingId !== null}
          pending={selectedDefinition?.id ? state.definitionPending.includes(selectedDefinition.id) : false}
          onChange={(field, value) => dispatch({ type: 'draft/change', field, value })}
          onSave={() => void saveDraft()}
          onDelete={() => void deleteDraft()}
          t={t}
        />
      </div>
    </main>
  );
}

function MaskEditor({
  draft,
  saving,
  deleting,
  pending,
  onChange,
  onSave,
  onDelete,
  t,
}: {
  readonly draft: MaskDraft | null;
  readonly saving: boolean;
  readonly deleting: boolean;
  readonly pending: boolean;
  readonly onChange: (field: 'name' | 'description' | 'body', value: string) => void;
  readonly onSave: () => void;
  readonly onDelete: () => void;
  readonly t: UITranslator;
}) {
  return (
    <section className="min-h-0 rounded-lg border bg-card p-4" aria-label={t('plugin.vivy/masks-ui.editorLabel')}>
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-sm font-semibold">{t('plugin.vivy/masks-ui.editorTitle')}</h2>
        {draft?.id && !draft.id.startsWith('builtin/') ? (
          <button type="button" className="rounded-md border border-destructive/50 px-2 py-1 text-xs text-destructive hover:bg-destructive/10" disabled={deleting || saving} onClick={onDelete}>
            {deleting ? t('plugin.vivy/masks-ui.saving') : t('plugin.vivy/masks-ui.delete')}
          </button>
        ) : null}
      </div>
      {!draft ? <p className="mt-4 text-sm text-muted-foreground">{t('plugin.vivy/masks-ui.editorHint')}</p> : (
        <div className="mt-4 space-y-3">
          <label className="block text-sm">
            <span className="mb-1 block text-xs text-muted-foreground">{t('plugin.vivy/masks-ui.name')}</span>
            <input className="w-full rounded-md border bg-background px-3 py-2" value={draft.name} onChange={(event) => onChange('name', event.target.value)} />
          </label>
          <label className="block text-sm">
            <span className="mb-1 block text-xs text-muted-foreground">{t('plugin.vivy/masks-ui.description')}</span>
            <input className="w-full rounded-md border bg-background px-3 py-2" value={draft.description} onChange={(event) => onChange('description', event.target.value)} />
          </label>
          <label className="block text-sm">
            <span className="mb-1 block text-xs text-muted-foreground">{t('plugin.vivy/masks-ui.body')}</span>
            <textarea className="min-h-48 w-full rounded-md border bg-background px-3 py-2 font-mono text-sm" value={draft.body} onChange={(event) => onChange('body', event.target.value)} />
          </label>
          <div className="flex items-center justify-between gap-2">
            {pending ? <span className="text-xs text-muted-foreground">{t('plugin.vivy/masks-ui.loading')}</span> : <span />}
            <button type="button" className="rounded-md bg-primary px-3 py-2 text-sm text-primary-foreground disabled:opacity-50" disabled={saving || pending || !draft.name.trim() || !draft.body.trim()} onClick={onSave}>
              {saving ? t('plugin.vivy/masks-ui.saving') : t('plugin.vivy/masks-ui.save')}
            </button>
          </div>
        </div>
      )}
    </section>
  );
}

function useFaceState(host?: FullUIHost): FaceStoreState {
  const subscribe = useCallback((listener: () => void) => {
    if (!host) return () => undefined;
    return host.store.subscribe(() => listener());
  }, [host]);
  const getSnapshot = useCallback(() => host?.store.getState() ?? EMPTY_FACE_STATE, [host]);
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

function createOperationID(): string {
  const cryptoObject = globalThis.crypto as Crypto | undefined;
  if (cryptoObject && 'randomUUID' in cryptoObject && typeof cryptoObject.randomUUID === 'function') return cryptoObject.randomUUID();
  const random = Math.random().toString(16).slice(2).padEnd(32, '0').slice(0, 32);
  return `${random.slice(0, 8)}-${random.slice(8, 12)}-4${random.slice(13, 16)}-8${random.slice(17, 20)}-${random.slice(20, 32)}`;
}

function toMaskError(cause: unknown): MaskUIError {
  if (isRecord(cause)) {
    const code = typeof cause.code === 'string' || typeof cause.code === 'number' ? cause.code : undefined;
    const currentRevision = typeof cause.current_revision === 'number' ? cause.current_revision : undefined;
    const referenceCount = typeof cause.reference_count === 'number' ? cause.reference_count : undefined;
    const message = typeof cause.message === 'string' ? cause.message : String(cause);
    return { code, currentRevision, referenceCount, message };
  }
  return { message: cause instanceof Error ? cause.message : String(cause) };
}

function formatError(t: UITranslator, error: MaskUIError): string {
  if (error.code === 'revision_conflict') return t('plugin.vivy/masks-ui.errors.revisionConflict');
  if (error.code === 'mask_in_use') return t('plugin.vivy/masks-ui.errors.maskInUse', { count: error.referenceCount ?? 0 });
  if (error.code === 'authorization_denied') return t('plugin.vivy/masks-ui.errors.authorizationDenied');
  if (error.code === 'mask_unavailable') return t('plugin.vivy/masks-ui.errors.unavailable');
  return error.message;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}
