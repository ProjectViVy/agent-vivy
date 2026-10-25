import type { ReactNode } from 'react';
import { TONE_CHIP, type StatusTone } from '@/lib/status';
import { cn } from '@/lib/utils';

export function StatusBadge({
  tone,
  title,
  className,
  children,
}: {
  tone: StatusTone;
  title?: string;
  className?: string;
  children: ReactNode;
}) {
  return (
    <span
      title={title}
      className={cn('inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-2xs font-medium leading-none', TONE_CHIP[tone], className)}
    >
      {children}
    </span>
  );
}
