import { useSyncExternalStore } from 'react';
import { BookOpenCheck, Code2, PenLine, Star } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';

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

export const MASK_OPTIONS: MaskOption[] = [
  {
    id: 'default',
    name: '默认助手',
    description: '通用问答与日常协作',
    capabilities: ['综合问答', '日常协作'],
    Icon: Star,
    iconClassName: 'bg-amber-100 text-amber-600 dark:bg-amber-500/15 dark:text-amber-300',
  },
  {
    id: 'programmer',
    name: '程序员',
    description: '编码、调试与工程验证',
    capabilities: ['编写代码', '排查问题', '验证结果'],
    Icon: Code2,
    iconClassName: 'bg-blue-100 text-blue-600 dark:bg-blue-500/15 dark:text-blue-300',
  },
  {
    id: 'researcher',
    name: '研究员',
    description: '检索、归纳与证据整理',
    capabilities: ['检索资料', '归纳观点', '整理证据'],
    Icon: BookOpenCheck,
    iconClassName: 'bg-emerald-100 text-emerald-600 dark:bg-emerald-500/15 dark:text-emerald-300',
  },
  {
    id: 'writer',
    name: '写作者',
    description: '表达、改写与内容打磨',
    capabilities: ['组织表达', '改写文本', '润色内容'],
    Icon: PenLine,
    iconClassName: 'bg-violet-100 text-violet-600 dark:bg-violet-500/15 dark:text-violet-300',
  },
];

const DEFAULT_MASK_ID = MASK_OPTIONS[0].id;

export function getActiveMaskId(): string {
  if (typeof window === 'undefined') return DEFAULT_MASK_ID;

  try {
    const stored = window.localStorage.getItem(ACTIVE_MASK_KEY);
    return MASK_OPTIONS.some((option) => option.id === stored) ? stored ?? DEFAULT_MASK_ID : DEFAULT_MASK_ID;
  } catch {
    return DEFAULT_MASK_ID;
  }
}

export function setActiveMaskId(id: string): void {
  if (typeof window === 'undefined' || !MASK_OPTIONS.some((option) => option.id === id)) return;

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

export function useActiveMask(): MaskOption {
  const activeMaskId = useActiveMaskId();
  return MASK_OPTIONS.find((option) => option.id === activeMaskId) ?? MASK_OPTIONS[0];
}
