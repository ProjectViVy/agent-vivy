import {
  FOLDED_PROVIDER_NAMES,
  PROVIDER_CATALOG,
  projectProviderEntry,
  type ProjectedProviderCatalogEntry,
  type ProviderCatalogEntry,
  type ProviderProfileStatus,
  type ProviderRuntimeBundle,
} from './provider-catalog';
import type { ProviderEntry } from '@/lib/api';

/**
 * 自定义供应商注册表的纯逻辑层（Agent-Diva custom provider CRUD 的 vivy
 * 适配，后端权威版）。
 *
 * - 注册表由后端持久化（settings/providers 系列 RPC，落 data/agent-home/
 *   settings.yaml），不再存 localStorage —— provider 写逻辑整体改为后端
 *   驱动，写入后后端同步更新对应环境变量值。
 * - 本模块只保留纯函数：校验 / 冲突 / 合并 / 折叠 / 检索 / API Key 存在性
 *   判定。数据源一律以参数传入（store 中 wire 形态 `ProviderEntry[]`），
 *   不进 zustand、不持有窗口级缓存。
 * - 密钥规则不变（D-010）：wire 只带 `api_key_set` 布尔；API Key 在提交时
 *   作为写-only 输入交给 `settings/providers/upsert`，永不回传。
 */

export const CUSTOM_PROVIDERS_KEY = 'vivy.ui.customProviders';

/** wire 形态的注册表供应商（后端返回；密钥永不在线）。 */
export type CustomProvider = ProviderEntry;

// Re-export the wire entry type so consumers import it from one place.
export type { ProviderEntry } from '@/lib/api';

/** 注册表 bundle 的 wire 取值。 */
export type ProviderRegistryBundle = 'openai' | 'anthropic';

/** 新增/编辑输入：apiKey 为写-only（空=清除该条目密钥；不参与读侧）。 */
export type CustomProviderInput = {
  displayName: string;
  bundle: ProviderRegistryBundle;
  baseUrl: string;
  defaultModel: string;
  models: string[];
  apiKey: string;
};

/** 逗号 / 中文逗号 / 换行 / 连续空白分隔，去空去重（“每行一个或用逗号分隔”录入口径）。 */
export function parseCustomModels(text: string): string[] {
  return [...new Set(text.split(/[\n,，]+/).map((model) => model.trim()).filter((model) => model.length > 0))];
}

/** 自动生成内部 id：优先 crypto.randomUUID，缺省按时间戳 + 随机数回退。 */
export function newCustomProviderId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return `custom-${crypto.randomUUID()}`;
  return `custom-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
}

function normalizeInput(input: CustomProviderInput): CustomProviderInput {
  return {
    displayName: input.displayName.trim(),
    bundle: input.bundle,
    baseUrl: input.baseUrl.trim(),
    defaultModel: input.defaultModel.trim(),
    models: parseCustomModels(input.models.join('\n')),
    apiKey: (input.apiKey ?? '').trim(),
  };
}

/** 逐条校验 wire 条目：坏数据（缺字段/空白显示名/非法束名/模型列表非字符串数组）整条丢弃。 */
export function isValidCustomProvider(value: unknown): value is ProviderEntry {
  if (typeof value !== 'object' || value === null) return false;
  const entry = value as Record<string, unknown>;
  if (typeof entry.id !== 'string' || entry.id.trim().length === 0) return false;
  if (typeof entry.display_name !== 'string' || entry.display_name.trim().length === 0) return false;
  if (entry.bundle !== 'openai' && entry.bundle !== 'anthropic') return false;
  if (typeof entry.base_url !== 'string' || entry.base_url.trim().length === 0) return false;
  if (typeof entry.default_model !== 'string') return false;
  if (!Array.isArray(entry.models) || !entry.models.every((model) => typeof model === 'string')) return false;
  if (typeof entry.api_key_set !== 'boolean') return false;
  return true;
}

/** 目录或既有注册表条目中已存在同 (bundle, base_url)（编辑时排除自身）。 */
export function hasBaseUrlConflict(
  providers: readonly ProviderEntry[],
  input: CustomProviderInput,
  selfId?: string,
): boolean {
  const registryConflict = providers.some(
    (entry) => entry.id !== selfId && entry.bundle === input.bundle && entry.base_url === input.baseUrl,
  );
  const catalogConflict = PROVIDER_CATALOG.some(
    (entry) => entry.bundle === input.bundle && entry.baseUrl === input.baseUrl,
  );
  return registryConflict || catalogConflict;
}

/** 面板/选择器用的合并条目：目录条目 + 注册表条目（后者带 custom 标记）。 */
export type MergedProviderEntry = ProjectedProviderCatalogEntry & {
  custom: boolean;
  registryId?: string;
  apiKeySet: boolean;
};

function toMerged(custom: ProviderEntry, profiles: readonly ProviderProfileStatus[]): MergedProviderEntry {
  return {
    ...projectProviderEntry({
    // 用注册表 id 作目录 name，避免与静态目录 name 撞车（key/检索/命中都稳定）。
    name: custom.id,
    displayName: custom.display_name,
    bundle: custom.bundle,
    baseUrl: custom.base_url,
    defaultModel: custom.default_model,
    models: [...custom.models],
    }, profiles),
    custom: true,
    registryId: custom.id,
    apiKeySet: custom.api_key_set,
  };
}

/** 目录在前、注册表在后；目录厂商密钥落地条（catalog-*）不出现在自定义列表。 */
export function allProviderEntries(
  providers: readonly ProviderEntry[],
  profiles: readonly ProviderProfileStatus[] = [],
): MergedProviderEntry[] {
  return [
    ...PROVIDER_CATALOG.map((entry) => ({ ...projectProviderEntry(entry, profiles), custom: false, apiKeySet: false })),
    ...providers.filter(isValidCustomProvider).filter((entry) => !isCatalogOverlayEntry(entry)).map((entry) => toMerged(entry, profiles)),
  ];
}

/** 按 displayName / name 检索合并集（大小写不敏感）；空串返回全部。 */
export function searchMergedProviders(entries: readonly MergedProviderEntry[], term: string): MergedProviderEntry[] {
  const needle = term.trim().toLowerCase();
  if (!needle) return entries.slice();
  return entries.filter(
    (entry) => entry.displayName.toLowerCase().includes(needle) || entry.name.toLowerCase().includes(needle),
  );
}

/** 与 provider-catalog.matchProviderEntry 同口径，但命中范围含注册表条目（目录优先）。 */
export function matchMergedProviderEntry(
  providers: readonly ProviderEntry[],
  bundle: string,
  baseUrl: string,
  profiles: readonly ProviderProfileStatus[] = [],
): MergedProviderEntry | undefined {
  const base = baseUrl.trim();
  if (base) {
    const catalog = PROVIDER_CATALOG.find((entry) => entry.bundle === bundle && entry.baseUrl === base);
    if (catalog) return { ...projectProviderEntry(catalog, profiles), custom: false, apiKeySet: false };
    const custom = providers.find((entry) => entry.bundle === bundle && entry.base_url === base);
    return custom ? toMerged(custom, profiles) : undefined;
  }
  const catalog = PROVIDER_CATALOG.find((entry) => entry.name === bundle && entry.bundle === bundle);
  return catalog ? { ...projectProviderEntry(catalog, profiles), custom: false, apiKeySet: false } : undefined;
}

/** 按 id 查注册表条目（编辑对话框预填/面板回显用）。 */
export function providerEntryById(providers: readonly ProviderEntry[], id: string): ProviderEntry | undefined {
  return providers.find((entry) => entry.id === id);
}

/**
 * 目录厂商密钥落地条 id 前缀。面板为目录厂商失焦写回密钥时按目录 name 生成
 * 稳定 id（`catalog-<name>`，如 catalog-deepseek），使重复失焦更新同一条目；
 * 该前缀条目不显示为自定义行（避免与目录行重复展示）。
 */
export const CATALOG_OVERLAY_PREFIX = 'catalog-';

/** 目录厂商密钥落地条 id（按目录 name 生成，稳定可重入）。 */
export function catalogOverlayId(catalogName: string): string {
  return `${CATALOG_OVERLAY_PREFIX}${catalogName}`;
}

/** 是否为目录厂商密钥落地条（面板自动生成，不在自定义列表显示）。 */
export function isCatalogOverlayEntry(entry: ProviderEntry): boolean {
  return entry.id.startsWith(CATALOG_OVERLAY_PREFIX);
}

/** 按端点 (bundle, base_url) 查注册表条目——与后端 ActiveKey 解析密钥同一口径。 */
export function providerEntryByEndpoint(
  providers: readonly ProviderEntry[],
  bundle: string,
  baseUrl: string,
): ProviderEntry | undefined {
  return providers.find((entry) => entry.bundle === bundle && entry.base_url === baseUrl);
}

/**
 * 目标端点 (bundle, base_url) 是否已有注册表密钥覆盖。目录厂商的密钥同样以
 * 注册表条目落盘（面板失焦写回，按端点命中），因此按端点判定而非 custom
 * 标记；UI 提交时不带密钥（后端权威解析：注册表条目命中即用其 key）。该
 * 布尔仅用于「已配置 API Key」提示，值永不回传。
 */
export function customApiKeySetFor(
  providers: readonly ProviderEntry[],
  bundle: string,
  baseUrl: string,
): boolean {
  const entry = providerEntryByEndpoint(providers, bundle, baseUrl);
  return !!entry && entry.api_key_set;
}

export type MergedFoldGroups = {
  visible: MergedProviderEntry[];
  custom: MergedProviderEntry[];
  more: MergedProviderEntry[];
};

/**
 * 非搜索态分组：目录可见 / 注册表（永不折叠）/ 目录折叠名单；搜索态平铺
 * （注册表直接混入 visible，more 恒空，调用方用 custom 标记区分）。
 */
export function splitMergedByFold(entries: readonly MergedProviderEntry[], searching: boolean): MergedFoldGroups {
  if (searching) return { visible: entries.slice(), custom: [], more: [] };
  return {
    visible: entries.filter((entry) => !entry.custom && !FOLDED_PROVIDER_NAMES.has(entry.name)),
    custom: entries.filter((entry) => entry.custom),
    more: entries.filter((entry) => !entry.custom && FOLDED_PROVIDER_NAMES.has(entry.name)),
  };
}
