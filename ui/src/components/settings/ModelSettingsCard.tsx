import { useEffect, useMemo, useState } from 'react';
import { Check, ChevronDown, ChevronRight, MoreHorizontal, Server } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  isFoldedProvider,
  matchProviderEntry,
  searchProviders,
  splitByFold,
  type ProviderCatalogEntry,
} from './provider-catalog';
import { useVivyStore } from '@/lib/store';
import { useTranslation } from '@/i18n';

type ModelFormValue = { provider: string; default_model: string; base_url: string };

/** 选中目录条目：厂商差异落到 base_url，provider 保持运行束名，模型沿用原始 id。 */
function applyProviderEntry(form: ModelFormValue, entry: ProviderCatalogEntry): ModelFormValue {
  return {
    provider: entry.bundle,
    base_url: entry.baseUrl,
    default_model: entry.models.includes(form.default_model) ? form.default_model : entry.defaultModel,
  };
}

function ProviderRow({
  entry,
  selected,
  isCurrent,
  disabled,
  currentBadge,
  onSelect,
}: {
  entry: ProviderCatalogEntry;
  selected: boolean;
  isCurrent: boolean;
  disabled: boolean;
  currentBadge: string;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onSelect}
      aria-pressed={selected}
      className={`flex w-full items-center gap-2.5 rounded-md border-l-4 px-2.5 py-2 text-left text-sm transition-colors disabled:pointer-events-none disabled:opacity-50 ${
        selected ? 'border-primary bg-accent font-medium' : 'border-transparent hover:bg-accent/50'
      }`}
    >
      <span
        className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-md ${
          selected ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'
        }`}
      >
        <Server className="h-4 w-4" aria-hidden="true" />
      </span>
      <span className="min-w-0 flex-1 truncate">{entry.displayName}</span>
      {isCurrent ? (
        <span className="shrink-0 rounded bg-primary/10 px-1.5 py-0.5 text-[10px] font-medium leading-none text-primary">
          {currentBadge}
        </span>
      ) : null}
    </button>
  );
}

/**
 * 设置页「模型」Tab 的真实配置卡：Agent-Diva 供应商目录移植。
 * 左栏是供应商列表（一批不常用供应商折叠在「更多供应商」之后，搜索时平铺），
 * 右栏是所选供应商的静态模型列表；点击目录条目只填充下方表单，保存仍走
 * settings/update 的 (provider, default_model, base_url) 三元组。
 */
export function ModelSettingsCard() {
  const settings = useVivyStore((state) => state.settings);
  const phase = useVivyStore((state) => state.settingsPhase);
  const error = useVivyStore((state) => state.settingsError);
  const load = useVivyStore((state) => state.loadSettings);
  const save = useVivyStore((state) => state.saveSettings);
  const { t } = useTranslation();
  const [form, setForm] = useState<ModelFormValue>({ provider: '', default_model: '', base_url: '' });
  const [searchTerm, setSearchTerm] = useState('');
  const [isMoreExpanded, setMoreExpanded] = useState(false);

  useEffect(() => { void load(); }, [load]);
  useEffect(() => {
    if (settings) setForm({ provider: settings.provider, default_model: settings.default_model, base_url: settings.base_url });
  }, [settings]);

  const searching = searchTerm.trim().length > 0;
  const { visible, more } = useMemo(
    () => splitByFold(searchProviders(searchTerm), searching),
    [searchTerm, searching],
  );
  const selectedEntry = matchProviderEntry(form.provider, form.base_url);
  const savedEntry = settings ? matchProviderEntry(settings.provider, settings.base_url) : undefined;

  // Agent-Diva 折叠移植：当前选中的供应商被折叠时自动展开，选中项永远可见。
  useEffect(() => {
    if (selectedEntry && isFoldedProvider(selectedEntry.name) && !searching) setMoreExpanded(true);
  }, [selectedEntry, searching]);

  const locked = settings?.read_only || phase === 'processing';

  const selectProvider = (entry: ProviderCatalogEntry) => {
    if (locked) return;
    setForm((current) => {
      const next = applyProviderEntry(current, entry);
      return next.provider === current.provider && next.base_url === current.base_url && next.default_model === current.default_model
        ? current
        : next;
    });
  };

  return (
    <>
      {phase === 'loading' && !settings ? (
        <div className="space-y-3">
          <div className="h-10 animate-pulse rounded bg-muted" />
          <div className="h-10 animate-pulse rounded bg-muted" />
          <div className="h-10 animate-pulse rounded bg-muted" />
        </div>
      ) : (
        <form
          className="space-y-4"
          onSubmit={async (event) => {
            event.preventDefault();
            await save(form);
          }}
        >
          <div className="grid gap-4 md:grid-cols-[minmax(0,260px)_minmax(0,1fr)]">
            <div className="space-y-2">
              <Input
                value={searchTerm}
                onChange={(event) => setSearchTerm(event.target.value)}
                placeholder={t('settingsModel.searchPlaceholder')}
                aria-label={t('settingsModel.searchPlaceholder')}
                className="h-9"
              />
              <div className="max-h-80 space-y-1 overflow-y-auto rounded-lg border bg-muted/30 p-1.5">
                {visible.map((entry) => (
                  <ProviderRow
                    key={entry.name}
                    entry={entry}
                    selected={entry.name === selectedEntry?.name}
                    isCurrent={entry.name === savedEntry?.name}
                    disabled={locked}
                    currentBadge={t('settingsModel.currentBadge')}
                    onSelect={() => selectProvider(entry)}
                  />
                ))}
                {more.length > 0 ? (
                  <button
                    type="button"
                    onClick={() => setMoreExpanded((expanded) => !expanded)}
                    aria-expanded={isMoreExpanded}
                    className="flex w-full items-center gap-2.5 rounded-md border border-dashed border-border border-l-4 border-l-transparent px-2.5 py-2 text-left text-sm text-muted-foreground transition-colors hover:border-primary/40 hover:bg-accent/40 hover:text-foreground"
                  >
                    <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-muted">
                      <MoreHorizontal className="h-4 w-4" aria-hidden="true" />
                    </span>
                    <span className="min-w-0 flex-1 truncate">{t('settingsModel.moreProviders')}</span>
                    <span className="shrink-0 text-[10px] leading-none">{more.length}</span>
                    {isMoreExpanded ? (
                      <ChevronDown className="h-4 w-4 shrink-0" aria-hidden="true" />
                    ) : (
                      <ChevronRight className="h-4 w-4 shrink-0" aria-hidden="true" />
                    )}
                  </button>
                ) : null}
                {isMoreExpanded
                  ? more.map((entry) => (
                      <ProviderRow
                        key={entry.name}
                        entry={entry}
                        selected={entry.name === selectedEntry?.name}
                        isCurrent={entry.name === savedEntry?.name}
                        disabled={locked}
                        currentBadge={t('settingsModel.currentBadge')}
                        onSelect={() => selectProvider(entry)}
                      />
                    ))
                  : null}
                {visible.length + more.length === 0 ? (
                  <p className="px-2.5 py-3 text-xs text-muted-foreground">{t('settingsModel.noMatch')}</p>
                ) : null}
              </div>
            </div>
            <div className="min-w-0">
              {selectedEntry ? (
                <div className="overflow-hidden rounded-lg border">
                  <div className="flex items-center justify-between gap-2 border-b px-3 py-2">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{selectedEntry.displayName}</p>
                      <p className="truncate text-xs text-muted-foreground">{selectedEntry.baseUrl || '—'}</p>
                    </div>
                    <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] leading-none text-muted-foreground">
                      {selectedEntry.bundle}
                    </span>
                  </div>
                  <div className="max-h-72 space-y-1 overflow-y-auto p-1.5">
                    {selectedEntry.models.length ? (
                      selectedEntry.models.map((model) => (
                        <button
                          key={model}
                          type="button"
                          disabled={locked}
                          onClick={() => setForm((current) => ({ ...current, default_model: model }))}
                          aria-pressed={form.default_model === model}
                          className="flex w-full items-center justify-between gap-2 rounded-md px-2.5 py-2 text-left text-sm transition-colors hover:bg-accent/50 disabled:pointer-events-none disabled:opacity-50"
                        >
                          <span className="min-w-0 truncate font-mono text-xs">{model}</span>
                          {form.default_model === model ? <Check className="h-4 w-4 shrink-0 text-primary" aria-hidden="true" /> : null}
                        </button>
                      ))
                    ) : (
                      <p className="px-2.5 py-3 text-xs text-muted-foreground">{t('settingsModel.noModels')}</p>
                    )}
                  </div>
                </div>
              ) : (
                <div className="flex h-full min-h-40 items-center rounded-lg border border-dashed p-4 text-xs text-muted-foreground">
                  {t('settingsModel.customProviderHint')}
                </div>
              )}
            </div>
          </div>
          <div className="grid gap-4 sm:grid-cols-3">
            <div className="space-y-2">
              <Label htmlFor="provider">Provider</Label>
              <Input
                id="provider"
                value={form.provider}
                onChange={(event) => setForm({ ...form, provider: event.target.value })}
                placeholder={settings?.config_provider || '配置默认值'}
                disabled={locked}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="model">默认模型</Label>
              <Input
                id="model"
                value={form.default_model}
                onChange={(event) => setForm({ ...form, default_model: event.target.value })}
                placeholder={settings?.config_model || 'Provider 默认值'}
                disabled={locked}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="base-url">Base URL</Label>
              <Input
                id="base-url"
                type="url"
                value={form.base_url}
                onChange={(event) => setForm({ ...form, base_url: event.target.value })}
                placeholder="https://api.example.com/v1"
                disabled={locked}
              />
            </div>
          </div>
          {settings?.read_only ? (
            <p className="rounded bg-amber-500/10 p-3 text-sm text-amber-700">此部署的设置为只读，请通过运行配置修改。</p>
          ) : (
            <Button type="submit" disabled={phase === 'processing'}>
              {phase === 'processing' ? '保存中…' : '保存真实设置'}
            </Button>
          )}
          {error ? <p className="rounded bg-destructive/10 p-3 text-sm text-destructive">{error}</p> : null}
        </form>
      )}
    </>
  );
}
