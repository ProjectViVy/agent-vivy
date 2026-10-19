import type { UITranslator } from '@vivy/ui-sdk';
import { Plus, RefreshCw, Wrench } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import type { CatalogIdentity, ToolkitInstallation } from './toolkit-client';
import { installationStateLabel, installationStateTone, sourceKindLabel } from './toolkit-view';

/**
 * The installed Toolkit collection.
 *
 * A row carries identity, the one state that decides what the user does next,
 * and the source needed to tell two installations apart. It owns no actions:
 * the whole row is the click target and every operation lives in the details.
 */
export function ToolkitLibrary({
  items,
  selectedId,
  catalog,
  firstLoad,
  refreshing,
  mutationsLocked,
  onSelect,
  onRefresh,
  onInstall,
  t,
}: {
  readonly items: readonly ToolkitInstallation[];
  readonly selectedId: string | null;
  readonly catalog: CatalogIdentity | null;
  readonly firstLoad: boolean;
  readonly refreshing: boolean;
  readonly mutationsLocked: boolean;
  readonly onSelect: (item: ToolkitInstallation) => void;
  readonly onRefresh: () => void;
  readonly onInstall: () => void;
  readonly t: UITranslator;
}) {
  return (
    <Card className="min-w-0" data-testid="toolkit-library">
      <CardHeader className="flex-row items-start justify-between gap-3 space-y-0 p-4 sm:p-5">
        <div className="min-w-0">
          <CardTitle className="text-base">{t('plugin.vivy/toolkits-ui.library')}</CardTitle>
          {catalog ? (
            <p className="mt-1 text-xs text-muted-foreground" data-testid="catalog-epoch">
              {t('plugin.vivy/toolkits-ui.catalogEpoch')} {catalog.epoch} ·{' '}
              {t('plugin.vivy/toolkits-ui.configRevision')} {catalog.config_revision}
            </p>
          ) : null}
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            data-action="refresh"
            aria-label={refreshing ? t('plugin.vivy/toolkits-ui.refreshing') : t('plugin.vivy/toolkits-ui.refresh')}
            aria-busy={refreshing}
            disabled={refreshing}
            onClick={onRefresh}
          >
            <RefreshCw className={refreshing ? 'h-4 w-4 animate-spin' : 'h-4 w-4'} aria-hidden />
          </Button>
          <Button
            type="button"
            size="sm"
            data-action="open-install"
            disabled={mutationsLocked}
            onClick={onInstall}
          >
            <Plus className="h-4 w-4" aria-hidden />
            {t('plugin.vivy/toolkits-ui.installTitle')}
          </Button>
        </div>
      </CardHeader>
      <CardContent className="p-4 pt-0 sm:p-5 sm:pt-0">
        {firstLoad ? (
          <ToolkitLibrarySkeleton label={t('plugin.vivy/toolkits-ui.loading')} />
        ) : items.length === 0 ? (
          <div className="flex items-center gap-3 rounded-xl border border-dashed p-4 text-sm text-muted-foreground">
            <Wrench className="h-4 w-4 shrink-0" aria-hidden />
            <p>{t('plugin.vivy/toolkits-ui.empty')}</p>
          </div>
        ) : (
          <ul className="space-y-2" aria-label={t('plugin.vivy/toolkits-ui.installed')}>
            {items.map((item) => (
              <li key={item.install_id}>
                <button
                  type="button"
                  data-testid="toolkit-row"
                  data-install-id={item.install_id}
                  data-state={item.state}
                  aria-pressed={selectedId === item.install_id}
                  onClick={() => onSelect(item)}
                  className={`w-full cursor-pointer rounded-xl border p-3 text-left transition-colors ${
                    selectedId === item.install_id ? 'border-primary/50 bg-primary/5' : 'bg-card hover:bg-accent/30'
                  }`}
                >
                  <span className="flex min-w-0 items-center justify-between gap-2">
                    <span className="min-w-0 truncate text-sm font-medium">{item.bundle_id}</span>
                    <Badge variant={installationStateTone(item.state)} className="shrink-0 text-[10px]">
                      {installationStateLabel(t, item.state)}
                    </Badge>
                  </span>
                  <span className="mt-1 block truncate text-xs text-muted-foreground" title={item.source.origin}>
                    {sourceKindLabel(t, item.source.kind)} · {item.source.origin}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

/**
 * Same shape as a real row, so the first response does not move the layout.
 *
 * The label is screen-reader only: a loading node must not add a second
 * `role="status"` next to the page notice.
 */
function ToolkitLibrarySkeleton({ label }: { readonly label: string }) {
  return (
    <div className="space-y-2" data-testid="toolkit-library-loading" aria-busy="true">
      <span className="sr-only">{label}</span>
      {[0, 1, 2].map((row) => (
        <div key={row} className="rounded-xl border p-3">
          <Skeleton className="h-4 w-2/5" />
          <Skeleton className="mt-2 h-3 w-3/5" />
        </div>
      ))}
    </div>
  );
}
