import { useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { Check, ChevronDown, ChevronRight, CircleDot, Loader2, Settings2 } from 'lucide-react';
import { useVivyStore } from '@/lib/store';
import type { Settings } from '@/lib/api';
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
import { MASK_OPTIONS, setActiveMaskId, useActiveMask, type MaskOption } from '@/components/masks/mask-catalog';

type ModelOption = {
  id: string;
  provider: string;
  model: string;
  description: string;
};

const MODEL_CATALOG: Record<string, ModelOption[]> = {
  openai: [
    { id: 'openai:gpt-4o', provider: 'openai', model: 'gpt-4o', description: '均衡响应与工具调用' },
    { id: 'openai:gpt-4o-mini', provider: 'openai', model: 'gpt-4o-mini', description: '更快、更轻量' },
    { id: 'openai:gpt-5', provider: 'openai', model: 'gpt-5', description: '复杂任务与长上下文' },
    { id: 'openai:gpt-5-mini', provider: 'openai', model: 'gpt-5-mini', description: '轻量任务与快速迭代' },
  ],
  mock: [
    { id: 'mock:mock', provider: 'mock', model: 'mock', description: '离线验证与 UI 测试' },
  ],
};

function displayProvider(provider: string): string {
  if (provider === 'openai') return 'OpenAI';
  if (provider === 'mock') return 'Mock';
  return provider || '默认';
}

function modelOptionsFor(settings: Settings | null): ModelOption[] {
  const provider = settings?.provider || settings?.config_provider || '';
  const model = settings?.default_model || settings?.config_model || '';
  const current: ModelOption = {
    id: `${provider || 'default'}:${model || 'default'}`,
    provider,
    model,
    description: '当前运行配置',
  };
  const catalog = MODEL_CATALOG[provider] || [];
  const options = [current, ...catalog];
  return options.filter((option, index) => options.findIndex((item) => item.provider === option.provider && item.model === option.model) === index);
}

function MaskMenu({ activeMask, onSelect }: { activeMask: MaskOption; onSelect: (id: string) => void }) {
  const navigate = useNavigate();
  return <DropdownMenu>
    <DropdownMenuTrigger asChild>
      <Button variant="ghost" className="h-9 max-w-[180px] gap-2 px-1.5 font-normal hover:bg-accent/70 sm:px-2.5" aria-label={`切换面具，当前为${activeMask.name}`} title="切换面具">
        <MaskIdentity option={activeMask} compact />
        <span className="hidden truncate text-sm sm:inline">{activeMask.name}</span>
        <ChevronDown className="hidden h-3.5 w-3.5 shrink-0 text-muted-foreground sm:block" />
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="start" className="w-80 max-w-[calc(100vw-1rem)] p-2">
      <DropdownMenuLabel className="px-2 pb-1 pt-0 text-xs font-normal text-muted-foreground">选择面具</DropdownMenuLabel>
      {MASK_OPTIONS.map((option) => <DropdownMenuItem key={option.id} className="cursor-pointer gap-3 rounded-lg p-2.5" onSelect={() => onSelect(option.id)}>
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
        <span>管理面具</span>
        <ChevronRight className="ml-auto h-3.5 w-3.5" />
      </DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>;
}

function ModelMenu({ settings }: { settings: Settings | null }) {
  const navigate = useNavigate();
  const saveSettings = useVivyStore((state) => state.saveSettings);
  const settingsPhase = useVivyStore((state) => state.settingsPhase);
  const [open, setOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const options = useMemo(() => modelOptionsFor(settings), [settings]);
  const currentProvider = settings?.provider || settings?.config_provider || '';
  const currentModel = settings?.default_model || settings?.config_model || '';
  const currentOption = options.find((option) => option.provider === currentProvider && option.model === currentModel) || options[0];
  const saving = settingsPhase === 'processing';
  const canChange = !!settings && !settings.read_only && !saving;

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
      <Button variant="ghost" className="h-9 max-w-[270px] gap-2 px-1.5 font-normal hover:bg-accent/70 sm:px-2.5" aria-label={`切换模型，当前为${displayProvider(currentProvider)} ${currentModel || '默认模型'}`} title="切换模型">
        {saving ? <Loader2 className="h-4 w-4 shrink-0 animate-spin text-primary" /> : <CircleDot className="h-4 w-4 shrink-0 text-foreground" />}
        <span className="hidden min-w-0 truncate text-sm md:inline">{displayProvider(currentProvider)}</span>
        <span className="hidden shrink-0 text-muted-foreground md:inline">|</span>
        <span className="hidden min-w-0 truncate text-sm text-muted-foreground md:inline">{currentModel || '默认模型'}</span>
        <ChevronDown className="hidden h-3.5 w-3.5 shrink-0 text-muted-foreground sm:block" />
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="start" className="w-80 max-w-[calc(100vw-1rem)] p-2">
      <DropdownMenuLabel className="px-2 pb-1 pt-0 text-xs font-normal text-muted-foreground">选择模型</DropdownMenuLabel>
      {settings ? <>
        <DropdownMenuLabel className="px-2 pt-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">当前配置</DropdownMenuLabel>
        {currentOption ? <DropdownMenuItem className="cursor-default gap-3 rounded-lg bg-accent/50 p-2.5" disabled>
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-muted text-foreground"><CircleDot className="h-4 w-4" /></span>
          <span className="min-w-0 flex-1">
            <span className="block truncate text-sm font-medium">{displayProvider(currentOption.provider)} <span className="font-normal text-muted-foreground">| {currentOption.model || '默认模型'}</span></span>
            <span className="block truncate text-xs text-muted-foreground">{currentOption.description}</span>
          </span>
          <Check className="h-4 w-4 shrink-0 text-primary" />
        </DropdownMenuItem> : null}
        {options.filter((option) => option.provider !== currentOption?.provider || option.model !== currentOption?.model).length ? <>
          <DropdownMenuSeparator className="my-2" />
          <DropdownMenuLabel className="px-2 pb-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">{displayProvider(currentProvider)} 可选模型</DropdownMenuLabel>
          {options.filter((option) => option.provider !== currentOption?.provider || option.model !== currentOption?.model).map((option) => <DropdownMenuItem key={option.id} disabled={!canChange} className="cursor-pointer gap-3 rounded-lg p-2.5" onSelect={(event) => { event.preventDefault(); void selectModel(option); }}>
            <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground"><CircleDot className="h-4 w-4" /></span>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm font-medium">{option.model}</span>
              <span className="block truncate text-xs text-muted-foreground">{option.description}</span>
            </span>
          </DropdownMenuItem>)}
        </> : null}
        {settings.read_only ? <p className="px-2 pb-1 pt-2 text-xs text-amber-600 dark:text-amber-300">当前部署为只读，请在运行配置中修改模型。</p> : null}
      </> : <DropdownMenuItem disabled className="cursor-default">正在读取模型配置…</DropdownMenuItem>}
      {error ? <p className="mx-2 mt-2 rounded-md bg-destructive/10 px-2.5 py-2 text-xs text-destructive">{error}</p> : null}
      <DropdownMenuSeparator className="my-2" />
      <DropdownMenuItem className="cursor-pointer gap-2 rounded-lg px-2.5 py-2 text-muted-foreground" onSelect={() => void navigate({ to: '/settings' })}>
        <Settings2 className="h-4 w-4" />
        <span>管理模型设置</span>
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
