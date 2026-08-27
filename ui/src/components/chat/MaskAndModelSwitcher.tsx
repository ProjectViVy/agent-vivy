import { useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { Bookmark, Check, ChevronDown, ChevronRight, CircleDot, Loader2, Settings2, X } from 'lucide-react';
import { useVivyStore } from '@/lib/store';
import type { Settings } from '@/lib/api';
import type { ProviderEntry } from '@/components/settings/custom-providers';
import {
  removeSavedModel,
  savedModelVendorLabel,
  useSavedModels,
  type SavedModelEntry,
} from '@/components/settings/saved-models';
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

function displayProvider(provider: string, baseUrl: string, providers: readonly ProviderEntry[], t: ReturnType<typeof useTranslation>['t']): string {
  // 目录/注册表命中时显示厂商名（如 provider=openai + DeepSeek 网关 → “DeepSeek”；
  // 自定义网关 → 注册的显示名，未注册 → baseUrl 主机名）。
  return savedModelVendorLabel({ provider, baseUrl, model: '' }, providers) || t('maskSwitcher.defaultProvider');
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
  const providers = useVivyStore((state) => state.providers);
  const savedModels = useSavedModels();
  const [open, setOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const currentProvider = settings?.provider || settings?.config_provider || '';
  const currentBaseUrl = settings?.base_url ?? '';
  const currentModel = settings?.default_model || settings?.config_model || '';
  const saving = settingsPhase === 'processing';
  const canChange = !!settings && !settings.read_only && !saving;
  const providerLabel = displayProvider(currentProvider, currentBaseUrl, providers, t);
  const savedOptions = savedModels.filter(
    (entry) => !(entry.provider === currentProvider && entry.baseUrl === currentBaseUrl && entry.model === currentModel),
  );

  const selectModel = async (entry: SavedModelEntry) => {
    if (!settings || !canChange || (entry.provider === currentProvider && entry.baseUrl === currentBaseUrl && entry.model === currentModel)) return;
    setError(null);
    try {
// settings/update replaces the whole document: carry the loaded
      // network_search preference and execute ceiling through unchanged;
      // the key overlay is resolved by the backend from the registry, so it
      // is never sent or echoed on select.
      await saveSettings({ provider: entry.provider, default_model: entry.model, base_url: entry.baseUrl, network_search: { provider: settings.network_search?.provider ?? '' }, execute_max_timeout_seconds: settings.execute_max_timeout_seconds });
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
        <DropdownMenuItem className="cursor-default gap-3 rounded-lg bg-accent/50 p-2.5" disabled>
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-muted text-foreground"><CircleDot className="h-4 w-4" /></span>
          <span className="min-w-0 flex-1">
            <span className="block truncate text-sm font-medium">{providerLabel} <span className="font-normal text-muted-foreground">| {currentModel || t('maskSwitcher.defaultModel')}</span></span>
            <span className="block truncate text-xs text-muted-foreground">{t('maskSwitcher.currentRuntimeConfig')}</span>
          </span>
          <Check className="h-4 w-4 shrink-0 text-primary" />
        </DropdownMenuItem>
        <DropdownMenuSeparator className="my-2" />
        <DropdownMenuLabel className="px-2 pb-1 pt-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">{t('maskSwitcher.savedModels')}</DropdownMenuLabel>
        {savedOptions.length ? savedOptions.map((entry) => (
          <DropdownMenuItem
            key={`${entry.provider}/${entry.baseUrl}/${entry.model}`}
            aria-disabled={!canChange || undefined}
            className={`group cursor-pointer gap-3 rounded-lg p-2.5${!canChange ? ' opacity-50' : ''}`}
            onSelect={(event) => { event.preventDefault(); void selectModel(entry); }}
          >
            <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground"><Bookmark className="h-4 w-4" /></span>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm font-medium">{entry.model}</span>
              <span className="block truncate text-xs text-muted-foreground">{savedModelVendorLabel(entry, providers)}</span>
            </span>
            <button
              type="button"
              onClick={(event) => { event.stopPropagation(); removeSavedModel(entry.provider, entry.baseUrl, entry.model); }}
              aria-label={t('maskSwitcher.removeSavedAria', { model: entry.model })}
              title={t('maskSwitcher.removeSavedAria', { model: entry.model })}
              className="shrink-0 cursor-pointer rounded p-1 text-muted-foreground opacity-0 transition-opacity hover:bg-accent hover:text-foreground group-hover:opacity-100"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </DropdownMenuItem>
        )) : <p className="px-2 py-1.5 text-xs text-muted-foreground">{t('maskSwitcher.noSavedModels')}</p>}
        {settings.read_only ? <p className="px-2 pb-1 pt-2 text-xs text-amber-600 dark:text-amber-300">{t('maskSwitcher.readOnlyHint')}</p> : null}
      </> : <DropdownMenuItem disabled className="cursor-default">{t('maskSwitcher.readingConfig')}</DropdownMenuItem>}
      {error ? <p className="mx-2 mt-2 rounded-md bg-destructive/10 px-2.5 py-2 text-xs text-destructive">{error}</p> : null}
      <DropdownMenuSeparator className="my-2" />
      <DropdownMenuItem className="cursor-pointer gap-2 rounded-lg px-2.5 py-2 text-muted-foreground" onSelect={() => void navigate({ to: '/settings', search: { tab: 'model' } })}>
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
