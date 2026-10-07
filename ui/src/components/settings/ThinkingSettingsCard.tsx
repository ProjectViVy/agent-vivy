import { useEffect, useState } from 'react';
import { Brain } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import * as api from '@/lib/api';
import type { ThinkingMode } from '@/lib/api';
import { useTranslation } from '@/i18n';

const ALIAS_MODES: ThinkingMode[] = ['auto', 'on', 'off'];

/**
 * 设置 → 通用 Tab 的「思考级别」卡：model/thinking 持久化默认偏好，
 * 下拉列出活跃模型声明的 levels + 三别名，副行报告内核解析出的有效级。
 */
export function ThinkingSettingsCard() {
  const { t } = useTranslation();
  const [report, setReport] = useState<api.ThinkingReport | null>(null);
  const [levels, setLevels] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let alive = true;
    Promise.all([api.getThinking(), api.thinkingLevels()])
      .then(([nextReport, nextLevels]) => {
        if (!alive) return;
        setReport(nextReport);
        setLevels(nextLevels.levels);
      })
      .catch((err) => {
        if (alive) setError(String(err));
      });
    return () => {
      alive = false;
    };
  }, []);

  const apply = async (level: ThinkingMode) => {
    setBusy(true);
    try {
      setReport(await api.setThinking(level));
      setError(null);
    } catch (err) {
      setError(String(err));
    } finally {
      setBusy(false);
    }
  };

  if (!report?.supports_thinking && levels.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2"><Brain className="size-4" />{t('thinkingSettings.title')}</CardTitle>
          <CardDescription>{t('thinkingSettings.description')}</CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">{t('thinkingSettings.unsupported')}</p>
        </CardContent>
      </Card>
    );
  }

  const options = [...ALIAS_MODES, ...levels.filter((level) => !ALIAS_MODES.includes(level as ThinkingMode))];

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2"><Brain className="size-4" />{t('thinkingSettings.title')}</CardTitle>
        <CardDescription>{t('thinkingSettings.description')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex items-center gap-3">
          <Label htmlFor="thinking-level" className="w-28 text-sm text-muted-foreground">{t('thinkingSettings.level')}</Label>
          <Select
            value={report?.thinking ?? 'auto'}
            onValueChange={(value) => apply(value as ThinkingMode)}
            disabled={busy || report?.read_only}
          >
            <SelectTrigger id="thinking-level" className="w-48"><SelectValue /></SelectTrigger>
            <SelectContent>
              {options.map((level) => (
                <SelectItem key={level} value={level}>{level}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        {report?.effective ? (
          <p className="text-xs text-muted-foreground">{t('thinkingSettings.effective', { level: report.effective })}</p>
        ) : null}
        {report?.read_only ? (
          <p className="text-xs text-amber-600 dark:text-amber-300">{t('settings.readOnlyNotice')}</p>
        ) : null}
        {error ? <p className="text-xs text-destructive">{error}</p> : null}
      </CardContent>
    </Card>
  );
}
