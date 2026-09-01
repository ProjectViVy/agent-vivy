import { useEffect, useState } from 'react';
import { Sparkles } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { settingsUpdateFrom } from '@/lib/api';
import { useVivyStore } from '@/lib/store';
import { useTranslation } from '@/i18n';

/**
 * 上下文压缩（真实）：配置持久化到 settings.yaml（settings/update），保存后
 * 立即（或最迟下一次 run）重建引擎的 Eino reduction + summarization 中间件；
 * 占用数字来自 session/context；「立即压缩」触发 context/compact 会话级压缩。
 */
export function CompactionSettingsCard() {
  const settings = useVivyStore((state) => state.settings);
  const sessionContext = useVivyStore((state) => state.sessionContext);
  const activeSessionId = useVivyStore((state) => state.activeSessionId);
  const saveSettings = useVivyStore((state) => state.saveSettings);
  const compactSession = useVivyStore((state) => state.compactSession);
  const loadSessionContext = useVivyStore((state) => state.loadSessionContext);
  const { t } = useTranslation();

  const base = settings?.compaction;
  const [enabled, setEnabled] = useState(base?.enabled ?? true);
  const [maxTokens, setMaxTokens] = useState(base?.max_tokens ?? 0);
  const [triggerPercent, setTriggerPercent] = useState(base?.trigger_percent ?? 80);
  const [keepRecent, setKeepRecent] = useState(base?.keep_recent ?? 12);
  const [saving, setSaving] = useState(false);
  const [compacting, setCompacting] = useState(false);
  const [feedback, setFeedback] = useState<string | null>(null);
  const locked = settings?.read_only || Boolean(settings?.frozen);

  useEffect(() => {
    if (!base) return;
    setEnabled(base.enabled);
    setMaxTokens(base.max_tokens);
    setTriggerPercent(base.trigger_percent);
    setKeepRecent(base.keep_recent);
  }, [base]);

  const save = async () => {
    setSaving(true);
    setFeedback(null);
    try {
      await saveSettings(settingsUpdateFrom(settings, {
        provider: settings?.provider,
        compaction: {
          enabled,
          max_tokens: Math.max(0, Number(maxTokens) || 0),
          trigger_percent: Math.min(100, Math.max(1, Number(triggerPercent) || 80)),
          keep_recent: Math.max(1, Number(keepRecent) || 1),
        },
      }));
      setFeedback(t('settings.compaction.saved'));
    } catch (error) {
      setFeedback(error instanceof Error ? error.message : String(error));
    } finally {
      setSaving(false);
    }
  };

  const compactNow = async () => {
    if (!activeSessionId) return;
    setCompacting(true);
    setFeedback(null);
    try {
      const result = await compactSession(activeSessionId);
      if (result.skipped) {
        setFeedback(t('settings.compaction.notNeeded'));
      } else {
        setFeedback(t('settings.compaction.done', {
          before: result.before_tokens.toLocaleString(),
          after: result.after_tokens.toLocaleString(),
        }));
      }
    } catch (error) {
      setFeedback(error instanceof Error ? error.message : String(error));
    } finally {
      setCompacting(false);
    }
  };

  const pressure = sessionContext && sessionContext.model_limit_tokens > 0
    ? Math.min(100, Math.round((sessionContext.feed_tokens / sessionContext.model_limit_tokens) * 100))
    : 0;
  const wouldCompact = Boolean(sessionContext?.compaction_enabled && sessionContext.would_compact);
  const last = sessionContext?.last_compaction;

  return (
    <Card>
      <CardHeader>
        <div className="mb-2 flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary"><Sparkles className="h-5 w-5" aria-hidden="true" /></div>
        <CardTitle>{t('settings.compaction.title')}</CardTitle>
        <CardDescription>{t('settings.compaction.description')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">
        <div className="flex items-center justify-between rounded-lg border p-3">
          <div className="min-w-0">
            <p className="font-medium">{t('settings.compaction.enabledLabel')}</p>
            <p className="mt-1 text-sm text-muted-foreground">{t('settings.compaction.enabledHint')}</p>
          </div>
          <Switch checked={enabled} disabled={locked} onCheckedChange={setEnabled} aria-label={t('settings.compaction.enabledLabel')} />
        </div>

        <div className="grid gap-4 sm:grid-cols-3">
          <div className="space-y-2">
            <Label htmlFor="compaction-max-tokens">{t('settings.compaction.maxTokensLabel')}</Label>
            <Input id="compaction-max-tokens" type="number" min={0} value={maxTokens} disabled={locked} onChange={(event) => setMaxTokens(Math.max(0, Number(event.target.value) || 0))} placeholder={String(base?.config_max_tokens || t('settings.compaction.autoPlaceholder'))} />
            <p className="text-xs text-muted-foreground">{t('settings.compaction.maxTokensHint')}</p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="compaction-trigger-percent">{t('settings.compaction.triggerLabel')}</Label>
            <Input id="compaction-trigger-percent" type="number" min={1} max={100} value={triggerPercent} disabled={locked} onChange={(event) => setTriggerPercent(Math.min(100, Math.max(1, Number(event.target.value) || 1)))} placeholder={String(base?.config_trigger_percent ?? 80)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="compaction-keep-recent">{t('settings.compaction.keepRecentLabel')}</Label>
            <Input id="compaction-keep-recent" type="number" min={1} value={keepRecent} disabled={locked} onChange={(event) => setKeepRecent(Math.max(1, Number(event.target.value) || 1))} placeholder={String(base?.config_keep_recent ?? 12)} />
          </div>
        </div>

        <div className="rounded-lg border p-3">
          <p className="text-sm font-medium">{t('settings.compaction.usageTitle')}</p>
          {activeSessionId && sessionContext ? (
            <div className="mt-2 space-y-2">
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">{t('settings.compaction.feedUsage')}</span>
                <span className="font-medium">{sessionContext.feed_tokens.toLocaleString()} / {sessionContext.model_limit_tokens.toLocaleString()} tokens</span>
              </div>
              <div className="h-2 overflow-hidden rounded-full bg-muted">
                <div className={`h-full rounded-full transition-all ${wouldCompact ? 'bg-destructive' : pressure >= 60 ? 'bg-amber-500' : 'bg-primary'}`} style={{ width: `${pressure}%` }} />
              </div>
              <div className="flex flex-wrap gap-2 text-xs text-muted-foreground">
                <Badge variant={wouldCompact ? 'secondary' : 'outline'}>{t('settings.compaction.pressureBadge', { percent: pressure })}</Badge>
                <span>{wouldCompact ? t('settings.compaction.thresholdReached') : t('settings.compaction.notNeeded')}</span>
                {Boolean(sessionContext.has_compaction_summary) ? <span>{t('settings.compaction.hasSummary')}</span> : null}
                {last ? <span>{t('settings.compaction.lastCompaction', { mode: last.mode, before: last.before_tokens.toLocaleString(), after: last.after_tokens.toLocaleString() })}</span> : null}
              </div>
            </div>
          ) : (
            <p className="mt-2 text-sm text-muted-foreground">{t('settings.compaction.openSessionHint')}</p>
          )}
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <Button type="button" disabled={locked || saving} onClick={() => void save()}>{saving ? t('settings.compaction.saving') : t('settings.compaction.save')}</Button>
          <Button type="button" variant="outline" disabled={!activeSessionId || compacting} onClick={() => void compactNow()}>
            {compacting ? t('settings.compaction.compacting') : t('settings.compaction.run')}
          </Button>
          <Button type="button" variant="ghost" disabled={!activeSessionId} onClick={() => void loadSessionContext()}>{t('settings.compaction.refresh')}</Button>
        </div>
        {feedback ? <p className="text-xs text-muted-foreground" aria-live="polite">{feedback}</p> : null}
      </CardContent>
    </Card>
  );
}