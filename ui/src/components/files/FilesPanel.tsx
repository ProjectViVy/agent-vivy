import { useCallback, useEffect, useMemo, useState } from 'react';
import hljs from 'highlight.js/lib/common';
import 'highlight.js/styles/github-dark.css';
import { Button } from '@/components/ui/button';
import { useVivyStore } from '@/lib/store';
import { listWorkspaceFiles, readWorkspaceFile, type WorkspaceFile } from '@/lib/api';
import { useTranslation } from '@/i18n';
import { RefreshCw } from 'lucide-react';
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
    return <p className="p-4 text-sm text-muted-foreground">{t('files.noRun')}</p>;
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
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
