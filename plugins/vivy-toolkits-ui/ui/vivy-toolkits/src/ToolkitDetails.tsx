import { useId, useState } from 'react';
import type { UITranslator } from '@vivy/ui-sdk';
import { Loader2, Play, PowerOff, Trash2 } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import type { ToolkitInspection } from './toolkit-client';
import {
  installationStateLabel,
  installationStateTone,
  isPending,
  shortDigest,
  sourceKindLabel,
  sourceKindOf,
  type PendingAction,
  type SourceKind,
} from './toolkit-view';

export interface SourceChange {
  readonly kind: SourceKind;
  readonly location: string;
  readonly ref: string;
}

/** Releases retained for rollback: everything except the installed one. */
function priorReleasesOf(inspection: ToolkitInspection): ToolkitInspection['releases'] {
  return inspection.releases.filter((item) => item.release_digest !== inspection.release.release_digest);
}

/**
 * The selected Toolkit: what it is, and the lifecycle actions available for its
 * current state.
 *
 * The update/rebind fields belong to this Toolkit alone (the caller remounts
 * this component per install id), so they can never be confused with the
 * install dialog's source input.
 */
export function ToolkitDetails({
  inspection,
  pending,
  mutationsLocked,
  onPrepare,
  onPrepareSource,
  onPrepareRollback,
  t,
}: {
  readonly inspection: ToolkitInspection;
  readonly pending: PendingAction | null;
  readonly mutationsLocked: boolean;
  readonly onPrepare: (mutation: 'enable' | 'disable' | 'uninstall') => void;
  readonly onPrepareSource: (mutation: 'update' | 'rebind', change: SourceChange) => void;
  readonly onPrepareRollback: (releaseDigest: string) => void;
  readonly t: UITranslator;
}) {
  const { installation, release, manifest } = inspection;
  const priorReleases = priorReleasesOf(inspection);
  const [kind, setKind] = useState<SourceKind>(() => sourceKindOf(installation.source.kind));
  const [location, setLocation] = useState(() => installation.source.origin);
  const [ref, setRef] = useState(() => installation.source.requested_ref || 'main');
  const [rollback, setRollback] = useState(() => priorReleases[0]?.release_digest ?? '');
  const kindLabelId = useId();
  const locationId = useId();
  const refId = useId();
  const rollbackId = useId();

  const state = installation.state;
  const updating = isPending(pending, 'source', 'update');
  const rebinding = isPending(pending, 'source', 'rebind');
  const rollingBack = isPending(pending, 'rollback');
  const sourceBusy = updating || rebinding || rollingBack;
  const sourceDisabled = mutationsLocked || sourceBusy || !location.trim();
  const rollbackValue = priorReleases.some((item) => item.release_digest === rollback)
    ? rollback
    : (priorReleases[0]?.release_digest ?? '');

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader className="gap-2 space-y-0 p-4 sm:p-5">
          <div className="flex flex-wrap items-center gap-1.5">
            <Badge variant={installationStateTone(state)}>{installationStateLabel(t, state)}</Badge>
            <Badge variant="secondary" className="font-normal">
              {sourceKindLabel(t, installation.source.kind)}
            </Badge>
            {manifest ? (
              <Badge variant="outline" className="font-mono font-normal">
                {manifest.version}
              </Badge>
            ) : null}
          </div>
          <h2 className="break-words text-lg font-semibold leading-tight">
            {manifest?.name ?? installation.bundle_id}
          </h2>
          <p className="break-all font-mono text-xs text-muted-foreground">{installation.bundle_id}</p>
        </CardHeader>
        <CardContent className="space-y-3 p-4 pt-0 sm:p-5 sm:pt-0">
          {manifest?.description ? (
            <p className="text-sm text-muted-foreground">{manifest.description}</p>
          ) : null}
          <dl className="grid gap-x-6 gap-y-1 text-xs sm:grid-cols-[max-content_minmax(0,1fr)]">
            <dt className="text-muted-foreground">{t('plugin.vivy/toolkits-ui.sourceKind')}</dt>
            <dd className="break-all font-mono">
              {installation.source.origin}
              {installation.source.requested_ref ? ` @ ${installation.source.requested_ref}` : ''}
              {installation.source.resolved_commit
                ? ` (${shortDigest(installation.source.resolved_commit)})`
                : ''}
            </dd>
            <dt className="text-muted-foreground">{t('plugin.vivy/toolkits-ui.releaseLabel')}</dt>
            <dd>{release.release_sequence}</dd>
          </dl>
          <p className="break-all text-xs">
            {t('plugin.vivy/toolkits-ui.releaseDigest')} <span className="font-mono">{installation.release_digest}</span>
          </p>

          <div className="flex flex-wrap gap-2 border-t pt-3">
            {state === 'disabled' ? (
              <Button
                type="button"
                data-action="prepare-enable"
                disabled={mutationsLocked || isPending(pending, 'prepare', 'enable')}
                onClick={() => onPrepare('enable')}
              >
                {isPending(pending, 'prepare', 'enable') ? (
                  <Loader2 className="h-4 w-4 animate-spin" aria-hidden />
                ) : (
                  <Play className="h-4 w-4" aria-hidden />
                )}
                {t('plugin.vivy/toolkits-ui.prepareEnable')}
              </Button>
            ) : null}
            {state === 'enabled' ? (
              <Button
                type="button"
                variant="outline"
                data-action="prepare-disable"
                disabled={mutationsLocked || isPending(pending, 'prepare', 'disable')}
                onClick={() => onPrepare('disable')}
              >
                {isPending(pending, 'prepare', 'disable') ? (
                  <Loader2 className="h-4 w-4 animate-spin" aria-hidden />
                ) : (
                  <PowerOff className="h-4 w-4" aria-hidden />
                )}
                {t('plugin.vivy/toolkits-ui.prepareDisable')}
              </Button>
            ) : null}
            {state === 'disabled' ? (
              <Button
                type="button"
                variant="destructive"
                data-action="prepare-uninstall"
                disabled={mutationsLocked || isPending(pending, 'prepare', 'uninstall')}
                onClick={() => onPrepare('uninstall')}
              >
                {isPending(pending, 'prepare', 'uninstall') ? (
                  <Loader2 className="h-4 w-4 animate-spin" aria-hidden />
                ) : (
                  <Trash2 className="h-4 w-4" aria-hidden />
                )}
                {t('plugin.vivy/toolkits-ui.prepareUninstall')}
              </Button>
            ) : null}
            {state === 'orphaned' ? (
              <p className="text-xs text-muted-foreground">{t('plugin.vivy/toolkits-ui.orphanedHint')}</p>
            ) : null}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="gap-1.5 space-y-0 p-4 sm:p-5">
          <h3 className="text-base font-semibold">{t('plugin.vivy/toolkits-ui.sourceChangeTitle')}</h3>
          <p className="text-xs text-muted-foreground">{t('plugin.vivy/toolkits-ui.sourceChangeHint')}</p>
        </CardHeader>
        <CardContent className="space-y-4 p-4 pt-0 sm:p-5 sm:pt-0">
          <div className="space-y-2">
            <span id={kindLabelId} className="text-sm font-medium">
              {t('plugin.vivy/toolkits-ui.sourceKind')}
            </span>
            <ToggleGroup
              type="single"
              value={kind}
              aria-labelledby={kindLabelId}
              className="justify-start"
              onValueChange={(value) => {
                if (value !== 'local' && value !== 'git') return;
                setKind(value);
              }}
            >
              <ToggleGroupItem value="local" aria-label={t('plugin.vivy/toolkits-ui.localSource')}>
                {t('plugin.vivy/toolkits-ui.localSource')}
              </ToggleGroupItem>
              <ToggleGroupItem value="git" aria-label={t('plugin.vivy/toolkits-ui.gitSource')}>
                {t('plugin.vivy/toolkits-ui.gitSource')}
              </ToggleGroupItem>
            </ToggleGroup>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor={locationId}>
              {kind === 'local'
                ? t('plugin.vivy/toolkits-ui.localPath')
                : t('plugin.vivy/toolkits-ui.gitAddress')}
            </Label>
            <Input
              id={locationId}
              value={location}
              autoComplete="off"
              spellCheck={false}
              onChange={(event) => setLocation(event.target.value)}
            />
          </div>

          {kind === 'git' ? (
            <div className="space-y-1.5">
              <Label htmlFor={refId}>{t('plugin.vivy/toolkits-ui.gitRef')}</Label>
              <Input
                id={refId}
                value={ref}
                autoComplete="off"
                spellCheck={false}
                onChange={(event) => setRef(event.target.value)}
              />
            </div>
          ) : (
            <p className="text-xs text-muted-foreground">{t('plugin.vivy/toolkits-ui.hostPathHint')}</p>
          )}

          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="outline"
              data-action="prepare-update"
              disabled={sourceDisabled}
              onClick={() => onPrepareSource('update', { kind, location: location.trim(), ref: ref.trim() || 'HEAD' })}
            >
              {updating ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden /> : null}
              {t('plugin.vivy/toolkits-ui.prepareUpdate')}
            </Button>
            <Button
              type="button"
              variant="outline"
              data-action="prepare-rebind"
              disabled={sourceDisabled}
              onClick={() => onPrepareSource('rebind', { kind, location: location.trim(), ref: ref.trim() || 'HEAD' })}
            >
              {rebinding ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden /> : null}
              {t('plugin.vivy/toolkits-ui.prepareRebind')}
            </Button>
          </div>

          {priorReleases.length > 0 ? (
            <div className="space-y-2 border-t pt-3">
              <Label htmlFor={rollbackId} className="text-sm">
                {t('plugin.vivy/toolkits-ui.rollbackRelease')}
              </Label>
              <Select value={rollbackValue} onValueChange={setRollback}>
                <SelectTrigger id={rollbackId} className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {priorReleases.map((item) => (
                    <SelectItem key={item.release_digest} value={item.release_digest}>
                      #{item.release_sequence} · {shortDigest(item.release_digest)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button
                type="button"
                variant="outline"
                data-action="prepare-rollback"
                disabled={mutationsLocked || sourceBusy || !rollbackValue}
                onClick={() => onPrepareRollback(rollbackValue)}
              >
                {rollingBack ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden /> : null}
                {t('plugin.vivy/toolkits-ui.prepareRollback')}
              </Button>
            </div>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
