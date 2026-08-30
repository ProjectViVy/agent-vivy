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
import { CHANNEL_PLATFORMS } from './channel-platforms';
import { PLATFORM_DESCRIPTIONS, PLATFORM_ICONS } from './channel-icons';
import { ChannelEditorForm } from './ChannelEditorForm';
import ChannelTutorialModal from './ChannelTutorialModal';

/**
 * 通道接入向导：只接入"编译进当前代、尚未配置"的通道（addablePlatforms
 * 由 channel/inspect 派生，不再是一张固定平台表）。步骤 = 选择平台 →
 * 允许的发送者 → 完成；写入走 channel/update（settings overlay），重启
 * 进程后开始监听。平台令牌是环境变量（token_env 只读展示），向导不
 * 收集任何密钥值。
 */

type WizardStep = 'platform' | 'credentials' | 'done';

type ChannelWizardData = {
  platform: string;
  allowFromText: string;
};

function ChannelWizardModal({
  open,
  addablePlatforms,
  onOpenChange,
  onComplete,
}: {
  open: boolean;
  /** 编译内且尚未配置的通道名（channel/inspect 派生）。 */
  addablePlatforms: string[];
  onOpenChange: (open: boolean) => void;
  onComplete: (data: ChannelWizardData) => void;
}) {
  const { t } = useTranslation();

  const [step, setStep] = useState<WizardStep>('platform');
  const [platform, setPlatform] = useState<string>('');
  const [allowFromText, setAllowFromText] = useState('');
  const [tutorialOpen, setTutorialOpen] = useState(false);

  // open 从 false→true 时重置到平台步。
  useEffect(() => {
    if (open) {
      setPlatform('');
      setAllowFromText('');
      setStep('platform');
    } else {
      setTutorialOpen(false);
    }
  }, [open]);

  const platformInfo = platform !== '' ? CHANNEL_PLATFORMS[platform] ?? null : null;

  const canNext = step === 'platform' ? Boolean(platform) : step === 'credentials';

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

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="max-h-[90dvh] max-w-2xl overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{t('channels.wizardTitle')}</DialogTitle>
            <DialogDescription>{t('channels.wizardSubtitle')}</DialogDescription>
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
            {step === 'platform' ? (
              <>
                <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  {t('channels.choosePlatform')}
                </p>
                {addablePlatforms.length > 0 ? (
                  <div className="grid grid-cols-[repeat(auto-fill,minmax(9rem,1fr))] gap-3">
                    {addablePlatforms.map((name) => {
                      const PlatformIcon = PLATFORM_ICONS[name];
                      const displayName = CHANNEL_PLATFORMS[name]?.displayName ?? name;
                      return (
                        <button
                          key={name}
                          type="button"
                          className={`flex cursor-pointer flex-col items-center gap-2 rounded-lg border p-4 transition-all hover:-translate-y-0.5 ${
                            platform === name ? 'border-primary bg-primary text-primary-foreground' : 'bg-card hover:border-primary'
                          }`}
                          onClick={() => setPlatform(name)}
                        >
                          {PlatformIcon ? (
                            <PlatformIcon className="h-6 w-6" />
                          ) : (
                            <span className="flex h-6 w-6 items-center justify-center text-sm font-semibold">
                              {name.slice(0, 1).toUpperCase()}
                            </span>
                          )}
                          <span className="text-sm font-semibold">{displayName}</span>
                          <span className={`text-center text-xs ${platform === name ? 'text-primary-foreground/90' : 'text-muted-foreground'}`}>
                            {PLATFORM_DESCRIPTIONS[name] ?? name}
                          </span>
                        </button>
                      );
                    })}
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground">{t('channels.wizardNoAddable')}</p>
                )}
              </>
            ) : null}

            {step === 'credentials' ? (
              <>
                <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  {t('channels.enterCredentials')}
                </p>
                {platformInfo ? (
                  <div className="rounded-lg border border-blue-500/20 bg-blue-500/5 p-4">
                    <div className="mb-2 flex items-center gap-2">
                      <Lightbulb className="h-4 w-4 text-primary" />
                      <h4 className="text-sm font-semibold">
                        {t('channels.quickGuideTitle', { platform: platformInfo.displayName })}
                      </h4>
                    </div>
                    <ol className="list-decimal space-y-1 pl-5 text-sm">
                      {platformInfo.quickGuideSteps.map((item, index) => (
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
                  enabled
                  allowFromText={allowFromText}
                  onAllowFromTextChange={setAllowFromText}
                />
                <p className="text-xs text-muted-foreground">{t('channels.wizardRestartNote')}</p>
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
            {step === 'credentials' ? (
              <Button type="button" variant="outline" onClick={() => setStep('platform')}>
                {t('channels.wizardBack')}
              </Button>
            ) : null}
            {step !== 'done' ? (
              <Button type="button" disabled={!canNext || addablePlatforms.length === 0} onClick={nextStep}>
                {t('channels.wizardNext')}
                <ChevronRight className="ml-1.5 h-4 w-4" />
              </Button>
            ) : (
              <Button
                type="button"
                onClick={() => {
                  onComplete({ platform, allowFromText });
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
        platformName={platform || null}
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
