import {
  type FaceStoreState,
  type FullUIHost,
  type UITranslator,
  usePluginHost,
  usePluginTranslation,
} from '@vivy/ui-sdk';
import {
  useCallback,
  useEffect,
  useReducer,
  useRef,
  useState,
  useSyncExternalStore,
} from 'react';
import { useNavigationGuard } from '@/hooks/use-navigation-guard';
import { CircleHelp } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { useIsMobile } from '@/hooks/use-mobile';
import { MaskDetails } from './MaskDetails';
import { MaskEditor } from './MaskEditor';
import { MaskHeader } from './MaskHeader';
import { MaskIdentity } from './MaskIdentity';
import { MaskLibrary } from './MaskLibrary';
import {
  MaskSession,
  maskErrorText,
  toMaskError,
  useMaskSession,
} from './mask-session';
import {
  initialMaskState,
  maskReducer,
  type MaskState,
  type MaskUIError,
} from './mask-state';
import { defaultIdentity, maskKind, type MaskChoice } from './mask-view';

const EMPTY_FACE_STATE = {
  activeSessionId: null,
  currentRun: null,
  connection: 'idle',
} as unknown as FaceStoreState;

export function MaskPage({
  session: supplied,
}: { readonly session?: MaskSession } = {}) {
  const host = usePluginHost();
  const { t } = usePluginTranslation();
  const face = useFaceState(host);
  const mobile = useIsMobile();
  const { session, state: shared } = useMaskSession(host, supplied);
  const [state, dispatch] = useReducer(maskReducer, initialMaskState);
  const stateRef = useRef<MaskState>(state);
  stateRef.current = state;
  const busyRef = useRef(false);
  const discardAction = useRef<(() => void) | null>(null);
  const navigationDecision = useRef<((blocked: boolean) => void) | null>(null);
  const initialPreview = useRef<string | null>(null);
  const [editorOpen, setEditorOpen] = useState(false);
  const [detailsOpen, setDetailsOpen] = useState(false);
  const [discardOpen, setDiscardOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [notice, setNotice] = useState('');
  const choices: readonly MaskChoice[] = [
    defaultIdentity(t),
    ...shared.catalog,
  ];
  const maskId = shared.selection?.mask_id ?? '';
  const known = !shared.sessionId || Boolean(shared.selection);
  const current = choices.find((m) => m.id === maskId);
  const currentName = known
    ? (current?.name ?? maskId)
    : t('plugin.vivy/masks-ui.identityPending');
  const canSelect = Boolean(
    shared.sessionId && shared.selection?.available && !shared.selectionPending,
  );
  const preview =
    choices.find((m) => m.id === state.openedId) ?? defaultIdentity(t);
  const pending =
    state.openedId !== null && state.definitionPending.includes(state.openedId);
  const running =
    face.currentRun?.session_id === shared.sessionId &&
    ['accepted', 'queued', 'active'].includes(face.currentRun?.status ?? '');
  const editorError = state.error ? formatError(t, state.error) : null;

  const openDefinition = useCallback(
    async (id: string) => {
      if (!id) {
        dispatch({ type: 'definition/default' });
        return;
      }
      const cached = stateRef.current.definitions[id];
      const metadata = session
        ?.getSnapshot()
        .catalog.find((item) => item.id === id);
      if (
        cached &&
        cached.digest === metadata?.digest &&
        cached.revision === metadata.revision
      ) {
        dispatch({ type: 'definition/open', definition: cached });
        return;
      }
      if (!session) return;
      const alreadyPending = stateRef.current.definitionPending.includes(id);
      dispatch({ type: 'definition/load-start', id });
      if (alreadyPending) return;
      try {
        const definition = await session.client.get(id);
        if (definition.id !== id)
          throw new Error(t('plugin.vivy/masks-ui.detailsUnavailable'));
        dispatch({ type: 'definition/loaded', definition });
      } catch (cause) {
        dispatch({
          type: 'definition/load-error',
          id,
          error: toMaskError(cause),
        });
      }
    },
    [session, t],
  );

  useEffect(() => {
    if (editorOpen) return;
    if (initialPreview.current === shared.sessionId || !shared.selection)
      return;
    if (
      shared.selection.mask_id &&
      !shared.catalog.some((m) => m.id === shared.selection?.mask_id)
    )
      return;
    initialPreview.current = shared.sessionId;
    void openDefinition(shared.selection.mask_id);
  }, [
    shared.sessionId,
    shared.selection,
    shared.catalog,
    openDefinition,
    editorOpen,
  ]);

  useNavigationGuard(
    editorOpen,
    () =>
      busyRef.current ||
      stateRef.current.draftDirty ||
      Boolean(stateRef.current.draft?.operationId),
    () =>
      busyRef.current
        ? Promise.resolve(true)
        : new Promise<boolean>((resolve) => {
            navigationDecision.current?.(true);
            navigationDecision.current = resolve;
            setDiscardOpen(true);
          }),
  );
  useEffect(
    () => () => {
      navigationDecision.current?.(true);
    },
    [],
  );

  const requestDiscard = (action: () => void) => {
    if (busyRef.current) return;
    if (stateRef.current.draftDirty || stateRef.current.draft?.operationId) {
      discardAction.current = action;
      setDiscardOpen(true);
      return;
    }
    action();
  };
  const closeEditor = () =>
    requestDiscard(() => {
      dispatch({ type: 'draft/cancel' });
      setEditorOpen(false);
    });
  const keepEditing = () => {
    navigationDecision.current?.(true);
    navigationDecision.current = null;
    discardAction.current = null;
    setDiscardOpen(false);
  };
  const discard = () => {
    const decision = navigationDecision.current;
    navigationDecision.current = null;
    decision?.(false);
    if (discardAction.current) discardAction.current();
    else {
      dispatch({ type: 'draft/cancel' });
      setEditorOpen(false);
    }
    discardAction.current = null;
    setDiscardOpen(false);
  };
  const startDraft = (copy = false) => {
    const draft = stateRef.current.draft;
    dispatch({
      type: 'draft/new',
      draft:
        copy && draft
          ? {
              name: t('plugin.vivy/masks-ui.copyName', { name: draft.name }),
              description: draft.description,
              body: draft.body,
            }
          : undefined,
    });
    setDetailsOpen(false);
    setEditorOpen(true);
    setNotice('');
  };
  const select = async (id: string, name?: string) => {
    if (await session?.select(id))
      setNotice(
        t(
          running
            ? 'plugin.vivy/masks-ui.switchedDuringRun'
            : 'plugin.vivy/masks-ui.switched',
          { name: name ?? choices.find((m) => m.id === id)?.name ?? id },
        ),
      );
  };
  const save = async (use: boolean) => {
    const draft = stateRef.current.draft;
    if (
      !session ||
      !draft ||
      busyRef.current ||
      !draft.name.trim() ||
      !draft.body.trim()
    )
      return;
    busyRef.current = true;
    const sessionId = session.getSnapshot().sessionId;
    const operationId = draft.operationId ?? globalThis.crypto.randomUUID();
    dispatch({ type: 'draft/save-start', operationId });
    try {
      const definition = draft.id
        ? await session.client.update({
            id: draft.id,
            expected_revision: draft.expectedRevision ?? 0,
            name: draft.name,
            description: draft.description,
            body: draft.body,
          })
        : await session.client.create({
            operation_id: operationId,
            name: draft.name,
            description: draft.description,
            body: draft.body,
          });
      dispatch({ type: 'draft/save-success', definition });
      session.recordDefinition(definition);
      setEditorOpen(false);
      setNotice(t('plugin.vivy/masks-ui.saved'));
      busyRef.current = false;
      void session.refreshCatalog();
      if (use && sessionId === session.getSnapshot().sessionId && sessionId)
        void select(definition.id, definition.name);
    } catch (cause) {
      const error = toMaskError(cause);
      dispatch({ type: 'draft/save-error', error });
      void session.refreshCatalog();
    } finally {
      busyRef.current = false;
    }
  };
  const reloadLatest = async () => {
    const draft = stateRef.current.draft;
    if (!session || !draft?.id || busyRef.current) return;
    busyRef.current = true;
    dispatch({ type: 'draft/save-start' });
    try {
      const latest = await session.client.get(draft.id);
      if (
        latest.id !== draft.id ||
        latest.revision <= (draft.expectedRevision ?? 0)
      )
        throw new Error(t('plugin.vivy/masks-ui.detailsUnavailable'));
      dispatch({ type: 'definition/open', definition: latest });
    } catch (cause) {
      dispatch({
        type: 'draft/save-error',
        error: {
          ...toMaskError(cause),
          code: 'revision_conflict',
          reloadFailed: true,
        },
      });
    } finally {
      busyRef.current = false;
    }
  };
  const remove = async () => {
    const draft = stateRef.current.draft;
    if (
      !session ||
      !draft?.id ||
      draft.expectedRevision === undefined ||
      busyRef.current
    )
      return;
    busyRef.current = true;
    dispatch({ type: 'delete/start', id: draft.id });
    try {
      await session.client.remove({
        id: draft.id,
        expected_revision: draft.expectedRevision,
      });
      dispatch({ type: 'delete/success', id: draft.id });
      session.recordDeletion(draft.id);
      setDeleteOpen(false);
      setDetailsOpen(false);
      setNotice(t('plugin.vivy/masks-ui.deleted'));
      void session.refreshCatalog();
    } catch (cause) {
      dispatch({ type: 'delete/error', error: toMaskError(cause) });
      void session.refreshCatalog();
    } finally {
      busyRef.current = false;
    }
  };

  if (!host)
    return (
      <div className="p-6 text-sm text-muted-foreground">
        {t('plugin.vivy/masks-ui.unavailable')}
      </div>
    );
  const details = (
    <MaskDetails
      mask={preview}
      draft={state.draft}
      active={known && maskId === preview.id}
      pending={pending}
      canSelect={canSelect}
      onUse={() => void select(preview.id)}
      onEdit={() => {
        dispatch({ type: 'error/clear' });
        setDetailsOpen(false);
        setEditorOpen(true);
      }}
      onCopy={() => startDraft(true)}
      onDelete={() => {
        dispatch({ type: 'error/clear' });
        setDeleteOpen(true);
      }}
      t={t}
    />
  );

  return (
    <main className="h-full overflow-auto bg-muted/15" data-testid="mask-page">
      <div className="mx-auto w-full max-w-6xl space-y-6 p-4 sm:p-6 lg:p-8">
        {shared.error ? (
          <div
            role="alert"
            className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive"
          >
            <span>
              {t('plugin.vivy/masks-ui.connectionError')} ·{' '}
              {maskErrorText(t, shared.error)}
            </span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                void session?.refreshCatalog();
                void session?.refreshSelection();
              }}
            >
              {t('plugin.vivy/masks-ui.retry')}
            </Button>
          </div>
        ) : null}
        {editorError && !editorOpen && !deleteOpen ? (
          <p role="alert" className="text-sm text-destructive">
            {editorError}
          </p>
        ) : null}
        {notice ? (
          <p
            role="status"
            className="rounded-xl bg-emerald-500/10 px-4 py-3 text-sm text-emerald-700 dark:text-emerald-300"
          >
            {notice}
          </p>
        ) : null}
        <section
          className="flex min-w-0 flex-wrap items-center gap-4 rounded-2xl border bg-card p-4 sm:p-5"
          data-active-mask={known ? maskId : undefined}
        >
          <MaskIdentity id={known ? maskId : 'pending'} large={!mobile} />
          <div className="min-w-0 flex-1">
            <p className="text-xs text-muted-foreground">
              {t('plugin.vivy/masks-ui.selectorTitle')}
            </p>
            <h2 className="mt-1 break-words text-lg font-semibold">
              {currentName}
            </h2>
            <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
              {!shared.sessionId
                ? t('plugin.vivy/masks-ui.noActiveSession')
                : shared.selection?.inactive_reason
                  ? t('plugin.vivy/masks-ui.errors.unavailable')
                  : running
                    ? t('plugin.vivy/masks-ui.nextRun')
                    : t('plugin.vivy/masks-ui.currentSession')}
            </p>
          </div>
          {known ? (
            <Badge variant="secondary" className="hidden sm:inline-flex">
              {t('plugin.vivy/masks-ui.currentBadge')}
            </Badge>
          ) : null}
          <MaskHeader
            context={{ sessionId: shared.sessionId, running }}
            session={session}
            label={t('plugin.vivy/masks-ui.changeMask')}
          />
        </section>
        <div className="grid min-w-0 items-start gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(20rem,0.62fr)]">
          <MaskLibrary
            choices={choices}
            selection={{ id: maskId, known, canSelect }}
            openedId={state.openedId}
            catalogPending={shared.catalogPending}
            catalogError={Boolean(shared.catalogError)}
            onView={(id) => {
              void openDefinition(id);
              if (mobile) setDetailsOpen(true);
            }}
            onUse={(id) => void select(id)}
            onNew={() => startDraft()}
            t={t}
          />
          <div className="hidden min-w-0 space-y-4 md:block">
            <div className="rounded-2xl border bg-card p-5 sm:p-6">
              {details}
            </div>
            <div className="flex items-start gap-3 rounded-xl border bg-card p-4 text-xs leading-relaxed text-muted-foreground">
              <CircleHelp className="mt-0.5 h-4 w-4 shrink-0" />
              {t('plugin.vivy/masks-ui.roleHint')}
            </div>
          </div>
        </div>
      </div>
      <Sheet open={mobile && detailsOpen} onOpenChange={setDetailsOpen}>
        <SheetContent side="bottom" className="max-h-[90dvh] rounded-t-2xl">
          <SheetHeader>
            <SheetTitle>{t('plugin.vivy/masks-ui.editorTitle')}</SheetTitle>
          </SheetHeader>
          <SheetBody className="p-5">{mobile ? details : null}</SheetBody>
        </SheetContent>
      </Sheet>
      <MaskEditor
        draft={state.draft}
        open={editorOpen}
        mobile={mobile}
        saving={state.draftSaving}
        canUse={canSelect}
        error={editorError}
        onClose={closeEditor}
        onChange={(field, value) =>
          dispatch({ type: 'draft/change', field, value })
        }
        onSave={(use) => void save(use)}
        onReload={
          state.error?.code === 'revision_conflict' && state.draft?.id
            ? () => requestDiscard(() => void reloadLatest())
            : undefined
        }
        t={t}
      />
      <Dialog
        open={discardOpen}
        onOpenChange={(next) => {
          if (!next) keepEditing();
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('plugin.vivy/masks-ui.discardTitle')}</DialogTitle>
            <DialogDescription>
              {t('plugin.vivy/masks-ui.discardDescription')}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={keepEditing}>
              {t('plugin.vivy/masks-ui.keepEditing')}
            </Button>
            <Button variant="destructive" onClick={discard}>
              {t('plugin.vivy/masks-ui.discard')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <Dialog
        open={deleteOpen}
        onOpenChange={(next) => {
          if (!busyRef.current) setDeleteOpen(next);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t('plugin.vivy/masks-ui.deleteTitle', { name: preview.name })}
            </DialogTitle>
            <DialogDescription>
              {t('plugin.vivy/masks-ui.deleteDescription')}
            </DialogDescription>
          </DialogHeader>
          {maskId === preview.id ? (
            <p className="text-sm text-amber-700 dark:text-amber-300">
              {t('plugin.vivy/masks-ui.deleteCurrentHint')}
            </p>
          ) : null}
          {editorError ? (
            <p role="alert" className="text-sm text-destructive">
              {editorError}
            </p>
          ) : null}
          <DialogFooter>
            <Button
              variant="outline"
              disabled={Boolean(state.deletingId)}
              onClick={() => setDeleteOpen(false)}
            >
              {t('plugin.vivy/masks-ui.cancel')}
            </Button>
            <Button
              variant="destructive"
              disabled={
                Boolean(state.deletingId) ||
                maskId === preview.id ||
                state.error?.code === 'mask_in_use'
              }
              onClick={() => void remove()}
            >
              {t('plugin.vivy/masks-ui.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </main>
  );
}

function useFaceState(host?: FullUIHost): FaceStoreState {
  const subscribe = useCallback(
    (listener: () => void) =>
      host ? host.store.subscribe(() => listener()) : () => undefined,
    [host],
  );
  const getSnapshot = useCallback(
    () => host?.store.getState() ?? EMPTY_FACE_STATE,
    [host],
  );
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}
function formatError(t: UITranslator, error: MaskUIError): string {
  if (error.reloadFailed) return t('plugin.vivy/masks-ui.errors.reloadFailed');
  if (error.code === 'revision_conflict')
    return t('plugin.vivy/masks-ui.errors.revisionConflict');
  if (error.code === 'mask_in_use')
    return t('plugin.vivy/masks-ui.errors.maskInUse', {
      count: error.referenceCount ?? 0,
    });
  return maskErrorText(t, error);
}
