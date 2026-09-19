import { useEffect, useState } from 'react';
import { ShieldCheck } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { settingsUpdateFrom } from '@/lib/api';
import { useVivyStore } from '@/lib/store';
import { useTranslation } from '@/i18n';

/** settings/update 的硬约束：等待时间不会超过审批有效期，也不超过一天。 */
function approvalWindowCap(configExpiration: number | undefined): number {
  if (typeof configExpiration === 'number' && configExpiration > 0) return Math.min(86400, configExpiration);
  return 86400;
}

/**
 * 审批超时（真实）：写入 settings.yaml 的 sandbox.approval_timeout_seconds，
 * 后端立即生效（runtime.sandbox.approval.timeout_seconds）。智能模式下审批
 * 到点仍无人处理时由系统代替用户同意，对话继续；关闭开关写入显式 0，表示
 * 不允许超时自动同意，审批只按有效期失效。
 */
export function ApprovalTimeoutCard() {
  const settings = useVivyStore((state) => state.settings);
  const saveSettings = useVivyStore((state) => state.saveSettings);
  const { t } = useTranslation();

  const base = settings?.sandbox;
  const effective = typeof base?.approval_timeout_seconds === 'number' ? base.approval_timeout_seconds : 0;
  const cap = approvalWindowCap(base?.approval_expiration_seconds);
  const fallback = base?.config_approval_timeout_seconds && base.config_approval_timeout_seconds > 0
    ? base.config_approval_timeout_seconds
    : cap;

  const [enabled, setEnabled] = useState(effective > 0);
  const [seconds, setSeconds] = useState(effective > 0 ? effective : fallback);
  const [saving, setSaving] = useState(false);
  const [feedback, setFeedback] = useState<{ kind: 'saved' } | { kind: 'error'; message: string } | null>(null);
  const locked = settings?.read_only || Boolean(settings?.frozen);

  useEffect(() => {
    setEnabled(effective > 0);
    setSeconds(effective > 0 ? effective : fallback);
  }, [effective, fallback]);

  const clamped = Math.min(cap, Math.max(1, Number(seconds) || 0));
  const invalid = enabled && (Number(seconds) < 1 || Number(seconds) > cap);

  const save = async () => {
    if (invalid) return;
    setSaving(true);
    setFeedback(null);
    try {
      const preset = base?.default_preset && base.default_preset !== 'custom'
        ? base.default_preset
        : (base?.config_default_preset && base.config_default_preset !== 'custom' ? base.config_default_preset : 'smart');
      await saveSettings(settingsUpdateFrom(settings, {
        provider: settings?.provider,
        sandbox: {
          default_preset: preset,
          deny_private_ips: base?.deny_private_ips ?? false,
          allowed_domains: base?.allowed_domains ?? [],
          approval_timeout_seconds: enabled ? clamped : 0,
        },
      }));
      setFeedback({ kind: 'saved' });
    } catch (error) {
      setFeedback({ kind: 'error', message: error instanceof Error ? error.message : String(error) });
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <div className="mb-2 flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary"><ShieldCheck className="h-5 w-5" aria-hidden="true" /></div>
        <CardTitle>{t('settings.approvalTimeout.title')}</CardTitle>
        <CardDescription>{t('settings.approvalTimeout.description')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center justify-between rounded-lg border p-3">
          <div className="min-w-0">
            <p className="font-medium">{t('settings.approvalTimeout.autoLabel')}</p>
            <p className="mt-1 text-sm text-muted-foreground">{t('settings.approvalTimeout.autoHint')}</p>
          </div>
          <Switch
            checked={enabled}
            disabled={locked}
            onCheckedChange={setEnabled}
            aria-label={t('settings.approvalTimeout.autoLabel')}
            data-testid="approval-timeout-switch"
          />
        </div>

        {enabled ? (
          <div className="space-y-2">
            <Label htmlFor="approval-timeout-seconds">{t('settings.approvalTimeout.secondsLabel')}</Label>
            <Input
              id="approval-timeout-seconds"
              type="number"
              min={1}
              max={cap}
              value={seconds}
              disabled={locked}
              onChange={(event) => setSeconds(Number(event.target.value) || 0)}
              placeholder={String(fallback)}
            />
            <p className="text-xs text-muted-foreground">{t('settings.approvalTimeout.secondsHint', { cap })}</p>
          </div>
        ) : null}

        <p className="text-xs text-muted-foreground" data-testid="approval-timeout-effective">
          {effective > 0
            ? t('settings.approvalTimeout.effectiveOn', { seconds: effective })
            : t('settings.approvalTimeout.effectiveOff')}
        </p>

        <div className="flex flex-wrap items-center gap-3">
          <Button type="button" disabled={locked || saving || invalid} onClick={() => void save()}>
            {saving ? t('settings.approvalTimeout.saving') : t('settings.approvalTimeout.save')}
          </Button>
          {invalid ? <span className="text-xs text-destructive" role="alert">{t('settings.approvalTimeout.invalid', { cap })}</span> : null}
        </div>
        {feedback ? (
          <p className="text-xs text-muted-foreground" aria-live="polite">
            {feedback.kind === 'error' ? feedback.message : t('settings.approvalTimeout.saved')}
          </p>
        ) : null}
      </CardContent>
    </Card>
  );
}