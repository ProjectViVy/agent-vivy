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
    color: 'bg-blue-100 text-blue-600 dark:bg-blue-500/15 dark:text-blue-300',
  },
  'builtin/researcher': {
    Icon: BookOpenCheck,
    color:
      'bg-emerald-100 text-emerald-600 dark:bg-emerald-500/15 dark:text-emerald-300',
  },
  'builtin/writer': {
    Icon: PenLine,
    color:
      'bg-violet-100 text-violet-600 dark:bg-violet-500/15 dark:text-violet-300',
  },
  '': {
    Icon: Star,
    color:
      'bg-amber-100 text-amber-600 dark:bg-amber-500/15 dark:text-amber-300',
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
    color: 'bg-primary/10 text-primary',
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
        compact && 'bg-transparent dark:bg-transparent',
      )}
    >
      <Icon className={compact ? 'h-4 w-4' : large ? 'h-7 w-7' : 'h-5 w-5'} />
    </span>
  );
}
