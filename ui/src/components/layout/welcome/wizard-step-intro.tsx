import { GitBranch, MessageSquare, Sparkles } from 'lucide-react';
import { useTranslation } from '@/i18n';

export function WizardStepIntro() {
  const { t } = useTranslation();
  const features = [
    { icon: MessageSquare, label: t('welcome.featureChat') },
    { icon: Sparkles, label: t('welcome.featureSkills') },
    { icon: GitBranch, label: t('welcome.featureLifecycle') },
  ];
  return (
    <div className="text-center">
      <div className="mb-4 flex justify-center">
        <Sparkles className="h-12 w-12 text-brand" aria-hidden="true" />
      </div>
      <h2 className="mb-3 text-title font-semibold">{t('welcome.introTitle')}</h2>
      <p className="mb-6 text-ui leading-relaxed text-fg-muted">{t('welcome.introBody')}</p>
      <div className="space-y-3 text-left">
        {features.map((feature) => {
          const Icon = feature.icon;
          return (
            <div key={feature.label} className="flex items-center gap-3 rounded-xl bg-sunken px-4 py-3 text-ui">
              <Icon className="h-5 w-5 shrink-0 text-brand" aria-hidden="true" />
              <span>{feature.label}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
