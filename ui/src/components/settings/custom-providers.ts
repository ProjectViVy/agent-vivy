import { useSyncExternalStore } from 'react';
import {
  FOLDED_PROVIDER_NAMES,
  PROVIDER_CATALOG,
  type ProviderCatalogEntry,
  type ProviderRuntimeBundle,
} from './provider-catalog';

/**
 * 自定义供应商注册表（Agent-Diva custom provider CRUD 的 vivy 适配）。
 *
 * - 唯一存储于 localStorage key `vivy.ui.customProviders`（真实功能，禁用 vivy.demo.*）。
 * - 自定义供应商保存显示名 + 运行束 + Base URL + 默认模型 + 模型列表 + 可选
 *   API Key（本地副本 vivy.ui.*；应用时随 settings/update 提交，后端落
 *   data/agent-home/settings.yaml，不回传界面、不打日志）。
 * - 与 saved-models.ts 同款持久化样板：模块级缓存 + useSyncExternalStore +
 *   自定义事件 / storage 事件广播，不进 zustand store。
 * - 注册表只改本地结构；点模型/选模型等运行配置变更仍走 ModelSettingsCard 的
 *   既有 saveSettings 路径。
 */

export const CUSTOM_PROVIDERS_KEY = 'vivy.ui.customProviders';

const CUSTOM_PROVIDERS_CHANGED_EVENT = 'vivy.ui.customProviders.changed';

export type CustomProvider = {
  /** 内部稳定主键（自动生成），重命名/编辑不改变；快捷列表经 baseUrl 关联，不引用 id */
  id: string;
  displayName: string;
  /** Vivy 后端 settings 只接受 openai / anthropic；mock 为内置离线束，不可自定义 */
  bundle: ProviderRuntimeBundle;
  baseUrl: string;
  defaultModel: string;
  models: string[];
  /** 可选 API Key（本地副本）；应用时提交给 settings/update，空串=清除覆盖层 */
  apiKey: string;
};

export type CustomProviderInput = Pick<CustomProvider, 'displayName' | 'bundle' | 'baseUrl' | 'defaultModel' | 'models' | 'apiKey'>;

/** 读侧形态：apiKey 可选——兼容字段引入前已保存的旧条目。 */
type StoredCustomProvider = Omit<CustomProvider, 'apiKey'> & { apiKey?: string };

/** 逐条校验：坏数据（缺字段/空白显示名/非法束名/模型列表非字符串数组）整条丢弃。 */
function isValidCustomProvider(value: unknown): value is StoredCustomProvider {
  if (typeof value !== 'object' || value === null) return false;
  const entry = value as Record<string, unknown>;
  if (typeof entry.id !== 'string' || entry.id.trim().length === 0) return false;
  if (typeof entry.displayName !== 'string' || entry.displayName.trim().length === 0) return false;
  if (entry.bundle !== 'openai' && entry.bundle !== 'anthropic') return false;
  if (typeof entry.baseUrl !== 'string' || entry.baseUrl.trim().length === 0) return false;
  if (typeof entry.defaultModel !== 'string') return false;
  if (!Array.isArray(entry.models) || !entry.models.every((model) => typeof model === 'string')) return false;
  if (entry.apiKey !== undefined && typeof entry.apiKey !== 'string') return false;
  return true;
}

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

/** 目录或既有自定义条目中已存在同 (bundle, baseUrl)（编辑时排除自身）。 */
function hasBaseUrlConflict(input: CustomProviderInput, selfId?: string): boolean {
  const customConflict = readCustomProviders().some(
    (entry) => entry.id !== selfId && entry.bundle === input.bundle && entry.baseUrl === input.baseUrl,
  );
  const catalogConflict = PROVIDER_CATALOG.some(
    (entry) => entry.bundle === input.bundle && entry.baseUrl === input.baseUrl,
  );
  return customConflict || catalogConflict;
}

let cachedRaw: string | null = null;
let cachedCustomProviders: CustomProvider[] = [];

function readCustomProviders(): CustomProvider[] {
  if (typeof window === 'undefined') return [];
  let raw: string | null = null;
  try {
    raw = window.localStorage.getItem(CUSTOM_PROVIDERS_KEY);
  } catch {
    // Local UI preference is best effort.
    return cachedCustomProviders;
  }
  if (raw === cachedRaw) return cachedCustomProviders;
  cachedRaw = raw;
  if (!raw) {
    cachedCustomProviders = [];
    return cachedCustomProviders;
  }
  try {
    const parsed: unknown = JSON.parse(raw);
    cachedCustomProviders = Array.isArray(parsed)
      ? parsed.filter(isValidCustomProvider).map((entry) => ({ ...entry, apiKey: entry.apiKey ?? '' }))
      : [];
  } catch {
    cachedCustomProviders = [];
  }
  return cachedCustomProviders;
}

function writeCustomProviders(next: CustomProvider[]): void {
  if (typeof window === 'undefined') return;
  cachedCustomProviders = next;
  cachedRaw = JSON.stringify(next);
  try {
    window.localStorage.setItem(CUSTOM_PROVIDERS_KEY, cachedRaw);
  } catch {
    // Local UI preference is best effort.
  }
  window.dispatchEvent(new Event(CUSTOM_PROVIDERS_CHANGED_EVENT));
}

export function getCustomProviders(): CustomProvider[] {
  return readCustomProviders();
}

/** 新增（尾部追加）；返回新条目，`(bundle, baseUrl)` 冲突时返回 null。 */
export function addCustomProvider(input: CustomProviderInput): CustomProvider | null {
  const normalized = normalizeInput(input);
  if (hasBaseUrlConflict(normalized)) return null;
  const entry: CustomProvider = { id: newCustomProviderId(), ...normalized };
  writeCustomProviders([...readCustomProviders(), entry]);
  return entry;
}

/** 按 id 编辑（改显示名/模型列表等，id 不变）；不存在或冲突时返回 false。 */
export function updateCustomProvider(id: string, input: CustomProviderInput): boolean {
  const current = readCustomProviders();
  const target = current.find((entry) => entry.id === id);
  if (!target) return false;
  const normalized = normalizeInput(input);
  if (hasBaseUrlConflict(normalized, id)) return false;
  writeCustomProviders(current.map((entry) => (entry.id === id ? { ...entry, ...normalized } : entry)));
  return true;
}

/** 按 id 移出注册表：已保存到快捷列表的书签与运行配置不受影响。 */
export function removeCustomProvider(id: string): void {
  const current = readCustomProviders();
  const next = current.filter((entry) => entry.id !== id);
  if (next.length === current.length) return;
  writeCustomProviders(next);
}

function subscribeToCustomProviders(onChange: () => void): () => void {
  if (typeof window === 'undefined') return () => undefined;
  window.addEventListener(CUSTOM_PROVIDERS_CHANGED_EVENT, onChange);
  window.addEventListener('storage', onChange);
  return () => {
    window.removeEventListener(CUSTOM_PROVIDERS_CHANGED_EVENT, onChange);
    window.removeEventListener('storage', onChange);
  };
}

export function useCustomProviders(): CustomProvider[] {
  return useSyncExternalStore(subscribeToCustomProviders, getCustomProviders, () => []);
}

/** 面板/选择器用的合并条目：目录条目 + 自定义条目（后者带 custom 标记）。 */
export type MergedProviderEntry = ProviderCatalogEntry & {
  custom: boolean;
  registryId?: string;
};

function toMerged(custom: CustomProvider): MergedProviderEntry {
  return {
    // 用注册表 id 作目录 name，避免与静态目录 name 撞车（key/检索/命中都稳定）。
    name: custom.id,
    displayName: custom.displayName,
    bundle: custom.bundle,
    baseUrl: custom.baseUrl,
    defaultModel: custom.defaultModel,
    models: custom.models,
    custom: true,
    registryId: custom.id,
  };
}

/** 目录在前、自定义在后。 */
export function allProviderEntries(): MergedProviderEntry[] {
  return [
    ...PROVIDER_CATALOG.map((entry) => ({ ...entry, custom: false })),
    ...readCustomProviders().map(toMerged),
  ];
}

/** 按 displayName / name 检索合并集（大小写不敏感）；空串返回全部。 */
export function searchMergedProviders(term: string): MergedProviderEntry[] {
  const needle = term.trim().toLowerCase();
  if (!needle) return allProviderEntries();
  return allProviderEntries().filter(
    (entry) => entry.displayName.toLowerCase().includes(needle) || entry.name.toLowerCase().includes(needle),
  );
}

/** 与 provider-catalog.matchProviderEntry 同口径，但命中范围含自定义条目（目录优先）。 */
export function matchMergedProviderEntry(bundle: string, baseUrl: string): MergedProviderEntry | undefined {
  const base = baseUrl.trim();
  if (base) {
    const catalog = PROVIDER_CATALOG.find((entry) => entry.bundle === bundle && entry.baseUrl === base);
    if (catalog) return { ...catalog, custom: false };
    const custom = readCustomProviders().find((entry) => entry.bundle === bundle && entry.baseUrl === base);
    return custom ? toMerged(custom) : undefined;
  }
  const catalog = PROVIDER_CATALOG.find((entry) => entry.name === bundle && entry.bundle === bundle);
  return catalog ? { ...catalog, custom: false } : undefined;
}

/** 目标三元组命中的自定义供应商 API Key：未命中或目录条目返回 ''（应用时随 settings/update 提交）。 */
export function customApiKeyFor(bundle: string, baseUrl: string): string {
  const merged = matchMergedProviderEntry(bundle, baseUrl);
  if (!merged?.custom || !merged.registryId) return '';
  return readCustomProviders().find((entry) => entry.id === merged.registryId)?.apiKey ?? '';
}

export type MergedFoldGroups = {
  visible: MergedProviderEntry[];
  custom: MergedProviderEntry[];
  more: MergedProviderEntry[];
};

/**
 * 非搜索态分组：目录可见 / 自定义（永不折叠）/ 目录折叠名单；搜索态平铺
 * （自定义直接混入 visible，more 恒空，调用方用 custom 标记区分）。
 */
export function splitMergedByFold(entries: readonly MergedProviderEntry[], searching: boolean): MergedFoldGroups {
  if (searching) return { visible: entries.slice(), custom: [], more: [] };
  return {
    visible: entries.filter((entry) => !entry.custom && !FOLDED_PROVIDER_NAMES.has(entry.name)),
    custom: entries.filter((entry) => entry.custom),
    more: entries.filter((entry) => !entry.custom && FOLDED_PROVIDER_NAMES.has(entry.name)),
  };
}
