import {
  type FaceStoreState,
  type FullUIHost,
  type UITranslator,
} from '@vivy/ui-sdk';
import { usePluginHost, usePluginTranslation } from '@vivy/ui-sdk';
import { useCallback, useReducer, useRef, useSyncExternalStore } from 'react';
import { Check, CircleHelp } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardFooter } from '@/components/ui/card';
import { MaskIdentity } from './MaskIdentity';
import { MaskSession, maskErrorText, toMaskError, useMaskSession } from './mask-session';
import {
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

export function MaskPage({ session: supplied }: { readonly session?: MaskSession } = {}) {
  const host = usePluginHost();
  const { t } = usePluginTranslation();
  const faceState = useFaceState(host);
  const { session, state: shared } = useMaskSession(host, supplied);
  const client = session?.client;
  const [state, dispatch] = useReducer(maskReducer, initialMaskState);
  const stateRef = useRef<MaskState>(state);
  stateRef.current = state;
  const loadCatalog = useCallback(async () => { await session?.refreshCatalog(); }, [session]);
  const selectMask = (maskId: string) => { void session?.select(maskId); };

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
      dispatch({ type: 'definition/loaded', definition });
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
      // Recover catalog metadata after a possibly committed response failure;
      // editor errors and the unsaved draft remain local to this page.
      await loadCatalog();
      dispatch({ type: 'draft/save-error', error });
      if (draft.id && error.code === 'revision_conflict') {
        try {
          const committed = await client.get(draft.id);
          dispatch({ type: 'definition/loaded', definition: committed });
        } catch (rereadCause) {
          dispatch({ type: 'draft/save-error', error: toMaskError(rereadCause) });
        }
      }
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
      const error = toMaskError(cause);
      await loadCatalog();
      dispatch({ type: 'delete/error', error });
    }
  }, [client, loadCatalog]);

  if (!host) {
    return <div className="p-6 text-sm text-muted-foreground">{t('plugin.vivy/masks-ui.unavailable')}</div>;
  }

  const activeRun = faceState.currentRun;
  const running = activeRun?.session_id === faceState.activeSessionId && (activeRun.status === 'accepted' || activeRun.status === 'queued' || activeRun.status === 'active');
  const selectedMaskId = shared.selection?.mask_id ?? '';
  const canSelect = Boolean(shared.sessionId && shared.selection?.available && !shared.selectionPending);
  const errorText = state.error ? formatError(t, state.error) : shared.error ? maskErrorText(t, shared.error) : null;
  const selectedDefinition = state.draft;

  return (
    <main className="h-full overflow-auto" data-testid="mask-page">
      <div className="mx-auto w-full max-w-6xl p-4 sm:p-6">
        {errorText ? <div className="mb-4 rounded-md border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive" role="alert">{errorText}</div> : null}
        <div className="mb-6 flex flex-wrap items-start justify-between gap-3">
          <p className="max-w-2xl text-sm text-muted-foreground">{!shared.sessionId ? t('plugin.vivy/masks-ui.noActiveSession') : shared.selection?.inactive_reason ? t('plugin.vivy/masks-ui.errors.unavailable') : running ? t('plugin.vivy/masks-ui.nextRun') : t('plugin.vivy/masks-ui.currentSession')}</p>
          <div className="flex items-center gap-2">
            <Badge variant="secondary" className="gap-1.5 px-3 py-1" data-active-mask={selectedMaskId}>
              <Check className="h-3.5 w-3.5" />
              {shared.selectionPending ? t('plugin.vivy/masks-ui.loading') : t('plugin.vivy/masks-ui.current', { name: shared.catalog.find((item) => item.id === selectedMaskId)?.name ?? (selectedMaskId || t('plugin.vivy/masks-ui.unmasked')) })}
            </Badge>
            {selectedMaskId ? <Button variant="ghost" size="sm" disabled={!canSelect} onClick={() => selectMask('')}>{t('plugin.vivy/masks-ui.unmasked')}</Button> : null}
          </div>
        </div>
        <div className="grid gap-6 lg:grid-cols-[minmax(0,1.2fr)_minmax(20rem,0.8fr)]">
          <section aria-label={t('plugin.vivy/masks-ui.catalogLabel')}>
            <div className="mb-3 flex items-start justify-between gap-3">
              <div><h2 className="text-sm font-semibold">{t('plugin.vivy/masks-ui.catalogTitle')}</h2><p className="mt-1 text-sm text-muted-foreground">{t('plugin.vivy/masks-ui.libraryHint')}</p></div>
              <Button variant="outline" size="sm" data-mask-action="new" onClick={() => dispatch({ type: 'draft/new' })}>{t('plugin.vivy/masks-ui.new')}</Button>
            </div>
            {shared.catalogPending && shared.catalog.length === 0 ? <p className="mt-4 text-sm text-muted-foreground">{t('plugin.vivy/masks-ui.loading')}</p> : null}
            {!shared.catalogPending && shared.catalog.length === 0 ? <p className="mt-4 text-sm text-muted-foreground">{t('plugin.vivy/masks-ui.empty')}</p> : null}
            <div className="grid gap-3 sm:grid-cols-2">
              {shared.catalog.map((item) => {
                const active = selectedMaskId === item.id;
                const selected = selectedDefinition?.id === item.id;
                return <Card key={item.id} data-mask-id={item.id} className={`gap-0 overflow-hidden py-0 transition-colors ${selected ? 'border-primary/60' : ''}`}>
                  <button type="button" aria-pressed={selected} className={`flex w-full cursor-pointer items-start gap-3 p-4 text-left transition-colors hover:bg-accent/60 ${selected ? 'bg-accent/35' : ''}`} onClick={() => void openDefinition(item)}>
                    <MaskIdentity id={item.id} />
                    <span className="min-w-0 flex-1"><span className="flex flex-wrap items-center gap-2"><span className="font-medium">{item.name}</span>{active ? <Badge variant="outline" className="text-[11px]">{t('plugin.vivy/masks-ui.currentBadge')}</Badge> : null}</span><span className="mt-1 block text-sm text-muted-foreground">{item.description}</span><span className="mt-2 block text-xs text-muted-foreground">{item.built_in ? t('plugin.vivy/masks-ui.builtIn') : t('plugin.vivy/masks-ui.custom')}</span></span>
                  </button>
                  <CardFooter className="border-t px-4 py-3"><Button data-mask-use size="sm" variant={active ? 'secondary' : 'outline'} disabled={!canSelect || active} onClick={() => selectMask(item.id)}>{active ? <><Check className="mr-1.5 h-3.5 w-3.5" />{t('plugin.vivy/masks-ui.inUse')}</> : t('plugin.vivy/masks-ui.useMask', { name: item.name })}</Button></CardFooter>
                </Card>;
              })}
            </div>
          </section>
          <div className="space-y-4">
            <Card className="py-4"><CardContent className="px-4">
              <MaskEditor
                draft={selectedDefinition}
                saving={state.draftSaving}
                deleting={state.deletingId !== null}
                pending={selectedDefinition?.id ? state.definitionPending.includes(selectedDefinition.id) : false}
                onChange={(field, value) => dispatch({ type: 'draft/change', field, value })}
                onSave={() => void saveDraft()}
                onDelete={() => void deleteDraft()}
                onDuplicate={() => selectedDefinition && dispatch({ type: 'draft/new', draft: { name: t('plugin.vivy/masks-ui.copyName', { name: selectedDefinition.name }), description: selectedDefinition.description, body: selectedDefinition.body } })}
                t={t}
              />
            </CardContent></Card>
            <div className="flex gap-3 rounded-xl border bg-card p-4 text-sm"><CircleHelp className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" /><p className="text-muted-foreground">{t('plugin.vivy/masks-ui.roleHint')}</p></div>
          </div>
        </div>
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
  onDuplicate,
  t,
}: {
  readonly draft: MaskDraft | null;
  readonly saving: boolean;
  readonly deleting: boolean;
  readonly pending: boolean;
  readonly onChange: (field: 'name' | 'description' | 'body', value: string) => void;
  readonly onSave: () => void;
  readonly onDelete: () => void;
  readonly onDuplicate: () => void;
  readonly t: UITranslator;
}) {
  const builtIn = Boolean(draft?.id?.startsWith('builtin/'));
  return (
    <section className="min-h-0" aria-label={t('plugin.vivy/masks-ui.editorLabel')}>
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
            <input readOnly={builtIn} className="w-full rounded-md border bg-background px-3 py-2" value={draft.name} onChange={(event) => onChange('name', event.target.value)} />
          </label>
          <label className="block text-sm">
            <span className="mb-1 block text-xs text-muted-foreground">{t('plugin.vivy/masks-ui.description')}</span>
            <input readOnly={builtIn} className="w-full rounded-md border bg-background px-3 py-2" value={draft.description} onChange={(event) => onChange('description', event.target.value)} />
          </label>
          <label className="block text-sm">
            <span className="mb-1 block text-xs text-muted-foreground">{t('plugin.vivy/masks-ui.body')}</span>
            <textarea readOnly={builtIn} className="min-h-48 w-full rounded-md border bg-background px-3 py-2 font-mono text-sm" value={draft.body} onChange={(event) => onChange('body', event.target.value)} />
          </label>
          <div className="flex items-center justify-between gap-2">
            {pending ? <span className="text-xs text-muted-foreground">{t('plugin.vivy/masks-ui.loading')}</span> : <span />}
            {builtIn ? <Button disabled={pending} onClick={onDuplicate}>{t('plugin.vivy/masks-ui.duplicate')}</Button> : <Button disabled={saving || pending || !draft.name.trim() || !draft.body.trim()} onClick={onSave}>{saving ? t('plugin.vivy/masks-ui.saving') : t('plugin.vivy/masks-ui.save')}</Button>}
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

function formatError(t: UITranslator, error: MaskUIError): string {
  if (error.code === 'revision_conflict') return t('plugin.vivy/masks-ui.errors.revisionConflict');
  if (error.code === 'mask_in_use') return t('plugin.vivy/masks-ui.errors.maskInUse', { count: error.referenceCount ?? 0 });
  if (error.code === 'authorization_denied') return t('plugin.vivy/masks-ui.errors.authorizationDenied');
  if (error.code === 'mask_unavailable') return t('plugin.vivy/masks-ui.errors.unavailable');
  return error.message;
}
