import { useState } from 'react';
import type { RunMode } from '@/lib/api';
import { regeneratePrompt } from '@/lib/chat-actions';
import { useVivyStore } from '@/lib/store';
import { Button } from '@/components/ui/button';
import { RecoverableError } from '@/components/feedback/RecoverableError';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useTranslation } from '@/i18n';
import { useIsMobile } from '@/hooks/use-mobile';
import { MessageBubble } from './MessageBubble';
import { ChatInput } from './ChatInput';
import { TodoProgressStrip } from './TodoProgressStrip';
import { SessionTodoPanel } from '@/components/planning/SessionTodoPanel';
import { cn } from '@/lib/utils';

export function ChatView({ sessionId }: { sessionId: string }) {
  const messages = useVivyStore((state) => state.messages);
  const phase = useVivyStore((state) => state.messagesPhase);
  const messagesError = useVivyStore((state) => state.messagesError);
  const runError = useVivyStore((state) => state.runError);
  const run = useVivyStore((state) => state.currentRun);
  const runBusy = useVivyStore((state) => state.runBusy);
  const streamingText = useVivyStore((state) => state.streamingText);
  const streamingReasoning = useVivyStore((state) => state.streamingReasoning);
  const sessionContext = useVivyStore((state) => state.sessionContext);
  const startRun = useVivyStore((state) => state.startRun);
  const selectSession = useVivyStore((state) => state.selectSession);
  const cancelRun = useVivyStore((state) => state.cancelCurrentRun);
  const todoPanelOpen = useVivyStore((state) => state.todoPanelOpen);
  const setTodoPanelOpen = useVivyStore((state) => state.setTodoPanelOpen);
  const mobile = useIsMobile();
  const { t } = useTranslation();
  const running = !!run && !['completed', 'failed', 'cancelled'].includes(run.status);

  const submit = async (text: string, mode: RunMode = 'normal') => {
    await startRun(sessionId, text, mode);
  };
  // 重新生成（对照 Agent-DIVA）：Journal 是追加式事实源，无法就地覆盖，
  // 映射为用目标助手消息之前最近一条用户输入重新走一轮。
  const regenerate = (messageId: string) => {
    const text = regeneratePrompt(messages, messageId);
    if (text === null || text.trim() === '') return;
    void submit(text);
  };
  const actionsDisabled = running || runBusy;
  const streamMessage = streamingText || streamingReasoning ? { id: `stream-${run?.id}`, run_id: run?.id, role: 'assistant' as const, content: streamingText, created_at: Date.now() } : null;

  return (
    <div className="flex h-full min-h-0">
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        <ScrollArea className="min-h-0 flex-1"><div className="mx-auto max-w-4xl p-4">
          {phase === 'loading' ? <div className="space-y-3 pt-4"><div className="h-16 w-2/3 animate-pulse rounded-2xl bg-muted"/><div className="ml-auto h-12 w-1/2 animate-pulse rounded-2xl bg-muted"/></div> : null}
          {phase === 'error' && !messages.length ? <div className="py-16"><RecoverableError error={messagesError} onRetry={() => void selectSession(sessionId)} /></div> : null}
          {phase === 'empty' && !streamMessage && !runError ? <div className="py-24"><p className="text-center text-lg text-muted-foreground">{t('chat.startNew')}</p></div> : null}
          {messages.map((message) => <MessageBubble key={message.id} message={message} canRegenerate={regeneratePrompt(messages, message.id) !== null} actionsDisabled={actionsDisabled} onRegenerate={() => regenerate(message.id)} />)}
          {streamMessage ? <MessageBubble message={streamMessage} reasoning={streamingReasoning} streaming /> : null}
          {runError ? <RecoverableError className="my-3" compact error={runError} /> : null}
        </div></ScrollArea>
        <TodoProgressStrip />
        <ChatInput onSend={submit} onCancel={cancelRun} running={running} disabled={runBusy} context={sessionContext} />
      </div>
      <aside className={cn('hidden min-h-0 shrink-0 overflow-hidden border-l bg-card md:flex', todoPanelOpen ? 'w-80' : 'w-0 border-l-0')}>
        {!mobile && todoPanelOpen ? <SessionTodoPanel onClose={() => setTodoPanelOpen(false)} /> : null}
      </aside>
    </div>
  );
}
