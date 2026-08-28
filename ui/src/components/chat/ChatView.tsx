import { useEffect, useRef, useState } from 'react';
import { AlertTriangle } from 'lucide-react';
import { preflight, type Preflight } from '@/lib/api';
import { regeneratePrompt } from '@/lib/chat-actions';
import { useVivyStore } from '@/lib/store';
import { Button } from '@/components/ui/button';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useTranslation } from '@/i18n';
import { useIsMobile } from '@/hooks/use-mobile';
import { MessageBubble } from './MessageBubble';
import { ChatInput } from './ChatInput';
import { TodoProgressStrip } from './TodoProgressStrip';
import { SessionTodoPanel } from '@/components/planning/SessionTodoPanel';
import { cn } from '@/lib/utils';

const TEXT_ENCODER = new TextEncoder();

export function ChatView({ sessionId }: { sessionId: string }) {
  const messages = useVivyStore((state) => state.messages);
  const phase = useVivyStore((state) => state.messagesPhase);
  const error = useVivyStore((state) => state.messagesError ?? state.runError);
  const run = useVivyStore((state) => state.currentRun);
  const runBusy = useVivyStore((state) => state.runBusy);
  const streamingText = useVivyStore((state) => state.streamingText);
  const streamingReasoning = useVivyStore((state) => state.streamingReasoning);
  const startRun = useVivyStore((state) => state.startRun);
  const cancelRun = useVivyStore((state) => state.cancelCurrentRun);
  const todoPanelOpen = useVivyStore((state) => state.todoPanelOpen);
  const setTodoPanelOpen = useVivyStore((state) => state.setTodoPanelOpen);
  const mobile = useIsMobile();
  const { t } = useTranslation();
  const [pending, setPending] = useState<{ text: string; result: Preflight } | null>(null);
  const [preflightBusy, setPreflightBusy] = useState(false);
  const [preflightError, setPreflightError] = useState<string | null>(null);
  const requestId = useRef(0);
  const running = !!run && !['completed', 'failed', 'cancelled'].includes(run.status);
  useEffect(() => { requestId.current += 1; setPending(null); setPreflightError(null); }, [sessionId]);

  const submit = async (text: string) => {
    const id = ++requestId.current; setPreflightBusy(true); setPreflightError(null);
    try { const result = await preflight(sessionId, text, 'normal'); if (id !== requestId.current || useVivyStore.getState().activeSessionId !== sessionId) return; if (result.status !== 'ready' || result.warnings?.length || result.blockers?.length) setPending({ text, result }); else await startRun(sessionId, text); }
    catch (error) { if (id === requestId.current) setPreflightError(error instanceof Error ? error.message : String(error)); }
    finally { if (id === requestId.current) setPreflightBusy(false); }
  };
  const continueRun = async () => { if (!pending || pending.result.status === 'blocked') return; const text = pending.text; setPending(null); await startRun(sessionId, text); };
  // 重新生成（对照 Agent-DIVA）：Journal 是追加式事实源，无法就地覆盖，
  // 映射为用目标助手消息之前最近一条用户输入重新走一轮（含预检）。
  const regenerate = (messageId: string) => {
    const text = regeneratePrompt(messages, messageId);
    if (text === null || text.trim() === '') return;
    void submit(text);
  };
  const actionsDisabled = running || preflightBusy || runBusy;
  const streamMessage = streamingText || streamingReasoning ? { id: `stream-${run?.id}`, run_id: run?.id, role: 'assistant' as const, content: streamingText, created_at: Date.now() } : null;
  const contextBytes = [...messages.map((message) => message.content), streamingText, streamingReasoning].reduce((total, content) => total + TEXT_ENCODER.encode(content).length, 0);

  return (
    <div className="flex h-full min-h-0">
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        <ScrollArea className="min-h-0 flex-1"><div className="mx-auto max-w-4xl p-4">
          {phase === 'loading' ? <div className="space-y-3 pt-4"><div className="h-16 w-2/3 animate-pulse rounded-2xl bg-muted"/><div className="ml-auto h-12 w-1/2 animate-pulse rounded-2xl bg-muted"/></div> : null}
          {phase === 'empty' && !streamMessage ? <div className="py-24 text-center text-muted-foreground"><p className="text-lg">{t('chat.startNew')}</p><p className="mt-1 text-sm">{t('chat.preflightHint')}</p></div> : null}
          {messages.map((message) => <MessageBubble key={message.id} message={message} canRegenerate={regeneratePrompt(messages, message.id) !== null} actionsDisabled={actionsDisabled} onRegenerate={() => regenerate(message.id)} />)}
          {streamMessage ? <MessageBubble message={streamMessage} reasoning={streamingReasoning} streaming /> : null}
          {error || preflightError ? <div className="my-3 rounded-lg bg-destructive/10 p-3 text-sm text-destructive">{preflightError || error}</div> : null}
        </div></ScrollArea>
        {pending ? <div className="border-t border-amber-500/30 bg-amber-500/10 px-4 py-3"><div className="mx-auto flex max-w-3xl flex-col gap-3 sm:flex-row sm:items-start"><div className="flex min-w-0 flex-1 items-start gap-3 text-sm"><AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-600"/><div className="min-w-0"><div className="font-medium">{pending.result.status === 'blocked' ? t('chat.preflightBlocked') : t('chat.preflightWarned')}</div>{[...(pending.result.blockers ?? []), ...(pending.result.warnings ?? [])].map((item) => <p key={item} className="mt-1 text-muted-foreground">{item}</p>)}</div></div><div className="flex shrink-0 gap-2 self-end sm:self-start"><Button variant="ghost" size="sm" onClick={() => setPending(null)}>{t('common.cancel')}</Button>{pending.result.status !== 'blocked' ? <Button size="sm" onClick={() => void continueRun()}>{t('chat.continue')}</Button> : null}</div></div></div> : null}
        <TodoProgressStrip />
        <ChatInput onSend={submit} onCancel={cancelRun} running={running} disabled={preflightBusy || runBusy} contextBytes={contextBytes} />
      </div>
      <aside className={cn('hidden min-h-0 shrink-0 overflow-hidden border-l bg-card md:flex', todoPanelOpen ? 'w-80' : 'w-0 border-l-0')}>
        {!mobile && todoPanelOpen ? <SessionTodoPanel onClose={() => setTodoPanelOpen(false)} /> : null}
      </aside>
    </div>
  );
}
