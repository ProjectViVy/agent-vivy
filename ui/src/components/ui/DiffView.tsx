import { Fragment, useMemo, useState } from 'react';
import type { ParsedDiff } from '@/lib/diff';
import { diffSplitRows, parseUnifiedDiff } from '@/lib/diff';
import { useTranslation } from '@/i18n';
import { cn } from '@/lib/utils';

// 统一/分栏双模式 diff 渲染（对照 Crush 呈现行为，VC-1f / D10）：
// 头部为 +N −M 统计与视图切换，正文行号 + 着色；截断提示由服务端
// [diff truncated] 标记驱动。解析交给 lib/diff.ts，这里只管呈现。
const NUM_CELL = 'w-10 min-w-10 select-none pr-2 text-right align-top text-muted-foreground/60';

function lineCellClass(type: ParsedDiff['hunks'][number]['lines'][number]['type']): string {
  if (type === 'add') return 'bg-emerald-500/10 text-emerald-800 dark:text-emerald-200';
  if (type === 'del') return 'bg-rose-500/10 text-rose-800 dark:text-rose-200';
  return '';
}

function UnifiedTable({ parsed }: { parsed: ParsedDiff }) {
  return (
    <table className="w-full border-collapse">
      <tbody>
        {parsed.hunks.map((hunk, hunkIndex) => (
          <Fragment key={hunkIndex}>
            <tr className="bg-muted/60 text-muted-foreground">
              <td colSpan={3} className="select-all px-2 py-0.5">{hunk.header}</td>
            </tr>
            {hunk.lines.map((line, lineIndex) => {
              if (line.type === 'meta') {
                return (
                  <tr key={lineIndex} className="text-muted-foreground">
                    <td className={NUM_CELL} />
                    <td className={NUM_CELL} />
                    <td className="whitespace-pre-wrap break-all px-2">{line.text}</td>
                  </tr>
                );
              }
              return (
                <tr key={lineIndex} className={lineCellClass(line.type)}>
                  <td className={NUM_CELL}>{line.old ?? ''}</td>
                  <td className={NUM_CELL}>{line.new ?? ''}</td>
                  <td className="whitespace-pre-wrap break-all px-2">{(line.type === 'add' ? '+' : line.type === 'del' ? '-' : ' ') + line.text}</td>
                </tr>
              );
            })}
          </Fragment>
        ))}
      </tbody>
    </table>
  );
}

function SplitTable({ parsed }: { parsed: ParsedDiff }) {
  return (
    <table className="w-full table-fixed border-collapse">
      <colgroup>
        <col className="w-10" />
        <col />
        <col className="w-10" />
        <col />
      </colgroup>
      <tbody>
        {parsed.hunks.map((hunk, hunkIndex) => (
          <Fragment key={hunkIndex}>
            <tr className="bg-muted/60 text-muted-foreground">
              <td colSpan={4} className="select-all px-2 py-0.5">{hunk.header}</td>
            </tr>
            {diffSplitRows(hunk).map((row, rowIndex) => (
              <tr key={rowIndex}>
                <td className={NUM_CELL}>{row.left?.old ?? ''}</td>
                <td className={cn('whitespace-pre-wrap break-all px-2', row.left ? lineCellClass(row.left.type) : 'bg-muted/20')}>{row.left?.text ?? ''}</td>
                <td className={NUM_CELL}>{row.right?.new ?? ''}</td>
                <td className={cn('whitespace-pre-wrap break-all px-2', row.right ? lineCellClass(row.right.type) : 'bg-muted/20')}>{row.right?.text ?? ''}</td>
              </tr>
            ))}
          </Fragment>
        ))}
      </tbody>
    </table>
  );
}

export function DiffView({ diff, className }: { diff: string; className?: string }) {
  const { t } = useTranslation();
  const [mode, setMode] = useState<'unified' | 'split'>('unified');
  const parsed = useMemo(() => parseUnifiedDiff(diff), [diff]);

  if (!parsed) {
    return <pre className={cn('overflow-auto whitespace-pre-wrap break-words rounded-lg bg-muted p-3 text-xs', className)}>{diff}</pre>;
  }

  const modeButton = (value: 'unified' | 'split', label: string) => (
    <button
      type="button"
      aria-pressed={mode === value}
      onClick={() => setMode(value)}
      className={cn(
        'rounded-md px-2 py-0.5 text-xs transition-colors hover:bg-accent hover:text-accent-foreground',
        mode === value ? 'bg-accent text-accent-foreground' : 'text-muted-foreground',
      )}
    >
      {label}
    </button>
  );

  return (
    <div className={cn('min-w-0', className)}>
      <div className="flex items-center justify-between gap-2 pb-1.5">
        <span className="font-mono text-xs" aria-label={t('diff.statsLabel', { additions: parsed.additions, deletions: parsed.deletions })}>
          <span className="text-emerald-600 dark:text-emerald-400">+{parsed.additions}</span>
          {' '}
          <span className="text-rose-600 dark:text-rose-400">−{parsed.deletions}</span>
        </span>
        <div className="flex items-center gap-0.5">
          {modeButton('unified', t('diff.unified'))}
          {modeButton('split', t('diff.split'))}
        </div>
      </div>
      <div className="max-h-96 overflow-auto rounded-lg border bg-muted/30 font-mono text-xs leading-5">
        {mode === 'unified' ? <UnifiedTable parsed={parsed} /> : <SplitTable parsed={parsed} />}
      </div>
      {parsed.truncated ? <p className="pt-1 text-xs text-muted-foreground">{t('diff.truncated')}</p> : null}
    </div>
  );
}
