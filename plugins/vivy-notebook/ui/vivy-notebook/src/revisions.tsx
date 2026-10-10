/**
 * Revision history: immutable metadata list with explicit view/adopt actions.
 * Adoption creates a new revision through the backend and never rewrites
 * history locally.
 */
import { useEffect, useRef, useState } from 'react';
import { History, RotateCcw } from 'lucide-react';
import { usePluginTranslation } from '@vivy/ui-sdk';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ScrollArea } from '@/components/ui/scroll-area';
import { newOperationKey, type NotebookClient } from './api';
import { NOTEBOOK_MAX_PAGE_ROWS, type Revision } from './types';
import { dateTimeLocale } from '@/i18n';

export interface NotebookRevisionsProps {
  readonly client: NotebookClient;
  readonly entryId: string;
  readonly headRevisionId?: string;
  readonly entryVersion: number;
  readonly refreshKey: number;
  readonly onView: (revisionId: string | null) => void;
  readonly viewingRevisionId?: string | null;
  readonly onAdopted?: () => void;
}

export function NotebookRevisions({ client, entryId, headRevisionId, entryVersion, refreshKey, onView, viewingRevisionId, onAdopted }: NotebookRevisionsProps) {
  const { t } = usePluginTranslation();
  const [revisions, setRevisions] = useState<readonly Revision[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const epoch = useRef(0);

  const load = async () => {
    const my = ++epoch.current;
    setLoading(true);
    setError(null);
    try {
      const page = await client.listRevisions({ entry_id: entryId, limit: NOTEBOOK_MAX_PAGE_ROWS });
      if (my === epoch.current) setRevisions(page.revisions);
    } catch (cause) {
      if (my === epoch.current) setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      if (my === epoch.current) setLoading(false);
    }
  };

  useEffect(() => { void load(); }, [client, entryId, refreshKey]);

  const adopt = async (revisionId: string) => {
    setBusy(revisionId);
    setActionError(null);
    try {
      await client.adoptRevision({ operationKey: newOperationKey(), entry_id: entryId, revision_id: revisionId, expected_version: entryVersion });
      onView(null);
      onAdopted?.();
      void load();
    } catch (cause) {
      setActionError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy(null);
    }
  };

  if (loading) {
    return (
      <div className="space-y-2 p-3" data-testid="notebook-revisions-loading">
        <div className="h-12 animate-pulse rounded-lg bg-muted" />
        <div className="h-12 animate-pulse rounded-lg bg-muted" />
      </div>
    );
  }
  if (error) {
    return (
      <div className="p-3 text-sm text-destructive" data-testid="notebook-revisions-error">
        <p>{error}</p>
        <Button className="mt-2" size="sm" variant="outline" onClick={() => void load()}>{t('common.retry')}</Button>
      </div>
    );
  }

  return (
    <ScrollArea className="min-h-0 flex-1" data-testid="notebook-revisions">
      {actionError ? <p className="m-3 rounded-lg border border-destructive/40 p-2 text-xs text-destructive">{actionError}</p> : null}
      {revisions.length === 0 ? (
        <p className="py-8 text-center text-sm text-muted-foreground">{t('plugin.vivy/notebook.noRevisions')}</p>
      ) : revisions.map((revision) => (
        <div key={revision.id} className="mx-2 mb-2 rounded-lg border bg-card p-3" data-testid="notebook-revision">
          <div className="flex items-center gap-2 text-xs">
            <History className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            <span className="font-medium">#{revision.sequence}</span>
            <Badge variant="secondary">{t(`plugin.vivy/notebook.origin.${revision.origin}`)}</Badge>
            {revision.id === headRevisionId ? <Badge>{t('plugin.vivy/notebook.head')}</Badge> : null}
            {revision.id === viewingRevisionId ? <Badge variant="outline">{t('plugin.vivy/notebook.viewing')}</Badge> : null}
            <span className="ml-auto text-muted-foreground">{new Date(revision.created_at * 1000).toLocaleString(dateTimeLocale())}</span>
          </div>
          <p className="mt-1 truncate text-xs text-muted-foreground">{revision.actor}</p>
          <div className="mt-2 flex gap-2">
            <Button size="sm" variant="outline" onClick={() => onView(revision.id === viewingRevisionId ? null : revision.id)} data-testid="notebook-revision-view">
              {revision.id === viewingRevisionId
                ? t('plugin.vivy/notebook.backToHead')
                : revision.origin === 'generated'
                  ? t('plugin.vivy/notebook.reports.viewGenerated')
                  : t('plugin.vivy/notebook.viewRevision')}
            </Button>
            {revision.id !== headRevisionId ? (
              <Button size="sm" variant="ghost" disabled={busy === revision.id} onClick={() => void adopt(revision.id)} data-testid="notebook-revision-adopt">
                <RotateCcw className="mr-1 h-3.5 w-3.5" />
                {revision.origin === 'generated' ? t('plugin.vivy/notebook.reports.useThisVersion') : t('plugin.vivy/notebook.adopt')}
              </Button>
            ) : null}
          </div>
        </div>
      ))}
    </ScrollArea>
  );
}
