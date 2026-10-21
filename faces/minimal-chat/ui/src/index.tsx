import { defineUIRoot, type FullUIHost, type FaceRunEvent } from '@vivy/ui-sdk';
import { useCallback, useRef, useState, useSyncExternalStore } from 'react';
import './style.css';

function toolRows(events: readonly FaceRunEvent[]) {
  const rows = new Map<string, { id: string; name: string; result: string; error: string; finished: boolean; createdAt: number }>();
  for (const event of events) {
    if (event.type !== 'tool.started' && event.type !== 'tool.finished') continue;
    const p = event.payload;
    if (typeof p.tool_call_id !== 'string') continue;
    const previous = rows.get(p.tool_call_id);
    rows.set(p.tool_call_id, {
      id: p.tool_call_id,
      name: typeof p.tool_name === 'string' ? p.tool_name : previous?.name ?? 'tool',
      result: typeof p.result === 'string' ? p.result : previous?.result ?? '',
      error: typeof p.error === 'string' ? p.error : previous?.error ?? '',
      finished: event.type === 'tool.finished' || previous?.finished === true,
      createdAt: previous?.createdAt ?? event.created_at,
    });
  }
  return [...rows.values()];
}

export function MinimalChat({ host }: { readonly host: FullUIHost }) {
  const subscribe = useCallback((listener: () => void) => host.store.subscribe(() => listener()), [host]);
  const snapshot = useCallback(() => host.store.getState(), [host]);
  const state = useSyncExternalStore(subscribe, snapshot, snapshot);
  const [draft, setDraft] = useState('');
  const [workspace, setWorkspace] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [localError, setLocalError] = useState('');
  const pending = useRef(false);
  const t = (key: string) => host.t(`plugin.vivy/minimal-chat.${key}`);
  const busy = submitting || state.runBusy || state.currentRun?.status === 'active';
  const events = state.runEvents.length ? state.runEvents : state.runLogs[state.currentRun?.id ?? ''] ?? [];
  const tools = toolRows(events);
  const conversation = [
    ...state.messages.filter((message) => message.content && !(message.role === 'tool' && tools.length && message.run_id === state.currentRun?.id))
      .map((message) => ({ kind: 'message' as const, key: `message:${message.id}`, createdAt: message.created_at, message })),
    ...tools.map((tool) => ({ kind: 'tool' as const, key: `tool:${tool.id}`, createdAt: tool.createdAt, tool })),
  ].sort((a, b) => a.createdAt - b.createdAt);
  const activeSession = state.sessions.find((session) => session.id === state.activeSessionId);
  const error = localError || state.initializationError || state.runError || state.messagesError;

  const submit = async () => {
    if (pending.current || busy || !state.initialized || !draft.trim()) return;
    pending.current = true;
    setSubmitting(true);
    setLocalError('');
    try {
      const targetWorkspace = workspace?.trim();
      let sessionId = state.activeSessionId;
      if (sessionId && targetWorkspace !== undefined && targetWorkspace !== activeSession?.workspace_path) {
        sessionId = (await state.chooseWorkspace(targetWorkspace)).id;
      }
      if (!sessionId) sessionId = (await state.createSession(undefined, targetWorkspace || undefined)).id;
      await host.store.getState().startRun(sessionId, { text: draft.trim() });
      setDraft('');
      setWorkspace(null);
    } catch (cause) {
      setLocalError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      pending.current = false;
      setSubmitting(false);
    }
  };
  const cancel = async () => {
    setLocalError('');
    try { await state.cancelCurrentRun(); }
    catch (cause) { setLocalError(cause instanceof Error ? cause.message : String(cause)); }
  };

  return (
    <main className="minimal-chat" data-testid="minimal-chat">
      <header>
        <h1>Vivy</h1>
        <p role="status" aria-live="polite" data-testid="minimal-status" data-run-id={state.currentRun?.id}>
          {t(!state.initialized ? 'connecting' : state.currentRun?.status ?? 'ready')}
        </p>
      </header>
      <section className="minimal-transcript" aria-label={t('conversation')}>
        {!state.messages.length && !tools.length && <p className="minimal-muted">{t('empty')}</p>}
        {conversation.map((entry) => entry.kind === 'message' ? (
          <article key={entry.key} data-testid="minimal-message">
            <h2>{t(entry.message.role)}</h2><pre>{entry.message.content}</pre>
          </article>
        ) : <article key={entry.key} data-testid={`minimal-tool-${entry.tool.id}`}>
          <h2>{entry.tool.name}</h2>
          {entry.tool.result && <pre>{entry.tool.result}</pre>}
          {entry.tool.error && <p role="alert">{entry.tool.error}</p>}
          {!entry.tool.result && !entry.tool.error && <p>{t(entry.tool.finished ? 'completed' : 'running')}</p>}
        </article>)}
        {state.streamingText && <article><h2>{t('assistant')}</h2><pre>{state.streamingText}</pre></article>}
      </section>
      <form onSubmit={(event) => { event.preventDefault(); void submit(); }}>
        {error && <p role="alert">{error}</p>}
        {state.initializationError && <button type="button" onClick={() => void state.retryInitialize()}>{t('retry')}</button>}
        <label htmlFor="minimal-workspace">{t('workspace')}</label>
        <input id="minimal-workspace" value={workspace ?? activeSession?.workspace_path ?? ''}
          disabled={state.messages.length > 0 || busy} onChange={(event) => setWorkspace(event.target.value)} />
        <label htmlFor="minimal-draft">{t('message')}</label>
        <textarea id="minimal-draft" rows={3} value={draft} disabled={submitting} onChange={(event) => setDraft(event.target.value)} />
        <div className="minimal-actions">
          {state.currentRun?.status === 'active' && <button type="button" data-testid="minimal-cancel" onClick={() => void cancel()}>{t('cancel')}</button>}
          <button className="minimal-primary" data-testid="minimal-send" type="submit"
            disabled={busy || !state.initialized || !draft.trim()}>{t('send')}</button>
        </div>
      </form>
    </main>
  );
}

export const root = defineUIRoot({ id: 'vivy.minimal-chat.root', render: (host) => <MinimalChat host={host} /> });
