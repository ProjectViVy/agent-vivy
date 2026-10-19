import type {
  ProviderCatalogEntry,
  ProviderEntry,
  ProviderEntryInput,
  ProviderProfileStatus,
  Settings,
} from '@/lib/api';
import {
  catalogRows,
  matchCatalogRow,
  normalizeProviderValue,
  providerSelection,
  type ProviderCatalogRow,
} from '@/components/settings/provider-catalog';
import {
  catalogOverlayId,
  customApiKeySetFor,
  providerEntryByEndpoint,
} from '@/components/settings/custom-providers';

/**
 * 欢迎向导「模型」步骤的纯投影层。
 *
 * 目录、端点、模型与可执行性全部来自后端 `settings/providers`（PROV-P4），
 * 这里只做三件事：
 *  - 把目录摊平成可执行端点行（沿用 provider-catalog 的同一口径）；
 *  - 从当前 settings 预填厂商 / 协议 / 模型，不发明任何供应商数据；
 *  - 把「写-only API Key + 选中三元组」换算成现有 RPC 的载荷。
 *
 * 本模块不持有任何厂商 / 端点 / 模型 / 密钥数据，也不发请求。
 */

export type WelcomeModelDraft = {
  vendor: string;
  adapter: string;
  model: string;
};

export const EMPTY_WELCOME_MODEL: WelcomeModelDraft = { vendor: '', adapter: '', model: '' };

/** 一个目录厂商及其可执行端点（协议）变体。 */
export type WelcomeVendorGroup = {
  vendor: string;
  displayName: string;
  endpoints: ProviderCatalogRow[];
};

/** 目录行：只保留本 Generation 可构造的端点（没有可执行 Profile 的适配器不出现）。 */
export function welcomeCatalogRows(
  catalog: readonly ProviderCatalogEntry[],
  profiles?: readonly ProviderProfileStatus[],
): ProviderCatalogRow[] {
  return catalogRows(catalog, profiles ?? []).filter((row) => row.executable);
}

/** 按厂商分组，保留后端目录顺序。 */
export function welcomeVendors(rows: readonly ProviderCatalogRow[]): WelcomeVendorGroup[] {
  const groups: WelcomeVendorGroup[] = [];
  for (const row of rows) {
    let group = groups.find((candidate) => candidate.vendor === row.vendor);
    if (!group) {
      group = { vendor: row.vendor, displayName: row.displayName, endpoints: [] };
      groups.push(group);
    }
    group.endpoints.push(row);
  }
  return groups;
}

/** 端点默认模型：存下的模型不在该端点清单里时回落到目录默认值。 */
function draftFor(row: ProviderCatalogRow, model: string | undefined): WelcomeModelDraft {
  const requested = (model ?? '').trim();
  return {
    vendor: row.vendor,
    adapter: row.adapter,
    model: requested && row.models.includes(requested) ? requested : row.defaultModel,
  };
}

/**
 * 从后端的 settings 文档预填选择：
 *  1. 原始 provider 取值（兼容迁移前的运行束名）+ base_url 精确命中；
 *  2. 归一后的密封适配器 + base_url 精确命中；
 *  3. 原始 config_provider + base_url 命中；
 *  4. config_provider 的目录默认端点；
 *  5. 无法判定时返回 null（让用户自己选，不猜）。
 */
export function prefillWelcomeModel(
  rows: readonly ProviderCatalogRow[],
  settings: Settings | null,
): WelcomeModelDraft | null {
  if (!rows.length || !settings) return null;
  const baseURL = (settings.base_url ?? '').trim();
  const storedModel = settings.default_model || settings.config_model || '';
  const storedProvider = typeof settings.provider === 'string' ? settings.provider.trim() : '';

  // 已存下的 endpoint 是权威选择：带地址时只认目录精确命中，自定义网关不回落
  // 到 config 目录厂商（否则向导一保存就会把用户自建网关覆盖成目录端点）。
  if (storedProvider && baseURL) {
    const { adapter } = normalizeProviderValue(storedProvider);
    const row = rows.find((candidate) => candidate.adapter === adapter && candidate.baseUrl === baseURL);
    return row ? draftFor(row, storedModel) : null;
  }

  const values = [settings.provider, settings.config_provider]
    .map((value) => (typeof value === 'string' ? value.trim() : ''))
    .filter((value) => value.length > 0);
  for (const value of values) {
    const { adapter, legacyVendor } = normalizeProviderValue(value);
    if (legacyVendor) {
      const row =
        rows.find((candidate) => candidate.vendor === legacyVendor && candidate.adapter === adapter) ??
        rows.find((candidate) => candidate.vendor === legacyVendor);
      if (row) return draftFor(row, storedModel);
    }
    if (!baseURL) {
      const matching = rows.filter((candidate) => candidate.adapter === adapter);
      if (matching.length === 1) return draftFor(matching[0], storedModel);
    }
  }
  return null;
}

export type WelcomeKeyStatus = {
  /** 所选端点是否已有可用密钥（注册表条目，或匹配当前选择的旧 overlay）。 */
  configured: boolean;
  /** API Key 写回时应复用的注册表条目 id（没有条目时是稳定的目录占位 id）。 */
  entryId: string;
  /** 目录端点密钥落地条 id（`catalog-<vendor>-<adapter>`）。 */
  overlayId: string;
};

/** 判定所选端点的密钥状态；值本身永远不来自 wire，只读 `api_key_set`。 */
export function welcomeKeyStatus(
  rows: readonly ProviderCatalogRow[],
  providers: readonly ProviderEntry[],
  settings: Settings | null,
  row: ProviderCatalogRow,
): WelcomeKeyStatus {
  const overlayId = catalogOverlayId(row.vendor, row.adapter);
  const entry = providerEntryByEndpoint(providers, row.adapter, row.baseUrl);
  let configured = customApiKeySetFor(providers, row.adapter, row.baseUrl);
  if (!configured && settings?.api_key_set) {
    const active = matchCatalogRow(rows, settings.provider, settings.base_url);
    configured = active?.vendor === row.vendor && active.adapter === row.adapter;
  }
  return { configured, entryId: entry?.id ?? overlayId, overlayId };
}

/**
 * `settings/providers/upsert` 载荷（密钥写-only）。
 *
 * 端点已有注册表条目时沿用其 id / 显示名 / 模型清单，避免同 (bundle, base_url)
 * 生成第二条记录，也避免覆盖用户手工加过的模型；没有条目时落到稳定的
 * `catalog-<vendor>-<adapter>` 占位条（与设置页同款，不显示为自定义行）。
 */
export function welcomeProviderInput(
  row: ProviderCatalogRow,
  apiKey: string,
  existing: ProviderEntry | undefined,
): ProviderEntryInput {
  const key = apiKey.trim();
  if (existing) {
    return {
      id: existing.id,
      display_name: existing.display_name,
      bundle: row.adapter,
      base_url: row.baseUrl,
      default_model: existing.default_model || row.defaultModel,
      models: existing.models.length ? [...existing.models] : [...row.models],
      api_key: key,
    };
  }
  return {
    id: catalogOverlayId(row.vendor, row.adapter),
    display_name: row.displayName,
    bundle: row.adapter,
    base_url: row.baseUrl,
    default_model: row.defaultModel,
    models: [...row.models],
    api_key: key,
  };
}

/** 选中三元组：provider 写密封适配器 id，地址与模型来自所选端点。 */
export function welcomeSelection(row: ProviderCatalogRow, model: string) {
  return providerSelection(row, model);
}

/** 下一步是否允许保存（read-only 时只前进、不写任何东西）。 */
export function canSaveWelcomeModel(input: {
  readOnly: boolean;
  row: ProviderCatalogRow | undefined;
  model: string;
  apiKey: string;
  keyConfigured: boolean;
  envKeyConfirmed: boolean;
}): boolean {
  if (input.readOnly) return true;
  if (!input.row || !input.model.trim()) return false;
  if (input.apiKey.trim()) return true;
  return input.keyConfigured || input.envKeyConfirmed;
}
