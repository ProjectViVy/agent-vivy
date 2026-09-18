import {
  catalogRows,
  isProviderValue,
  matchCatalogRow,
  normalizeProviderAdapter,
  providerEntryRow,
  type ProviderCatalogRow,
  type ProviderProfileStatus,
} from './provider-catalog';
import type { ProviderCatalogEntry, ProviderEntry, ProviderValue } from '@/lib/api';

/**
 * 自定义供应商注册表的纯逻辑层（后端权威版）。
 *
 * - 注册表由后端持久化（settings/providers 系列 RPC，落 data/agent-home/
 *   settings.yaml），不再存 localStorage —— provider 写逻辑整体改为后端
 *   驱动，写入后后端同步更新对应环境变量值。
 * - 本模块只保留纯函数：校验 / 冲突 / 合并 / 检索 / API Key 存在性判定。
 *   数据源一律以参数传入（目录来自 store 的 wire `ProviderCatalogEntry[]`，
 *   注册表来自 wire `ProviderEntry[]`），不进 zustand、不持有窗口级缓存。
 * - 词汇与后端一致：合并行按**端点**展开（厂商 + 适配器变体），provider /
 *   bundle 一律是密封适配器 id，旧文档的运行束名由归一函数兼容读取。
 * - 密钥规则不变（D-010）：wire 只带 `api_key_set` 布尔；API Key 在提交时
 *   作为写-only 输入交给 `settings/providers/upsert`，永不回传。
 */

export const CUSTOM_PROVIDERS_KEY = 'vivy.ui.customProviders';

/** wire 形态的注册表供应商（后端返回；密钥永不在线）。 */
export type CustomProvider = ProviderEntry;

// Re-export the wire entry type so consumers import it from one place.
export type { ProviderEntry } from '@/lib/api';

/** 注册表 bundle 的 wire 取值：密封适配器 id（旧文档的运行束名同样被后端接受）。 */
export type ProviderRegistryBundle = ProviderValue;

/** 注册表条目 bundle 合法性判定（单一口径，避免各处各写一份白名单）。 */
export function isProviderRegistryBundle(value: unknown): value is ProviderRegistryBundle {
  return isProviderValue(value);
}

/**
 * 支持「刷新模型列表」（上游 GET /models）的唯一规则：适配器是
 * openai-completions 且 base_url 为 http(s)。与后端
 * settings.IsOpenAICompatibleSelection（同样是适配器判定，且归一旧运行束名）
 * 加 control.go 的 http(s) 门控完全一致——两侧读同一个适配器属性，不存在
 * 各自维护的厂商白名单。
 */
export function supportsModelRefresh(adapter: string, baseUrl = ''): boolean {
  if (normalizeProviderAdapter(adapter) !== 'openai-completions') return false;
  const base = baseUrl.trim();
  return base.startsWith('http://') || base.startsWith('https://');
}

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

/** 逐条校验 wire 条目：坏数据（缺字段/空白显示名/非法 provider 值/模型列表非字符串数组）整条丢弃。 */
export function isValidCustomProvider(value: unknown): value is ProviderEntry {
  if (typeof value !== 'object' || value === null) return false;
  const entry = value as Record<string, unknown>;
  if (typeof entry.id !== 'string' || entry.id.trim().length === 0) return false;
  if (typeof entry.display_name !== 'string' || entry.display_name.trim().length === 0) return false;
  if (!isProviderRegistryBundle(entry.bundle)) return false;
  if (typeof entry.base_url !== 'string' || entry.base_url.trim().length === 0) return false;
  if (typeof entry.default_model !== 'string') return false;
  if (!Array.isArray(entry.models) || !entry.models.every((model) => typeof model === 'string')) return false;
  if (typeof entry.api_key_set !== 'boolean') return false;
  return true;
}

/** 目录（按端点）或既有注册表条目中已存在同 (adapter, base_url)（编辑时排除自身）。 */
export function hasBaseUrlConflict(
  catalog: readonly ProviderCatalogEntry[],
  providers: readonly ProviderEntry[],
  input: CustomProviderInput,
  selfId?: string,
): boolean {
  const adapter = normalizeProviderAdapter(input.bundle);
  const registryConflict = providers.some(
    (entry) => entry.id !== selfId && normalizeProviderAdapter(entry.bundle) === adapter && entry.base_url === input.baseUrl,
  );
  const catalogConflict = catalogRows(catalog).some(
    (row) => row.adapter === adapter && row.baseUrl === input.baseUrl,
  );
  return registryConflict || catalogConflict;
}

/** 面板/选择器用的合并条目：目录端点行 + 注册表行（后者带 custom 标记）。 */
export type MergedProviderEntry = ProviderCatalogRow & {
  custom: boolean;
  registryId?: string;
  apiKeySet: boolean;
};

function toMerged(custom: ProviderEntry, profiles: readonly ProviderProfileStatus[]): MergedProviderEntry {
  return {
    ...providerEntryRow(custom, profiles),
    custom: true,
    registryId: custom.id,
    apiKeySet: custom.api_key_set,
  };
}

/** 目录端点在前（每端点一行）、注册表在后；目录厂商密钥落地条（catalog-*）不出现在自定义列表。 */
export function allProviderEntries(
  catalog: readonly ProviderCatalogEntry[],
  providers: readonly ProviderEntry[],
  profiles: readonly ProviderProfileStatus[] = [],
): MergedProviderEntry[] {
  return [
    ...catalogRows(catalog, profiles).map((row) => ({ ...row, custom: false, apiKeySet: false })),
    ...providers.filter(isValidCustomProvider).filter((entry) => !isCatalogOverlayEntry(entry)).map((entry) => toMerged(entry, profiles)),
  ];
}

/** 按 displayName / vendor 检索合并集（大小写不敏感）；空串返回全部。 */
export function searchMergedProviders(entries: readonly MergedProviderEntry[], term: string): MergedProviderEntry[] {
  const needle = term.trim().toLowerCase();
  if (!needle) return entries.slice();
  return entries.filter(
    (entry) => entry.displayName.toLowerCase().includes(needle) || entry.vendor.toLowerCase().includes(needle),
  );
}

/** 目录优先命中的合并反查：目录端点在前，注册表条目（含自定义克隆）兜底。 */
export function matchMergedProviderEntry(
  catalog: readonly ProviderCatalogEntry[],
  providers: readonly ProviderEntry[],
  providerValue: string,
  baseUrl: string,
  profiles: readonly ProviderProfileStatus[] = [],
): MergedProviderEntry | undefined {
  const row = matchCatalogRow(catalogRows(catalog, profiles), providerValue, baseUrl);
  if (row) return { ...row, custom: false, apiKeySet: false };
  if (!baseUrl.trim()) return undefined;
  const custom = providerEntryByEndpoint(providers, providerValue, baseUrl);
  return custom ? toMerged(custom, profiles) : undefined;
}

/** 按 id 查注册表条目（编辑对话框预填/面板回显用）。 */
export function providerEntryById(providers: readonly ProviderEntry[], id: string): ProviderEntry | undefined {
  return providers.find((entry) => entry.id === id);
}

/**
 * 目录厂商密钥落地条 id 前缀。面板为目录端点失焦写回密钥时按 (vendor, adapter)
 * 生成稳定 id（`catalog-<vendor>-<adapter>`），使重复失焦更新同一条目，且同一
 * 厂商的两个端点各落一条；该前缀条目不显示为自定义行（避免与目录行重复展示）。
 */
export const CATALOG_OVERLAY_PREFIX = 'catalog-';

/** 目录端点密钥落地条 id（按厂商 + 适配器生成，稳定可重入）。 */
export function catalogOverlayId(vendor: string, adapter: string): string {
  return `${CATALOG_OVERLAY_PREFIX}${vendor}-${adapter}`;
}

/** 是否为目录厂商密钥落地条（面板自动生成，不在自定义列表显示）。 */
export function isCatalogOverlayEntry(entry: ProviderEntry): boolean {
  return entry.id.startsWith(CATALOG_OVERLAY_PREFIX);
}

/** 按端点 (adapter, base_url) 查注册表条目——与后端 Settings.FindProvider 同一口径（两侧都归一旧运行束名）。 */
export function providerEntryByEndpoint(
  providers: readonly ProviderEntry[],
  adapter: string,
  baseUrl: string,
): ProviderEntry | undefined {
  const target = normalizeProviderAdapter(adapter);
  return providers.find(
    (entry) => normalizeProviderAdapter(entry.bundle) === target && entry.base_url === baseUrl,
  );
}

/**
 * 目标端点 (adapter, base_url) 是否已有注册表密钥覆盖。目录厂商的密钥同样以
 * 注册表条目落盘（面板失焦写回，按端点命中），因此按端点判定而非 custom
 * 标记；UI 提交时不带密钥（后端权威解析：注册表条目命中即用其 key）。该
 * 布尔仅用于「已配置 API Key」提示，值永不回传。
 */
export function customApiKeySetFor(
  providers: readonly ProviderEntry[],
  adapter: string,
  baseUrl: string,
): boolean {
  const entry = providerEntryByEndpoint(providers, adapter, baseUrl);
  return !!entry && entry.api_key_set;
}