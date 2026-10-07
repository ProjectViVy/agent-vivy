import { useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react';
import { AlertTriangle, Check, Code2, Eye, GitPullRequest, Loader2, RefreshCw, Save, X } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { dateTimeLocale } from '@/i18n';
import { usePluginHost, usePluginTranslation, type FaceStoreState, type FullUIHost } from '@vivy/ui-sdk';
import {
  PERSONA_KINDS,
  PersonaClient,
  type PersonaDocument,
  type PersonaInitialization,
  type PersonaKind,
  type PersonaOutcome,
  type PersonaReview,
  type PersonaStatus,
} from './persona-client';

const REQUIRED_KINDS = ['identity', 'relationship', 'redline', 'user', 'world'] as const;

const KIND_LABELS: Record<PersonaKind, string> = {
  identity: 'IDENTITY.MD',
  relationship: 'RELATIONSHIP.MD',
  redline: 'REDLINE.MD',
  user: 'USER.MD',
  world: 'WORLD.MD',
  dream: 'DREAM.MD',
  dark: 'DARK.MD',
  mission: 'MISSION.MD',
};

const EMPTY_INITIALIZATION: PersonaInitialization = {
  identity: '',
  relationship: '',
  redline: '',
  user: '',
  world: '',
};

const EMPTY_FACE_STATE = {
  activeSessionId: null,
  connection: 'idle',
  currentRun: null,
} as unknown as FaceStoreState;

/** The host store is the only source of the active, authenticated session. */
function useFaceState(host?: FullUIHost): FaceStoreState {
  const subscribe = useCallback((listener: () => void) => host?.store.subscribe(listener) ?? (() => undefined), [host]);
  const getSnapshot = useCallback(() => host?.store.getState() ?? EMPTY_FACE_STATE, [host]);
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

export function PersonaMemoryView() {
  const host = usePluginHost();
  const { t } = usePluginTranslation();
  const faceState = useFaceState(host);
  const client = useMemo(() => (host ? PersonaClient.fromRPC(host.rpc) : null), [host]);
  const sessionId = faceState.activeSessionId;
  const [selectedKind, setSelectedKind] = useState<PersonaKind>('identity');
  const [tab, setTab] = useState<'current' | 'pending'>('current');
  const [mode, setMode] = useState<'source' | 'preview'>('source');
  const [status, setStatus] = useState<PersonaStatus | null>(null);
  const [personaDocument, setPersonaDocument] = useState<PersonaDocument | null>(null);
  const [draft, setDraft] = useState('');
  const [reviews, setReviews] = useState<PersonaReview[] | null>(null);
  const [reviewsNextCursor, setReviewsNextCursor] = useState<string | null>(null);
  const [initialization, setInitialization] = useState<PersonaInitialization>(EMPTY_INITIALIZATION);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [sessionCreating, setSessionCreating] = useState(false);
  const [decidingId, setDecidingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const requestSeq = useRef(0);
  const sessionRef = useRef(sessionId);
  const kindRef = useRef(selectedKind);
  sessionRef.current = sessionId;
  kindRef.current = selectedKind;

  const isCurrent = useCallback((seq: number, wantedSession: string, wantedKind: PersonaKind) => (
    seq === requestSeq.current && sessionRef.current === wantedSession && kindRef.current === wantedKind
  ), []);

  const isCurrentSession = useCallback((wantedSession: string) => sessionRef.current === wantedSession, []);

  const outcomeMessage = useCallback(<T,>(outcome: PersonaOutcome<T>, fallback: string) => {
    if (outcome.error?.message) return outcome.error.message;
    if (outcome.error?.code) return outcome.error.code;
    return outcome.status || fallback;
  }, []);

  const load = useCallback(async (wantedSession: string, wantedKind: PersonaKind) => {
    if (!client) return;
    const seq = ++requestSeq.current;
    setLoading(true);
    setError(null);
    setNotice(null);
    setStatus(null);
    setPersonaDocument(null);
    setReviews(null);
    setReviewsNextCursor(null);
    try {
      const statusOutcome = await client.status(wantedSession);
      if (!isCurrent(seq, wantedSession, wantedKind)) return;
      if (statusOutcome.status !== 'ok' || !statusOutcome.value) {
        setError(outcomeMessage(statusOutcome, t('plugin.vivy/persona.errors.loadFailed')));
        return;
      }
      setStatus(statusOutcome.value);
      if (statusOutcome.value.persona.state === 'uninitialized') return;

      const [documentOutcome, reviewOutcome] = await Promise.all([
        client.read(wantedSession, wantedKind),
        client.listReviews(wantedSession, wantedKind),
      ]);
      if (!isCurrent(seq, wantedSession, wantedKind)) return;
      if (documentOutcome.status !== 'ok' || !documentOutcome.value) {
        setError(outcomeMessage(documentOutcome, t('plugin.vivy/persona.errors.loadFailed')));
      } else {
        setPersonaDocument(documentOutcome.value);
        setDraft(documentOutcome.value.content);
      }
      if (reviewOutcome.status !== 'ok' || !reviewOutcome.value) {
        setError((current) => current ?? outcomeMessage(reviewOutcome, t('plugin.vivy/persona.errors.reviewsLoadFailed')));
      } else {
        setReviews([...reviewOutcome.value.items]);
        setReviewsNextCursor(reviewOutcome.value.next_cursor ?? null);
      }
    } catch (cause) {
      if (isCurrent(seq, wantedSession, wantedKind)) setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      if (isCurrent(seq, wantedSession, wantedKind)) setLoading(false);
    }
  }, [client, isCurrent, outcomeMessage, t]);

  useEffect(() => {
    // Invalidate every previous session/kind request before clearing its data.
    requestSeq.current += 1;
    setStatus(null);
    setPersonaDocument(null);
    setDraft('');
    setReviews(null);
    setReviewsNextCursor(null);
    setError(null);
    setNotice(null);
    setLoading(Boolean(sessionId && client));
    setSaving(false);
    setDecidingId(null);
    setInitialization(EMPTY_INITIALIZATION);
    if (sessionId && client) void load(sessionId, selectedKind);
  }, [client, load, selectedKind, sessionId]);

  const dirty = personaDocument !== null && draft !== personaDocument.content;

  const refresh = useCallback(() => {
    if (!sessionId || loading || saving || decidingId) return;
    if (dirty && !window.confirm(t('plugin.vivy/persona.dirtyConfirm'))) return;
    void load(sessionId, selectedKind);
  }, [decidingId, dirty, load, loading, saving, selectedKind, sessionId, t]);

  const handleInitialize = useCallback(async () => {
    if (!client || !sessionId || saving) return;
    if (REQUIRED_KINDS.some((kind) => !initialization[kind].trim())) {
      setError(t('plugin.vivy/persona.initialize.required'));
      return;
    }
    const wantedSession = sessionId;
    const seq = ++requestSeq.current;
    setSaving(true);
    setError(null);
    setNotice(null);
    try {
      const outcome = await client.initialize(wantedSession, initialization, 'Initialize persona from the Persona page');
      if (!isCurrentSession(wantedSession)) return;
      if (outcome.status !== 'ok') {
        setError(outcomeMessage(outcome, t('plugin.vivy/persona.errors.initializeFailed')));
        return;
      }
      setNotice(t('plugin.vivy/persona.initialize.saved'));
      await load(wantedSession, selectedKind);
    } catch (cause) {
      if (isCurrentSession(wantedSession)) setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      if (isCurrentSession(wantedSession) && (seq === requestSeq.current || status?.persona.state === 'uninitialized')) setSaving(false);
    }
  }, [client, initialization, isCurrentSession, load, outcomeMessage, saving, selectedKind, sessionId, status?.persona.state, t]);

  const handleSave = useCallback(async () => {
    if (!client || !sessionId || !personaDocument || !dirty || saving || decidingId) return;
    const wantedSession = sessionId;
    const wantedKind = selectedKind;
    const baseRevision = personaDocument.revision;
    const content = draft;
    const seq = ++requestSeq.current;
    setSaving(true);
    setError(null);
    setNotice(null);
    try {
      const outcome = await client.save(wantedSession, {
        kind: wantedKind,
        content,
        base_revision: baseRevision,
        reason: 'user edit',
      });
      if (!isCurrent(seq, wantedSession, wantedKind)) return;
      if (outcome.status !== 'ok' || !outcome.value) {
        setError(outcomeMessage(outcome, t('plugin.vivy/persona.errors.saveFailed')));
        return;
      }
      setPersonaDocument(outcome.value.document);
      setDraft(outcome.value.document.content);
      setNotice(t('plugin.vivy/persona.saved'));
      const reviewsOutcome = await client.listReviews(wantedSession, wantedKind);
      if (isCurrent(seq, wantedSession, wantedKind) && reviewsOutcome.status === 'ok' && reviewsOutcome.value) {
        setReviews([...reviewsOutcome.value.items]);
        setReviewsNextCursor(reviewsOutcome.value.next_cursor ?? null);
      }
    } catch (cause) {
      if (isCurrent(seq, wantedSession, wantedKind)) setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      if (isCurrent(seq, wantedSession, wantedKind)) setSaving(false);
    }
  }, [client, decidingId, dirty, draft, isCurrent, outcomeMessage, personaDocument, saving, selectedKind, sessionId, t]);

  const handleSelectKind = useCallback((kind: PersonaKind) => {
    if (kind === selectedKind || saving || decidingId) return;
    if (dirty && !window.confirm(t('plugin.vivy/persona.dirtyConfirm'))) return;
    setSelectedKind(kind);
    setTab('current');
    setMode('source');
  }, [decidingId, dirty, saving, selectedKind, t]);

  const handleReviewDecision = useCallback(async (review: PersonaReview, decision: 'accept' | 'reject') => {
    if (!client || !sessionId || decidingId || saving) return;
    if (dirty && !window.confirm(t('plugin.vivy/persona.dirtyConfirm'))) return;
    const wantedSession = sessionId;
    const wantedKind = selectedKind;
    const seq = ++requestSeq.current;
    setDecidingId(review.id);
    setError(null);
    setNotice(null);
    try {
      const outcome = await client.decideReview(wantedSession, review.id, decision);
      if (!isCurrent(seq, wantedSession, wantedKind)) return;
      if (outcome.status !== 'ok') {
        setError(outcomeMessage(outcome, decision === 'accept' ? t('plugin.vivy/persona.errors.acceptFailed') : t('plugin.vivy/persona.errors.rejectFailed')));
        return;
      }
      setNotice(decision === 'accept' ? t('plugin.vivy/persona.review.accepted') : t('plugin.vivy/persona.review.rejected'));
      await load(wantedSession, wantedKind);
    } catch (cause) {
      if (isCurrent(seq, wantedSession, wantedKind)) setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      if (isCurrentSession(wantedSession)) setDecidingId(null);
    }
  }, [client, decidingId, dirty, isCurrent, isCurrentSession, load, outcomeMessage, saving, selectedKind, sessionId, t]);

  const handleCreateSession = useCallback(async () => {
    if (!host || sessionCreating) return;
    setSessionCreating(true);
    setError(null);
    try {
      // This host-owned action creates and selects the session in one
      // operation, keeping the Persona page on the same active session as
      // the shell.
      await host.store.getState().createSession();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setSessionCreating(false);
    }
  }, [host, sessionCreating]);

  if (!host || !client) return <Unavailable message={t('plugin.vivy/persona.unavailable')} />;
  if (!sessionId) {
    return (
      <Unavailable
        message={t('plugin.vivy/persona.noSession')}
        actionLabel={t('plugin.vivy/persona.noSessionCreate')}
        actionBusy={sessionCreating}
        error={error}
        onAction={() => void handleCreateSession()}
      />
    );
  }
  if (loading && !status) {
    return <div className="flex h-full items-center justify-center"><Loader2 className="h-8 w-8 animate-spin text-muted-foreground" /></div>;
  }
  if (status?.persona.state === 'uninitialized') {
    return (
      <InitializationPanel
        values={initialization}
        saving={saving}
        error={error}
        onChange={(kind, value) => setInitialization((current) => ({ ...current, [kind]: value }))}
        onSubmit={() => void handleInitialize()}
        t={t}
      />
    );
  }

  const pendingCount = reviews?.filter((review) => review.state === 'pending').length ?? 0;
  const operationBusy = saving || decidingId !== null;

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="overflow-x-auto border-b p-2">
        <div className="flex min-w-max gap-1">
          {PERSONA_KINDS.map((kind) => (
            <Button
              key={kind}
              variant={selectedKind === kind ? 'default' : 'ghost'}
              size="sm"
              onClick={() => handleSelectKind(kind)}
              disabled={operationBusy}
              className="shrink-0 gap-1.5"
            >
              {KIND_LABELS[kind]}
              {personaDocument?.kind === kind ? <span className="text-[10px] opacity-70">r{personaDocument.revision}</span> : null}
            </Button>
          ))}
        </div>
      </div>

      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <Tabs value={tab} onValueChange={(value) => setTab(value as typeof tab)} className="flex min-h-0 flex-1 flex-col">
          <div className="flex flex-col gap-3 px-4 pt-4 sm:flex-row sm:items-center sm:justify-between">
            <TabsList className="w-full sm:w-auto">
              <TabsTrigger value="current" disabled={operationBusy}>{t('plugin.vivy/persona.current')}</TabsTrigger>
              <TabsTrigger value="pending" disabled={operationBusy} className="gap-1.5">
                {t('plugin.vivy/persona.pending')}
                {pendingCount > 0 ? <Badge variant="secondary" className="h-4 px-1 text-[10px]">{pendingCount}</Badge> : null}
              </TabsTrigger>
            </TabsList>
            <div className="flex flex-wrap gap-2">
              {tab === 'current' ? (
                <>
                  <Button variant={mode === 'source' ? 'default' : 'outline'} size="sm" disabled={operationBusy} onClick={() => setMode('source')}>
                    <Code2 className="mr-1 h-4 w-4" />{t('plugin.vivy/persona.source')}
                  </Button>
                  <Button variant={mode === 'preview' ? 'default' : 'outline'} size="sm" disabled={operationBusy} onClick={() => setMode('preview')}>
                    <Eye className="mr-1 h-4 w-4" />{t('plugin.vivy/persona.preview')}
                  </Button>
                  {dirty ? (
                    <Button onClick={() => void handleSave()} disabled={operationBusy} size="sm">
                      {saving ? <Loader2 className="mr-1 h-4 w-4 animate-spin" /> : <Save className="mr-1 h-4 w-4" />}
                      {t('common.save')}
                    </Button>
                  ) : null}
                </>
              ) : null}
              <Button variant="ghost" size="sm" onClick={refresh} disabled={loading || operationBusy} aria-label={t('common.refresh')}>
                <RefreshCw className={loading ? 'h-4 w-4 animate-spin' : 'h-4 w-4'} />
              </Button>
            </div>
          </div>

          {error ? (
            <div className="mx-4 mt-3 rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive" role="alert">
              <div className="flex items-center gap-2"><AlertTriangle className="h-4 w-4" /><span>{error}</span></div>
              <Button className="mt-2" size="sm" variant="outline" onClick={refresh} disabled={loading || operationBusy}>{t('common.retry')}</Button>
            </div>
          ) : null}
          {notice ? <p className="mx-4 mt-3 text-sm text-emerald-600 dark:text-emerald-400" role="status">{notice}</p> : null}

          <TabsContent value="current" className="mt-0 min-h-0 flex-1 overflow-hidden p-4">
            {mode === 'source' ? (
              <textarea
                name="document"
                value={draft}
                onChange={(event) => setDraft(event.target.value)}
                disabled={operationBusy}
                className="h-full min-h-48 w-full resize-none rounded-lg border bg-background p-3 font-mono text-sm outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring"
                placeholder={t('plugin.vivy/persona.editorPlaceholder')}
                aria-label={KIND_LABELS[selectedKind]}
              />
            ) : (
              <ScrollArea className="h-full rounded-lg border p-4">
                <pre className="whitespace-pre-wrap break-words font-mono text-sm">{draft}</pre>
              </ScrollArea>
            )}
          </TabsContent>

          <TabsContent value="pending" className="mt-0 min-h-0 flex-1 overflow-hidden p-4">
            <ScrollArea className="h-full">
              {reviewsNextCursor ? <p className="mb-3 text-xs text-muted-foreground">{t('plugin.vivy/persona.reviewsLimited')}</p> : null}
              {reviews === null && loading ? (
                <div className="flex justify-center py-12"><Loader2 className="h-6 w-6 animate-spin text-muted-foreground" /></div>
              ) : reviews === null ? null : reviews.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-16 text-muted-foreground">
                  <GitPullRequest className="mb-3 h-10 w-10 opacity-40" />
                  <p className="text-sm">{t('plugin.vivy/persona.emptyPending')}</p>
                </div>
              ) : (
                <div className="space-y-3">
                  {reviews.map((review) => (
                    <ReviewCard key={review.id} review={review} deciding={decidingId === review.id} blocked={operationBusy} onDecision={(decision) => void handleReviewDecision(review, decision)} t={t} />
                  ))}
                </div>
              )}
            </ScrollArea>
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
}

function Unavailable({
  message,
  actionLabel,
  actionBusy = false,
  error,
  onAction,
}: {
  readonly message: string;
  readonly actionLabel?: string;
  readonly actionBusy?: boolean;
  readonly error?: string | null;
  readonly onAction?: () => void;
}) {
  return (
    <div className="flex h-full items-center justify-center overflow-auto p-6 text-center">
      <div className="max-w-md space-y-3">
        <p className="text-sm text-muted-foreground">{message}</p>
        {error ? <p className="text-sm text-destructive" role="alert">{error}</p> : null}
        {actionLabel && onAction ? (
          <Button onClick={onAction} disabled={actionBusy}>
            {actionBusy ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
            {actionLabel}
          </Button>
        ) : null}
      </div>
    </div>
  );
}

function InitializationPanel({
  values,
  saving,
  error,
  onChange,
  onSubmit,
  t,
}: {
  readonly values: PersonaInitialization;
  readonly saving: boolean;
  readonly error: string | null;
  readonly onChange: (kind: keyof PersonaInitialization, value: string) => void;
  readonly onSubmit: () => void;
  readonly t: (key: string, args?: Record<string, string | number>) => string;
}) {
  return (
    <div className="h-full overflow-auto p-4 sm:p-6">
      <Card className="mx-auto max-w-3xl">
        <CardHeader>
          <CardTitle>{t('plugin.vivy/persona.initialize.title')}</CardTitle>
          <p className="text-sm text-muted-foreground">{t('plugin.vivy/persona.initialize.description')}</p>
        </CardHeader>
        <CardContent className="space-y-4">
          {error ? <div className="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive" role="alert">{error}</div> : null}
          {REQUIRED_KINDS.map((kind) => (
            <label key={kind} className="block space-y-1.5">
              <span className="text-sm font-medium">{t(`plugin.vivy/persona.initialize.fields.${kind}`)} <span className="text-destructive">*</span></span>
              <textarea
                name={kind}
                required
                value={values[kind]}
                onChange={(event) => onChange(kind, event.target.value)}
                className="min-h-24 w-full resize-y rounded-lg border bg-background p-3 font-mono text-sm outline-none ring-offset-background focus-visible:ring-2 focus-visible:ring-ring"
              />
            </label>
          ))}
          <Button onClick={onSubmit} disabled={saving}>
            {saving ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Save className="mr-2 h-4 w-4" />}
            {t('plugin.vivy/persona.initialize.submit')}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}

function ReviewCard({
  review,
  deciding,
  blocked,
  onDecision,
  t,
}: {
  readonly review: PersonaReview;
  readonly deciding: boolean;
  readonly blocked: boolean;
  readonly onDecision: (decision: 'accept' | 'reject') => void;
  readonly t: (key: string, args?: Record<string, string | number>) => string;
}) {
  const stateLabel = review.state === 'pending'
    ? t('plugin.vivy/persona.pendingState.pending')
    : review.state === 'accepted'
      ? t('plugin.vivy/persona.pendingState.accepted')
      : review.state === 'rejected'
        ? t('plugin.vivy/persona.pendingState.rejected')
        : review.state;
  return (
    <Card>
      <CardContent className="p-4">
        <div className="mb-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-2">
            <Badge variant={review.state === 'pending' ? 'secondary' : review.state === 'accepted' ? 'default' : 'destructive'}>{stateLabel}</Badge>
            <span className="text-xs text-muted-foreground">{formatDateTime(review.created_at)}</span>
          </div>
          {review.state === 'pending' ? (
            <div className="flex gap-2">
              <Button size="sm" variant="outline" disabled={blocked || deciding} onClick={() => onDecision('accept')}>
                {deciding ? <Loader2 className="mr-1 h-4 w-4 animate-spin" /> : <Check className="mr-1 h-4 w-4" />}
                {t('common.accept')}
              </Button>
              <Button size="sm" variant="outline" disabled={blocked || deciding} onClick={() => onDecision('reject')}>
                <X className="mr-1 h-4 w-4" />{t('common.reject')}
              </Button>
            </div>
          ) : null}
        </div>
        <p className="mb-2 text-sm text-muted-foreground">
          {review.reason ? t('plugin.vivy/persona.review.reason', { reason: review.reason }) : t('plugin.vivy/persona.review.noReason')}
        </p>
        <pre className="overflow-x-auto whitespace-pre-wrap rounded-lg bg-muted p-3 text-xs">{review.proposed_markdown}</pre>
      </CardContent>
    </Card>
  );
}

function formatDateTime(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString(dateTimeLocale());
}
