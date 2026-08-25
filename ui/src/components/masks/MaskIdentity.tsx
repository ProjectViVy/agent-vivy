import { cn } from '@/lib/utils';
import type { MaskOption } from './mask-catalog';

export function MaskIdentity({ option, compact = false }: { option: MaskOption; compact?: boolean }) {
  const Icon = option.Icon;

  return (
    <span
      aria-hidden="true"
      className={cn(
        'flex shrink-0 items-center justify-center rounded-full',
        compact ? 'h-7 w-7' : 'h-10 w-10',
        option.iconClassName,
      )}
    >
      <Icon className={compact ? 'h-3.5 w-3.5' : 'h-5 w-5'} />
    </span>
  );
}
