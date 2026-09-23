import { useCallback, useEffect, useMemo, useState } from 'react';
import hljs from 'highlight.js/lib/common';
import 'highlight.js/styles/github-dark.css';
import { Button } from '@/components/ui/button';
import { useVivyStore } from '@/lib/store';
import { listWorkspaceFiles, readWorkspaceFile, type WorkspaceFile } from '@/lib/api';
import { useTranslation } from '@/i18n';
import { Package, RefreshCw } from 'lucide-react';
import { cn } from '@/lib/utils';

function formatBytes(size: number): string {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

function languageFor(path: string): string | null {
  const ext = path.slice(path.lastIndexOf('.') + 1).toLowerCase();
  if (!ext || ext === path) return null;
  return hljs.getLanguage(ext) ? ext : null;
}

function escapeHtml(value: string): string {
  return value.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function highlighted(content: string, path: string): string {
  const lang = languageFor(path);
  if (!lang) return escapeHtml(content);
  try {
    return hljs.highlight(content, { language: lang, ignoreIllegals: true }).value;
  } catch {
    return escapeHtml(content);
  }
}

/** 交付组区块：加法的已提交交付组列表，与工作区修改文件列表互相独立。
 * 点条目滚动聚焦聊天里的原组卡片（data-delivery-set 锚）。 */
function DeliveriesSection() {
  const { t } = useTranslation();
  const deliverySets = useVivyStore((state) => state.deliverySets);
  const deliverySetsPhase = useVivyStore((state) => state.deliverySetsPhase);
  const loadDeliverySets = useVivyStore((state) => state.loadDeliverySets);
  const activeSessionId = useVivyStore((state) => state.activeSessionId);

  useEffect(() => {
    void loadDeliverySets();
  }, [loadDeliverySets, activeSessionId]);

  const focusSet = (id: string) => {
    document.getElementById(`deliverable-${id}`)?.scrollIntoView({ behavior: 'smooth', block: 'center' });
  };

  if (deliverySetsPhase === 'loading' || deliverySetsPhase === 'idle') {
    return <p className="p-3 text-xs text-muted-foreground">{t('files.loading')}</p>;
  }
  if (deliverySets.length === 0) return null;
  return (
    <div className="border-b">
      <p className="px-3 pt-2 pb-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">{t('files.deliveries')}</p>
      <ul>
        {deliverySets.map((set) => (
          <li key={set.id}>
            <button
              type="button"
              onClick={() => focusSet(set.id)}
              className="flex w-full min-w-0 items-center gap-1.5 px-3 py-1.5 text-left text-xs hover:bg-accent"
              aria-label={t('files.deliveryFocus', { name: set.title ?? set.id })}
            >
              <Package className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden />
              <span className="min-w-0 flex-1 truncate">
                {(set.title ?? '') !== '' ? `${set.title} · ` : ''}
                {t('deliverableCard.summary', { count: set.items.length })}
              </span>
              {set.failures.length > 0 ? <span className="shrink-0 text-[10px] text-amber-600">{t('deliverableCard.failures', { count: set.failures.length })}</span> : null}
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function FilesPanel() {
  const { t } = useTranslation();
  const currentRun = useVivyStore((state) => state.currentRun);
  const [files, setFiles] = useState<WorkspaceFile[]>([]);
  const [truncated, setTruncated] = useState(false);
  const [selected, setSelected] = useState<string | null>(null);
  const [content, setContent] = useState<string | null>(null);
  const [binary, setBinary] = useState(false);
  const [fileTruncated, setFileTruncated] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const runId = currentRun?.id ?? null;

  const refresh = useCallback(async () => {
    if (!runId) return;
    setLoading(true);
    setError(null);
    try {
      const result = await listWorkspaceFiles(runId);
      setFiles(result.files);
      setTruncated(result.truncated);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [runId]);

  useEffect(() => {
    setFiles([]);
    setSelected(null);
    setContent(null);
    setBinary(false);
    setFileTruncated(false);
    setError(null);
    void refresh();
  }, [refresh]);

  const openFile = useCallback(async (path: string) => {
    if (!runId) return;
    setSelected(path);
    setContent(null);
    setBinary(false);
    setFileTruncated(false);
    try {
      const result = await readWorkspaceFile(runId, path);
      setContent(result.binary ? null : result.content);
      setBinary(result.binary);
      setFileTruncated(result.truncated);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }, [runId]);

  const html = useMemo(() => (selected && content !== null ? highlighted(content, selected) : null), [selected, content]);

  if (!runId) {
    return (
      <div className="flex h-full min-h-0 flex-col">
        <DeliveriesSection />
        <p className="p-4 text-sm text-muted-foreground">{t('files.noRun')}</p>
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <DeliveriesSection />
      <div className="flex items-center justify-between border-b px-3 py-2">
        <span className="text-xs text-muted-foreground">{t('files.count', { count: files.length })}</span>
        <Button variant="ghost" size="icon" className="h-7 w-7" title={t('files.refresh')} aria-label={t('files.refresh')} onClick={() => void refresh()} disabled={loading}>
          <RefreshCw className={cn('h-4 w-4', loading && 'animate-spin')} />
        </Button>
      </div>
      {error ? <p className="px-3 py-2 text-xs text-destructive">{error}</p> : null}
      <div className="grid min-h-0 flex-1 grid-rows-[minmax(0,auto)_minmax(0,1fr)] md:grid-cols-[minmax(0,180px)_minmax(0,1fr)] md:grid-rows-1">
        <div className="min-h-0 overflow-y-auto border-b md:border-b-0 md:border-r">
          {files.length === 0 && !loading ? <p className="p-3 text-xs text-muted-foreground">{t('files.empty')}</p> : null}
          {files.map((file) => (
            <button
              key={file.path}
              type="button"
              onClick={() => void openFile(file.path)}
              className={cn(
                'block w-full truncate px-3 py-1.5 text-left text-xs hover:bg-accent',
                selected === file.path && 'bg-accent font-medium',
              )}
              title={`${file.path} (${formatBytes(file.size)})`}
            >
              {file.path}
            </button>
          ))}
          {truncated ? <p className="p-3 text-xs text-muted-foreground">{t('files.listTruncated')}</p> : null}
        </div>
        <div className="min-h-0 overflow-auto">
          {binary ? <p className="p-3 text-xs text-muted-foreground">{t('files.binary')}</p> : null}
          {fileTruncated ? <p className="border-b px-3 py-2 text-xs text-amber-600">{t('files.truncated')}</p> : null}
          {html !== null ? (
            <pre className="hljs m-0 min-h-full whitespace-pre p-3 text-xs">
              <code dangerouslySetInnerHTML={{ __html: html }} />
            </pre>
          ) : !binary && selected ? (
            <p className="p-3 text-xs text-muted-foreground">{t('files.loading')}</p>
          ) : null}
          {!selected ? <p className="p-3 text-xs text-muted-foreground">{t('files.pick')}</p> : null}
        </div>
      </div>
    </div>
  );
}
