import { useEffect, useState } from 'react';
import { CornerDownLeft } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';

import { settingsUpdateFrom, type BusyEnter } from '@/lib/api';
import { useVivyStore } from '@/lib/store';
import { useTranslation } from '@/i18n';

const OPTIONS = [
  { value: 'queue', label: 'settings.busyEnter.queueLabel', hint: 'settings.busyEnter.queueHint' },
  { value: 'steer', label: 'settings.busyEnter.steerLabel', hint: 'settings.busyEnter.steerHint' },
] as const;

/**
 * 设置 → 通用 Tab 的「运行中发送」卡：model/busy_enter 持久化默认投递方式。
 *
 * 运行中按 Enter（或点主发送按钮）按这里的选择走，Cmd/Ctrl+Enter 反向：
 * queue 排到本轮之后，steer 插到本轮下一个步骤。
 */
export function ComposerSettingsCard() {
  const settings = useVivyStore((state) => state.settings);
  const saveSettings = useVivyStore((state) => state.saveSettings);
  const { t } = useTranslation();

  const effective: BusyEnter = settings?.busy_enter === 'steer' ? 'steer' : 'queue';
  const [mode, setMode] = useState<BusyEnter>(effective);
  const [saving, setSaving] = useState(false);
  const [feedback, setFeedback] = useState<{ kind: 'saved' } | { kind: 'error'; message: string } | null>(null);
  const locked = settings?.read_only || Boolean(settings?.frozen);

  useEffect(() => {
    setMode(effective);
  }, [effective]);

  const save = async () => {
    setSaving(true);
    setFeedback(null);
    try {
      await saveSettings(settingsUpdateFrom(settings, { busy_enter: mode }));
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
        <div className="mb-2 flex h-9 w-9 items-center justify-center rounded-lg bg-brand-soft text-brand-ink"><CornerDownLeft className="h-5 w-5" aria-hidden="true" /></div>
        <CardTitle>{t('settings.busyEnter.title')}</CardTitle>
        <CardDescription>{t('settings.busyEnter.description')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div role="radiogroup" aria-label={t('settings.busyEnter.title')} className="space-y-2">
          {OPTIONS.map((option) => (
            <div key={option.value} className="flex items-start gap-3 rounded-lg border border-line p-3">
              <input type="radio" name="busy-enter" value={option.value} checked={mode === option.value} aria-checked={mode === option.value} onChange={() => setMode(option.value)} disabled={locked || saving} id={`busy-enter-${option.value}`} className="mt-0.5 accent-brand-fill" data-testid={`busy-enter-${option.value}`} />
              <div className="min-w-0">
                <Label htmlFor={`busy-enter-${option.value}`} className="font-medium">{t(option.label)}</Label>
                <p className="mt-1 text-ui text-fg-muted">{t(option.hint)}</p>
              </div>
            </div>
          ))}
        </div>
        <p className="text-meta text-fg-muted" data-testid="busy-enter-effective">
          {t('settings.busyEnter.effective', { mode: t(effective === 'steer' ? 'settings.busyEnter.steerLabel' : 'settings.busyEnter.queueLabel') })}
        </p>
        <div className="flex flex-wrap items-center gap-3">
          <Button type="button" disabled={locked || saving || mode === effective} onClick={() => void save()}>
            {saving ? t('settings.busyEnter.saving') : t('settings.busyEnter.save')}
          </Button>
        </div>
        {feedback ? (
          <p className="text-meta text-fg-muted" aria-live="polite">
            {feedback.kind === 'error' ? feedback.message : t('settings.busyEnter.saved')}
          </p>
        ) : null}
      </CardContent>
    </Card>
  );
}
