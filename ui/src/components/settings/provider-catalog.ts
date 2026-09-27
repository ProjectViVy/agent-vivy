import type {
  ProviderAdapterId,
  ProviderAdapterState,
  ProviderCapabilityState,
  ProviderCatalogEntry,
  ProviderEntry,
  ProviderProfileStatus,
  ProviderValue,
} from '@/lib/api';

export type { ProviderCapabilityState, ProviderProfileStatus } from '@/lib/api';

/**
 * 供应商目录的纯投影层。目录、端点变体、模型列表与可执行性全部来自后端
 * `settings/providers`（PROV-P4）；本模块不持有任何厂商 / 端点 / 模型数据。
 *
 * - 一行 = 一个目录端点（厂商 + 协议适配器变体）。DeepSeek 因此贡献两行
 *   （openai-completions / anthropic-messages），共用同一个 display_name。
 * - 可执行性 = 端点自带能力 与 编译后 Profile 状态 的合取；Profile 以适配器
 *   id 为键，与后端 profileSelectionError 同一口径（`EXECUTABLE_STATES`）。
 * - 选择三元组 (adapter, base_url, default_model)：provider 是适配器 id，
 *   旧文档的运行束名只在读取侧归一（normalizeProviderValue）。
 */

/** 可执行的 Profile 状态（与后端 modelhost.ProfileState 同口径）。 */
export const EXECUTABLE_STATES: ReadonlySet<ProviderCapabilityState> = new Set([
  'COMPILED', 'UNCONFIGURED', 'READY',
]);

/**
 * 迁移前的运行束名 -> 密封适配器（MIGRATION.md §3），与后端
 * settings.NormalizeAdapter / internal/app/settings/provider_migration.go 的
 * 别名表逐条对应。这是**读取兼容层**：写侧一律写适配器 id。
 */
const LEGACY_ADAPTER_ALIASES: Readonly<Record<string, ProviderAdapterId>> = {
  deepseek: 'openai-completions',
  openai: 'openai-completions',
  anthropic: 'anthropic-messages',
};

/** 一个 provider / bundle 取值归一后的形态。 */
export type NormalizedProviderValue = {
  /** 密封适配器 id；无法识别的取值原样返回（由后端在写侧拒绝）。 */
  adapter: string;
  /** 旧文档命名的厂商（如 'deepseek'）；已经是适配器 id 时为空串。 */
  legacyVendor: string;
};

/** 把 wire 上的 provider / bundle 取值归一为适配器词汇（与后端同口径）。 */
export function normalizeProviderValue(value: string): NormalizedProviderValue {
  const adapter = LEGACY_ADAPTER_ALIASES[value];
  return adapter ? { adapter, legacyVendor: value } : { adapter: value, legacyVendor: '' };
}

/** 只取适配器的归一（后端 settings.NormalizeAdapter 同口径）。 */
export function normalizeProviderAdapter(value: string): string {
  return normalizeProviderValue(value).adapter;
}

/** 是否为密封适配器 id（写侧合法取值）。 */
export function isProviderAdapter(value: unknown): value is ProviderAdapterId {
  return value === 'openai-completions' || value === 'openai-responses' || value === 'anthropic-messages';
}

/** 是否为迁移前的运行束名（旧文档 / 旧 localStorage 条目可能持有）。 */
export function isLegacyProviderValue(value: unknown): boolean {
  return typeof value === 'string' && Object.hasOwn(LEGACY_ADAPTER_ALIASES, value);
}

/** provider / bundle 的合法取值：密封适配器 id 或迁移前的运行束名。 */
export function isProviderValue(value: unknown): value is ProviderValue {
  return isProviderAdapter(value) || isLegacyProviderValue(value);
}

/** 目录 / 注册表合并后的一行：一个端点变体的可渲染投影。 */
export type ProviderCatalogRow = {
  /** 目录厂商 id（如 'deepseek'）；注册表行用注册表 id 占位（检索与行 key 稳定）。 */
  vendor: string;
  displayName: string;
  /** 协议适配器 id（写侧选择值；注册表旧条目可能是运行束名）。 */
  adapter: ProviderValue;
  /** 端点地址；注册表行保留自己的 base_url。 */
  baseUrl: string;
  defaultModel: string;
  models: string[];
  /** Profile 叠加后的能力状态。 */
  capabilityState?: ProviderCapabilityState;
  executable: boolean;
};

/** 行身份：(vendor, adapter) 在目录里唯一（后端 EndpointForAdapter 每厂商每适配器至多一条）。 */
export function providerRowKey(row: Pick<ProviderCatalogRow, 'vendor' | 'adapter'>): string {
  return `${row.vendor}/${row.adapter}`;
}

/** 适配器对应的编译 Profile 状态；无该 Profile 时为空。 */
function profileState(
  adapter: string,
  profiles: readonly ProviderProfileStatus[] | undefined,
): ProviderCapabilityState | undefined {
  if (!profiles?.length) return undefined;
  const target = normalizeProviderAdapter(adapter);
  return profiles.find((candidate) => normalizeProviderAdapter(candidate.id) === target)?.state;
}

/** 适配器是否可执行：无 Profile 列表时按端点自带能力判定（兼容旧后端）。 */
export function isProviderExecutable(
  adapter: string,
  profiles: readonly ProviderProfileStatus[] | undefined,
): boolean {
  if (!profiles?.length) return true;
  const state = profileState(adapter, profiles);
  return !!state && EXECUTABLE_STATES.has(state);
}

/** 端点自带状态里只有 deferred 对「能力徽标」有意义（SUPPORTED = 无徽标）。 */
function declaredCapability(state: ProviderAdapterState | undefined): ProviderCapabilityState | undefined {
  return state === 'DEFERRED-INDEFINITE' ? state : undefined;
}

/**
 * 投影一行：`declared` 是端点自带的执行性（注册表条目没有，目录端点有），
 * Profile 叠加始终参与判定，deferred 端点因此永远不可选。
 */
export function projectProviderRow(
  row: Omit<ProviderCatalogRow, 'capabilityState' | 'executable'>,
  declared: { executable?: boolean; state?: ProviderAdapterState } = {},
  profiles: readonly ProviderProfileStatus[] = [],
): ProviderCatalogRow {
  return {
    ...row,
    models: [...row.models],
    capabilityState: profileState(row.adapter, profiles) ?? declaredCapability(declared.state),
    executable: (declared.executable ?? true) && isProviderExecutable(row.adapter, profiles),
  };
}

/** 把后端目录摊平成行：每个厂商的每个端点一行。 */
export function catalogRows(
  catalog: readonly ProviderCatalogEntry[],
  profiles: readonly ProviderProfileStatus[] = [],
): ProviderCatalogRow[] {
  return catalog.flatMap((entry) =>
    entry.endpoints.map((endpoint) => projectProviderRow({
      vendor: entry.vendor,
      displayName: entry.display_name,
      adapter: endpoint.adapter,
      baseUrl: endpoint.base_url,
      defaultModel: endpoint.default_model,
      models: endpoint.models,
    }, { executable: endpoint.executable, state: endpoint.state }, profiles)),
  );
}

/**
 * 按 (provider 取值, base_url) 反查目录行：
 * - 带地址：适配器 + base_url 精确命中；
 * - 不带地址：旧文档的运行束名回落到同名厂商在该适配器下的端点（后端
 *   NormalizeProviderSelection(...).LegacyVendor 同口径）。
 */
export function matchCatalogRow(
  rows: readonly ProviderCatalogRow[],
  providerValue: string,
  baseUrl: string,
): ProviderCatalogRow | undefined {
  const { adapter, legacyVendor } = normalizeProviderValue(providerValue.trim());
  const base = baseUrl.trim();
  if (base) return rows.find((row) => row.adapter === adapter && row.baseUrl === base);
  if (!legacyVendor) return undefined;
  return rows.find((row) => row.vendor === legacyVendor && row.adapter === adapter);
}

/** 注册表条目投影成行（写侧取值原样保留，读侧由归一函数统一口径）。 */
export function providerEntryRow(
  entry: ProviderEntry,
  profiles: readonly ProviderProfileStatus[] = [],
): ProviderCatalogRow {
  return projectProviderRow({
    // 用注册表 id 作 vendor，避免与目录厂商 id 撞车（检索/行 key 都稳定）。
    vendor: entry.id,
    displayName: entry.display_name,
    adapter: entry.bundle,
    baseUrl: entry.base_url,
    defaultModel: entry.default_model,
    models: entry.models,
  }, {}, profiles);
}

/** 仅对可执行的端点生成设置选择三元组：provider = 适配器 id，base_url 为该端点地址。 */
export function providerSelection(row: ProviderCatalogRow, model: string) {
  if (!row.executable) return undefined;
  return { provider: row.adapter, base_url: row.baseUrl, default_model: model };
}