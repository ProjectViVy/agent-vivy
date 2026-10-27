/** One tone vocabulary for every status chip and badge in the kernel UI. */
export type StatusTone = 'ok' | 'warn' | 'danger' | 'info' | 'brand' | 'neutral';

export const TONE_CHIP: Record<StatusTone, string> = {
  ok: 'border-ok/40 bg-ok-soft text-ok',
  warn: 'border-warn/40 bg-warn-soft text-warn',
  danger: 'border-danger/40 bg-danger-soft text-danger',
  info: 'border-info/40 bg-info-soft text-info',
  brand: 'border-brand/40 bg-brand-soft text-brand-ink',
  neutral: 'border-line bg-sunken text-fg-muted',
};
