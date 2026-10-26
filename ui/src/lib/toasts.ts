import { create } from 'zustand';
import { t } from '@/i18n';
import { classifyFailure } from './failure';

export type ToastTone = 'danger' | 'warn' | 'ok' | 'info';

export interface ToastAction {
  label: string;
  onClick: () => void;
}

export interface ToastInput {
  tone: ToastTone;
  title: string;
  detail?: string;
  technical?: string;
  action?: ToastAction;
  durationMs?: number;
}

export interface ToastRecord extends ToastInput {
  id: string;
  count: number;
}

const DEFAULT_DURATION: Record<ToastTone, number | null> = {
  danger: null,
  warn: null,
  ok: 2500,
  info: 3500,
};

const MAX_TOASTS = 3;

interface ToastTimer {
  remaining: number;
  startedAt: number;
  handle?: ReturnType<typeof setTimeout>;
}

const timers = new Map<string, ToastTimer>();
let paused = false;
let seq = 0;

const store = create<{ items: readonly ToastRecord[] }>(() => ({ items: [] }));

export function useToasts(): readonly ToastRecord[] {
  return store((state) => state.items);
}

function fingerprint(record: ToastInput): string {
  return [record.tone, record.title, record.technical ?? record.detail ?? ''].join('|');
}

function clearTimer(id: string) {
  const timer = timers.get(id);
  if (!timer) return;
  if (timer.handle) clearTimeout(timer.handle);
  timers.delete(id);
}

function arm(id: string, durationMs: number | null) {
  clearTimer(id);
  if (durationMs === null) return;
  const timer: ToastTimer = { remaining: durationMs, startedAt: Date.now() };
  timers.set(id, timer);
  if (!paused) start(id, timer);
}

function start(id: string, timer: ToastTimer) {
  timer.startedAt = Date.now();
  timer.handle = setTimeout(() => {
    timers.delete(id);
    remove(id);
  }, timer.remaining);
}

function remove(id: string) {
  store.setState((state) => ({ items: state.items.filter((item) => item.id !== id) }));
}

function setItems(items: readonly ToastRecord[]) {
  for (const item of store.getState().items) {
    if (!items.some((next) => next.id === item.id)) clearTimer(item.id);
  }
  store.setState({ items });
}

export function notify(input: ToastInput): string {
  const record = store.getState().items.find((item) => fingerprint(item) === fingerprint(input));
  if (record) {
    const merged: ToastRecord = { ...record, ...input, id: record.id, count: record.count + 1 };
    setItems([merged, ...store.getState().items.filter((item) => item.id !== record.id)].slice(0, MAX_TOASTS));
    arm(record.id, input.durationMs ?? DEFAULT_DURATION[input.tone]);
    return record.id;
  }

  seq += 1;
  const id = typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
    ? `toast_${crypto.randomUUID()}`
    : `toast_${Date.now()}_${seq}`;
  setItems([{ ...input, id, count: 1 }, ...store.getState().items].slice(0, MAX_TOASTS));
  arm(id, input.durationMs ?? DEFAULT_DURATION[input.tone]);
  return id;
}

export function notifyFailure(
  error: unknown,
  opts: { title: string; onRetry?: () => void; retryLabel?: string },
): string {
  const failure = classifyFailure(error);
  return notify({
    tone: 'danger',
    title: opts.title,
    detail: failure.detail,
    technical: failure.technical,
    action: opts.onRetry
      ? { label: opts.retryLabel ?? t('common.retry'), onClick: opts.onRetry }
      : undefined,
  });
}

export function notifySuccess(title: string, detail?: string): string {
  return notify({ tone: 'ok', title, detail });
}

export function dismissToast(id: string): void {
  clearTimer(id);
  remove(id);
}

export function pauseToasts(): void {
  if (paused) return;
  paused = true;
  const now = Date.now();
  for (const timer of timers.values()) {
    if (!timer.handle) continue;
    clearTimeout(timer.handle);
    timer.handle = undefined;
    timer.remaining = Math.max(0, timer.remaining - (now - timer.startedAt));
  }
}

export function resumeToasts(): void {
  if (!paused) return;
  paused = false;
  for (const [id, timer] of timers) {
    if (timer.handle) continue;
    start(id, timer);
  }
}

export function resetToasts(): void {
  paused = false;
  for (const id of [...timers.keys()]) clearTimer(id);
  store.setState({ items: [] });
}
