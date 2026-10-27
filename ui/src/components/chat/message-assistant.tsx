import { useEffect, useRef, useState } from 'react';
import { Check, Copy, GitFork, RefreshCw, Rewind } from 'lucide-react';
import type { Message } from '@/lib/api';
import { useTranslation } from '@/i18n';
import { notify } from '@/lib/toasts';
import { copyTextToClipboard } from '@/lib/clipboard';
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { MarkdownBody } from './markdown-block';
import { MessageAction, formatTime } from './message-actions';

export function MessageAssistantBubble({
  message,
  streaming = false,
  canRegenerate = false,
  actionsDisabled = false,
  onRegenerate,
  onRewind,
  onFork,
}: {
  message: Message;
  streaming?: boolean;
  /** 助手消息之前存在用户消息（有可重发的输入） */
  canRegenerate?: boolean;
  /** 运行期间禁用重试，对应 Agent-DIVA 的 isTyping */
  actionsDisabled?: boolean;
  onRegenerate?: () => void;
  /** 回退确认：该消息（含）起退出上下文，输入框预填原文。 */
  onRewind?: () => void | Promise<void>;
  /** 分叉确认：以该消息为止的历史复制新会话并跳转。 */
  onFork?: () => void | Promise<void>;
}) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);
  const [confirming, setConfirming] = useState<'rewind' | 'fork' | null>(null);
  const [pending, setPending] = useState(false);
  const copyTimer = useRef<number | null>(null);
  useEffect(() => () => { if (copyTimer.current !== null) window.clearTimeout(copyTimer.current); }, []);

  const copy = async () => {
    if (!(await copyTextToClipboard(message.content))) {
      notify({ tone: 'danger', title: t('feedback.operation.copy') });
      return;
    }
    setCopied(true);
    if (copyTimer.current !== null) window.clearTimeout(copyTimer.current);
    copyTimer.current = window.setTimeout(() => setCopied(false), 1500);
  };

  return (
    <article data-message-id={message.id} className="group my-4 flex min-w-0 justify-start">
      <div className="flex min-w-0 max-w-[min(78%,100%)] flex-col items-start">
        <div className="w-fit max-w-full min-w-0 rounded-2xl border border-line bg-raised px-4 py-3 text-read shadow-sm">
          <MarkdownBody content={message.content} streaming={streaming} />
        </div>
        {streaming ? null : (
          <div className="mt-1.5 flex items-center gap-0.5 px-1 opacity-60 transition-opacity group-hover:opacity-100 focus-within:opacity-100">
            <span className="mr-0.5 text-2xs text-fg-subtle">{formatTime(message.created_at)}</span>
            <MessageAction label={copied ? t('chat.copied') : t('chat.copy')} onClick={() => void copy()}>
              {copied ? <Check className="h-3.5 w-3.5 text-brand-ink" /> : <Copy className="h-3.5 w-3.5" />}
            </MessageAction>
            <MessageAction
              label={t('chat.regenerate')}
              disabled={actionsDisabled || !canRegenerate || !onRegenerate}
              onClick={onRegenerate}
            >
              <RefreshCw className="h-3.5 w-3.5" />
            </MessageAction>
            <MessageAction label={t('chat.rewind')} disabled={actionsDisabled || !onRewind} onClick={() => setConfirming('rewind')}>
              <Rewind className="h-3.5 w-3.5" />
            </MessageAction>
            <MessageAction label={t('chat.fork')} disabled={actionsDisabled || !onFork} onClick={() => setConfirming('fork')}>
              <GitFork className="h-3.5 w-3.5" />
            </MessageAction>
          </div>
        )}
        <AlertDialog open={confirming !== null} onOpenChange={(open) => { if (!open && !pending) setConfirming(null); }}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>{confirming === 'fork' ? t('chat.forkConfirmTitle') : t('chat.rewindConfirmTitle')}</AlertDialogTitle>
              <AlertDialogDescription>{confirming === 'fork' ? t('chat.forkConfirmBody') : t('chat.rewindConfirmBody')}</AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel disabled={pending}>{t('chat.editCancel')}</AlertDialogCancel>
              <AlertDialogAction
                disabled={pending}
                onClick={async (event) => {
                  event.preventDefault();
                  const action = confirming;
                  if (!action) return;
                  setPending(true);
                  try {
                    if (action === 'rewind') await onRewind?.();
                    if (action === 'fork') await onFork?.();
                    setConfirming(null);
                  } catch {
                    // ChatView reports history failures once as a chatAction toast.
                  } finally {
                    setPending(false);
                  }
                }}
              >
                {confirming === 'fork' ? t('chat.fork') : t('chat.rewind')}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    </article>
  );
}
