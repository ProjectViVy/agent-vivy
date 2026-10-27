import type { ReactNode } from 'react';
import { dateTimeLocale } from '@/i18n';

export const ACTION_BUTTON =
  'flex h-7 w-7 items-center justify-center rounded-md text-fg-muted transition-colors hover:bg-hover hover:text-fg disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:bg-transparent';

export function formatTime(timestamp: number): string {
  if (!timestamp) return '';
  return new Date(timestamp).toLocaleTimeString(dateTimeLocale(), { hour: '2-digit', minute: '2-digit' });
}

export function MessageAction({
  label,
  onClick,
  disabled = false,
  children,
}: {
  label: string;
  onClick?: () => void;
  disabled?: boolean;
  children: ReactNode;
}) {
  return (
    <button type="button" className={ACTION_BUTTON} disabled={disabled} onClick={onClick} title={label} aria-label={label}>
      {children}
    </button>
  );
}
