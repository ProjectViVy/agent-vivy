import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { Bookmark, Check, ChevronDown, ChevronRight, MoreHorizontal, Pencil, Plus, RefreshCw, Server, X } from 'lucide-react';
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
      <div className="flex shrink-0 items-center gap-0.5 pr-1">{actions}</div>
    </div>
  );
}

/** 新增/编辑自定义供应商的对话框：显示名（别名）+ 运行束 + Base URL（地址）+ 默认模型 + 模型列表 + API Key。 */
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
 * 设置页「模型」Tab 的真实配置卡：顶部是已选模型 chips；左栏供应商列表
 * （静态目录 + 自定义供应商，自定义行常驻编辑按钮 + hover 删除）；右栏所选
 * 供应商的模型列表（头部「从官方同步」刷新 + 「新增」手加模型），模型列表上方
 * 是 API Key 填写（自定义供应商可编辑，目录厂商禁用并提示环境变量注入）。
 * 点击模型/新增模型 = 立即选用并保存；无底部表单（显式提交边界已并入模型点击）。
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
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [searchTerm, setSearchTerm] = useState('');
  const [isMoreExpanded, setMoreExpanded] = useState(false);
  const [customDialog, setCustomDialog] = useState<{ open: boolean; editing: CustomProvider | null }>({ open: false, editing: null });
  const [addingModel, setAddingModel] = useState(false);
  const [newModelId, setNewModelId] = useState('');
  const [panelKey, setPanelKey] = useState('');
  const [refreshNote, setRefreshNote] = useState<string | null>(null);

  useEffect(() => { void load(); }, [load]);

  const allMerged = useMemo(() => allProviderEntries(), [customProviders]);
  const searching = searchTerm.trim().length > 0;
  const { visible, custom, more } = useMemo(
    () => splitMergedByFold(searching ? searchMergedProviders(searchTerm) : allMerged, searching),
    [searching, searchTerm, allMerged],
  );
  const savedEntry = settings ? matchMergedProviderEntry(settings.provider, settings.base_url) : undefined;
  const selectedEntry = (selectedName ? allMerged.find((entry) => entry.name === selectedName) : undefined) ?? savedEntry;
  const selectedRegistry = selectedEntry?.custom && selectedEntry.registryId
    ? customProviders.find((provider) => provider.id === selectedEntry.registryId) ?? null
    : null;

  // 默认选中当前运行配置对应的供应商（运行供应商折叠时自动展开，保证可见）。
  useEffect(() => {
    if (selectedName === null && savedEntry) setSelectedName(savedEntry.name);
  }, [selectedName, savedEntry]);
  useEffect(() => {
    if (selectedEntry && !selectedEntry.custom && isFoldedProvider(selectedEntry.name) && !searching) setMoreExpanded(true);
  }, [selectedEntry, searching]);

  // 面板 API Key 跟随所选供应商：自定义回显注册密钥；目录清空（提交即清覆盖层）。
  useEffect(() => {
    setPanelKey(selectedRegistry?.apiKey ?? '');
    // eslint-disable-next-line react-hooks/exhaustive-deps -- 跟随所选条目与注册表变化
  }, [selectedEntry?.name, customProviders]);

  const locked = settings?.read_only || phase === 'processing';

  /** 所选供应商的模型（含「新增」手加）：立即选用并保存（自定义条目带其密钥）。 */
  const applyModelNow = async (entry: ProviderCatalogEntry, model: string) => {
    if (locked) return;
    addSavedModel({ provider: entry.bundle, baseUrl: entry.baseUrl, model });
    try {
      await save({ provider: entry.bundle, default_model: model, base_url: entry.baseUrl, api_key: customApiKeyFor(entry.bundle, entry.baseUrl) });
    } catch {
      // settingsError 已由 store 记录并渲染；已加入快捷列表保留。
    }
  };

  /** 已选模型 chip：与顶栏快捷切换同语义——立即选用并保存（自定义条目带其密钥）。 */
  const applySavedNow = async (entry: SavedModelEntry) => {
    if (locked) return;
    try {
      await save({ provider: entry.provider, default_model: entry.model, base_url: entry.baseUrl, api_key: customApiKeyFor(entry.provider, entry.baseUrl) });
    } catch {
      // settingsError 已由 store 记录并渲染。
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

  /** 面板 API Key：自定义供应商在失焦时写回注册表，随模型点击应用；目录条目禁用。 */
  const commitPanelKey = () => {
    if (!selectedRegistry) return;
    updateCustomProvider(selectedRegistry.id, {
      displayName: selectedRegistry.displayName,
      bundle: selectedRegistry.bundle,
      baseUrl: selectedRegistry.baseUrl,
      defaultModel: selectedRegistry.defaultModel,
      models: selectedRegistry.models,
      apiKey: panelKey.trim(),
    });
  };

  /** 从官方目录同步：重新载入合并视图并给出反馈。真实在线同步见 UI-PROV-RPC（静态快照）。 */
  const handleRefresh = () => {
    setRefreshNote(t('settingsModel.refreshedModels'));
    window.setTimeout(() => setRefreshNote(null), 1800);
  };

  /** 「新增」手加模型：自定义供应商同时持久化进注册表列表；随后与点击模型同语义立即应用。 */
  const confirmAddModel = () => {
    const id = newModelId.trim();
    if (!id || !selectedEntry || locked) return;
    if (selectedEntry.custom && selectedEntry.registryId) {
      const registry = customProviders.find((provider) => provider.id === selectedEntry.registryId);
      if (registry) {
        updateCustomProvider(registry.id, {
          displayName: registry.displayName,
          bundle: registry.bundle,
          baseUrl: registry.baseUrl,
          defaultModel: registry.defaultModel,
          models: [...registry.models, id],
          apiKey: registry.apiKey,
        });
      }
    }
    void applyModelNow(selectedEntry, id);
    setNewModelId('');
    setAddingModel(false);
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
      onSelect={() => setSelectedName(entry.name)}
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
            className="cursor-pointer rounded p-1 text-muted-foreground opacity-0 transition-opacity hover:bg-accent hover:text-destructive group-hover:opacity-100"
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
        <div className="space-y-4">
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
          {settings?.read_only ? (
            <p className="rounded bg-amber-500/10 p-3 text-sm text-amber-700">此部署的设置为只读，请通过运行配置修改。</p>
          ) : null}
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
                    <div className="flex shrink-0 items-center gap-0.5">
                      <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] leading-none text-muted-foreground">
                        {selectedEntry.bundle}
                      </span>
                      <button
                        type="button"
                        onClick={handleRefresh}
                        aria-label={t('settingsModel.refreshModels')}
                        title={t('settingsModel.refreshModels')}
                        className="cursor-pointer rounded p-1 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                      >
                        <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />
                      </button>
                      <button
                        type="button"
                        onClick={() => setAddingModel((adding) => !adding)}
                        aria-label={t('settingsModel.addModel')}
                        title={t('settingsModel.addModel')}
                        className="cursor-pointer rounded p-1 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                      >
                        <Plus className="h-3.5 w-3.5" aria-hidden="true" />
                      </button>
                    </div>
                  </div>
                  <div className="space-y-1.5 border-b px-3 py-2">
                    <Label htmlFor="panel-api-key" className="text-xs text-muted-foreground">{t('settingsModel.apiKey')}</Label>
                    <Input
                      id="panel-api-key"
                      type="password"
                      value={panelKey}
                      onChange={(event) => setPanelKey(event.target.value)}
                      onBlur={commitPanelKey}
                      placeholder={selectedEntry.custom ? t('settingsModel.apiKeyPlaceholder') : t('settingsModel.catalogKeyHint')}
                      autoComplete="off"
                      disabled={locked || !selectedEntry.custom}
                    />
                    <p className="text-xs text-muted-foreground">{selectedEntry.custom ? t('settingsModel.apiKeyHint') : t('settingsModel.catalogKeyHint')}</p>
                    {selectedEntry.name === savedEntry?.name && settings?.api_key_set ? (
                      <p className="text-xs text-muted-foreground">{t('settingsModel.apiKeyConfigured')}</p>
                    ) : null}
                  </div>
                  <div className="max-h-72 space-y-1 overflow-y-auto p-1.5">
                    {addingModel ? (
                      <div className="flex items-center gap-1 px-0.5">
                        <Input
                          autoFocus
                          value={newModelId}
                          onChange={(event) => setNewModelId(event.target.value)}
                          onKeyDown={(event) => {
                            if (event.key === 'Enter') confirmAddModel();
                            if (event.key === 'Escape') { setAddingModel(false); setNewModelId(''); }
                          }}
                          placeholder={t('settingsModel.addModelPlaceholder')}
                          aria-label={t('settingsModel.addModel')}
                          className="h-8 font-mono text-xs"
                        />
                        <button
                          type="button"
                          onClick={confirmAddModel}
                          aria-label={t('settingsModel.addModelConfirm')}
                          title={t('settingsModel.addModelConfirm')}
                          className="cursor-pointer rounded p-1 text-primary transition-colors hover:bg-accent"
                        >
                          <Check className="h-4 w-4" aria-hidden="true" />
                        </button>
                        <button
                          type="button"
                          onClick={() => { setAddingModel(false); setNewModelId(''); }}
                          aria-label={t('common.cancel')}
                          title={t('common.cancel')}
                          className="cursor-pointer rounded p-1 text-muted-foreground transition-colors hover:bg-accent"
                        >
                          <X className="h-4 w-4" aria-hidden="true" />
                        </button>
                      </div>
                    ) : null}
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
                            aria-pressed={isCurrent}
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
                    {refreshNote ? <p className="px-2.5 py-1 text-[11px] text-muted-foreground">{refreshNote}</p> : null}
                  </div>
                </div>
              ) : (
                <div className="flex h-full min-h-40 items-center rounded-lg border border-dashed p-4 text-xs text-muted-foreground">
                  {t('settingsModel.customProviderHint')}
                </div>
              )}
            </div>
          </div>
          {error ? <p className="rounded bg-destructive/10 p-3 text-sm text-destructive">{error}</p> : null}
        </div>
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
