import { CheckCircle2, Palette } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useTheme } from '@/hooks/use-theme';
import { useTranslation } from '@/i18n';

/**
 * 设置页的真实主题选择卡。预览块用 data-theme-preview + data-appearance
 * 让每个家族在局部重新声明令牌，因此预览就是应用后的真实颜色，
 * 不再维护第二套渐变色字面量。
 */
export function ThemePicker() {
  const { theme, themes, setTheme } = useTheme();
  const { t } = useTranslation();

  return (
    <Card data-testid="theme-picker">
      <CardHeader>
        <div className="mb-2 flex h-9 w-9 items-center justify-center rounded-lg bg-brand-soft text-brand-ink">
          <Palette className="h-5 w-5" aria-hidden="true" />
        </div>
        <CardTitle>{t('settings.themesTitle')}</CardTitle>
        <CardDescription>{t('settings.themesDescription')}</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3" role="group" aria-label={t('settings.themesGroupAria')}>
          {themes.map((item) => (
            <button
              key={item.id}
              type="button"
              aria-pressed={theme === item.id}
              className={`group cursor-pointer overflow-hidden rounded-xl border bg-raised text-left transition-colors hover:border-brand ${
                theme === item.id ? 'border-brand ring-2 ring-brand/20' : 'border-line'
              }`}
              onClick={() => setTheme(item.id)}
            >
              <div
                data-theme-preview={item.id}
                data-appearance={item.appearance}
                className="border-b border-line bg-canvas p-4"
              >
                <div className="flex items-center justify-between">
                  <span className="h-2.5 w-2.5 rounded-full bg-brand" aria-hidden="true" />
                  <span className="rounded-full bg-brand-fill px-2 py-0.5 text-2xs font-semibold tracking-wide text-brand-fg">
                    {t('settings.themeSwatchSample')}
                  </span>
                </div>
                <div className="mt-3 rounded-lg border border-line bg-raised p-2.5">
                  <div className="h-1.5 w-20 rounded-full bg-fg" />
                  <div className="mt-1.5 h-1.5 w-full rounded-full bg-fg-muted" />
                  <div className="mt-1.5 h-1.5 w-2/3 rounded-full bg-fg-subtle" />
                </div>
                <div className="mt-3 flex gap-1.5" aria-hidden="true">
                  <span className="h-1.5 w-6 rounded-full bg-ok" />
                  <span className="h-1.5 w-6 rounded-full bg-warn" />
                  <span className="h-1.5 w-6 rounded-full bg-danger" />
                  <span className="h-1.5 w-6 rounded-full bg-info" />
                </div>
              </div>
              <div className="flex items-start justify-between gap-2 p-4">
                <div className="min-w-0">
                  <div className="text-ui font-medium text-fg">{t(`themes.${item.id}.label`)}</div>
                  <p className="mt-1 text-meta text-fg-muted">{t(`themes.${item.id}.description`)}</p>
                </div>
                {theme === item.id ? (
                  <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-brand-ink" aria-label={t('settings.themeSelected')} />
                ) : null}
              </div>
            </button>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
