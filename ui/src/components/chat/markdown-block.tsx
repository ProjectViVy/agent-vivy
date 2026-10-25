// Markdown body shared by every transcript bubble: GFM tables/lists from
// remark-gfm, prose typography from styles.css. Fenced code renders through its
// own header (language + copy) instead of an anonymous scrolling block.
import { Children, isValidElement, useEffect, useRef, useState, type ReactNode } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Check, Copy } from 'lucide-react';
import { useTranslation } from '@/i18n';
import { copyTextToClipboard } from '@/lib/clipboard';
import { cn } from '@/lib/utils';

function textOf(node: ReactNode): string {
  if (node === null || node === undefined || typeof node === 'boolean') return '';
  if (typeof node === 'string' || typeof node === 'number') return String(node);
  if (Array.isArray(node)) return node.map(textOf).join('');
  if (isValidElement<{ children?: ReactNode }>(node)) return textOf(node.props.children);
  return '';
}

function languageOf(children: ReactNode): string {
  const [first] = Children.toArray(children);
  if (isValidElement<{ className?: string }>(first)) {
    return /language-([\w+-]+)/.exec(first.props.className ?? '')?.[1] ?? '';
  }
  return '';
}

function CodeBlock({ children }: { children?: ReactNode }) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);
  const timer = useRef<number | null>(null);
  useEffect(() => () => { if (timer.current !== null) window.clearTimeout(timer.current); }, []);
  const language = languageOf(children);
  const label = copied ? t('chat.copied') : t('chat.copyCode');

  return (
    <div className="not-prose my-2 overflow-hidden rounded-lg border border-line bg-sunken" data-code-block="">
      <div className="flex min-h-7 items-center gap-2 border-b border-line px-2">
        <span className="min-w-0 flex-1 truncate font-mono text-2xs tracking-wide text-fg-muted">
          {language !== '' ? language : t('chat.code')}
        </span>
        <button
          type="button"
          className="inline-flex h-6 shrink-0 items-center gap-1 rounded px-1.5 text-2xs text-fg-muted transition-colors hover:bg-hover hover:text-fg"
          aria-label={label}
          title={label}
          onClick={async () => {
            if (!(await copyTextToClipboard(textOf(children).trimEnd()))) return;
            setCopied(true);
            if (timer.current !== null) window.clearTimeout(timer.current);
            timer.current = window.setTimeout(() => setCopied(false), 1500);
          }}
        >
          {copied ? <Check className="h-3 w-3 text-ok" aria-hidden /> : <Copy className="h-3 w-3" aria-hidden />}
          {copied ? t('chat.copied') : t('chat.copy')}
        </button>
      </div>
      <pre className="overflow-x-auto p-3 font-mono text-meta leading-ui text-fg">{children}</pre>
    </div>
  );
}

export function MarkdownBody({
  content,
  className,
  streaming = false,
}: {
  content: string;
  className?: string;
  streaming?: boolean;
}) {
  const empty = content.trim() === '';
  return (
    <div
      className={cn(
        'prose prose-sm max-w-none break-words dark:prose-invert',
        streaming && !empty && 'streaming-caret',
        className,
      )}
    >
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={{ pre: CodeBlock }}>
        {content}
      </ReactMarkdown>
      {streaming && empty ? (
        <span
          data-streaming-caret=""
          aria-hidden
          className="inline-block h-[1em] w-[2px] translate-y-[0.15em] rounded-full bg-brand motion-safe:animate-pulse"
        />
      ) : null}
    </div>
  );
}
