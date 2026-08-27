import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { Bookmark, Check, ChevronDown, ChevronRight, MoreHorizontal, Pencil, Plus, Server, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import {
  isFoldedProvider,
  type ProviderCatalogEntry,
  type ProviderRuntimeBundle,
} from './provider-catalog';
import {
  addCustomProvider,
  allProviderEntries,
  customApiKeyFor,
  matchMergedProviderEntry,
  parseCustomModels,
  removeCustomProvider,
  searchMergedProviders,
  splitMergedByFold,
  updateCustomProvider,
  useCustomProviders,
  type CustomProvider,
  type CustomProviderInput,
  type MergedProviderEntry,
} from './custom-providers';
import {
  addSavedModel,
  removeSavedModel,
  savedModelVendorLabel,
  useSavedModels,
  type SavedModelEntry,
} from './saved-models';
import { useVivyStore } from '@/lib/store';
import { useTranslation } from '@/i18n';

type ModelFormValue = { provider: string; default_model: string; base_url: string; api_key: string };

/** 选中目录/注册表条目：厂商差异落到 base_url，provider 保持运行束名，模型沿用原始 id。
 *  api_key 由调用方经 customApiKeyFor 按所选条目解析（纯函数不读注册表）。 */
function applyProviderEntry(form: ModelFormValue, entry: ProviderCatalogEntry): Omit<ModelFormValue, 'api_key'> {
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
  customBadge,
  actions,
  onSelect,
}: {
  entry: ProviderCatalogEntry;
  selected: boolean;
  isCurrent: boolean;
  disabled: boolean;
  currentBadge: string;
  customBadge?: string;
  /** 自定义行的编辑/删除等行内动作；存在时行根改为外层分组容器（避免 button 内嵌 button）。 */
  actions?: ReactNode;
  onSelect: () => void;
}) {
  const row = (
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
      {customBadge ? (
        <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium leading-none text-muted-foreground">
          {customBadge}
        </span>
      ) : null}
      {isCurrent ? (
        <span className="shrink-0 rounded bg-primary/10 px-1.5 py-0.5 text-[10px] font-medium leading-none text-primary">
          {currentBadge}
        </span>
      ) : null}
    </button>
  );
  if (!actions) return row;
  return (
    <div className="group flex items-center gap-1">
      <div className="min-w-0 flex-1">{row}</div>
      <div className="flex shrink-0 items-center gap-0.5 pr-1 opacity-0 transition-opacity group-hover:opacity-100">{actions}</div>
    </div>
  );
}

/** 新增/编辑自定义供应商的对话框：显示名 + 运行束 + Base URL + 默认模型 + 模型列表。 */
function CustomProviderDialog({
  open,
  editing,
  onOpenChange,
  onSave,
}: {
  open: boolean;
  editing: CustomProvider | null;
  onOpenChange: (open: boolean) => void;
  onSave: (input: CustomProviderInput) => boolean;
}) {
  const { t } = useTranslation();
  const [displayName, setDisplayName] = useState('');
  const [bundle, setBundle] = useState<ProviderRuntimeBundle>('openai');
  const [baseUrl, setBaseUrl] = useState('');
  const [defaultModel, setDefaultModel] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [modelsText, setModelsText] = useState('');
  const [fieldError, setFieldError] = useState<Partial<Record<'displayName' | 'baseUrl', string>>>({});
  const [submitError, setSubmitError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setDisplayName(editing?.displayName ?? '');
    setBundle(editing?.bundle ?? 'openai');
    setBaseUrl(editing?.baseUrl ?? '');
    setDefaultModel(editing?.defaultModel ?? '');
    setApiKey(editing?.apiKey ?? '');
    setModelsText(editing?.models.join('\n') ?? '');
    setFieldError({});
    setSubmitError(null);
  }, [open, editing]);

  const submit = () => {
    const name = displayName.trim();
    const url = baseUrl.trim();
    const errors: typeof fieldError = {};
    if (!name) errors.displayName = t('settingsModel.errors.displayNameRequired');
    if (!url) errors.baseUrl = t('settingsModel.errors.baseUrlRequired');
    else {
      let valid = false;
      try {
        const parsed = new URL(url);
        valid = parsed.protocol === 'http:' || parsed.protocol === 'https:';
      } catch {
        valid = false;
      }
      if (!valid) errors.baseUrl = t('settingsModel.errors.baseUrlInvalid');
    }
    setFieldError(errors);
    if (Object.keys(errors).length) return;
    const saved = onSave({
      displayName: name,
      bundle,
      baseUrl: url,
      defaultModel: defaultModel.trim(),
      models: parseCustomModels(modelsText),
      apiKey: apiKey.trim(),
    });
    if (!saved) {
      setSubmitError(t('settingsModel.errors.duplicateBaseUrl'));
      return;
    }
    setSubmitError(null);
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{editing ? t('settingsModel.customDialogTitleEdit') : t('settingsModel.customDialogTitleNew')}</DialogTitle>
          <DialogDescription>{t('settingsModel.customDialogHint')}</DialogDescription>
        </DialogHeader>
        <div className="grid gap-3">
          <div className="space-y-1.5">
            <Label htmlFor="custom-display-name">{t('settingsModel.displayName')}</Label>
            <Input id="custom-display-name" value={displayName} onChange={(event) => setDisplayName(event.target.value)} />
            {fieldError.displayName ? <p className="text-xs text-destructive">{fieldError.displayName}</p> : null}
          </div>
          <div className="space-y-1.5">
            <Label>{t('settingsModel.bundle')}</Label>
            <Select value={bundle} onValueChange={(value) => setBundle(value as ProviderRuntimeBundle)}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="openai">{t('settingsModel.bundleOpenai')}</SelectItem>
                <SelectItem value="anthropic">{t('settingsModel.bundleAnthropic')}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="custom-base-url">{t('settingsModel.baseUrl')}</Label>
            <Input
              id="custom-base-url"
              type="url"
              value={baseUrl}
              onChange={(event) => setBaseUrl(event.target.value)}
              placeholder="https://api.example.com/v1"
            />
            {fieldError.baseUrl ? <p className="text-xs text-destructive">{fieldError.baseUrl}</p> : null}
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="custom-default-model">{t('settingsModel.defaultModel')}</Label>
            <Input id="custom-default-model" value={defaultModel} onChange={(event) => setDefaultModel(event.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="custom-api-key">{t('settingsModel.apiKey')}</Label>
            <Input
              id="custom-api-key"
              type="password"
              value={apiKey}
              onChange={(event) => setApiKey(event.target.value)}
              placeholder={t('settingsModel.apiKeyPlaceholder')}
              autoComplete="off"
            />
            <p className="text-xs text-muted-foreground">{t('settingsModel.apiKeyHint')}</p>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="custom-models">{t('settingsModel.modelsList')}</Label>
            <Textarea id="custom-models" rows={5} value={modelsText} onChange={(event) => setModelsText(event.target.value)} placeholder={t('settingsModel.modelsListHint')} />
          </div>
          {submitError ? <p className="rounded bg-destructive/10 p-2.5 text-sm text-destructive">{submitError}</p> : null}
        </div>
        <DialogFooter className="gap-2">
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>{t('settingsModel.cancel')}</Button>
          <Button type="button" onClick={submit}>{t('settingsModel.save')}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/**
 * 设置页「模型」Tab 的真实配置卡：Agent-Diva 供应商目录移植 + 「已选模型」快捷切换
 * + 自定义供应商注册表。顶部是已选模型 chips（点击=立即选用并保存，行内 X=仅移除）；
 * 左栏供应商列表（静态目录 + 自定义供应商，自定义行带「自定义」标记与编辑/删除）,
 * 右栏模型列表；模型行点击=填表单 + 加入快捷列表 + 立即保存。
 * 三输入框 + 「保存真实设置」保留为自定义组合的显式提交边界。
 */
export function ModelSettingsCard() {
  const settings = useVivyStore((state) => state.settings);
  const phase = useVivyStore((state) => state.settingsPhase);
  const error = useVivyStore((state) => state.settingsError);
  const load = useVivyStore((state) => state.loadSettings);
  const save = useVivyStore((state) => state.saveSettings);
  const savedModels = useSavedModels();
  const customProviders = useCustomProviders();
  const { t } = useTranslation();
  const [form, setForm] = useState<ModelFormValue>({ provider: '', default_model: '', base_url: '', api_key: '' });
  const [searchTerm, setSearchTerm] = useState('');
  const [isMoreExpanded, setMoreExpanded] = useState(false);
  const [customDialog, setCustomDialog] = useState<{ open: boolean; editing: CustomProvider | null }>({ open: false, editing: null });

  useEffect(() => { void load(); }, [load]);
  useEffect(() => {
    if (settings) setForm({ provider: settings.provider, default_model: settings.default_model, base_url: settings.base_url, api_key: customApiKeyFor(settings.provider, settings.base_url) });
  }, [settings, customProviders]);

  const searching = searchTerm.trim().length > 0;
  const { visible, custom, more } = useMemo(
    () => splitMergedByFold(searching ? searchMergedProviders(searchTerm) : allProviderEntries(), searching),
    [searchTerm, searching, customProviders],
  );
  const selectedEntry = matchMergedProviderEntry(form.provider, form.base_url);
  const savedEntry = settings ? matchMergedProviderEntry(settings.provider, settings.base_url) : undefined;

  // Agent-Diva 折叠移植：当前选中的供应商被折叠时自动展开，选中项永远可见（自定义永不折叠）。
  useEffect(() => {
    if (selectedEntry && !selectedEntry.custom && isFoldedProvider(selectedEntry.name) && !searching) setMoreExpanded(true);
  }, [selectedEntry, searching]);

  const locked = settings?.read_only || phase === 'processing';

  const selectProvider = (entry: ProviderCatalogEntry) => {
    if (locked) return;
    setForm((current) => {
      const next = applyProviderEntry(current, entry);
      const same = next.provider === current.provider && next.base_url === current.base_url && next.default_model === current.default_model;
      // 表单密钥跟随所选条目：自定义供应商显示其注册密钥，目录条目清空（提交即清覆盖层）。
      return same && current.api_key === customApiKeyFor(next.provider, next.base_url)
        ? current
        : { ...next, api_key: customApiKeyFor(next.provider, next.base_url) };
    });
  };

  /** 所选供应商目录模型行：填表单 + 加入快捷列表（幂等）+ 立即保存（diva 式一步到位；自定义条目带其密钥）。 */
  const applyModelNow = async (entry: ProviderCatalogEntry, model: string) => {
    if (locked) return;
    const next = applyProviderEntry(form, entry);
    const triple = { provider: next.provider, default_model: model, base_url: next.base_url, api_key: customApiKeyFor(next.provider, next.base_url) };
    setForm({ ...next, default_model: model, api_key: customApiKeyFor(next.provider, next.base_url) });
    addSavedModel({ provider: next.provider, baseUrl: next.base_url, model });
    try {
      await save(triple);
    } catch {
      // settingsError 已由 store 记录并渲染在下方 error 段；保留已填表单与已加入列表。
    }
  };

  /** 已选模型 chip：与顶栏快捷切换同语义——立即选用并保存（自定义条目带其密钥）。 */
  const applySavedNow = async (entry: SavedModelEntry) => {
    if (locked) return;
    try {
      await save({ provider: entry.provider, default_model: entry.model, base_url: entry.baseUrl, api_key: customApiKeyFor(entry.provider, entry.baseUrl) });
    } catch {
      // settingsError 已由 store 记录并渲染在下方 error 段。
    }
  };

  const isCurrentModelRow = (entry: ProviderCatalogEntry, model: string) =>
    !!settings && !!savedEntry && savedEntry.name === entry.name && settings.default_model === model;

  const isSavedModelRow = (entry: ProviderCatalogEntry, model: string) =>
    savedModels.some(
      (item) => item.provider === entry.bundle && item.baseUrl === entry.baseUrl && item.model === model,
    ) && !isCurrentModelRow(entry, model);

  const openCustomProviderDialog = (entry?: MergedProviderEntry) => {
    const editing = entry?.custom && entry.registryId
      ? customProviders.find((provider) => provider.id === entry.registryId) ?? null
      : null;
    setCustomDialog({ open: true, editing });
  };

  const saveCustomProvider = (input: CustomProviderInput): boolean => {
    if (customDialog.editing) return updateCustomProvider(customDialog.editing.id, input);
    return addCustomProvider(input) !== null;
  };

  const renderRow = (entry: MergedProviderEntry) => (
    <ProviderRow
      key={entry.name}
      entry={entry}
      selected={entry.name === selectedEntry?.name}
      isCurrent={entry.name === savedEntry?.name}
      disabled={locked}
      currentBadge={t('settingsModel.currentBadge')}
      customBadge={entry.custom ? t('settingsModel.customBadge') : undefined}
      onSelect={() => selectProvider(entry)}
      actions={entry.custom ? (
        <>
          <button
            type="button"
            onClick={() => openCustomProviderDialog(entry)}
            aria-label={t('settingsModel.editAria', { name: entry.displayName })}
            title={t('settingsModel.editAria', { name: entry.displayName })}
            className="cursor-pointer rounded p-1 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
          >
            <Pencil className="h-3.5 w-3.5" aria-hidden="true" />
          </button>
          <button
            type="button"
            onClick={() => removeCustomProvider(entry.registryId!)}
            aria-label={t('settingsModel.removeAria', { name: entry.displayName })}
            title={t('settingsModel.removeAria', { name: entry.displayName })}
            className="cursor-pointer rounded p-1 text-muted-foreground transition-colors hover:bg-accent hover:text-destructive"
          >
            <X className="h-3.5 w-3.5" aria-hidden="true" />
          </button>
        </>
      ) : undefined}
    />
  );

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
            await save(form).catch(() => undefined);
          }}
        >
          <div className="space-y-1.5">
            <p className="text-sm font-medium">{t('settingsModel.savedTitle')}</p>
            {savedModels.length ? (
              <div className="flex flex-wrap gap-1.5">
                {savedModels.map((entry) => (
                  <span
                    key={`${entry.provider}/${entry.baseUrl}/${entry.model}`}
                    className="inline-flex max-w-full items-center rounded-full border bg-muted/40"
                  >
                    <button
                      type="button"
                      disabled={locked}
                      onClick={() => void applySavedNow(entry)}
                      className="min-w-0 cursor-pointer truncate rounded-full py-1 pl-2.5 pr-1 text-xs transition-colors hover:bg-accent/60 disabled:pointer-events-none disabled:opacity-50"
                    >
                      <span className="font-medium">{savedModelVendorLabel(entry)}</span>
                      <span className="text-muted-foreground"> · </span>
                      <span className="font-mono text-[11px]">{entry.model}</span>
                    </button>
                    <button
                      type="button"
                      onClick={() => removeSavedModel(entry.provider, entry.baseUrl, entry.model)}
                      aria-label={t('settingsModel.removeSaved', { model: entry.model })}
                      title={t('settingsModel.removeSaved', { model: entry.model })}
                      className="cursor-pointer rounded-full p-1 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                    >
                      <X className="h-3.5 w-3.5" aria-hidden="true" />
                    </button>
                  </span>
                ))}
              </div>
            ) : (
              <p className="rounded-md border border-dashed px-3 py-2 text-xs text-muted-foreground">{t('settingsModel.savedEmpty')}</p>
            )}
          </div>
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
                {visible.map(renderRow)}
                {custom.map(renderRow)}
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
                {isMoreExpanded ? more.map(renderRow) : null}
                {visible.length + more.length === 0 ? (
                  <p className="px-2.5 py-3 text-xs text-muted-foreground">{t('settingsModel.noMatch')}</p>
                ) : null}
                <button
                  type="button"
                  onClick={() => openCustomProviderDialog()}
                  className="flex w-full items-center gap-2.5 rounded-md border border-dashed border-border border-l-4 border-l-transparent px-2.5 py-2 text-left text-sm text-muted-foreground transition-colors hover:border-primary/40 hover:bg-accent/40 hover:text-foreground"
                >
                  <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-muted">
                    <Plus className="h-4 w-4" aria-hidden="true" />
                  </span>
                  <span className="min-w-0 flex-1 truncate">{t('settingsModel.addCustomProvider')}</span>
                </button>
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
                      selectedEntry.models.map((model) => {
                        const isCurrent = isCurrentModelRow(selectedEntry, model);
                        const isSaved = isSavedModelRow(selectedEntry, model);
                        return (
                          <button
                            key={model}
                            type="button"
                            disabled={locked}
                            onClick={() => void applyModelNow(selectedEntry, model)}
                            aria-pressed={form.default_model === model}
                            title={isSaved && !isCurrent ? t('settingsModel.added') : undefined}
                            className="flex w-full items-center justify-between gap-2 rounded-md px-2.5 py-2 text-left text-sm transition-colors hover:bg-accent/50 disabled:pointer-events-none disabled:opacity-50"
                          >
                            <span className="min-w-0 truncate font-mono text-xs">{model}</span>
                            {isCurrent ? (
                              <Check className="h-4 w-4 shrink-0 text-primary" aria-hidden="true" />
                            ) : isSaved ? (
                              <Bookmark className="h-4 w-4 shrink-0 text-muted-foreground/70" aria-hidden="true" />
                            ) : null}
                          </button>
                        );
                      })
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
          <div className="grid gap-4 sm:grid-cols-2">
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
            <div className="space-y-2">
              <Label htmlFor="model-api-key">{t('settingsModel.apiKey')}</Label>
              <Input
                id="model-api-key"
                type="password"
                value={form.api_key}
                onChange={(event) => setForm({ ...form, api_key: event.target.value })}
                placeholder={t('settingsModel.apiKeyPlaceholder')}
                autoComplete="off"
                disabled={locked}
              />
              <p className="text-xs text-muted-foreground">{t('settingsModel.apiKeyHint')}</p>
            </div>
          </div>
          {settings?.read_only ? (
            <p className="rounded bg-amber-500/10 p-3 text-sm text-amber-700">此部署的设置为只读，请通过运行配置修改。</p>
          ) : (
            <Button type="submit" disabled={phase === 'processing'}>
              {phase === 'processing' ? '保存中…' : '保存真实设置'}
            </Button>
          )}
          {settings?.api_key_set ? <p className="text-xs text-muted-foreground">{t('settingsModel.apiKeyConfigured')}</p> : null}
          {error ? <p className="rounded bg-destructive/10 p-3 text-sm text-destructive">{error}</p> : null}
        </form>
      )}
      <CustomProviderDialog
        open={customDialog.open}
        editing={customDialog.editing}
        onOpenChange={(open) => setCustomDialog({ open, editing: open ? customDialog.editing : null })}
        onSave={saveCustomProvider}
      />
    </>
  );
}
