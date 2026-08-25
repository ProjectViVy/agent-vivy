import { CheckCircle2, Palette } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useTheme } from '@/hooks/use-theme';
import { useTranslation } from '@/i18n';

/** 设置页的真实主题选择卡：点击立即切换全局皮肤并持久化到当前浏览器。 */
export function ThemePicker() {
  const { theme, themes, setTheme } = useTheme();
  const { t } = useTranslation();

  return (
    <Card>
      <CardHeader>
        <div className="mb-2 flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
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
              className={`group cursor-pointer overflow-hidden rounded-xl border text-left transition-colors hover:border-primary ${theme === item.id ? 'border-primary ring-2 ring-primary/20' : ''}`}
              onClick={() => setTheme(item.id)}
            >
              {/* 预览色板用字面量：需同时展示所有皮肤，不能跟随当前主题 token */}
              <div className="h-20 p-4" style={{ background: item.preview }}>
                <div className="space-y-2">
                  <div className="h-2 w-24 rounded-full" style={{ background: item.accent, opacity: 0.55 }} />
                  <div className="h-2 w-full rounded-full" style={{ background: item.accent, opacity: 0.35 }} />
                  <div className="h-2 w-3/4 rounded-full" style={{ background: item.accent, opacity: 0.2 }} />
                </div>
              </div>
              <div className="p-4">
                <div className="flex items-center justify-between gap-2">
                  <span className="font-medium" style={{ color: item.accent }}>{t(`themes.${item.id}.label`)}</span>
                  {theme === item.id ? <CheckCircle2 className="h-4 w-4 text-primary" aria-label={t('settings.themeSelected')} /> : null}
                </div>
                <p className="mt-1 text-sm text-muted-foreground">{t(`themes.${item.id}.description`)}</p>
              </div>
            </button>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
