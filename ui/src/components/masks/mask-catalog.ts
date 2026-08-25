import { useSyncExternalStore } from 'react';
import { BookOpenCheck, Code2, PenLine, Star } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { t } from '@/i18n';

export const ACTIVE_MASK_KEY = 'vivy.ui.activeMask';

const ACTIVE_MASK_CHANGED_EVENT = 'vivy.ui.activeMask.changed';

export type MaskOption = {
  id: string;
  name: string;
  description: string;
  capabilities: string[];
  Icon: LucideIcon;
  iconClassName: string;
};

type MaskEntry = {
  id: string;
  Icon: LucideIcon;
  iconClassName: string;
};

const MASK_ENTRIES: MaskEntry[] = [
  { id: 'default', Icon: Star, iconClassName: 'bg-amber-100 text-amber-600 dark:bg-amber-500/15 dark:text-amber-300' },
  { id: 'programmer', Icon: Code2, iconClassName: 'bg-blue-100 text-blue-600 dark:bg-blue-500/15 dark:text-blue-300' },
  { id: 'researcher', Icon: BookOpenCheck, iconClassName: 'bg-emerald-100 text-emerald-600 dark:bg-emerald-500/15 dark:text-emerald-300' },
  { id: 'writer', Icon: PenLine, iconClassName: 'bg-violet-100 text-violet-600 dark:bg-violet-500/15 dark:text-violet-300' },
];

export const MASK_IDS = MASK_ENTRIES.map((entry) => entry.id);

/** 词典里的 capabilities 是数组，按索引逐个取词；未命中时 t 返回 key 本身，据此判断边界。 */
function catalogCapabilities(id: string): string[] {
  const capabilities: string[] = [];
  for (let i = 0; i < 16; i += 1) {
    const key = `masks.catalog.${id}.capabilities.${i}`;
    const value = t(key);
    if (value === key) break;
    capabilities.push(value);
  }
  return capabilities;
}

/** 按当前界面语言构建面具目录（词条在 i18n 词典 masks.catalog.<id>.*）。 */
export function maskOptions(): MaskOption[] {
  return MASK_ENTRIES.map((entry) => ({
    id: entry.id,
    name: t(`masks.catalog.${entry.id}.name`),
    description: t(`masks.catalog.${entry.id}.description`),
    capabilities: catalogCapabilities(entry.id),
    Icon: entry.Icon,
    iconClassName: entry.iconClassName,
  }));
}

const DEFAULT_MASK_ID = MASK_ENTRIES[0].id;

export function getActiveMaskId(): string {
  if (typeof window === 'undefined') return DEFAULT_MASK_ID;

  try {
    const stored = window.localStorage.getItem(ACTIVE_MASK_KEY);
    return MASK_IDS.includes(stored ?? '') ? stored ?? DEFAULT_MASK_ID : DEFAULT_MASK_ID;
  } catch {
    return DEFAULT_MASK_ID;
  }
}

export function setActiveMaskId(id: string): void {
  if (typeof window === 'undefined' || !MASK_IDS.includes(id)) return;

  try {
    window.localStorage.setItem(ACTIVE_MASK_KEY, id);
  } catch {
    // Local UI preference is best effort.
  }

  window.dispatchEvent(new Event(ACTIVE_MASK_CHANGED_EVENT));
}

function subscribeToActiveMask(onChange: () => void): () => void {
  if (typeof window === 'undefined') return () => undefined;

  window.addEventListener(ACTIVE_MASK_CHANGED_EVENT, onChange);
  window.addEventListener('storage', onChange);
  return () => {
    window.removeEventListener(ACTIVE_MASK_CHANGED_EVENT, onChange);
    window.removeEventListener('storage', onChange);
  };
}

export function useActiveMaskId(): string {
  return useSyncExternalStore(subscribeToActiveMask, getActiveMaskId, () => DEFAULT_MASK_ID);
}

/** 当前生效的面具（含本地化文案）。语言切换后随订阅组件重渲染返回新词条。 */
export function useActiveMask(): MaskOption {
  const activeMaskId = useActiveMaskId();
  const options = maskOptions();
  return options.find((option) => option.id === activeMaskId) ?? options[0];
}
