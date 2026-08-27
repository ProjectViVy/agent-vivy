import { useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { Check, ChevronDown, ChevronRight, CircleDot, Loader2, Settings2 } from 'lucide-react';
import { useVivyStore } from '@/lib/store';
import type { Settings } from '@/lib/api';
import { matchProviderEntry, type ProviderCatalogEntry } from '@/components/settings/provider-catalog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Button } from '@/components/ui/button';
import { MaskIdentity } from '@/components/masks/MaskIdentity';
import { maskOptions, setActiveMaskId, useActiveMask, type MaskOption } from '@/components/masks/mask-catalog';
import { useTranslation } from '@/i18n';

type ModelOption = {
  id: string;
  /** Vivy 运行束名（保存到 settings.provider 的值） */
  provider: string;
  model: string;
  /** 描述词条 key（目录是静态数据，文案经 i18n 解析）；未收录的模型无副标题 */
  descriptionKey?: string;
};

/** 目录模型的精选描述（沿用原有文案）。 */
const MODEL_DESCRIPTION_KEYS: Record<string, string> = {
  'openai:gpt-4o': 'maskSwitcher.models.balanced',
  'openai:gpt-4o-mini': 'maskSwitcher.models.lighter',
  'openai:gpt-5': 'maskSwitcher.models.complex',
  'openai:gpt-5-mini': 'maskSwitcher.models.lightTasks',
  'mock:mock': 'maskSwitcher.models.offline',
};

function displayProvider(provider: string, baseUrl: string, t: ReturnType<typeof useTranslation>['t']): string {
  // 目录命中时显示厂商名（如 provider=openai + DeepSeek 网关 → “DeepSeek”）。
  return matchProviderEntry(provider, baseUrl)?.displayName || provider || t('maskSwitcher.defaultProvider');
}

function modelOptionsFor(settings: Settings | null, vendorEntry: ProviderCatalogEntry | undefined): ModelOption[] {
  const provider = settings?.provider || settings?.config_provider || '';
  const model = settings?.default_model || settings?.config_model || '';
  const current: ModelOption = {
    id: `${provider || 'default'}:${model || 'default'}`,
    provider,
    model,
    descriptionKey: 'maskSwitcher.currentRuntimeConfig',
  };
  const catalog: ModelOption[] = vendorEntry
    ? vendorEntry.models.map((modelId) => ({
        id: `${vendorEntry.bundle}:${modelId}`,
        provider: vendorEntry.bundle,
        model: modelId,
        descriptionKey: MODEL_DESCRIPTION_KEYS[`${vendorEntry.bundle}:${modelId}`],
      }))
    : [];
  const options = [current, ...catalog];
  return options.filter((option, index) => options.findIndex((item) => item.provider === option.provider && item.model === option.model) === index);
}

function MaskMenu({ activeMask, onSelect }: { activeMask: MaskOption; onSelect: (id: string) => void }) {
  const navigate = useNavigate();
  const { t } = useTranslation();
  return <DropdownMenu>
    <DropdownMenuTrigger asChild>
      <Button variant="ghost" className="h-9 max-w-[180px] gap-2 px-1.5 font-normal hover:bg-accent/70 sm:px-2.5" aria-label={t('maskSwitcher.switchMaskAria', { name: activeMask.name })} title={t('maskSwitcher.switchMaskTitle')}>
        <MaskIdentity option={activeMask} compact />
        <span className="hidden truncate text-sm sm:inline">{activeMask.name}</span>
        <ChevronDown className="hidden h-3.5 w-3.5 shrink-0 text-muted-foreground sm:block" />
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="start" className="w-80 max-w-[calc(100vw-1rem)] p-2">
      <DropdownMenuLabel className="px-2 pb-1 pt-0 text-xs font-normal text-muted-foreground">{t('maskSwitcher.chooseMask')}</DropdownMenuLabel>
      {maskOptions().map((option) => <DropdownMenuItem key={option.id} className="cursor-pointer gap-3 rounded-lg p-2.5" onSelect={() => onSelect(option.id)}>
        <MaskIdentity option={option} />
        <span className="min-w-0 flex-1">
          <span className="block truncate text-sm font-medium">{option.name}</span>
          <span className="block truncate text-xs text-muted-foreground">{option.description}</span>
        </span>
        {activeMask.id === option.id ? <Check className="h-4 w-4 shrink-0 text-primary" /> : null}
      </DropdownMenuItem>)}
      <DropdownMenuSeparator className="my-2" />
      <DropdownMenuItem className="cursor-pointer gap-2 rounded-lg px-2.5 py-2 text-muted-foreground" onSelect={() => void navigate({ to: '/masks' })}>
        <Settings2 className="h-4 w-4" />
        <span>{t('maskSwitcher.manageMasks')}</span>
        <ChevronRight className="ml-auto h-3.5 w-3.5" />
      </DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>;
}

function ModelMenu({ settings }: { settings: Settings | null }) {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const saveSettings = useVivyStore((state) => state.saveSettings);
  const settingsPhase = useVivyStore((state) => state.settingsPhase);
  const [open, setOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const currentProvider = settings?.provider || settings?.config_provider || '';
  const currentBaseUrl = settings?.base_url ?? '';
  const vendorEntry = useMemo(() => matchProviderEntry(currentProvider, currentBaseUrl), [currentProvider, currentBaseUrl]);
  const options = useMemo(() => modelOptionsFor(settings, vendorEntry), [settings, vendorEntry]);
  const currentModel = settings?.default_model || settings?.config_model || '';
  const currentOption = options.find((option) => option.provider === currentProvider && option.model === currentModel) || options[0];
  const saving = settingsPhase === 'processing';
  const canChange = !!settings && !settings.read_only && !saving;
  const providerLabel = displayProvider(currentProvider, currentBaseUrl, t);

  const selectModel = async (option: ModelOption) => {
    if (!settings || !canChange || (option.provider === currentProvider && option.model === currentModel)) return;
    setError(null);
    try {
      await saveSettings({ provider: option.provider, default_model: option.model, base_url: settings.base_url });
      setOpen(false);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    }
  };

  return <DropdownMenu open={open} onOpenChange={(nextOpen) => { setOpen(nextOpen); if (!nextOpen) setError(null); }}>
    <DropdownMenuTrigger asChild>
      <Button variant="ghost" className="h-9 max-w-[270px] gap-2 px-1.5 font-normal hover:bg-accent/70 sm:px-2.5" aria-label={t('maskSwitcher.switchModelAria', { provider: providerLabel, model: currentModel || t('maskSwitcher.defaultModel') })} title={t('maskSwitcher.switchModelTitle')}>
        {saving ? <Loader2 className="h-4 w-4 shrink-0 animate-spin text-primary" /> : <CircleDot className="h-4 w-4 shrink-0 text-foreground" />}
        <span className="hidden min-w-0 truncate text-sm md:inline">{providerLabel}</span>
        <span className="hidden shrink-0 text-muted-foreground md:inline">|</span>
        <span className="hidden min-w-0 truncate text-sm text-muted-foreground md:inline">{currentModel || t('maskSwitcher.defaultModel')}</span>
        <ChevronDown className="hidden h-3.5 w-3.5 shrink-0 text-muted-foreground sm:block" />
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="start" className="w-80 max-w-[calc(100vw-1rem)] p-2">
      <DropdownMenuLabel className="px-2 pb-1 pt-0 text-xs font-normal text-muted-foreground">{t('maskSwitcher.chooseModel')}</DropdownMenuLabel>
      {settings ? <>
        <DropdownMenuLabel className="px-2 pt-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">{t('maskSwitcher.currentConfig')}</DropdownMenuLabel>
        {currentOption ? <DropdownMenuItem className="cursor-default gap-3 rounded-lg bg-accent/50 p-2.5" disabled>
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-muted text-foreground"><CircleDot className="h-4 w-4" /></span>
          <span className="min-w-0 flex-1">
            <span className="block truncate text-sm font-medium">{displayProvider(currentOption.provider, currentBaseUrl, t)} <span className="font-normal text-muted-foreground">| {currentOption.model || t('maskSwitcher.defaultModel')}</span></span>
            <span className="block truncate text-xs text-muted-foreground">{currentOption.descriptionKey ? t(currentOption.descriptionKey) : null}</span>
          </span>
          <Check className="h-4 w-4 shrink-0 text-primary" />
        </DropdownMenuItem> : null}
        {options.filter((option) => option.provider !== currentOption?.provider || option.model !== currentOption?.model).length ? <>
          <DropdownMenuSeparator className="my-2" />
          <DropdownMenuLabel className="px-2 pb-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">{t('maskSwitcher.optionalModels', { provider: providerLabel })}</DropdownMenuLabel>
          {options.filter((option) => option.provider !== currentOption?.provider || option.model !== currentOption?.model).map((option) => <DropdownMenuItem key={option.id} disabled={!canChange} className="cursor-pointer gap-3 rounded-lg p-2.5" onSelect={(event) => { event.preventDefault(); void selectModel(option); }}>
            <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground"><CircleDot className="h-4 w-4" /></span>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm font-medium">{option.model}</span>
              {option.descriptionKey ? <span className="block truncate text-xs text-muted-foreground">{t(option.descriptionKey)}</span> : null}
            </span>
          </DropdownMenuItem>)}
        </> : null}
        {settings.read_only ? <p className="px-2 pb-1 pt-2 text-xs text-amber-600 dark:text-amber-300">{t('maskSwitcher.readOnlyHint')}</p> : null}
      </> : <DropdownMenuItem disabled className="cursor-default">{t('maskSwitcher.readingConfig')}</DropdownMenuItem>}
      {error ? <p className="mx-2 mt-2 rounded-md bg-destructive/10 px-2.5 py-2 text-xs text-destructive">{error}</p> : null}
      <DropdownMenuSeparator className="my-2" />
      <DropdownMenuItem className="cursor-pointer gap-2 rounded-lg px-2.5 py-2 text-muted-foreground" onSelect={() => void navigate({ to: '/settings' })}>
        <Settings2 className="h-4 w-4" />
        <span>{t('maskSwitcher.manageModelSettings')}</span>
        <ChevronRight className="ml-auto h-3.5 w-3.5" />
      </DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>;
}

export function MaskAndModelSwitcher() {
  const activeMask = useActiveMask();
  const settings = useVivyStore((state) => state.settings);

  return <div className="flex min-w-0 max-w-full items-center gap-1 rounded-xl border bg-background/70 p-0.5 shadow-sm">
    <MaskMenu activeMask={activeMask} onSelect={setActiveMaskId} />
    <span aria-hidden="true" className="h-5 w-px shrink-0 bg-border" />
    <ModelMenu settings={settings} />
  </div>;
}
