import type { UITranslator } from '@vivy/ui-sdk';
import { AlertTriangle, Check, Loader2, ShieldAlert, X } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import type { PreparedCandidate } from './toolkit-client';
import {
  changeVerbLabel,
  componentKindOf,
  isPending,
  mutationLabel,
  shortDigest,
  type CandidateTarget,
  type PendingAction,
} from './toolkit-view';

/**
 * A prepared change under review.
 *
 * It is the only place a catalog change is approved, so it names the Toolkit it
 * applies to, keeps its evidence (digests, component diff, drain blockers) and
 * stays on screen until the outcome is confirmed.
 */
export function ToolkitCandidate({
  candidate,
  target,
  pending,
  onPublish,
  onCancel,
  t,
}: {
  readonly candidate: PreparedCandidate;
  readonly target: CandidateTarget;
  readonly pending: PendingAction | null;
  readonly onPublish: () => void;
  readonly onCancel: () => void;
  readonly t: UITranslator;
}) {
  const prepared = candidate.operation.prepared;
  const changes = prepared?.changes ?? [];
  const blockers = candidate.operation.blockers;
  const publishing = isPending(pending, 'publish');
  const cancelling = isPending(pending, 'cancel');
  const busy = publishing || cancelling;
  const name = target.name || target.installId;
  return (
    <Card className="border-primary/40" data-testid="prepared-candidate">
      <CardHeader className="gap-2 space-y-0 p-4 sm:p-5">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <Badge variant="secondary" className="shrink-0">
            {t('plugin.vivy/toolkits-ui.candidateTitle')}
          </Badge>
          {target.mutationKind ? (
            <Badge variant="outline" className="shrink-0">
              {mutationLabel(t, target.mutationKind)}
            </Badge>
          ) : null}
        </div>
        <CardTitle className="text-base">
          <span className="text-muted-foreground">{t('plugin.vivy/toolkits-ui.candidateTargetPrefix')} </span>
          <span className="break-words">{name}</span>
        </CardTitle>
        <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
          <span className="break-all">
            {t('plugin.vivy/toolkits-ui.candidateDigest')}:{' '}
            <span className="font-mono" title={candidate.candidate_digest}>
              {shortDigest(candidate.candidate_digest)}
            </span>
          </span>
          <span>
            {t('plugin.vivy/toolkits-ui.basedOnEpoch')} {candidate.expected_catalog.epoch}
          </span>
          <span className="break-all">
            {t('plugin.vivy/toolkits-ui.operationID')}:{' '}
            <span className="font-mono" title={candidate.operation.id}>
              {shortDigest(candidate.operation.id, 8)}
            </span>
          </span>
        </div>
      </CardHeader>
      <CardContent className="space-y-4 p-4 pt-0 sm:p-5 sm:pt-0">
        <section aria-label={t('plugin.vivy/toolkits-ui.changeList')} className="space-y-1.5">
          <h4 className="text-xs font-medium text-muted-foreground">{t('plugin.vivy/toolkits-ui.changeList')}</h4>
          {changes.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t('plugin.vivy/toolkits-ui.noComponentChanges')}</p>
          ) : (
            <ul className="space-y-1.5">
              {changes.map((change) => (
                <li
                  key={`${change.kind}/${change.local_id}`}
                  data-change={change.change}
                  className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 rounded-lg border p-2"
                >
                  <span className="min-w-0 break-all font-mono text-xs">
                    {change.kind}/{change.local_id}: {changeVerbLabel(t, change.change)}
                  </span>
                  {change.before_digest || change.after_digest ? (
                    <span className="break-all font-mono text-[10px] text-muted-foreground" title={`${change.before_digest ?? ''} → ${change.after_digest ?? ''}`}>
                      {shortDigest(change.before_digest ?? '∅', 8)} → {shortDigest(change.after_digest ?? '∅', 8)}
                    </span>
                  ) : null}
                  {change.requires_fresh_consent ? (
                    <Badge variant="destructive" className="text-[10px]">
                      {t('plugin.vivy/toolkits-ui.freshConsent')}
                    </Badge>
                  ) : null}
                  {componentKindOf(change.kind) === 'masks' && change.change === 'removed' ? (
                    <span className="text-xs text-muted-foreground">
                      {t('plugin.vivy/toolkits-ui.maskRemovalHint')}{' '}
                      <a className="underline" href="/masks">
                        {t('plugin.vivy/toolkits-ui.openMasks')}
                      </a>
                    </span>
                  ) : null}
                </li>
              ))}
            </ul>
          )}
        </section>

        {blockers.length > 0 ? (
          <section aria-label={t('plugin.vivy/toolkits-ui.blockers')} className="space-y-1.5">
            <h4 className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
              <AlertTriangle className="h-3.5 w-3.5" aria-hidden />
              {t('plugin.vivy/toolkits-ui.blockers')}
            </h4>
            <ul className="space-y-1">
              {blockers.map((blocker) => (
                <li key={blocker.run_id} className="flex min-w-0 flex-wrap gap-x-3 text-xs text-muted-foreground">
                  <span className="break-all font-mono">
                    {t('plugin.vivy/toolkits-ui.blockerRun')} {blocker.run_id}
                  </span>
                  <span className="break-all font-mono">
                    {t('plugin.vivy/toolkits-ui.blockerSession')} {blocker.session_id}
                  </span>
                </li>
              ))}
            </ul>
            <p className="text-xs text-muted-foreground">{t('plugin.vivy/toolkits-ui.blockerHint')}</p>
          </section>
        ) : null}

        <p className="flex items-start gap-2 rounded-lg border border-dashed p-2 text-xs text-muted-foreground">
          <ShieldAlert className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden />
          {t('plugin.vivy/toolkits-ui.candidateMutationLock')}
        </p>

        <div className="flex flex-wrap justify-end gap-2">
          <Button type="button" variant="outline" data-action="cancel-candidate" disabled={busy} onClick={onCancel}>
            {cancelling ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden /> : <X className="h-4 w-4" aria-hidden />}
            {t('plugin.vivy/toolkits-ui.cancelCandidate')}
          </Button>
          <Button type="button" data-action="publish-candidate" disabled={busy} onClick={onPublish}>
            {publishing ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden /> : <Check className="h-4 w-4" aria-hidden />}
            {t('plugin.vivy/toolkits-ui.publish')}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
