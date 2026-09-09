import { CheckCircle2, Languages } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useTranslation } from '@/i18n';
import { useVivyStore } from '@/lib/store';

/**
 * 设置页的真实语言选择卡：通过后端保存全局设置，只应用后端确认的语言。
 */
export function LanguagePicker() {
  const { t, locales } = useTranslation();
  const settings = useVivyStore((state) => state.settings);
  const phase = useVivyStore((state) => state.settingsPhase);
  const error = useVivyStore((state) => state.settingsError);
  const saveLocale = useVivyStore((state) => state.saveLocale);
  const selectedLocale = settings?.locale;
  const disabled = !settings || phase === 'processing' || settings.locale_read_only;

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
              aria-pressed={selectedLocale === option.id}
              disabled={disabled}
              className={`cursor-pointer rounded-xl border p-4 text-left transition-colors hover:border-primary disabled:cursor-not-allowed disabled:opacity-60 ${selectedLocale === option.id ? 'border-primary bg-primary/5 ring-2 ring-primary/20' : ''}`}
              onClick={() => { void saveLocale(option.id).catch(() => undefined); }}
            >
              <div className="flex items-center justify-between gap-2">
                <span className="font-medium">{option.nativeLabel}</span>
                <span className="flex items-center gap-2">
                  <span className="rounded bg-muted px-2 py-1 text-xs font-semibold">{option.code}</span>
                  {selectedLocale === option.id ? <CheckCircle2 className="h-4 w-4 text-primary" aria-label={t('settings.themeSelected')} /> : null}
                </span>
              </div>
              <p className="mt-1 text-sm text-muted-foreground">
                {selectedLocale === option.id ? t('language.currentLanguage') : option.label}
              </p>
            </button>
          ))}
        </div>
        <p className="mt-4 text-xs text-muted-foreground">{t('language.switchHint')}</p>
        {error ? <p className="mt-4 rounded bg-destructive/10 p-3 text-sm text-destructive" role="alert">{t('settingsModel.errors.saveFailed')}: {error}</p> : null}
      </CardContent>
    </Card>
  );
}
