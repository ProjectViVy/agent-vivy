/**
 * Report generation controls bound to the sealed `vivy.reports.*` actions.
 * The panel appears only while the backend reports capability answers; it
 * drives progress strictly by the admitted Run ID — never by elapsed time,
 * a polling ledger, or a guessed percentage. Reconnection re-reads the same
 * run's authoritative state.
 */
import { useEffect, useMemo, useRef, useState } from 'react';
import { FileClock, Loader2, Square } from 'lucide-react';
import { usePluginHost, usePluginTranslation } from '@vivy/ui-sdk';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { NotebookError, ReportsClient, newOperationKey } from './api';
import {
  NOTEBOOK_ERROR_CODES,
  type Entry,
  type ReportPeriod,
  type ReportResult,
  type ReportSettings,
  type ReportWindowSelector,
} from './types';

const PERIODS: readonly ReportPeriod[] = ['daily', 'weekly', 'monthly'];
const WINDOWS: readonly ReportWindowSelector[] = ['completed', 'current'];
const TERMINAL = new Set(['completed', 'failed', 'cancelled']);
/** Bounded status re-reads while a run is active; not a progress meter. */
const STATUS_POLL_MS = 1500;

export interface NotebookReportsProps {
  /** Selected entry, when it belongs to a report series. */
  readonly entry: Entry | null;
  /** Dirty-editor gate: generation may not disturb an unsaved buffer. */
  readonly onGenerated?: (result: ReportResult) => void;
}

type Capability = 'unknown' | 'available' | 'absent';

export function NotebookReports({ entry, onGenerated }: NotebookReportsProps) {
  const host = usePluginHost();
  const { t } = usePluginTranslation();
  const client = useMemo(() => (host?.rpc ? ReportsClient.fromRPC(host.rpc) : null), [host?.rpc]);

  const [capability, setCapability] = useState<Capability>('unknown');
  const [period, setPeriod] = useState<ReportPeriod>('daily');
  const [windowSel, setWindowSel] = useState<ReportWindowSelector>('completed');
  const [settings, setSettings] = useState<ReportSettings | null>(null);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState<{ timezone: string; schedule: string; section: string; provider: string; model: string; enabled: boolean } | null>(null);
  const [active, setActive] = useState<ReportResult | null>(null);
  const [opKey, setOpKey] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const epoch = useRef(0);

  const isReportEntry = entry?.kind === 'report' || !!entry?.report_series_id;

  // Probe the capability once: a settings.read on the selected period
  // answers with the materialized default row when reports are compiled in.
  useEffect(() => {
    if (!client) return;
    const my = ++epoch.current;
    client.readSettings({ period })
      .then((row) => {
        if (my !== epoch.current) return;
        setCapability('available');
        setSettings(row);
      })
      .catch((cause) => {
        if (my !== epoch.current) return;
        if (cause instanceof NotebookError && cause.code === NOTEBOOK_ERROR_CODES.capabilityUnavailable) {
          setCapability('absent');
        } else {
          // Settings row absent still means reports exist; capability is
          // real even when the read surfaces not_found/invalid.
          setCapability('available');
          setSettings(null);
        }
      });
  }, [client, period]);

  // Track the admitted run only by its Run ID; reconnection issues the same
  // bounded status read rather than guessing state from socket history.
  useEffect(() => {
    if (!client || !active || TERMINAL.has(active.status)) {
      if (pollRef.current) { clearInterval(pollRef.current); pollRef.current = null; }
      return;
    }
    const runId = active.run_id;
    const tick = async () => {
      try {
        const result = await client.get({ run_id: runId });
        setActive(result);
        if (TERMINAL.has(result.status)) {
          if (result.generation) onGenerated?.(result);
          setOpKey(null);
        }
      } catch (cause) {
        if (cause instanceof NotebookError && cause.code === NOTEBOOK_ERROR_CODES.outcomeUnknown) {
          setError(t('plugin.vivy/notebook.reports.outcomeUnknown'));
          setOpKey(null);
          setActive((current) => (current ? { ...current, status: 'recovery_required' } : current));
        }
      }
    };
    pollRef.current = setInterval(tick, STATUS_POLL_MS);
    return () => { if (pollRef.current) { clearInterval(pollRef.current); pollRef.current = null; } };
  }, [client, active?.run_id, active?.status]);

  if (capability === 'absent' || !client) return null;

  const generating = !!active && !TERMINAL.has(active.status);
  const submit = async () => {
    if (generating) return;
    setError(null);
    const key = opKey ?? newOperationKey();
    setOpKey(key);
    try {
      const admission = await client.generate({
        period, window: windowSel, operationKey: key,
        target: isReportEntry ? { entry_id: entry!.id } : undefined,
      });
      const result = await client.get({ run_id: admission.run_id });
      setActive(result);
      if (admission.busy) setError(t('plugin.vivy/notebook.reports.busy'));
      if (TERMINAL.has(result.status)) {
        if (result.generation) onGenerated?.(result);
        setOpKey(null);
      }
    } catch (cause) {
      if (cause instanceof NotebookError) {
        // A lost ack keeps the same operation key so a retry replays the
        // admission instead of minting a second run.
        if (cause.retryable) { setError(t('plugin.vivy/notebook.reports.retryAfterLostAck')); return; }
        setError(cause.message);
        setOpKey(null);
        return;
      }
      setError(String(cause));
      setOpKey(null);
    }
  };

  const cancel = async () => {
    if (!active) return;
    try {
      await client.cancel({ run_id: active.run_id });
      setActive((current) => (current ? { ...current, status: 'cancelled' } : current));
      setOpKey(null);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  };

  return (
    <div className="flex items-center gap-2" data-testid="notebook-reports">
      <FileClock className="h-4 w-4 text-muted-foreground" aria-hidden />
      <select
        aria-label={t('plugin.vivy/notebook.reports.period')}
        className="h-8 rounded-md border bg-background px-2 text-sm"
        value={period}
        disabled={generating}
        onChange={(event) => setPeriod(event.target.value as ReportPeriod)}
        data-testid="report-period"
      >
        {PERIODS.map((value) => (
          <option key={value} value={value}>{t(`plugin.vivy/notebook.reports.period.${value}`)}</option>
        ))}
      </select>
      <select
        aria-label={t('plugin.vivy/notebook.reports.window')}
        className="h-8 rounded-md border bg-background px-2 text-sm"
        value={windowSel}
        disabled={generating}
        onChange={(event) => setWindowSel(event.target.value as ReportWindowSelector)}
        data-testid="report-window"
      >
        {WINDOWS.map((value) => (
          <option key={value} value={value}>{t(`plugin.vivy/notebook.reports.window.${value}`)}</option>
        ))}
      </select>
      <Button
        size="sm"
        variant={isReportEntry ? 'outline' : 'default'}
        disabled={generating}
        onClick={() => void submit()}
        data-testid="report-generate"
      >
        {generating && <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" aria-hidden />}
        {isReportEntry ? t('plugin.vivy/notebook.reports.regenerate') : t('plugin.vivy/notebook.reports.generate')}
      </Button>
      {active && (
        <Badge variant={TERMINAL.has(active.status) ? 'secondary' : 'default'} data-testid="report-status">
          {t(`plugin.vivy/notebook.reports.status.${active.status}`, { defaultValue: active.status })}
        </Badge>
      )}
      {generating && (
        <Button size="sm" variant="ghost" onClick={() => void cancel()} data-testid="report-cancel">
          <Square className="mr-1 h-3.5 w-3.5" aria-hidden />
          {t('plugin.vivy/notebook.reports.cancel')}
        </Button>
      )}
      {settings?.enabled === false && windowSel === 'completed' && (
        <span className="text-xs text-muted-foreground" data-testid="report-settings-note">
          {t('plugin.vivy/notebook.reports.settingsDisabled')}
        </span>
      )}
      {settings && !editing && (
        <Button size="sm" variant="ghost" data-testid="report-settings-open"
          onClick={() => {
            setEditing(true);
            setDraft({ timezone: settings.timezone || 'UTC', schedule: settings.schedule_expr || '0 9 * * *',
              section: settings.section_id, provider: settings.provider || '', model: settings.model_id || '',
              enabled: settings.enabled });
          }}>
          {t('plugin.vivy/notebook.reports.settings')}
        </Button>
      )}
      {error && <span className="text-xs text-destructive" role="alert">{error}</span>}
      {editing && draft && settings && (
        <div className="mt-2 grid gap-2 rounded border p-2" data-testid="report-settings-form">
          <label className="flex items-center gap-2 text-xs">
            <input type="checkbox" checked={draft.enabled}
              onChange={(e) => setDraft({ ...draft, enabled: e.target.checked })} data-testid="settings-enabled" />
            {t('plugin.vivy/notebook.reports.settings.enabled')}
          </label>
          <input className="rounded border bg-background px-2 py-1 text-xs" value={draft.schedule}
            onChange={(e) => setDraft({ ...draft, schedule: e.target.value })}
            placeholder={t('plugin.vivy/notebook.reports.settings.schedule')} data-testid="settings-schedule" />
          <input className="rounded border bg-background px-2 py-1 text-xs" value={draft.timezone}
            onChange={(e) => setDraft({ ...draft, timezone: e.target.value })}
            placeholder={t('plugin.vivy/notebook.reports.settings.timezone')} data-testid="settings-timezone" />
          <input className="rounded border bg-background px-2 py-1 text-xs" value={draft.section}
            onChange={(e) => setDraft({ ...draft, section: e.target.value })}
            placeholder={t('plugin.vivy/notebook.reports.settings.destination')} data-testid="settings-section" />
          <input className="rounded border bg-background px-2 py-1 text-xs" value={draft.provider}
            onChange={(e) => setDraft({ ...draft, provider: e.target.value })}
            placeholder={t('plugin.vivy/notebook.reports.settings.provider')} data-testid="settings-provider" />
          <input className="rounded border bg-background px-2 py-1 text-xs" value={draft.model}
            onChange={(e) => setDraft({ ...draft, model: e.target.value })}
            placeholder={t('plugin.vivy/notebook.reports.settings.model')} data-testid="settings-model" />
          <div className="flex gap-2">
            <Button size="sm" data-testid="settings-save" onClick={() => void (async () => {
              setError(null);
              try {
                const res = await client.writeSettings({
                  period, expected_revision: settings.revision, operation_key: newOperationKey(),
                  timezone: draft.timezone, section_id: draft.section,
                  provider: draft.provider || undefined, model_id: draft.model || undefined,
                  enabled: draft.enabled, schedule_expr: draft.schedule,
                });
                setSettings(res.settings);
                setEditing(false);
              } catch (cause) {
                if (cause instanceof NotebookError && cause.code === 'idempotency_conflict') {
                  const fresh = await client.readSettings({ period });
                  setSettings(fresh);
                  setEditing(false);
                  setError(t('plugin.vivy/notebook.reports.settings.conflict'));
                } else {
                  setError(cause instanceof Error ? cause.message : String(cause));
                }
              }
            })()}>
              {t('plugin.vivy/notebook.reports.settings.save')}
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setEditing(false)}>
              {t('plugin.vivy/notebook.reports.settings.discard')}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
