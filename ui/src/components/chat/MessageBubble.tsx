import ReactMarkdown from 'react-markdown';
import type { Message } from '@/lib/api';

export function MessageBubble({ message, reasoning, streaming }: { message: Message; reasoning?: string; streaming?: boolean }) {
  const user = message.role === 'user';
  if (message.role === 'tool') return <div className="mx-auto my-3 max-w-2xl rounded-xl border bg-muted/40 p-3 text-sm"><div className="mb-1 text-xs font-medium text-muted-foreground">工具结果</div><pre className="overflow-x-auto whitespace-pre-wrap">{message.content}</pre></div>;
  return <article className={`my-4 flex ${user ? 'justify-end' : 'justify-start'}`}><div className={`max-w-[78%] rounded-2xl px-4 py-3 text-sm leading-relaxed shadow-sm ${user ? 'bg-primary text-primary-foreground' : 'border border-border bg-card text-foreground'}`}>
    {!user && reasoning ? <details className="mb-3 border-b border-border pb-2 text-xs text-muted-foreground"><summary className="cursor-pointer">思考过程{streaming ? '（进行中）' : ''}</summary><div className="mt-2 whitespace-pre-wrap">{reasoning}</div></details> : null}
    <div className="prose prose-sm max-w-none break-words dark:prose-invert"><ReactMarkdown>{message.content || (streaming ? '…' : '')}</ReactMarkdown></div>
  </div></article>;
}
