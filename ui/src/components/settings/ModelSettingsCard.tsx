import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { Bookmark, Check, Pencil, Plus, RefreshCw, Server, X } from 'lucide-react';
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
  isProviderAdapter,
  normalizeProviderAdapter,
  providerRowKey,
  providerSelection,
  isProviderExecutable,
  type ProviderCatalogRow,
} from './provider-catalog';
import {
  allProviderEntries,
  catalogOverlayId,
  customApiKeySetFor,
  matchMergedProviderEntry,
  newCustomProviderId,
  parseCustomModels,
  providerEntryById,
  providerEntryByEndpoint,
  searchMergedProviders,
  supportsModelRefresh,
  type CustomProviderInput,
  type MergedProviderEntry,
  type ProviderEntry,
  type ProviderRegistryBundle,
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

/** 对话框「新增」模式的预填字段（目录条目克隆为自定义时带入原地址/模型）。 */
type CustomProviderPreset = Pick<CustomProviderInput, 'displayName' | 'bundle' | 'baseUrl' | 'defaultModel' | 'models'>;

/** 编辑态视图：wire 条目 → 对话框可读的 camelCase 形态（apiKey 为本地编辑副本，不来自 wire）。 */
function providerView(entry: ProviderEntry): CustomProviderPreset & { id: string; apiKey: string } {
  return {
    id: entry.id,
    displayName: entry.display_name,
    bundle: entry.bundle,
    baseUrl: entry.base_url,
    defaultModel: entry.default_model,
    models: [...entry.models],
    apiKey: '',
  };
}

/** 目录行的适配器收窄到注册表写侧取值（目录只给密封适配器，见 api.ProviderAdapterId）。 */
function asRegistryBundle(adapter: string): ProviderRegistryBundle {
  return isProviderAdapter(adapter) ? adapter : 'openai-completions';
}

function ProviderRow({
  entry,
  selected,
  isCurrent,
  disabled,
  currentBadge,
  customBadge,
  capabilityBadge,
  showAdapter,
  actions,
  onSelect,
}: {
  entry: ProviderCatalogRow;
  selected: boolean;
  isCurrent: boolean;
  disabled: boolean;
  currentBadge: string;
  customBadge?: string;
  capabilityBadge?: string;
  /** 同厂商有多个端点变体时行内标出协议适配器（同名行因此可区分）。 */
  showAdapter?: boolean;
  /** 自定义行的编辑/删除等行内动作；存在时行根改为外层分组容器（避免 button 内嵌 button）。 */
  actions?: ReactNode;
  onSelect: () => void;
}) {
  const row = (
    <button
      type="button"
      data-testid={`provider-row-${entry.vendor}-${entry.adapter}`}
      disabled={disabled}
      onClick={onSelect}
      aria-pressed={selected}
      className={`flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm transition-colors disabled:pointer-events-none disabled:opacity-50 ${
        selected ? 'bg-accent font-medium text-accent-foreground' : 'hover:bg-accent/60'
      }`}
    >
      <span
        className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-md ${
          selected ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'
        }`}
      >
        <Server className="h-4 w-4" aria-hidden="true" />
      </span>
      <span data-testid={`provider-row-${entry.vendor}-${entry.adapter}-name`} className="min-w-0 flex-1 truncate">{entry.displayName}</span>
      {showAdapter ? (
        <span className="shrink-0 font-mono text-[10px] leading-none text-muted-foreground">{entry.adapter}</span>
      ) : null}
      {customBadge ? (
        <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium leading-none text-muted-foreground">
          {customBadge}
        </span>
      ) : null}
      {capabilityBadge ? (
        <span
          data-testid={`provider-capability-${entry.vendor}-${entry.adapter}`}
          className="shrink-0 rounded bg-amber-500/10 px-1.5 py-0.5 text-[10px] font-medium leading-none text-amber-700 dark:text-amber-300"
        >
          {capabilityBadge}
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
  preset,
  onOpenChange,
  onSave,
}: {
  open: boolean;
  editing: CustomProviderPreset & { id: string; apiKey: string } | null;
  /** 新增模式的预填内容（如目录条目克隆为自定义）；editing 优先。 */
  preset?: CustomProviderPreset | null;
  onOpenChange: (open: boolean) => void;
  /** 后端 RPC 保存（真写注册表）；返回 false 表示冲突/校验失败，对话框保留并提示。 */
  onSave: (input: CustomProviderInput) => Promise<boolean>;
}) {
  const { t } = useTranslation();
  const [displayName, setDisplayName] = useState('');
  // 新增自定义条目的默认适配器取 openai-completions：自定义条目本质是第三方
  // 网关，OpenAI 兼容族是厂商中立口径；应用级默认选择（后端 active=deepseek）
  // 由运行配置与欢迎向导承载，不在这里改写。
  const [bundle, setBundle] = useState<ProviderRegistryBundle>('openai-completions');
  const [baseUrl, setBaseUrl] = useState('');
  const [defaultModel, setDefaultModel] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [modelsText, setModelsText] = useState('');
  const [fieldError, setFieldError] = useState<Partial<Record<'displayName' | 'baseUrl', string>>>({});
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setDisplayName(editing?.displayName ?? preset?.displayName ?? '');
    setBundle(editing?.bundle ?? preset?.bundle ?? 'openai-completions');
    setBaseUrl(editing?.baseUrl ?? preset?.baseUrl ?? '');
    setDefaultModel(editing?.defaultModel ?? preset?.defaultModel ?? '');
    setApiKey(editing?.apiKey ?? '');
    setModelsText(editing?.models.join('\n') ?? preset?.models.join('\n') ?? '');
    setFieldError({});
    setSubmitError(null);
    setSaving(false);
  }, [open, editing, preset]);

  const submit = async () => {
    if (saving) return;
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
    setSaving(true);
    try {
      const saved = await onSave({
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
    } catch {
      setSubmitError(t('settingsModel.errors.saveFailed'));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {editing
              ? t('settingsModel.customDialogTitleEdit')
              : preset
                ? t('settingsModel.customDialogTitleManage')
                : t('settingsModel.customDialogTitleNew')}
          </DialogTitle>
          <DialogDescription>
            {editing
              ? t('settingsModel.customDialogHint')
              : preset
                ? t('settingsModel.customDialogHintManage')
                : t('settingsModel.customDialogHint')}
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3">
          <div className="space-y-1.5">
            <Label htmlFor="custom-display-name">{t('settingsModel.displayName')}</Label>
            <Input id="custom-display-name" value={displayName} onChange={(event) => setDisplayName(event.target.value)} />
            {fieldError.displayName ? <p className="text-xs text-destructive">{fieldError.displayName}</p> : null}
          </div>
          <div className="space-y-1.5">
            <Label>{t('settingsModel.adapter')}</Label>
            <Select value={bundle} onValueChange={(value) => setBundle(value as ProviderRegistryBundle)}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {/* 协议适配器是唯一写侧词汇：自定义端点就是这两种可执行协议，
                    DeepSeek 等厂商差异由 baseUrl + 模型 id 表达（旧运行束名由
                    后端读取时归一）。 */}
                <SelectItem value="openai-completions">{t('settingsModel.bundleOpenai')}</SelectItem>
                <SelectItem value="anthropic-messages">{t('settingsModel.bundleAnthropic')}</SelectItem>
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
          <Button type="button" variant="outline" disabled={saving} onClick={() => onOpenChange(false)}>{t('settingsModel.cancel')}</Button>
          <Button type="button" disabled={saving} onClick={() => void submit()}>{t('settingsModel.save')}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/**
 * 设置页「模型」Tab 的真实配置卡：顶部是已选模型 chips；左栏供应商列表
 * （后端目录端点 + 自定义供应商，自定义行常驻编辑按钮 + hover 删除）；右栏
 * 头部是所选端点名/地址/协议适配器 + 编辑（编辑：自定义=打开编辑对话框；
 * 目录=预填克隆为自定义后改地址），下方 API Key 填写（自定义与目录端点均可
 * 编辑，失焦按端点写回本机用户工作区，同一端点只落一条注册表密钥），再下方
 * 模型列表（兜底为空时提示，列表顶部「新增」按钮
 * 手加模型）。
 * 点击模型/新增模型 = 立即选用并保存；无底部表单（显式提交边界已并入模型点击）。
 * 目录数据只来自 store 的 `settings/providers` 快照（PROV-P4）：应答前渲染
 * loading，卡片自身不持有任何厂商/端点/模型数据。
 */
export function ModelSettingsCard() {
  const settings = useVivyStore((state) => state.settings);
  const phase = useVivyStore((state) => state.settingsPhase);
  const error = useVivyStore((state) => state.settingsError);
  const load = useVivyStore((state) => state.loadSettings);
  const save = useVivyStore((state) => state.saveSettings);
  const providers = useVivyStore((state) => state.providers);
  const catalog = useVivyStore((state) => state.catalog);
  const providersPhase = useVivyStore((state) => state.providersPhase);
  const loadProviders = useVivyStore((state) => state.loadProviders);
  const saveProvider = useVivyStore((state) => state.saveProvider);
  const removeProvider = useVivyStore((state) => state.removeProvider);
  const refreshProvider = useVivyStore((state) => state.refreshProvider);
  const providersError = useVivyStore((state) => state.providersError);
  const savedModels = useSavedModels();
  const { t } = useTranslation();
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [searchTerm, setSearchTerm] = useState('');
  const [customDialog, setCustomDialog] = useState<{ open: boolean; editing: CustomProviderPreset & { id: string; apiKey: string } | null; preset: CustomProviderPreset | null }>({
    open: false,
    editing: null,
    preset: null,
  });
  const [addingModel, setAddingModel] = useState(false);
  const [newModelId, setNewModelId] = useState('');
  const [panelKey, setPanelKey] = useState('');
/** 密钥输入是否被用户改过；未改前失焦不提交（防单纯聚焦/切走误清已配密钥）。 */
  const [panelKeyDirty, setPanelKeyDirty] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [refreshNote, setRefreshNote] = useState<string | null>(null);

  useEffect(() => { void load(); void loadProviders(); }, [load, loadProviders]);

  const allMerged = useMemo(
    () => allProviderEntries(catalog, providers, settings?.provider_profiles),
    [catalog, providers, settings?.provider_profiles],
  );
  const searching = searchTerm.trim().length > 0;
  const rows = useMemo(
    () => (searching ? searchMergedProviders(allMerged, searchTerm) : allMerged),
    [searching, searchTerm, allMerged],
  );
  /** 同厂商有多个端点变体（如 DeepSeek / OpenAI）时行内标出协议适配器。 */
  const multiEndpointVendors = useMemo(() => {
    const counts = new Map<string, number>();
    for (const entry of allMerged) if (!entry.custom) counts.set(entry.vendor, (counts.get(entry.vendor) ?? 0) + 1);
    return new Set([...counts].filter(([, count]) => count > 1).map(([vendor]) => vendor));
  }, [allMerged]);
  const savedEntry = settings
    ? matchMergedProviderEntry(catalog, providers, settings.provider, settings.base_url, settings.provider_profiles)
    : undefined;
  const selectedEntry = (selectedKey ? allMerged.find((entry) => providerRowKey(entry) === selectedKey) : undefined) ?? savedEntry;
  const selectedRegistry = selectedEntry?.custom && selectedEntry.registryId
    ? providerEntryById(providers, selectedEntry.registryId) ?? null
    : null;

  // 目录只来自后端 settings/providers：到达之前不渲染任何厂商行。
  const catalogLoading = !catalog.length && (providersPhase === 'idle' || providersPhase === 'loading');

  // 默认选中当前运行配置对应的端点行。
  useEffect(() => {
    if (selectedKey === null && savedEntry) setSelectedKey(providerRowKey(savedEntry));
  }, [selectedKey, savedEntry]);

  // 面板 API Key 的编辑态回显：注册表只在线程内回显 apiKeySet，不携带值；
  // 这里仅保留「已配置」提示，输入框内容在失焦时作为写-only 值提交。
  useEffect(() => {
    setPanelKey('');
    setPanelKeyDirty(false);
    setRefreshNote(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- 跟随所选条目与注册表变化
  }, [selectedEntry ? providerRowKey(selectedEntry) : null, providers]);

  const locked = !!settings?.read_only || phase === 'processing';

  /** 所选端点的模型（含「新增」手加）：立即选用并保存；密钥由后端按注册表解析，不回传。 */
  const applyModelNow = async (entry: MergedProviderEntry, model: string) => {
    if (locked || !entry.executable) return;
    const selection = providerSelection(entry, model);
    if (!selection) return;
    addSavedModel({ provider: entry.adapter, baseUrl: entry.baseUrl, model });
    try {
      await save(selection);
    } catch {
      // settingsError 已由 store 记录并渲染；已加入快捷列表保留。
    }
  };

  /** 已选模型 chip：与顶栏快捷切换同语义——立即选用并保存（密钥后端解析）。 */
  const applySavedNow = async (entry: SavedModelEntry) => {
    if (locked || !isProviderExecutable(entry.provider, settings?.provider_profiles)) return;
    try {
      // 写侧一律写适配器：旧快捷条目里的运行束名在这里归一。
      await save({ provider: normalizeProviderAdapter(entry.provider), default_model: entry.model, base_url: entry.baseUrl });
    } catch {
      // settingsError 已由 store 记录并渲染。
    }
  };

  const isCurrentModelRow = (entry: ProviderCatalogRow, model: string) =>
    !!settings && !!savedEntry && providerRowKey(savedEntry) === providerRowKey(entry) && settings.default_model === model;

  const isSavedModelRow = (entry: ProviderCatalogRow, model: string) =>
    savedModels.some(
      (item) =>
        normalizeProviderAdapter(item.provider) === normalizeProviderAdapter(entry.adapter) &&
        item.baseUrl === entry.baseUrl &&
        item.model === model,
    ) && !isCurrentModelRow(entry, model);

  const openCustomProviderDialog = (entry?: MergedProviderEntry) => {
    if (entry?.custom && entry.registryId) {
      const wire = providerEntryById(providers, entry.registryId);
      setCustomDialog({ open: true, editing: wire ? providerView(wire) : null, preset: null });
      return;
    }
    setCustomDialog(
      entry
        ? {
            open: true,
            editing: null,
            preset: {
              displayName: entry.displayName,
              bundle: asRegistryBundle(entry.adapter),
              baseUrl: entry.baseUrl,
              defaultModel: entry.defaultModel,
              models: entry.models,
            },
          }
        : { open: true, editing: null, preset: null },
    );
  };

  /** 后端写注册表：编辑=按 id upsert；新增=生成新 id upsert。冲突由后端校验拒绝。 */
  const saveCustomProvider = async (input: CustomProviderInput): Promise<boolean> => {
    const editing = customDialog.editing;
    try {
      if (editing) {
        await saveProvider({
          id: editing.id,
          display_name: input.displayName,
          bundle: input.bundle,
          base_url: input.baseUrl,
          default_model: input.defaultModel,
          models: input.models,
          api_key: input.apiKey,
        });
      } else {
        await saveProvider({
          id: newCustomProviderId(),
          display_name: input.displayName,
          bundle: input.bundle,
          base_url: input.baseUrl,
          default_model: input.defaultModel,
          models: input.models,
          api_key: input.apiKey,
        });
      }
      return true;
    } catch {
      // providersError 已由 store 记录并渲染；对话框保留以便重试/修改。
      return false;
    }
  };

  /**
   * 面板 API Key（写-only，失焦提交）：自定义供应商写回其注册表条目；目录端点
   * 按 (adapter, base_url) 落盘——端点已有注册表条目（含既有自定义克隆）则
   * 更新其密钥，否则生成 `catalog-<vendor>-<adapter>` 落地条（隐藏于自定义
   * 列表，后端 ActiveKey 按端点解析）。输入未被修改过就失焦时不提交（防误清
   * 已配密钥/防凭空建条目）。
   */
  const commitPanelKey = async () => {
    if (!selectedEntry) return;
    if (!panelKeyDirty) return; // 未修改过就失焦：不写任何东西（防误清/防凭空建条目）
    const key = panelKey.trim();
    try {
      if (selectedRegistry) {
        await saveProvider({
          id: selectedRegistry.id,
          display_name: selectedRegistry.display_name,
          bundle: selectedRegistry.bundle,
          base_url: selectedRegistry.base_url,
          default_model: selectedRegistry.default_model,
          models: selectedRegistry.models,
          api_key: key,
        });
        setPanelKeyDirty(false);
        return;
      }
      if (selectedEntry.custom) {
        setPanelKeyDirty(false);
        return;
      } // 自定义条目注册表缺失：保持原 no-op
      const existing = providerEntryByEndpoint(providers, selectedEntry.adapter, selectedEntry.baseUrl);
      if (existing) {
        await saveProvider({
          id: existing.id,
          display_name: existing.display_name,
          bundle: existing.bundle,
          base_url: existing.base_url,
          default_model: existing.default_model,
          models: existing.models,
          api_key: key,
        });
      } else if (key !== '') {
        // 端点尚无注册表条目且本轮没有输入值：不凭空创建空密钥落地条。
        await saveProvider({
          id: catalogOverlayId(selectedEntry.vendor, selectedEntry.adapter),
          display_name: selectedEntry.displayName,
          bundle: selectedEntry.adapter,
          base_url: selectedEntry.baseUrl,
          default_model: selectedEntry.defaultModel,
          models: [...selectedEntry.models],
          api_key: key,
        });
      }
      setPanelKeyDirty(false);
    } catch {
      // saveProvider 已将可读错误写入 providersError；保留 dirty 和输入值，
      // 让用户修正配置后再次失焦即可重试，同时避免 unhandled rejection。
    }
  };

  /**
   * 「新增」手加模型：自定义供应商先落注册表再应用（两写并发会竞争同一
   * settings 文档）；应用语义与点击模型一致。
   */
  const confirmAddModel = async () => {
    const id = newModelId.trim();
    if (!id || !selectedEntry || locked) return;
    if (selectedEntry.custom && selectedEntry.registryId) {
      const registry = providerEntryById(providers, selectedEntry.registryId);
      if (registry) {
        try {
          await saveProvider({
            id: registry.id,
            display_name: registry.display_name,
            bundle: registry.bundle,
            base_url: registry.base_url,
            default_model: registry.default_model,
            models: [...registry.models, id],
            api_key: panelKey.trim(),
          });
        } catch {
          // providersError 已由 store 记录并渲染；仍应用本地选择。
        }
      }
    }
    void applyModelNow(selectedEntry, id);
    setNewModelId('');
    setAddingModel(false);
  };

  /**
   * 「刷新」：从上游 GET /models 拉取模型列表并保存本地。已有注册表条目按
   * id 刷新（密钥保留）；目录端点无注册表行时克隆为自定义条目以持久化。
   * 门控只有一条（supportsModelRefresh）：适配器是 openai-completions 且
   * base_url 为 http(s)——与后端读的是同一个适配器属性；Anthropic 原生端点
   * 没有 GET /models 协议，地址非 http(s) 的端点后端也会拒绝，按钮不显示。
   */
  const refreshSelectedProvider = async () => {
    if (!selectedEntry || locked || refreshing) return;
    if (!supportsModelRefresh(selectedEntry.adapter, selectedEntry.baseUrl)) return;
    setRefreshing(true);
    setRefreshNote(null);
    try {
      const saved = selectedEntry.custom && selectedEntry.registryId
        ? await refreshProvider({ id: selectedEntry.registryId })
        : await refreshProvider({
            bundle: selectedEntry.adapter,
            base_url: selectedEntry.baseUrl,
            display_name: selectedEntry.displayName,
            default_model: selectedEntry.defaultModel,
          });
      setRefreshNote(t('settingsModel.refreshed', { count: saved.models.length }));
    } catch {
      // 失败已由 store 写入 providersError 并在卡片底部渲染。
    } finally {
      setRefreshing(false);
    }
  };

  const renderRow = (entry: MergedProviderEntry) => (
    <ProviderRow
      key={providerRowKey(entry)}
      entry={entry}
      selected={!!selectedEntry && providerRowKey(entry) === providerRowKey(selectedEntry)}
      isCurrent={!!savedEntry && providerRowKey(entry) === providerRowKey(savedEntry)}
      disabled={locked || !entry.executable}
      currentBadge={t('settingsModel.currentBadge')}
      customBadge={entry.custom ? t('settingsModel.customBadge') : undefined}
      capabilityBadge={!entry.executable ? entry.capabilityState : undefined}
      showAdapter={!entry.custom && multiEndpointVendors.has(entry.vendor)}
      onSelect={() => setSelectedKey(providerRowKey(entry))}
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
            onClick={() => void removeProvider(entry.registryId!).catch(() => undefined)}
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
      ) : catalogLoading ? (
        // 目录只来自后端 settings/providers：应答前没有任何厂商行可渲染。
        <div data-testid="provider-catalog-loading" role="status" aria-busy="true" className="space-y-3">
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
                      disabled={locked || !isProviderExecutable(entry.provider, settings?.provider_profiles)}
                      onClick={() => void applySavedNow(entry)}
                      className="min-w-0 cursor-pointer truncate rounded-full py-1 pl-2.5 pr-1 text-xs transition-colors hover:bg-accent/60 disabled:pointer-events-none disabled:opacity-50"
                    >
                      <span className="font-medium">{savedModelVendorLabel(entry, providers, catalog)}</span>
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
            <p className="rounded bg-amber-500/10 p-3 text-sm text-amber-600 dark:text-amber-300">{t('settings.readOnlyNotice')}</p>
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
              <div className="max-h-80 space-y-1 overflow-y-auto rounded-lg border bg-card p-1.5">
                {rows.map(renderRow)}
                {rows.length === 0 ? (
                  <p className="px-2.5 py-3 text-xs text-muted-foreground">{t('settingsModel.noMatch')}</p>
                ) : null}
                <button
                  type="button"
                  onClick={() => openCustomProviderDialog()}
                  className="flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm text-muted-foreground transition-colors hover:bg-accent/60 hover:text-foreground"
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
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium">{selectedEntry.displayName}</p>
                      <p className="truncate text-xs text-muted-foreground">{selectedEntry.baseUrl || '—'}</p>
                    </div>
                    <div className="flex shrink-0 items-center gap-0.5">
                      <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] leading-none text-muted-foreground">
                        {selectedEntry.adapter}
                      </span>
                      <button
                        type="button"
                        onClick={() => openCustomProviderDialog(selectedEntry)}
                        aria-label={t('settingsModel.editAddressAria')}
                        title={t('settingsModel.editAddressAria')}
                        className="cursor-pointer rounded p-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                      >
                        <Pencil className="h-4 w-4" aria-hidden="true" />
                      </button>
                    </div>
                  </div>
                  <div className="border-b px-3 py-2">
                    <div className="flex items-center justify-between gap-2">
                      <Label htmlFor="panel-api-key" className="text-xs text-muted-foreground">{t('settingsModel.apiKey')}</Label>
                      {!!savedEntry && providerRowKey(selectedEntry) === providerRowKey(savedEntry) && customApiKeySetFor(providers, settings?.provider ?? '', settings?.base_url ?? '') ? (
                        <span className="text-[11px] text-muted-foreground">{t('settingsModel.apiKeyConfigured')}</span>
                      ) : null}
                    </div>
                    <Input
                      id="panel-api-key"
                      type="password"
                      className="mt-1.5"
                      value={panelKey}
                      onChange={(event) => { setPanelKey(event.target.value); setPanelKeyDirty(true); }}
                      onBlur={() => void commitPanelKey()}
                      placeholder={t('settingsModel.apiKeyPlaceholder')}
                      autoComplete="off"
                      disabled={locked}
                    />
                    <p className="mt-1.5 text-xs text-muted-foreground">{t('settingsModel.apiKeyHint')}</p>
                  </div>
                  <div>
                    <div className="flex items-center justify-between gap-2 border-b px-3 py-2">
                      <p className="text-xs font-medium text-muted-foreground">{t('settingsModel.modelsTitle', { provider: selectedEntry.displayName })}</p>
                      <div className="flex shrink-0 items-center gap-0.5">
                        {supportsModelRefresh(selectedEntry.adapter, selectedEntry.baseUrl) ? (
                          <button
                            type="button"
                            onClick={() => void refreshSelectedProvider()}
                            disabled={locked || refreshing}
                            aria-label={refreshing ? t('settingsModel.refreshing') : t('settingsModel.refreshModels')}
                            title={t('settingsModel.refreshModels')}
                            className="cursor-pointer rounded p-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground disabled:pointer-events-none disabled:opacity-50"
                          >
                            <RefreshCw className={`h-4 w-4 ${refreshing ? 'animate-spin' : ''}`} aria-hidden="true" />
                          </button>
                        ) : null}
                        <button
                          type="button"
                          onClick={() => setAddingModel((adding) => !adding)}
                          aria-label={t('settingsModel.addModel')}
                          title={t('settingsModel.addModel')}
                          className="cursor-pointer rounded p-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                        >
                          <Plus className="h-4 w-4" aria-hidden="true" />
                        </button>
                      </div>
                    </div>
                    {refreshNote ? (
                      <p className="border-b px-3 py-1.5 text-xs text-muted-foreground">{refreshNote}</p>
                    ) : null}
                    <div className="max-h-72 space-y-1 overflow-y-auto p-1.5">
                    {addingModel ? (
                      <div className="flex items-center gap-1 px-0.5">
                        <Input
                          autoFocus
                          value={newModelId}
                          onChange={(event) => setNewModelId(event.target.value)}
                          onKeyDown={(event) => {
                            if (event.key === 'Enter') void confirmAddModel();
                            if (event.key === 'Escape') { setAddingModel(false); setNewModelId(''); }
                          }}
                          placeholder={t('settingsModel.addModelPlaceholder')}
                          aria-label={t('settingsModel.addModel')}
                          className="h-8 font-mono text-xs"
                        />
                        <button
                          type="button"
                          onClick={() => void confirmAddModel()}
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
                            data-testid={`provider-model-${model}`}
                            disabled={locked || !selectedEntry.executable}
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
                  </div>
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
          {providersError ? <p className="rounded bg-destructive/10 p-3 text-sm text-destructive">{providersError}</p> : null}
        </div>
      )}
      <CustomProviderDialog
        open={customDialog.open}
        editing={customDialog.editing}
        preset={customDialog.preset}
        onOpenChange={(open) => setCustomDialog({ open, editing: open ? customDialog.editing : null, preset: open ? customDialog.preset : null })}
        onSave={saveCustomProvider}
      />
    </>
  );
}
