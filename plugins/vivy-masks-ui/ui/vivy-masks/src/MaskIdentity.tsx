import { BookOpenCheck, Code2, PenLine, Star, VenetianMask } from 'lucide-react';
import { cn } from '@/lib/utils';
const identities = {
  'builtin/programmer': { Icon: Code2, color: 'bg-blue-100 text-blue-600 dark:bg-blue-500/15 dark:text-blue-300' },
  'builtin/researcher': { Icon: BookOpenCheck, color: 'bg-emerald-100 text-emerald-600 dark:bg-emerald-500/15 dark:text-emerald-300' },
  'builtin/writer': { Icon: PenLine, color: 'bg-violet-100 text-violet-600 dark:bg-violet-500/15 dark:text-violet-300' },
  '': { Icon: Star, color: 'bg-amber-100 text-amber-600 dark:bg-amber-500/15 dark:text-amber-300' },
};
export function MaskIdentity({ id, compact = false }: { id: string; compact?: boolean }) {
  const { Icon, color } = identities[id as keyof typeof identities] ?? { Icon: VenetianMask, color: 'bg-primary/10 text-primary' };
  return <span aria-hidden="true" className={cn('flex shrink-0 items-center justify-center rounded-full', compact ? 'h-7 w-7' : 'h-10 w-10', color)}><Icon className={compact ? 'h-3.5 w-3.5' : 'h-5 w-5'} /></span>;
}
