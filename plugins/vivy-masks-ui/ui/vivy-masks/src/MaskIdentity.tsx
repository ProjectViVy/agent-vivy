import {
  BookOpenCheck,
  Code2,
  PenLine,
  Star,
  VenetianMask,
} from 'lucide-react';
import { cn } from '@/lib/utils';
const identities = {
  'builtin/programmer': {
    Icon: Code2,
    color: 'bg-info-soft text-info-fg',
  },
  'builtin/researcher': {
    Icon: BookOpenCheck,
    color: 'bg-ok-soft text-ok-fg',
  },
  'builtin/writer': {
    Icon: PenLine,
    color: 'bg-brand-soft text-brand',
  },
  '': {
    Icon: Star,
    color: 'bg-warn-soft text-warn-fg',
  },
};
export function MaskIdentity({
  id,
  compact = false,
  large = false,
}: {
  id: string;
  compact?: boolean;
  large?: boolean;
}) {
  const { Icon, color } = identities[id as keyof typeof identities] ?? {
    Icon: VenetianMask,
    color: 'bg-brand-soft text-brand',
  };
  return (
    <span
      aria-hidden="true"
      className={cn(
        'flex shrink-0 items-center justify-center',
        compact
          ? 'h-4 w-4 rounded-lg bg-transparent'
          : large
            ? 'h-16 w-16 rounded-2xl'
            : 'h-11 w-11 rounded-xl',
        color,
        compact && 'bg-transparent',
      )}
    >
      <Icon className={compact ? 'h-4 w-4' : large ? 'h-7 w-7' : 'h-5 w-5'} />
    </span>
  );
}
