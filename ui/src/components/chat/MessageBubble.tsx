import { useEffect, useRef, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import { Check, Copy, GitFork, Pencil, RefreshCw, Rewind } from 'lucide-react';
import type { Message } from '@/lib/api';
import { dateTimeLocale, useTranslation } from '@/i18n';
import { parseToolResultDiff } from '@/lib/diff';
import { DiffView } from '@/components/ui/DiffView';

// 消息功能栏（对照 Agent-DIVA msg-actions 移植，用户侧交互参考 ChatGPT）：
// 用户消息（蓝色气泡）：下方悬停才浮现的复制 + 编辑（占位）按钮，
// 平时 opacity-0 不占视觉、保留布局位避免悬停跳动；
// 助手消息：时间戳 + 复制（启用，带「已复制」反馈）+ 重新生成
// （重发之前最近一条用户消息）+ 回退 / 分叉（占位）。
// 密钥式敏感操作不存在；复制仅使用浏览器剪贴板。
// 工具结果是文件变更 JSON（FileMutationResult，含 diff 字段）时改用
// DiffView 呈现统一/分栏 diff，原始 JSON 折叠保留；其余工具结果维持纯文本。
const ACTION_BUTTON = 'flex h-6 w-6 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:bg-transparent';

function formatTime(timestamp: number): string {
  if (!timestamp) return '';
  return new Date(timestamp).toLocaleTimeString(dateTimeLocale(), { hour: '2-digit', minute: '2-digit' });
}

/** 复制到剪贴板：优先 Clipboard API；受限 webview（如内嵌浏览器）拒绝时退回 execCommand。 */
async function copyTextToClipboard(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    try {
      const area = document.createElement('textarea');
      area.value = text;
      area.style.position = 'fixed';
      area.style.opacity = '0';
      document.body.appendChild(area);
      area.select();
      const succeeded = document.execCommand('copy');
      area.remove();
      return succeeded;
    } catch {
      return false;
    }
  }
}

function ToolResultBubble({ message }: { message: Message }) {
  const { t } = useTranslation();
  const toolDiff = parseToolResultDiff(message.content);
  if (!toolDiff) {
    return <div className="mx-auto my-3 max-w-2xl min-w-0 rounded-xl border bg-muted/40 p-3 text-sm"><div className="mb-1 text-xs font-medium text-muted-foreground">{t('chat.toolResult')}</div><pre className="overflow-x-auto whitespace-pre-wrap break-words">{message.content}</pre></div>;
  }
  return (
    <div className="mx-auto my-3 max-w-2xl min-w-0 rounded-xl border bg-muted/40 p-3 text-sm">
      <div className="mb-1 flex min-w-0 items-baseline gap-2 text-xs font-medium text-muted-foreground">
        <span className="shrink-0">{t('chat.toolResult')}</span>
        {toolDiff.path ? <code className="min-w-0 truncate font-mono">{toolDiff.path}</code> : null}
      </div>
      <DiffView diff={toolDiff.diff} />
      <details className="mt-2">
        <summary className="cursor-pointer text-xs text-muted-foreground">{t('chat.toolResultRaw')}</summary>
        <pre className="mt-1 overflow-x-auto whitespace-pre-wrap break-words">{message.content}</pre>
      </details>
    </div>
  );
}

export function MessageBubble({
  message,
  reasoning,
  streaming,
  canRegenerate = false,
  actionsDisabled = false,
  onRegenerate,
}: {
  message: Message;
  reasoning?: string;
  streaming?: boolean;
  /** 助手消息之前存在用户消息（有可重发的输入） */
  canRegenerate?: boolean;
  /** 运行期间禁用重试，对应 Agent-DIVA 的 isTyping */
  actionsDisabled?: boolean;
  onRegenerate?: () => void;
}) {
  const { t } = useTranslation();
  const user = message.role === 'user';
  const [copied, setCopied] = useState(false);
  const copyTimer = useRef<number | null>(null);
  useEffect(() => () => { if (copyTimer.current !== null) window.clearTimeout(copyTimer.current); }, []);

  const copy = async () => {
    if (!(await copyTextToClipboard(message.content))) return;
    setCopied(true);
    if (copyTimer.current !== null) window.clearTimeout(copyTimer.current);
    copyTimer.current = window.setTimeout(() => setCopied(false), 1500);
  };

  if (message.role === 'tool') return <ToolResultBubble message={message} />;
  // channel 出处徽章（CH-C1-N3）：ui 轮无 provenance，不出任何标记。
  const origin = message.provenance
    ? [message.provenance.channel || message.provenance.source, message.provenance.chat_id].filter(Boolean).join(' · ')
    : '';
  if (user) {
    return <article className="group my-4 flex min-w-0 justify-end"><div className="flex min-w-0 max-w-[min(78%,100%)] flex-col items-end">
      {origin ? <div className="mb-0.5 px-1 text-[10px] text-muted-foreground">{origin}</div> : null}
      <div className="w-fit max-w-full min-w-0 overflow-hidden rounded-2xl bg-primary px-4 py-3 text-sm leading-relaxed text-primary-foreground shadow-sm">
        {message.attachments?.length ? (
          <div className="mb-2 flex flex-wrap justify-end gap-1.5">
            {message.attachments.map((item, index) => (
              <img key={`${item.name ?? 'image'}-${index}`} src={item.data_url} alt={item.name || item.mime_type} className="max-h-40 max-w-full rounded-xl border border-white/20 object-cover" />
            ))}
          </div>
        ) : null}
        <div className="prose prose-sm max-w-none break-words dark:prose-invert"><ReactMarkdown>{message.content || (streaming ? '…' : '')}</ReactMarkdown></div>
      </div>
      {streaming ? null : (
        <div className="mt-1 flex items-center gap-0.5 px-1 opacity-0 transition-opacity duration-150 group-hover:opacity-100">
          <button type="button" className={ACTION_BUTTON} onClick={() => void copy()} title={copied ? t('chat.copied') : t('chat.copy')} aria-label={copied ? t('chat.copied') : t('chat.copy')}>
            {copied ? <Check className="h-3 w-3 text-primary" /> : <Copy className="h-3 w-3" />}
          </button>
          <button type="button" className={ACTION_BUTTON} disabled title={`${t('chat.edit')} (${t('chat.pending')})`} aria-label={t('chat.edit')}>
            <Pencil className="h-3 w-3" />
          </button>
        </div>
      )}
    </div></article>;
  }
  return <article className="group my-4 flex min-w-0 justify-start"><div className="flex min-w-0 max-w-[min(78%,100%)] flex-col items-start">
    <div className="w-fit max-w-full min-w-0 overflow-hidden rounded-2xl border border-border bg-card px-4 py-3 text-sm leading-relaxed shadow-sm">
      {reasoning ? <details className="mb-3 border-b border-border pb-2 text-xs text-muted-foreground"><summary className="cursor-pointer">{streaming ? t('chat.thinkingStreaming') : t('chat.thinking')}</summary><div className="mt-2 whitespace-pre-wrap">{reasoning}</div></details> : null}
      <div className="prose prose-sm max-w-none break-words dark:prose-invert"><ReactMarkdown>{message.content || (streaming ? '…' : '')}</ReactMarkdown></div>
    </div>
    {streaming ? null : (
      <div className="mt-1.5 flex items-center gap-0.5 px-1 opacity-60 transition-opacity group-hover:opacity-100 justify-start">
        <span className="mr-0.5 text-[10px] text-muted-foreground">{formatTime(message.created_at)}</span>
        <button type="button" className={ACTION_BUTTON} onClick={() => void copy()} title={copied ? t('chat.copied') : t('chat.copy')} aria-label={copied ? t('chat.copied') : t('chat.copy')}>
          {copied ? <Check className="h-3 w-3 text-primary" /> : <Copy className="h-3 w-3" />}
        </button>
        <button type="button" className={ACTION_BUTTON} disabled={actionsDisabled || !canRegenerate || !onRegenerate} onClick={onRegenerate} title={t('chat.regenerate')} aria-label={t('chat.regenerate')}>
          <RefreshCw className="h-3 w-3" />
        </button>
        <button type="button" className={ACTION_BUTTON} disabled title={`${t('chat.rewind')} (${t('chat.pending')})`} aria-label={t('chat.rewind')}>
          <Rewind className="h-3 w-3" />
        </button>
        <button type="button" className={ACTION_BUTTON} disabled title={`${t('chat.fork')} (${t('chat.pending')})`} aria-label={t('chat.fork')}>
          <GitFork className="h-3 w-3" />
        </button>
      </div>
    )}
  </div></article>;
}
