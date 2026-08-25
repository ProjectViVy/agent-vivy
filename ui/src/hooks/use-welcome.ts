import { useSyncExternalStore } from 'react';

// 首次使用引导单一入口：完成标记持久化（localStorage）与打开状态广播。
// 与 hooks/use-theme.ts、i18n/index.ts 同构：模块级状态 + useSyncExternalStore，
// 不进 vivy store（向导属于一次性 UI 引导，不是运行时领域状态）。
const STORAGE_KEY = 'vivy.ui.welcome.completed';

function readCompleted(): boolean {
  try {
    return window.localStorage.getItem(STORAGE_KEY) === '1';
  } catch {
    // storage 不可用（隐私模式等）时视为未完成，向导仍可展示一次
    return false;
  }
}

/** 用户是否已完成（或跳过）欢迎向导；只读查询，不订阅。 */
export function isWelcomeCompleted(): boolean {
  return readCompleted();
}

/** 持久化完成标记。写入失败不影响本次会话关闭向导。 */
export function markWelcomeCompleted(): void {
  try {
    window.localStorage.setItem(STORAGE_KEY, '1');
  } catch {
    /* 标记失败时按已关闭处理，下次进入会再引导一次 */
  }
}

let open = false;
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of listeners) listener();
}

/** 打开向导（首次进入自动触发，或从设置页手动重跑）。 */
export function openWelcome(): void {
  if (open) return;
  open = true;
  emit();
}

/** 结束向导：写入完成标记并关闭。跳过与完成共用同一出口。 */
export function completeWelcome(): void {
  markWelcomeCompleted();
  if (!open) return;
  open = false;
  emit();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function getSnapshot(): boolean {
  return open;
}

/** 向导是否打开；仅在打开状态变化时触发重渲染。 */
export function useWelcomeOpen(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, () => false);
}

/** 仅供测试：重置模块内打开状态，不触碰 localStorage。 */
export function resetWelcomeForTests(): void {
  open = false;
}
