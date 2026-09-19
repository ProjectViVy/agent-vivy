// 工具调用行（对照 DSH `ui-tool` 的 ToolRow 契约）：
// 一行 24px 折叠行 = [状态图标/圆点][标题][分隔点][参数摘要]，行尾可带 +N -M 后缀；
// 展开后是一张正文卡（diff / 终端 / 输入输出），失败时错误首行替换摘要。
// 状态只由折叠数据推导：没有结果即在运行，error 即失败，interrupted 即已停止。

import { useMemo, useState } from 'react';
import { ChevronDown, ChevronRight } from 'lucide-react';
import { useTranslation } from '@/i18n';
import { cn } from '@/lib/utils';
import type { FoldedToolCall } from '@/lib/run-rows';
import { DiffView } from '@/components/ui/DiffView';
import {
  TOOL_BODY_MAX_LINES, headTailCap, splitLines, toolDiff, toolExitStatus, toolIcon, toolPath,
  toolSummary, toolTitleKey, toolVariant,
} from './tool-presentation';

function firstLine(text: string): string {
  const newline = text.indexOf('\n');
  return newline === -1 ? text : text.slice(0, newline);
}

/** 头尾折叠的正文段：折叠时保留前后各半，并给出“展开剩余 N 行”。 */
function FoldSection({ label, text, mono = false, tone }: { label: string; text: string; mono?: boolean; tone?: 'error' }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const lines = useMemo(() => splitLines(text), [text]);
  const { head, tail, hidden } = headTailCap(lines, TOOL_BODY_MAX_LINES, open);
  const shown = hidden > 0 ? [...head, '…', ...tail] : head;
  return (
    <div className="min-w-0">
      <div className="mb-0.5 text-[10px] font-medium tracking-wide text-muted-foreground">{label}</div>
      <pre
        className={cn(
          'overflow-x-auto whitespace-pre-wrap break-words rounded-lg bg-muted/40 p-2 text-xs leading-5',
          mono && 'font-mono',
          tone === 'error' && 'bg-destructive/10 text-destructive',
        )}
      >
        {shown.join('\n')}
      </pre>
      {hidden > 0 ? (
        <button
          type="button"
          className="mt-1 text-[11px] text-muted-foreground underline-offset-2 hover:underline"
          onClick={() => setOpen(true)}
        >
          {t('chat.toolMoreLines', { count: hidden })}
        </button>
      ) : null}
    </div>
  );
}

export function ToolRow({ call }: { call: FoldedToolCall }) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(false);
  const variant = toolVariant(call.name);
  const Icon = toolIcon(call.name);
  const diff = useMemo(() => toolDiff(call.result), [call.result]);
  const exit = useMemo(() => toolExitStatus(call.result), [call.result]);
  const args = useMemo(
    () => (call.args !== null ? JSON.stringify(call.args, null, 2) : call.argsRaw),
    [call.args, call.argsRaw],
  );
  const output = variant === 'shell' ? exit.body : call.result;

  const failure = call.status === 'error' ? firstLine(call.error !== '' ? call.error : call.result) : '';
  const summary = failure !== '' ? failure : toolSummary(call);
  const suffix = failure === '' && diff !== null && diff.additions !== null && diff.deletions !== null
    ? `+${diff.additions} -${diff.deletions}`
    : '';
  const path = diff?.path ?? toolPath(call);
  const expandable = args !== '' || output !== '' || call.error !== '' || diff !== null;
  const open = expanded && expandable;
  const Chevron = open ? ChevronDown : ChevronRight;
  const statusLabel = call.status === 'running' ? t('chat.toolRunning')
    : call.status === 'error' ? t('chat.toolFailed')
      : call.status === 'stopped' ? t('chat.toolStopped')
        : call.status === 'awaiting-approval' ? t('chat.toolAwaitingApproval')
          : '';

  return (
    <div className="my-2 min-w-0" data-tool-row={call.name} data-state={call.status}>
      <button
        type="button"
        className={cn(
          'group/tool relative flex h-6 w-full min-w-0 items-center overflow-hidden rounded-md text-left',
          expandable ? 'cursor-pointer' : 'cursor-default',
        )}
        aria-expanded={expandable ? open : undefined}
        onClick={() => { if (expandable) setExpanded((value) => !value); }}
      >
        {statusLabel !== '' ? <span className="sr-only">{statusLabel}</span> : null}
        <span className="relative mr-1.5 inline-flex h-4 w-4 shrink-0 items-center justify-center text-muted-foreground">
          {call.status === 'error' ? (
            <span className="h-2 w-2 rounded-full bg-destructive" />
          ) : call.status === 'stopped' || call.status === 'awaiting-approval' ? (
            <span className="h-2 w-2 rounded-full bg-amber-500" />
          ) : (
            <>
              <Icon className={cn('h-3.5 w-3.5 transition-opacity duration-100', expandable && 'group-hover/tool:opacity-0', open && 'opacity-0')} />
              <Chevron className={cn('absolute h-3.5 w-3.5 transition-opacity duration-100', open ? 'opacity-100' : 'opacity-0 group-hover/tool:opacity-100')} />
            </>
          )}
        </span>
        <span className="shrink-0 text-[13px] leading-6 text-muted-foreground">{t(toolTitleKey(call.name))}</span>
        {summary !== '' ? (
          <>
            <span aria-hidden className="mx-2 h-0.5 w-0.5 shrink-0 rounded-full bg-muted-foreground/60" />
            <span className={cn('min-w-0 flex-1 truncate text-[13px] leading-6', failure !== '' ? 'text-destructive' : 'text-muted-foreground/80')}>
              {summary}
            </span>
          </>
        ) : <span className="min-w-0 flex-1" />}
        {suffix !== '' ? <span className="ml-2 shrink-0 font-mono text-[11px] text-muted-foreground">{suffix}</span> : null}
        {call.status === 'running' ? (
          <span aria-hidden className="row-sweep pointer-events-none absolute inset-y-0 left-0 w-1/3 bg-gradient-to-r from-transparent via-background/70 to-transparent" />
        ) : null}
      </button>

      {open ? (
        <div className="mt-1 space-y-2 rounded-lg border border-border bg-card/60 p-2" data-tool-body="">
          {path !== '' ? <div className="truncate font-mono text-[11px] text-muted-foreground">{path}</div> : null}
          {diff !== null ? (
            <DiffView diff={diff.diff} />
          ) : (
            <>
              {variant === 'shell' && (exit.exitCode !== '' || exit.signal !== '') ? (
                <div className={cn('inline-block rounded px-1.5 py-0.5 font-mono text-[11px]', exit.exitCode === '0' ? 'bg-muted text-muted-foreground' : 'bg-destructive/10 text-destructive')}>
                  {exit.exitCode !== '' ? t('chat.toolExitCode', { code: exit.exitCode }) : t('chat.toolSignal', { signal: exit.signal })}
                </div>
              ) : null}
              {args !== '' && variant !== 'shell' ? <FoldSection label={t('chat.toolInput')} text={args} mono /> : null}
              {variant === 'shell' && args !== '' ? <FoldSection label={t('chat.toolInput')} text={args} mono /> : null}
              {call.error !== '' ? (
                <FoldSection label={t('chat.toolOutput')} text={call.error} mono tone="error" />
              ) : variant === 'shell' ? (
                output !== '' ? <FoldSection label={t('chat.toolOutput')} text={output} mono /> : <div className="text-[11px] text-muted-foreground">{t('chat.toolNoOutput')}</div>
              ) : output !== '' ? (
                <FoldSection label={t('chat.toolOutput')} text={output} mono={variant === 'read' || variant === 'search'} />
              ) : (
                <div className="text-[11px] text-muted-foreground">{t('chat.toolNoOutput')}</div>
              )}
            </>
          )}
        </div>
      ) : null}
    </div>
  );
}
