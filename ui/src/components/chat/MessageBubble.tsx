import ReactMarkdown from 'react-markdown';
import type { Message } from '@/lib/api';
import { useTranslation } from '@/i18n';

export function MessageBubble({ message, reasoning, streaming }: { message: Message; reasoning?: string; streaming?: boolean }) {
  const { t } = useTranslation();
  const user = message.role === 'user';
  if (message.role === 'tool') return <div className="mx-auto my-3 max-w-2xl min-w-0 rounded-xl border bg-muted/40 p-3 text-sm"><div className="mb-1 text-xs font-medium text-muted-foreground">{t('chat.toolResult')}</div><pre className="overflow-x-auto whitespace-pre-wrap break-words">{message.content}</pre></div>;
  return <article className={`my-4 flex min-w-0 ${user ? 'justify-end' : 'justify-start'}`}><div className={`min-w-0 max-w-[min(78%,100%)] overflow-hidden rounded-2xl px-4 py-3 text-sm leading-relaxed shadow-sm ${user ? 'bg-primary text-primary-foreground' : 'border border-border bg-card text-foreground'}`}>
    {!user && reasoning ? <details className="mb-3 border-b border-border pb-2 text-xs text-muted-foreground"><summary className="cursor-pointer">{streaming ? t('chat.thinkingStreaming') : t('chat.thinking')}</summary><div className="mt-2 whitespace-pre-wrap">{reasoning}</div></details> : null}
    <div className="prose prose-sm max-w-none break-words dark:prose-invert"><ReactMarkdown>{message.content || (streaming ? '…' : '')}</ReactMarkdown></div>
  </div></article>;
}
