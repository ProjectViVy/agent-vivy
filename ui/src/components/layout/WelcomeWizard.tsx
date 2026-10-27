import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import {
  ArrowLeft,
  ArrowRight,
  Bot,
  MessageSquare,
  SkipForward,
  Sparkles,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useVivyStore } from '@/lib/store';
import { useTranslation } from '@/i18n';
import { completeWelcome, useWelcomeOpen } from '@/hooks/use-welcome';
import { providerEntryByEndpoint } from '@/components/settings/custom-providers';
import { settingsUpdateFrom } from '@/lib/api';
import {
  EMPTY_WELCOME_MODEL,
  canSaveWelcomeModel,
  prefillWelcomeModel,
  welcomeCatalogRows,
  welcomeKeyStatus,
  welcomeProviderInput,
  welcomeSelection,
  welcomeVendors,
  type WelcomeModelDraft,
} from './welcome-model';
import { WizardStepIntro } from './welcome/wizard-step-intro';
import { WizardStepModel, type WizardStepModelProps } from './welcome/wizard-step-model';
import { WizardStepDone, type WelcomeNavigateTarget } from './welcome/wizard-step-done';
import { cn } from '@/lib/utils';

// 首次使用引导向导（移植自 Agent-Diva 的 WelcomeWizard）：
// 介绍 → 模型配置 → 完成导航。模型配置走真实 settings/update。
//
// PROV-P4：向导不再持有任何 provider 数据。模型步骤对齐设置页的目录口径：
// 厂商与协议端点来自后端 `settings/providers` 目录，模型来自所选端点的
// models 清单，选中三元组写 `{provider: 密封适配器, base_url, default_model}`。
// API Key 由注册表承载（写-only；wire 只回 api_key_set），在向导里填写后
// 通过 `settings/providers/upsert` 落盘，永不回显或写日志。
//
// 本壳只持有状态、effects 与保存流程；三个步骤视图在 ./welcome/ 下，
// 与 composer 拆分同口径（壳拥有数据，步骤视图纯展示）。
const WELCOME_STEPS = [
  { id: 'intro', icon: Sparkles, labelKey: 'welcome.stepIntro' },
  { id: 'model', icon: Bot, labelKey: 'welcome.stepModel' },
  { id: 'done', icon: MessageSquare, labelKey: 'welcome.stepDone' },
] as const;

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
  const providers = useVivyStore((state) => state.providers);
  const catalog = useVivyStore((state) => state.catalog);
  const providersPhase = useVivyStore((state) => state.providersPhase);
  const providersError = useVivyStore((state) => state.providersError);
  const loadProviders = useVivyStore((state) => state.loadProviders);
  const saveProvider = useVivyStore((state) => state.saveProvider);
  const [step, setStep] = useState(0);
  const [form, setForm] = useState<WelcomeModelDraft>(EMPTY_WELCOME_MODEL);
  const [apiKey, setApiKey] = useState('');
  const [envKeyConfirmed, setEnvKeyConfirmed] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const steps = WELCOME_STEPS.map((entry) => ({ ...entry, label: t(entry.labelKey) }));
  const readOnly = !settings || settings.read_only;

  // 目录行只来自后端 settings/providers（PROV-P4）；profiles 决定端点可执行性。
  const rows = useMemo(
    () => welcomeCatalogRows(catalog, settings?.provider_profiles),
    [catalog, settings?.provider_profiles],
  );
  const vendors = useMemo(() => welcomeVendors(rows), [rows]);
  const selectedRow = rows.find(
    (candidate) => candidate.vendor === form.vendor && candidate.adapter === form.adapter,
  ) ?? null;
  const vendorEndpoints = rows.filter((candidate) => candidate.vendor === form.vendor);
  const keyStatus = selectedRow ? welcomeKeyStatus(rows, providers, settings, selectedRow) : null;
  const keyConfigured = keyStatus?.configured ?? false;
  const catalogLoading = !catalog.length && (providersPhase === 'idle' || providersPhase === 'loading');
  const catalogFailed = !rows.length && providersPhase === 'error';

  // 每次打开都从第一步重来，并按后端当前设置预填；未配置时留空由用户选择。
  useEffect(() => {
    if (!open) return;
    setStep(0);
    setSaving(false);
    setError(null);
    setApiKey('');
    setEnvKeyConfirmed(false);
    const initialRows = welcomeCatalogRows(catalog, settings?.provider_profiles);
    setForm(prefillWelcomeModel(initialRows, settings) ?? EMPTY_WELCOME_MODEL);
    // 打开时刷新一次目录，保证向导用的是后端当前能力而不是首屏快照。
    void loadProviders();
    // settings/catalog 在打开瞬间取快照即可；打开后由用户操作驱动。
    // eslint-disable-next-line react-hooks/exhaustive-deps -- open is the reset trigger
  }, [open]);

  // 首次打开时目录可能尚在应答途中：目录到达后补一次预填（不覆盖用户选择）。
  useEffect(() => {
    if (!open || form.vendor || !rows.length) return;
    const prefilled = prefillWelcomeModel(rows, settings);
    if (prefilled) setForm(prefilled);
  }, [open, rows, settings, form.vendor]);

  // 目录刷新后原选择失效（端点/厂商被移除）时清空，避免写出陈旧数据。
  useEffect(() => {
    if (!open || !form.vendor) return;
    if (rows.some((candidate) => candidate.vendor === form.vendor && candidate.adapter === form.adapter)) return;
    setForm(EMPTY_WELCOME_MODEL);
  }, [open, rows, form.vendor, form.adapter]);

  const resetKeyDraft = () => {
    setApiKey('');
    setEnvKeyConfirmed(false);
  };

  const chooseVendor = (vendor: string) => {
    const endpoints = rows.filter((candidate) => candidate.vendor === vendor);
    const first = endpoints[0];
    if (!first) return;
    setForm({ vendor, adapter: first.adapter, model: first.defaultModel });
    resetKeyDraft();
    setError(null);
  };

  const chooseProtocol = (adapter: string) => {
    const endpoint = rows.find(
      (candidate) => candidate.vendor === form.vendor && candidate.adapter === adapter,
    );
    if (!endpoint) return;
    setForm({ ...form, adapter, model: endpoint.defaultModel });
    resetKeyDraft();
    setError(null);
  };

  const chooseModel = (model: string) => {
    setForm({ ...form, model });
    setError(null);
  };

  const changeApiKey = (value: string) => {
    setApiKey(value);
    if (value.trim()) setEnvKeyConfirmed(false);
    setError(null);
  };

  const changeEnvKeyConfirmed = (checked: boolean) => {
    setEnvKeyConfirmed(checked);
    setError(null);
  };

  const goBack = () => {
    setError(null);
    setStep((current) => Math.max(current - 1, 0));
  };

  const goNext = async () => {
    if (step === 1 && !readOnly) {
      if (!selectedRow) {
        setError(t('welcome.selectionRequired'));
        return;
      }
      if (!canSaveWelcomeModel({
        readOnly,
        row: selectedRow,
        model: form.model,
        apiKey,
        keyConfigured,
        envKeyConfirmed,
      })) {
        setError(t('welcome.keyRequired'));
        return;
      }
      const selection = welcomeSelection(selectedRow, form.model);
      if (!selection) {
        setError(t('welcome.selectionRequired'));
        return;
      }
      setSaving(true);
      setError(null);
      try {
        // 密钥写-only：只在用户输入新值时 upsert；留空保留注册表里的已有密钥。
        if (apiKey.trim()) {
          const existing = providerEntryByEndpoint(providers, selectedRow.adapter, selectedRow.baseUrl);
          await saveProvider(welcomeProviderInput(selectedRow, apiKey, existing));
        }
        // 整文档替换：未改动的 overlay 分区必须随选择一起回传。
        await saveSettings(settingsUpdateFrom(settings, selection));
      } catch (cause) {
        setSaving(false);
        setError(t('welcome.saveFailed', {
          error: cause instanceof Error ? cause.message : String(cause),
        }));
        return;
      }
      setSaving(false);
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

  const nextDisabled = saving
    || (step === 1 && !readOnly && (!selectedRow || !form.model));

  if (!open) return null;

  const modelStepProps: WizardStepModelProps = {
    form,
    rows,
    vendors,
    selectedRow,
    vendorEndpoints,
    keyConfigured,
    apiKey,
    envKeyConfirmed,
    saving,
    readOnly,
    frozen: settings?.frozen ?? false,
    error,
    catalogLoading,
    catalogFailed,
    providersError,
    onChooseVendor: chooseVendor,
    onChooseProtocol: chooseProtocol,
    onChooseModel: chooseModel,
    onApiKeyChange: changeApiKey,
    onEnvKeyConfirmedChange: changeEnvKeyConfirmed,
    onSubmit: () => void goNext(),
    onRetryCatalog: () => void loadProviders(),
  };

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
            className="welcome-float absolute rounded-full bg-brand/10"
            style={{ left: orb.left, top: orb.top, width: orb.size, height: orb.size, animationDelay: `${orb.delay}s` }}
          />
        ))}
      </div>

      <div className="relative flex max-h-[90dvh] w-full max-w-lg flex-col overflow-hidden rounded-3xl border border-line bg-raised text-fg shadow-xl">
        {/* 品牌头 */}
        <div className="border-b border-line px-7 pb-5 pt-6 text-center">
          <div className="mb-3 flex items-center justify-center gap-3">
            <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-brand-soft text-brand-ink shadow-sm">
              <Bot className="h-6 w-6" aria-hidden="true" />
            </div>
            <div className="text-left">
              <h1 className="text-page font-bold leading-tight tracking-wide">Vivy</h1>
              <p className="text-2xs uppercase tracking-[0.18em] text-fg-muted">Project ViVY</p>
            </div>
          </div>
          <p className="text-ui text-fg-muted">{t('welcome.subtitle')}</p>
        </div>

        {/* 步骤进度 */}
        <div className="border-b border-line bg-sunken px-7 py-4">
          <div className="mb-4 h-1 overflow-hidden rounded-full bg-line">
            <div
              className="h-full rounded-full bg-brand transition-all duration-300"
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
                        ? 'border-brand bg-brand-fill text-brand-fg shadow-sm'
                        : index < step
                          ? 'border-brand/30 bg-brand-soft text-brand-ink'
                          : 'border-line bg-surface text-fg-muted',
                    )}
                  >
                    <Icon className="h-3.5 w-3.5" aria-hidden="true" />
                  </span>
                  <span className="whitespace-nowrap text-2xs font-medium text-fg-muted">{entry.label}</span>
                </button>
              );
            })}
          </div>
        </div>

        {/* 内容区 */}
        <div className="min-h-0 flex-1 overflow-y-auto px-7 py-6">
          {step === 0
            ? <WizardStepIntro />
            : step === 1
              ? <WizardStepModel {...modelStepProps} />
              : <WizardStepDone onFinish={finish} />}
        </div>

        {/* 底部操作 */}
        <div className="flex items-center justify-between gap-3 border-t border-line bg-sunken px-7 py-4">
          <div>
            {step === 0 ? (
              <Button type="button" variant="ghost" size="sm" className="text-meta text-fg-muted" onClick={skip}>
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
            <Button type="button" size="sm" onClick={() => void goNext()} disabled={nextDisabled}>
              {saving ? t('welcome.saving') : t('welcome.next')}
              {!saving ? <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" /> : null}
            </Button>
          ) : null}
        </div>
      </div>
    </div>
  );
}
