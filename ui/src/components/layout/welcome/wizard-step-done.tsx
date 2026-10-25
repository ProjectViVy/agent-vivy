import { ArrowRight, Bot, MessageSquare, Settings, Sparkles } from 'lucide-react';
import { useTranslation } from '@/i18n';

export type WelcomeNavigateTarget = 'chat' | 'settings' | 'skills';

export function WizardStepDone({ onFinish }: { onFinish: (target: WelcomeNavigateTarget) => void }) {
  const { t } = useTranslation();
  return (
    <div className="text-center">
      <div className="mb-4 flex justify-center">
        <Bot className="h-12 w-12 text-brand" aria-hidden="true" />
      </div>
      <h2 className="mb-3 text-title font-semibold">{t('welcome.doneTitle')}</h2>
      <p className="mb-6 text-ui leading-relaxed text-fg-muted">{t('welcome.doneBody')}</p>
      <div className="space-y-2.5 text-left">
        <button
          type="button"
          className="flex w-full items-center gap-3.5 rounded-2xl border border-brand/30 bg-brand-soft p-4 transition-colors hover:bg-hover"
          onClick={() => onFinish('chat')}
        >
          <MessageSquare className="h-6 w-6 shrink-0 text-brand" aria-hidden="true" />
          <span className="flex min-w-0 flex-1 flex-col">
            <span className="text-ui font-semibold">{t('welcome.startChat')}</span>
            <span className="text-2xs text-fg-muted">{t('welcome.startChatDesc')}</span>
          </span>
          <ArrowRight className="h-4 w-4 shrink-0 text-brand" aria-hidden="true" />
        </button>
        <button
          type="button"
          className="flex w-full items-center gap-3.5 rounded-2xl border border-line bg-surface p-4 transition-colors hover:bg-hover"
          onClick={() => onFinish('settings')}
        >
          <Settings className="h-5 w-5 shrink-0 text-fg-muted" aria-hidden="true" />
          <span className="flex min-w-0 flex-1 flex-col">
            <span className="text-ui font-semibold">{t('welcome.goSettings')}</span>
            <span className="text-2xs text-fg-muted">{t('welcome.goSettingsDesc')}</span>
          </span>
        </button>
        <button
          type="button"
          className="flex w-full items-center gap-3.5 rounded-2xl border border-line bg-surface p-4 transition-colors hover:bg-hover"
          onClick={() => onFinish('skills')}
        >
          <Sparkles className="h-5 w-5 shrink-0 text-fg-muted" aria-hidden="true" />
          <span className="flex min-w-0 flex-1 flex-col">
            <span className="text-ui font-semibold">{t('welcome.goSkills')}</span>
            <span className="text-2xs text-fg-muted">{t('welcome.goSkillsDesc')}</span>
          </span>
        </button>
      </div>
    </div>
  );
}
