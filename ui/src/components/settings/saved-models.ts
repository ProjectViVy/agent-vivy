import { useSyncExternalStore } from 'react';
import { matchMergedProviderEntry, type ProviderEntry } from './custom-providers';

/**
 * 「已选模型」快捷切换列表（Agent-Diva savedModels 移植）。
 *
 * - 唯一存储于 localStorage key `vivy.ui.savedModels`（真实功能，禁用 vivy.demo.*）。
 * - 模型是运行三元组 (provider, base_url, model) 的扁平数组：无密钥、无冗余 displayName，
 *   显示名渲染时经 savedModelVendorLabel / matchMergedProviderEntry 解析，单一权威来源。
 * - 与 mask-catalog.ts 同款持久化样板：模块级缓存 + useSyncExternalStore +
 *   自定义事件 / storage 事件广播，不进 zustand store。
 * - 移除只动本地列表，不会改写运行配置（不移植 Agent-Diva 的移除即清理副作用）。
 */

export const SAVED_MODELS_KEY = 'vivy.ui.savedModels';

const SAVED_MODELS_CHANGED_EVENT = 'vivy.ui.savedModels.changed';

export type SavedModelEntry = {
  /** Vivy 运行束名（保存到 settings.provider 的值，如 openai/anthropic/mock） */
  provider: string;
  /** OpenAI 兼容网关地址；空使用运行束默认地址 */
  baseUrl: string;
  /** 原始模型 id（不携带网关前缀） */
  model: string;
};

/** 逐条校验：坏数据（缺字段/非字符串/空白）整条丢弃。 */
function isValidEntry(value: unknown): value is SavedModelEntry {
  if (typeof value !== 'object' || value === null) return false;
  const entry = value as Record<string, unknown>;
  return (
    typeof entry.provider === 'string' &&
    entry.provider.trim().length > 0 &&
    typeof entry.baseUrl === 'string' &&
    typeof entry.model === 'string' &&
    entry.model.trim().length > 0
  );
}

/** 缓存原始字符串与解析结果：相同 raw 返回同一数组引用，保证 useSyncExternalStore 快照稳定。 */
let cachedRaw: string | null = null;
let cachedSavedModels: SavedModelEntry[] = [];

function readSavedModels(): SavedModelEntry[] {
  if (typeof window === 'undefined') return [];
  let raw: string | null = null;
  try {
    raw = window.localStorage.getItem(SAVED_MODELS_KEY);
  } catch {
    // Local UI preference is best effort.
    return cachedSavedModels;
  }
  if (raw === cachedRaw) return cachedSavedModels;
  cachedRaw = raw;
  if (!raw) {
    cachedSavedModels = [];
    return cachedSavedModels;
  }
  try {
    const parsed: unknown = JSON.parse(raw);
    cachedSavedModels = Array.isArray(parsed) ? parsed.filter(isValidEntry) : [];
  } catch {
    cachedSavedModels = [];
  }
  return cachedSavedModels;
}

function writeSavedModels(next: SavedModelEntry[]): void {
  if (typeof window === 'undefined') return;
  cachedSavedModels = next;
  cachedRaw = JSON.stringify(next);
  try {
    window.localStorage.setItem(SAVED_MODELS_KEY, cachedRaw);
  } catch {
    // Local UI preference is best effort.
  }
  window.dispatchEvent(new Event(SAVED_MODELS_CHANGED_EVENT));
}

export function getSavedModels(): SavedModelEntry[] {
  return readSavedModels();
}

/** 按 (provider, base_url, model) 三元组去重：已存在则幂等跳过，否则尾部追加（无上限，与 diva 一致）。 */
export function addSavedModel(entry: SavedModelEntry): void {
  const current = readSavedModels();
  const exists = current.some(
    (item) => item.provider === entry.provider && item.baseUrl === entry.baseUrl && item.model === entry.model,
  );
  if (exists) return;
  writeSavedModels([...current, entry]);
}

/** 按三元组过滤移除；未命中时列表不变。 */
export function removeSavedModel(provider: string, baseUrl: string, model: string): void {
  const current = readSavedModels();
  const next = current.filter(
    (item) => !(item.provider === provider && item.baseUrl === baseUrl && item.model === model),
  );
  if (next.length === current.length) return;
  writeSavedModels(next);
}

/**
 * 快捷列表条目的厂商标签：
 * 目录/自定义注册表命中 → displayName；否则带 Base URL → 主机名（含端口）；
 * 再否则回退原始 provider 束名。标签经 baseUrl 关联，注册表重命名即全局生效。
 * providers 为后端权威的注册表快照（wire 形态，无密钥）。
 */
export function savedModelVendorLabel(entry: SavedModelEntry, providers: readonly ProviderEntry[]): string {
  const merged = matchMergedProviderEntry(providers, entry.provider, entry.baseUrl);
  if (merged) return merged.displayName;
  if (entry.baseUrl) {
    try {
      const host = new URL(entry.baseUrl).host;
      if (host) return host;
    } catch {
      // 非法 URL 回退到原始 provider。
    }
  }
  return entry.provider;
}

function subscribeToSavedModels(onChange: () => void): () => void {
  if (typeof window === 'undefined') return () => undefined;
  window.addEventListener(SAVED_MODELS_CHANGED_EVENT, onChange);
  window.addEventListener('storage', onChange);
  return () => {
    window.removeEventListener(SAVED_MODELS_CHANGED_EVENT, onChange);
    window.removeEventListener('storage', onChange);
  };
}

/** 设置页与顶栏同源消费；跨组件/跨标签页变更即时同步。 */
export function useSavedModels(): SavedModelEntry[] {
  return useSyncExternalStore(subscribeToSavedModels, getSavedModels, () => []);
}
