import { useCallback, useEffect, useState } from 'react';
import { AlertTriangle, Pencil } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import type { UITranslator } from '@vivy/ui-sdk';
import { MEMORY_REASONS, type MemoryClient } from './memory-client';

const KNOWN_RULES_SOURCE = new Set(['default', 'file']);

interface RulesView {
  readonly content: string;
  readonly source: string;
  readonly revision: string;
}

interface RulesPanelProps {
  readonly client: MemoryClient;
  readonly t: UITranslator;
}

/**
 * MEMRULES handbook editor. rules.write is guarded by a content-digest CAS:
 * the token from the latest rules.read goes back as base_revision, and a
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
        setView({ content: outcome.content ?? '', source: outcome.source ?? 'file', revision: outcome.revision ?? '' });
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
      const outcome = await client.rulesWrite({ content: draft, base_revision: view.revision });
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
        <div className="h-16 animate-pulse rounded-lg bg-muted" />
        <div className="h-16 animate-pulse rounded-lg bg-muted" />
      </div>
    );
  }

  if (error && !view) {
    return (
      <div className="p-6">
        <div className="mx-auto max-w-xl rounded-xl border border-destructive/30 bg-destructive/10 p-4 text-sm text-destructive" role="alert">
          <div className="flex items-center gap-2">
            <AlertTriangle className="h-4 w-4" />
            <strong>{t('plugin.vivy/memory.loadFailed')}</strong>
          </div>
          <p className="mt-1">{error}</p>
          <Button className="mt-3" size="sm" variant="outline" onClick={() => void load()}>{t('common.retry')}</Button>
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
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <span className="font-medium text-foreground">MEMRULES</span>
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
          <div className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive" role="alert">
            <p className="flex items-center gap-2"><AlertTriangle className="h-4 w-4" />{t('plugin.vivy/memory.conflict')}</p>
            <Button className="mt-2" size="sm" variant="outline" onClick={() => { setDraft(null); void load(); }}>{t('common.refresh')}</Button>
          </div>
        ) : error ? (
          <p className="text-sm text-destructive" role="alert">{error}</p>
        ) : null}
        {editing ? (
          <Textarea
            className="min-h-[50vh] font-mono text-sm"
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            aria-label="MEMRULES"
          />
        ) : (
          <pre className="whitespace-pre-wrap break-words rounded-lg border bg-muted/40 p-4 text-sm leading-7">{view.content}</pre>
        )}
      </div>
    </div>
  );
}

function rulesSourceLabel(t: UITranslator, source: string): string {
  return KNOWN_RULES_SOURCE.has(source) ? t(`plugin.vivy/memory.rulesSource.${source}`) : source;
}
