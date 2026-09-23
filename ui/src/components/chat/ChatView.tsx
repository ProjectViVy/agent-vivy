import { useMemo, useState } from 'react';
import type { AttachmentInput, Face, RunMode, ThinkingMode } from '@/lib/api';
import { regeneratePrompt } from '@/lib/chat-actions';
import { buildTranscriptRows, foldRunEvents, type RunRow } from '@/lib/run-rows';
import { useVivyStore } from '@/lib/store';
import { Button } from '@/components/ui/button';
import { RecoverableError } from '@/components/feedback/RecoverableError';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useTranslation } from '@/i18n';
import { useIsMobile } from '@/hooks/use-mobile';
import { MessageBubble } from './MessageBubble';
import { ReasoningRow } from './ReasoningRow';
import { ToolRow } from './ToolRow';
import { ChatInput } from './ChatInput';
import { TodoProgressStrip } from './TodoProgressStrip';
import { WorkControlBar } from './WorkControlBar';
import { SessionTodoPanel } from '@/components/planning/SessionTodoPanel';
import { cn } from '@/lib/utils';

export function ChatView({ sessionId }: { sessionId: string }) {
  const messages = useVivyStore((state) => state.messages);
  const phase = useVivyStore((state) => state.messagesPhase);
  const messagesError = useVivyStore((state) => state.messagesError);
  const runError = useVivyStore((state) => state.runError);
  const run = useVivyStore((state) => state.currentRun);
  const runEvents = useVivyStore((state) => state.runEvents);
  const runLogs = useVivyStore((state) => state.runLogs);
  const runBusy = useVivyStore((state) => state.runBusy);
  const sessionContext = useVivyStore((state) => state.sessionContext);
  const startRun = useVivyStore((state) => state.startRun);
	const editSession = useVivyStore((state) => state.editSession);
  const enqueueMessage = useVivyStore((state) => state.enqueueMessage);
  const selectSession = useVivyStore((state) => state.selectSession);
  const cancelRun = useVivyStore((state) => state.cancelCurrentRun);
  const rewindSession = useVivyStore((state) => state.rewindSession);
  const forkSession = useVivyStore((state) => state.forkSession);
  const [draftPreset, setDraftPreset] = useState<{ text: string; seq: number } | null>(null);
  const [actionError, setActionError] = useState<unknown>(null);
	const [historyAction, setHistoryAction] = useState(false);
  const todoPanelOpen = useVivyStore((state) => state.todoPanelOpen);
  const setTodoPanelOpen = useVivyStore((state) => state.setTodoPanelOpen);
  const codeMode = useVivyStore((state) => state.codeMode);
  const mobile = useIsMobile();
  const { t } = useTranslation();
  // Face selection is owned by the explicit code-mode control. The legacy
  // mask catalog remains a presentation choice and cannot silently change
  // send, queue, edit, or regenerate semantics.
  const face: Face | undefined = codeMode ? 'code' : undefined;
  const running = !!run && !['completed', 'failed', 'cancelled'].includes(run.status);

  const submit = async (text: string, mode: RunMode = 'normal', attachments?: AttachmentInput[], thinking?: ThinkingMode) => {
    await startRun(sessionId, text, mode, face, attachments, thinking);
  };
  // 重新生成（对照 Agent-DIVA）：Journal 是追加式事实源，无法就地覆盖，
  // 映射为用目标助手消息之前最近一条用户输入重新走一轮。
  const regenerate = (messageId: string) => {
    const text = regeneratePrompt(messages, messageId);
    if (text === null || text.trim() === '') return;
    void submit(text);
  };
	const actionsDisabled = running || runBusy || historyAction;
  // 编辑 / 回退 / 分叉（设计 §5）：Journal 追加式，编辑 = 回退到该输入再重发；
  // 回退后把仍在上下文里的最近一条用户输入预填进输入框；分叉成功后跳到新会话。
  const handleEdit = async (messageId: string, newText: string) => {
    setActionError(null);
	setHistoryAction(true);
    try {
	  await editSession(sessionId, messageId, newText, 'normal', face);
	} catch (error) { setActionError(error); throw error; }
	finally { setHistoryAction(false); }
  };
  const handleRewind = async (messageId: string) => {
    setActionError(null);
	setHistoryAction(true);
    try {
      const remaining = await rewindSession(sessionId, messageId);
      const lastUser = [...remaining].reverse().find((message) => message.role === 'user');
      if (lastUser) setDraftPreset({ text: lastUser.content, seq: Date.now() });
	} catch (error) { setActionError(error); throw error; }
	finally { setHistoryAction(false); }
  };
  const handleFork = async (messageId: string) => {
    setActionError(null);
	setHistoryAction(true);
    try {
      const forkedId = await forkSession(sessionId, messageId);
      await selectSession(forkedId);
	} catch (error) { setActionError(error); throw error; }
	finally { setHistoryAction(false); }
  };
  // 转写行：事件（run/log 或实时订阅）折叠出思考、工具与正文行，投影消息提供
  // 用户输入并接上真实的助手消息。实时运行也走这一条路径——同一段增量只有这一个
  // 渲染者（此前的流式兜底气泡会让思考/正文各出现两份）；拿不到事件的运行仍按
  // 投影渲染（助手气泡 + 工具结果卡）。
  const runRows = useMemo(() => {
    const map: Record<string, ReturnType<typeof foldRunEvents>> = {};
    for (const [runId, events] of Object.entries(runLogs)) map[runId] = foldRunEvents(runId, events);
    if (run && runEvents.length > 0) map[run.id] = foldRunEvents(run.id, runEvents, { active: running });
    return map;
  }, [runLogs, run, runEvents, running]);
  const transcript = useMemo(
    () => buildTranscriptRows({ messages, runRows, liveRunId: run?.id ?? null }),
    [messages, runRows, run],
  );

  return (
    <div className="flex h-full min-h-0">
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        <WorkControlBar sessionId={sessionId} />
        <ScrollArea className="min-h-0 flex-1"><div className="mx-auto max-w-4xl p-4">
          {phase === 'loading' ? <div className="space-y-3 pt-4"><div className="h-16 w-2/3 animate-pulse rounded-2xl bg-muted"/><div className="ml-auto h-12 w-1/2 animate-pulse rounded-2xl bg-muted"/></div> : null}
          {phase === 'error' && !messages.length ? <div className="py-16"><RecoverableError error={messagesError} onRetry={() => void selectSession(sessionId)} /></div> : null}
          {phase === 'empty' && transcript.length === 0 && !runError ? <div className="py-24"><p className="text-center text-lg text-muted-foreground">{t('chat.startNew')}</p></div> : null}
          {transcript.map((row) => {
            if (row.kind === 'user' || row.kind === 'assistant') return <MessageBubble key={row.id} message={row.message} streaming={row.kind === 'assistant' && row.streaming} canRegenerate={regeneratePrompt(messages, row.message.id) !== null} actionsDisabled={actionsDisabled} onRegenerate={() => regenerate(row.message.id)} onEditConfirm={(text) => handleEdit(row.message.id, text)} onRewind={() => handleRewind(row.message.id)} onFork={() => handleFork(row.message.id)} />;
            if (row.kind === 'toolResult') return <MessageBubble key={row.id} message={row.message} />;
            return <RunRowView key={row.id} row={row.row} />;
          })}
          {runError ? <RecoverableError className="my-3" compact error={runError} /> : null}
          {actionError ? <RecoverableError className="my-3" compact error={actionError} onRetry={() => setActionError(null)} /> : null}
        </div></ScrollArea>
        <TodoProgressStrip />
        <ChatInput onSend={submit} onQueue={(text, mode, attachments, thinking) => enqueueMessage(text, mode, face, attachments, thinking)} onCancel={cancelRun} running={running} disabled={runBusy} context={sessionContext} draftPreset={draftPreset} />
      </div>
      <aside className={cn('hidden min-h-0 shrink-0 overflow-hidden border-l bg-card md:flex', todoPanelOpen ? 'w-80' : 'w-0 border-l-0')}>
        {!mobile && todoPanelOpen ? <SessionTodoPanel onClose={() => setTodoPanelOpen(false)} /> : null}
      </aside>
    </div>
  );
}

/** 事件折叠出的非消息行：思考行、工具调用行、上下文压缩通知。 */
function RunRowView({ row }: { row: RunRow }) {
  const { t } = useTranslation();
  if (row.kind === 'reasoning') return <ReasoningRow text={row.text} running={row.running} />;
  if (row.kind === 'tool') return <ToolRow call={row.call} />;
  if (row.kind === 'assistant') return null;
  return (
    <div className="my-2 text-center text-[11px] text-muted-foreground">
      {t('chat.toolCompacted', { detail: row.text === '' ? '' : ` · ${row.text}` })}
    </div>
  );
}
