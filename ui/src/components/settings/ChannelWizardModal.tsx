import { useEffect, useState } from 'react';
import { Check, ChevronRight, Lightbulb } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { useTranslation } from '@/i18n';
import { fieldDefaults, normalizeChannelConfig, validateConfig } from './channel-schema';
import { CHANNEL_PLATFORMS, isRetiredChannel } from './channel-platforms';
import { PLATFORM_DESCRIPTIONS, PLATFORM_ICONS } from './channel-icons';
import { ChannelEditorForm } from './ChannelEditorForm';
import ChannelTutorialModal from './ChannelTutorialModal';

/**
 * 通道配置向导（移植自 Agent-Diva ChannelWizardModal.vue）。
 * 步骤 = 选择平台 → 凭据配置 → 完成（Diva 源码 steps 数组即此三步；
 * 「测试连接」步在当前来源为不可达死代码，后端接入连接测试能力后再补）。
 */

type WizardStep = 'platform' | 'credentials' | 'done';

type ChannelWizardData = {
  platform: string;
  credentials: Record<string, unknown>;
};

function ChannelWizardModal({
  open,
  initialPlatform,
  initialCredentials,
  onOpenChange,
  onComplete,
}: {
  open: boolean;
  /** 编辑模式：预选平台并直达凭据步 */
  initialPlatform?: string;
  initialCredentials?: Record<string, unknown>;
  onOpenChange: (open: boolean) => void;
  onComplete: (data: ChannelWizardData) => void;
}) {
  const { t } = useTranslation();
  const isEditMode = Boolean(initialPlatform && !isRetiredChannel(initialPlatform));

  const [step, setStep] = useState<WizardStep>('platform');
  const [platform, setPlatform] = useState<string>(initialPlatform ?? '');
  const [credentials, setCredentials] = useState<Record<string, unknown>>(
    () => normalizeChannelConfig(initialPlatform ?? '', {
      ...fieldDefaults(initialPlatform ?? ''),
      ...(initialCredentials ?? {}),
    }),
  );
  const [tutorialOpen, setTutorialOpen] = useState(false);

  // open 从 false→true 时重置：新建回到平台步；编辑直达凭据步。
  useEffect(() => {
    if (!open) {
      setTutorialOpen(false);
      return;
    }
    if (initialPlatform && !isRetiredChannel(initialPlatform)) {
      setPlatform(initialPlatform);
      setCredentials(
        normalizeChannelConfig(initialPlatform, {
          ...fieldDefaults(initialPlatform),
          ...(initialCredentials ?? {}),
        }),
      );
      setStep('credentials');
    } else {
      setPlatform('');
      setCredentials({});
      setStep('platform');
    }
  }, [open]); // eslint-disable-line react-hooks/exhaustive-deps

  const handleOpenChange = (next: boolean) => {
    onOpenChange(next);
  };

  const selectPlatform = (next: string) => {
    setPlatform(next);
    setCredentials({
      ...fieldDefaults(next),
      ...credentials,
    });
  };

  const canNext =
    step === 'platform'
      ? Boolean(platform)
      : step === 'credentials'
        ? validateConfig(platform, credentials).valid
        : false;

  const nextStep = () => {
    if (step === 'platform') setStep('credentials');
    else if (step === 'credentials') setStep('done');
  };

  const steps: Array<{ key: WizardStep; title: string }> = [
    { key: 'platform', title: t('channels.wizardStepPlatform') },
    { key: 'credentials', title: t('channels.wizardStepCredentials') },
    { key: 'done', title: t('channels.wizardStepDone') },
  ];
  const stepIndex = steps.findIndex((s) => s.key === step);
  const currentPlatform = platform ? CHANNEL_PLATFORMS[platform] ?? null : null;

  return (
    <>
      <Dialog open={open} onOpenChange={handleOpenChange}>
        <DialogContent className="max-h-[90dvh] max-w-2xl overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {isEditMode ? t('channels.wizardEditTitle') : t('channels.wizardTitle')}
            </DialogTitle>
            <DialogDescription>
              {isEditMode ? t('channels.wizardEditSubtitle') : t('channels.wizardSubtitle')}
            </DialogDescription>
          </DialogHeader>

          {/* 步骤指示器 */}
          <div className="flex gap-4 border-b pb-4">
            {steps.map((s, index) => (
              <div key={s.key} className="flex flex-1 flex-col items-center gap-1.5">
                <div
                  className={`flex h-8 w-8 items-center justify-center rounded-full border-2 text-xs font-semibold transition-colors ${
                    index < stepIndex
                      ? 'border-primary bg-primary text-primary-foreground'
                      : s.key === step
                        ? 'border-primary text-primary'
                        : 'border-border text-muted-foreground'
                  }`}
                >
                  {index < stepIndex ? <Check className="h-3.5 w-3.5" /> : index + 1}
                </div>
                <span className="text-[10px] text-muted-foreground">{s.title}</span>
              </div>
            ))}
          </div>

          <div className="space-y-4">
            {step === 'platform' && !isEditMode ? (
              <>
                <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  {t('channels.choosePlatform')}
                </p>
                <div className="grid grid-cols-[repeat(auto-fill,minmax(9rem,1fr))] gap-3">
                  {Object.entries(CHANNEL_PLATFORMS).map(([name, info]) => {
                    const Icon = PLATFORM_ICONS[name];
                    return (
                      <button
                        key={name}
                        type="button"
                        className={`flex cursor-pointer flex-col items-center gap-2 rounded-lg border p-4 transition-all hover:-translate-y-0.5 ${
                          platform === name ? 'border-primary bg-primary text-primary-foreground' : 'bg-card hover:border-primary'
                        }`}
                        onClick={() => selectPlatform(name)}
                      >
                        <Icon className="h-6 w-6" />
                        <span className="text-sm font-semibold">{info.displayName}</span>
                        <span className={`text-center text-xs ${platform === name ? 'text-primary-foreground/90' : 'text-muted-foreground'}`}>
                          {PLATFORM_DESCRIPTIONS[name]}
                        </span>
                      </button>
                    );
                  })}
                </div>
              </>
            ) : null}

            {step === 'credentials' ? (
              <>
                <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  {t('channels.enterCredentials')}
                </p>
                {currentPlatform ? (
                  <div className="rounded-lg border border-blue-500/20 bg-blue-500/5 p-4">
                    <div className="mb-2 flex items-center gap-2">
                      <Lightbulb className="h-4 w-4 text-primary" />
                      <h4 className="text-sm font-semibold">
                        {t('channels.quickGuideTitle', { platform: currentPlatform.displayName })}
                      </h4>
                    </div>
                    <ol className="list-decimal space-y-1 pl-5 text-sm">
                      {currentPlatform.quickGuideSteps.map((item, index) => (
                        <li key={index}>{item}</li>
                      ))}
                    </ol>
                    <Button type="button" variant="outline" size="sm" className="mt-3" onClick={() => setTutorialOpen(true)}>
                      {t('channels.tutorialOpen')}
                    </Button>
                  </div>
                ) : null}
                <ChannelEditorForm
                  platform={platform}
                  config={credentials}
                  onFieldChange={(field, value) => {
                    setCredentials((current) => ({ ...current, [field.key]: value }));
                  }}
                />
              </>
            ) : null}

            {step === 'done' ? (
              <div className="flex flex-col items-center py-8 text-center">
                <div className="mb-5 flex h-20 w-20 items-center justify-center rounded-full bg-emerald-500 text-white">
                  <Check className="h-12 w-12" />
                </div>
                <h4 className="mb-1 text-xl font-semibold">{t('channels.wizardDone')}</h4>
                <p className="text-sm text-muted-foreground">{t('channels.wizardDoneHint')}</p>
              </div>
            ) : null}
          </div>

          <DialogFooter className="border-t pt-4">
            {step === 'credentials' && !isEditMode ? (
              <Button type="button" variant="outline" onClick={() => setStep('platform')}>
                {t('channels.wizardBack')}
              </Button>
            ) : null}
            {step !== 'done' ? (
              <Button type="button" disabled={!canNext} onClick={nextStep}>
                {t('channels.wizardNext')}
                <ChevronRight className="ml-1.5 h-4 w-4" />
              </Button>
            ) : (
              <Button
                type="button"
                onClick={() => {
                  onComplete({
                    platform,
                    credentials: normalizeChannelConfig(platform, credentials),
                  });
                  onOpenChange(false);
                }}
              >
                {t('channels.wizardFinish')}
              </Button>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ChannelTutorialModal
        open={tutorialOpen}
        platformName={platform}
        onOpenChange={setTutorialOpen}
        onStartConfig={() => {
          // 教程「开始配置」回到向导：凭据步已在当前，无需跳转。
        }}
      />
    </>
  );
}

export default ChannelWizardModal;

export type { ChannelWizardData };