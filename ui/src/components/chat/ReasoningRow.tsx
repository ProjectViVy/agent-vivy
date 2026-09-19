// 思考行（对照 DSH `ui-chat` 的 ReasoningRow）：默认收起的折叠行，标题稳定，
// 摘要来自数据——运行中显示最后一行、结束后显示第一行；两种状态都左对齐并按行宽
// 从右端截断，因此摘要始终从左向右生长（不对齐到行尾，避免看起来从右往左打印）。
// 展开后正文是纯文本、表头吸顶，长思考链也能随时收起。
// `**` 只在摘要里剥掉，正文保持原文。

import { useState } from 'react';
import { Brain, ChevronDown, ChevronRight } from 'lucide-react';
import { useTranslation } from '@/i18n';
import { cn } from '@/lib/utils';

function firstLine(text: string): string {
  const newline = text.indexOf('\n');
  return newline === -1 ? text : text.slice(0, newline);
}

function lastLine(text: string): string {
  const visible = text.trimEnd();
  const newline = visible.lastIndexOf('\n');
  return newline === -1 ? visible : visible.slice(newline + 1);
}

export function ReasoningRow({ text, running }: { text: string; running: boolean }) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(false);
  const summary = (running ? lastLine(text) : firstLine(text)).replaceAll('**', '').trim();
  const Chevron = expanded ? ChevronDown : ChevronRight;

  return (
    <div className="my-2 min-w-0" data-reasoning-row="" data-state={running ? 'running' : 'ok'} data-expanded={expanded || undefined}>
      <button
        type="button"
        className={cn(
          'group/reasoning relative flex h-6 w-full min-w-0 items-center overflow-hidden rounded-md text-left',
          expanded && 'sticky top-0 z-[1] bg-background',
        )}
        aria-expanded={expanded}
        onClick={() => setExpanded((value) => !value)}
      >
        {running ? <span className="sr-only" aria-live="polite">{t('chat.toolRunning')}</span> : null}
        <span className="relative mr-1.5 inline-flex h-4 w-4 shrink-0 items-center justify-center text-muted-foreground">
          <Brain className={cn('h-3.5 w-3.5 transition-opacity duration-100', 'group-hover/reasoning:opacity-0', expanded && 'opacity-0')} />
          <Chevron className={cn('absolute h-3.5 w-3.5 transition-opacity duration-100', expanded ? 'opacity-100' : 'opacity-0 group-hover/reasoning:opacity-100')} />
        </span>
        <span className="shrink-0 text-[13px] leading-6 text-muted-foreground">{t('chat.thinking')}</span>
        {!expanded && summary !== '' ? (
          <>
            <span aria-hidden className="mx-2 h-0.5 w-0.5 shrink-0 rounded-full bg-muted-foreground/60" />
            <span className="min-w-0 flex-1 truncate text-[13px] leading-6 text-muted-foreground/80">{summary}</span>
          </>
        ) : null}
        {running ? (
          <span aria-hidden className="row-sweep pointer-events-none absolute inset-y-0 left-0 w-1/3 bg-gradient-to-r from-transparent via-background/70 to-transparent" />
        ) : null}
      </button>
      {expanded ? (
        <div className="whitespace-pre-wrap break-words pb-2 pl-[22px] text-xs leading-5 text-muted-foreground" data-reasoning-body="">
          {text}
        </div>
      ) : null}
    </div>
  );
}
