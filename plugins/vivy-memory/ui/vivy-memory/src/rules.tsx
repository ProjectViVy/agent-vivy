import { useCallback, useEffect, useState } from 'react';
import { Pencil } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { FailureNotice } from '@/components/feedback/FailureNotice';
import { Textarea } from '@/components/ui/textarea';
import type { UITranslator } from '@vivy/ui-sdk';
import { MEMORY_REASONS, type MemoryClient } from './memory-client';

const KNOWN_RULES_SOURCE = new Set(['default', 'file']);

interface RulesView {
  readonly content: string;
  readonly source: string;
  readonly exists: boolean;
}

interface RulesPanelProps {
  readonly client: MemoryClient;
  readonly t: UITranslator;
}

/**
 * MEMRULES handbook editor. rules.write is guarded by exact content and existence CAS:
 * the values from the latest rules.read go back as preconditions, and a
 * conflict reloads the authoritative copy instead of overwriting it.
 */
export function MemoryRulesPanel({ client, t }: RulesPanelProps) {
  const [view, setView] = useState<RulesView | null>(null);
  const [draft, setDraft] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [conflict, setConflict] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const outcome = await client.rulesRead();
      if (outcome.status === 'listed') {
        setView({ content: outcome.content ?? '', source: outcome.source ?? 'file', exists: outcome.exists === true });
      } else {
        setError(outcome.reason ?? outcome.status);
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setLoading(false);
    }
  }, [client]);

  useEffect(() => { void load(); }, [load]);

  const save = async () => {
    if (busy || draft === null || !view) return;
    const trimmed = draft.trim();
    if (!trimmed) return;
    setBusy(true);
    setError(null);
    setConflict(false);
    try {
      const outcome = await client.rulesWrite({ content: draft, expected_content: view.content, expected_exists: view.exists });
      if (outcome.status === 'applied') {
        setDraft(null);
        await load();
        return;
      }
      if (outcome.reason === MEMORY_REASONS.revisionConflict) {
        setConflict(true);
      } else {
        setError(outcome.reason ?? outcome.status);
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy(false);
    }
  };

  if (loading && !view) {
    return (
      <div className="space-y-2 p-6">
        <div className="h-16 animate-pulse rounded-lg bg-sunken" />
        <div className="h-16 animate-pulse rounded-lg bg-sunken" />
      </div>
    );
  }

  if (error && !view) {
    return (
      <div className="p-6">
        <div className="mx-auto max-w-xl">
          <FailureNotice variant="compact" title={t('plugin.vivy/memory.loadFailed')} error={error} onRetry={() => void load()} />
        </div>
      </div>
    );
  }

  if (!view) return null;

  const editing = draft !== null;

  return (
    <div className="h-full overflow-auto p-4 sm:p-6">
      <div className="mx-auto max-w-3xl space-y-3">
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-2 text-ui text-fg-muted">
            <span className="font-medium text-fg">MEMRULES</span>
            <Badge variant="outline">{rulesSourceLabel(t, view.source)}</Badge>
          </div>
          {editing ? (
            <div className="flex gap-2">
              <Button variant="outline" size="sm" onClick={() => { setDraft(null); setConflict(false); setError(null); }}>{t('common.cancel')}</Button>
              <Button size="sm" disabled={busy || !draft.trim()} onClick={() => void save()}>{t('common.save')}</Button>
            </div>
          ) : (
            <Button variant="outline" size="sm" onClick={() => { setDraft(view.content); setError(null); setConflict(false); }}>
              <Pencil className="mr-1.5 h-3.5 w-3.5" />{t('common.edit')}
            </Button>
          )}
        </div>
        {conflict ? (
          <FailureNotice variant="compact" title={t('plugin.vivy/memory.conflict')} onRetry={() => { setDraft(null); void load(); }} retryLabel={t('common.refresh')} />
        ) : error ? (
          <FailureNotice variant="compact" title={t('errors.genericTitle')} error={error} />
        ) : null}
        {editing ? (
          <Textarea
            className="min-h-[50vh] font-mono text-ui"
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            aria-label="MEMRULES"
          />
        ) : (
          <pre className="whitespace-pre-wrap break-words rounded-lg border bg-sunken/40 p-4 text-ui leading-7">{view.content}</pre>
        )}
      </div>
    </div>
  );
}

function rulesSourceLabel(t: UITranslator, source: string): string {
  return KNOWN_RULES_SOURCE.has(source) ? t(`plugin.vivy/memory.rulesSource.${source}`) : source;
}
