import { CheckCircle2, Languages } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useTranslation } from '@/i18n';

/**
 * 设置页的真实语言选择卡：点击立即切换全局界面语言并持久化到当前浏览器。
 * 与 ThemePicker 同构；语言切换由 src/i18n 的模块级 store 广播给所有订阅组件。
 */
export function LanguagePicker() {
  const { t, locale, setLocale, locales } = useTranslation();

  return (
    <Card>
      <CardHeader>
        <div className="mb-2 flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <Languages className="h-5 w-5" aria-hidden="true" />
        </div>
        <CardTitle>{t('language.title')}</CardTitle>
        <CardDescription>{t('language.description')}</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="grid gap-4 sm:grid-cols-2" role="group" aria-label={t('language.title')}>
          {locales().map((option) => (
            <button
              key={option.id}
              type="button"
              aria-pressed={locale === option.id}
              className={`cursor-pointer rounded-xl border p-4 text-left transition-colors hover:border-primary ${locale === option.id ? 'border-primary bg-primary/5 ring-2 ring-primary/20' : ''}`}
              onClick={() => setLocale(option.id)}
            >
              <div className="flex items-center justify-between gap-2">
                <span className="font-medium">{option.nativeLabel}</span>
                <span className="flex items-center gap-2">
                  <span className="rounded bg-muted px-2 py-1 text-xs font-semibold">{option.code}</span>
                  {locale === option.id ? <CheckCircle2 className="h-4 w-4 text-primary" aria-label={t('settings.themeSelected')} /> : null}
                </span>
              </div>
              <p className="mt-1 text-sm text-muted-foreground">
                {locale === option.id ? t('language.currentLanguage') : option.label}
              </p>
            </button>
          ))}
        </div>
        <p className="mt-4 text-xs text-muted-foreground">{t('language.switchHint')}</p>
      </CardContent>
    </Card>
  );
}
