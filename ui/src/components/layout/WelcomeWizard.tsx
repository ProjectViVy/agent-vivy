import { useEffect, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import {
  ArrowLeft,
  ArrowRight,
  Bot,
  ExternalLink,
  GitBranch,
  MessageSquare,
  Settings,
  SkipForward,
  Sparkles,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useVivyStore } from '@/lib/store';
import { useTranslation } from '@/i18n';
import { completeWelcome, useWelcomeOpen } from '@/hooks/use-welcome';
import { cn } from '@/lib/utils';

// 首次使用引导向导（移植自 Agent-Diva 的 WelcomeWizard）：
// 介绍 → 模型配置 → 完成导航。模型配置走真实 settings/update；
// 密钥按 D-010 只由运行环境注入，向导不收集任何 secret。
// provider 是运行时的模型束名（openai/anthropic/mock），DeepSeek 等
// OpenAI 兼容服务通过 base_url 网关接入，而不是自造 provider 名。
const DEEPSEEK_PLATFORM_URL = 'https://platform.deepseek.com/';
const SUGGESTED_DEFAULTS = { provider: 'openai', model: '', baseUrl: '' };

type WelcomeNavigateTarget = 'chat' | 'settings' | 'skills';

const FLOATING_ORBS = [
  { left: '6%', top: '12%', size: 18, delay: 0 },
  { left: '14%', top: '74%', size: 12, delay: 0.6 },
  { left: '84%', top: '16%', size: 16, delay: 1.1 },
  { left: '74%', top: '80%', size: 14, delay: 1.6 },
  { left: '90%', top: '48%', size: 10, delay: 0.8 },
  { left: '4%', top: '58%', size: 20, delay: 1.3 },
];

export function WelcomeWizard() {
  const open = useWelcomeOpen();
  const { t } = useTranslation();
  const navigate = useNavigate();
  const settings = useVivyStore((state) => state.settings);
  const saveSettings = useVivyStore((state) => state.saveSettings);
  const [step, setStep] = useState(0);
  const [form, setForm] = useState({ provider: '', default_model: '', base_url: '' });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const steps = [
    { id: 'intro', icon: Sparkles, label: t('welcome.stepIntro') },
    { id: 'model', icon: Bot, label: t('welcome.stepModel') },
    { id: 'done', icon: MessageSquare, label: t('welcome.stepDone') },
  ];
  const readOnly = settings?.read_only ?? false;

  // 每次打开都从第一步重来，并按当前真实设置预填；未配置时给 DeepSeek 快速开始建议。
  useEffect(() => {
    if (!open) return;
    setStep(0);
    setSaving(false);
    setError(null);
    setForm({
      provider: settings?.provider || settings?.config_provider || SUGGESTED_DEFAULTS.provider,
      default_model: settings?.default_model || settings?.config_model || SUGGESTED_DEFAULTS.model,
      base_url: settings?.base_url || SUGGESTED_DEFAULTS.baseUrl,
    });
    // settings 在打开瞬间取快照即可，向导内不再跟随外部变化
  }, [open]);

  const goBack = () => { setError(null); setStep((current) => Math.max(current - 1, 0)); };

  const goNext = async () => {
    if (step === 1 && !readOnly) {
      const changed = !settings
        || settings.provider !== form.provider
        || settings.default_model !== form.default_model
        || settings.base_url !== form.base_url;
      if (changed) {
        setSaving(true);
        setError(null);
        try {
          await saveSettings({ provider: form.provider, default_model: form.default_model, base_url: form.base_url });
        } catch (cause) {
          setSaving(false);
          setError(t('welcome.saveFailed', { error: cause instanceof Error ? cause.message : String(cause) }));
          return;
        }
        setSaving(false);
      }
    }
    setError(null);
    setStep((current) => Math.min(current + 1, steps.length - 1));
  };

  const finish = (target: WelcomeNavigateTarget) => {
    completeWelcome();
    if (target === 'chat') void navigate({ to: '/' });
    else if (target === 'settings') void navigate({ to: '/settings', search: { tab: 'model' } });
    else if (target === 'skills') void navigate({ to: '/skills' });
  };

  const skip = () => completeWelcome();

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-[200] flex items-center justify-center bg-background/80 p-4 backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
      aria-label={t('welcome.title')}
    >
      {/* 漂浮装饰光点：Agent-Diva 的浮动爱心改为 Vivy 的中性光斑 */}
      <div className="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden="true">
        {FLOATING_ORBS.map((orb, index) => (
          <span
            key={index}
            className="welcome-float absolute rounded-full bg-primary/10"
            style={{ left: orb.left, top: orb.top, width: orb.size, height: orb.size, animationDelay: `${orb.delay}s` }}
          />
        ))}
      </div>

      <div className="relative flex max-h-[90dvh] w-full max-w-lg flex-col overflow-hidden rounded-3xl border bg-card text-card-foreground shadow-xl">
        {/* 品牌头 */}
        <div className="border-b px-7 pb-5 pt-6 text-center">
          <div className="mb-3 flex items-center justify-center gap-3">
            <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-primary/10 text-primary shadow-sm">
              <Bot className="h-6 w-6" aria-hidden="true" />
            </div>
            <div className="text-left">
              <h1 className="text-[22px] font-bold leading-tight tracking-wide">Vivy</h1>
              <p className="text-[11px] uppercase tracking-[0.18em] text-muted-foreground">Project ViVY</p>
            </div>
          </div>
          <p className="text-sm text-muted-foreground">{t('welcome.subtitle')}</p>
        </div>

        {/* 步骤进度 */}
        <div className="border-b bg-muted/40 px-7 py-4">
          <div className="mb-4 h-1 overflow-hidden rounded-full bg-border">
            <div
              className="h-full rounded-full bg-primary transition-all duration-300"
              style={{ width: `${(step / (steps.length - 1)) * 100}%` }}
            />
          </div>
          <div className="flex justify-between">
            {steps.map((entry, index) => {
              const Icon = entry.icon;
              return (
                <button
                  key={entry.id}
                  type="button"
                  className={cn(
                    'flex flex-col items-center gap-1.5 transition-opacity',
                    index === step ? 'opacity-100' : index < step ? 'opacity-80' : 'opacity-50',
                  )}
                  disabled={index > step}
                  onClick={() => index < step && setStep(index)}
                >
                  <span
                    className={cn(
                      'flex h-8 w-8 items-center justify-center rounded-full border-2 transition-colors',
                      index === step
                        ? 'border-primary bg-primary text-primary-foreground shadow-sm'
                        : index < step
                          ? 'border-primary/30 bg-primary/10 text-primary'
                          : 'border-border bg-background text-muted-foreground',
                    )}
                  >
                    <Icon className="h-3.5 w-3.5" aria-hidden="true" />
                  </span>
                  <span className="whitespace-nowrap text-[10px] font-medium text-muted-foreground">{entry.label}</span>
                </button>
              );
            })}
          </div>
        </div>

        {/* 内容区 */}
        <div className="min-h-0 flex-1 overflow-y-auto px-7 py-6">
          {step === 0 ? (
            <div className="text-center">
              <div className="mb-4 flex justify-center">
                <Sparkles className="h-12 w-12 text-primary" aria-hidden="true" />
              </div>
              <h2 className="mb-3 text-lg font-semibold">{t('welcome.introTitle')}</h2>
              <p className="mb-6 text-sm leading-relaxed text-muted-foreground">{t('welcome.introBody')}</p>
              <div className="space-y-3 text-left">
                {[
                  { icon: MessageSquare, label: t('welcome.featureChat') },
                  { icon: Sparkles, label: t('welcome.featureSkills') },
                  { icon: GitBranch, label: t('welcome.featureLifecycle') },
                ].map((feature) => {
                  const Icon = feature.icon;
                  return (
                    <div key={feature.label} className="flex items-center gap-3 rounded-xl bg-muted/50 px-4 py-3 text-sm">
                      <Icon className="h-5 w-5 shrink-0 text-primary" aria-hidden="true" />
                      <span>{feature.label}</span>
                    </div>
                  );
                })}
              </div>
            </div>
          ) : step === 1 ? (
            <form
              className="space-y-5"
              onSubmit={(event) => { event.preventDefault(); if (!saving) void goNext(); }}
            >
              <div className="flex gap-4">
                <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                  <Bot className="h-6 w-6" aria-hidden="true" />
                </div>
                <div className="min-w-0">
                  <h3 className="mb-1 text-base font-semibold">{t('welcome.modelTitle')}</h3>
                  <p className="text-xs leading-relaxed text-muted-foreground">{t('welcome.modelBody')}</p>
                </div>
              </div>
              <Button type="button" variant="outline" size="sm" onClick={() => window.open(DEEPSEEK_PLATFORM_URL, '_blank', 'noopener,noreferrer')}>
                <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
                {t('welcome.openInBrowser')}
              </Button>
              <div className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="welcome-provider">{t('welcome.provider')}</Label>
                  <Input
                    id="welcome-provider"
                    value={form.provider}
                    placeholder={t('welcome.providerPlaceholder')}
                    disabled={saving || readOnly}
                    onChange={(event) => setForm({ ...form, provider: event.target.value })}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="welcome-model">{t('welcome.model')}</Label>
                  <Input
                    id="welcome-model"
                    value={form.default_model}
                    placeholder={t('welcome.modelPlaceholder')}
                    disabled={saving || readOnly}
                    onChange={(event) => setForm({ ...form, default_model: event.target.value })}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="welcome-base-url">{t('welcome.baseUrl')}</Label>
                  <Input
                    id="welcome-base-url"
                    type="url"
                    value={form.base_url}
                    placeholder={t('welcome.baseUrlPlaceholder')}
                    disabled={saving || readOnly}
                    onChange={(event) => setForm({ ...form, base_url: event.target.value })}
                  />
                </div>
              </div>
              <p className="rounded-lg bg-muted/50 p-3 text-xs leading-relaxed text-muted-foreground">
                {readOnly ? t('welcome.readOnlyNotice') : t('welcome.secretNote')}
              </p>
              {error ? <p className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive" role="alert">{error}</p> : null}
            </form>
          ) : (
            <div className="text-center">
              <div className="mb-4 flex justify-center">
                <Bot className="h-12 w-12 text-primary" aria-hidden="true" />
              </div>
              <h2 className="mb-3 text-lg font-semibold">{t('welcome.doneTitle')}</h2>
              <p className="mb-6 text-sm leading-relaxed text-muted-foreground">{t('welcome.doneBody')}</p>
              <div className="space-y-2.5 text-left">
                <button
                  type="button"
                  className="flex w-full items-center gap-3.5 rounded-2xl border border-primary/30 bg-primary/5 p-4 transition-colors hover:border-primary/60 hover:bg-primary/10"
                  onClick={() => finish('chat')}
                >
                  <MessageSquare className="h-6 w-6 shrink-0 text-primary" aria-hidden="true" />
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="text-sm font-semibold">{t('welcome.startChat')}</span>
                    <span className="text-[11px] text-muted-foreground">{t('welcome.startChatDesc')}</span>
                  </span>
                  <ArrowRight className="h-4 w-4 shrink-0 text-primary" aria-hidden="true" />
                </button>
                <button
                  type="button"
                  className="flex w-full items-center gap-3.5 rounded-2xl border bg-background p-4 transition-colors hover:border-primary/40 hover:bg-muted/50"
                  onClick={() => finish('settings')}
                >
                  <Settings className="h-5 w-5 shrink-0 text-muted-foreground" aria-hidden="true" />
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="text-sm font-semibold">{t('welcome.goSettings')}</span>
                    <span className="text-[11px] text-muted-foreground">{t('welcome.goSettingsDesc')}</span>
                  </span>
                </button>
                <button
                  type="button"
                  className="flex w-full items-center gap-3.5 rounded-2xl border bg-background p-4 transition-colors hover:border-primary/40 hover:bg-muted/50"
                  onClick={() => finish('skills')}
                >
                  <Sparkles className="h-5 w-5 shrink-0 text-muted-foreground" aria-hidden="true" />
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="text-sm font-semibold">{t('welcome.goSkills')}</span>
                    <span className="text-[11px] text-muted-foreground">{t('welcome.goSkillsDesc')}</span>
                  </span>
                </button>
              </div>
            </div>
          )}
        </div>

        {/* 底部操作 */}
        <div className="flex items-center justify-between gap-3 border-t bg-muted/30 px-7 py-4">
          <div>
            {step === 0 ? (
              <Button type="button" variant="ghost" size="sm" className="text-xs text-muted-foreground" onClick={skip}>
                <SkipForward className="h-3.5 w-3.5" aria-hidden="true" />
                {t('welcome.skip')}
              </Button>
            ) : (
              <Button type="button" variant="secondary" size="sm" onClick={goBack} disabled={saving}>
                <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
                {t('welcome.back')}
              </Button>
            )}
          </div>
          {step < steps.length - 1 ? (
            <Button type="button" size="sm" onClick={() => void goNext()} disabled={saving}>
              {saving ? t('welcome.saving') : t('welcome.next')}
              {!saving ? <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" /> : null}
            </Button>
          ) : null}
        </div>
      </div>
    </div>
  );
}
