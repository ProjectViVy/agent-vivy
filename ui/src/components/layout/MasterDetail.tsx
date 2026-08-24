import type { ReactNode } from 'react';
import { ChevronLeft } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

type MasterDetailProps = {
  selected: boolean;
  onBack: () => void;
  master: ReactNode;
  detail: ReactNode;
  columnsClassName?: string;
  backLabel?: string;
  className?: string;
};

export function MasterDetail({
  selected,
  onBack,
  master,
  detail,
  columnsClassName = 'md:grid-cols-[20rem_minmax(0,1fr)]',
  backLabel = '返回列表',
  className,
}: MasterDetailProps) {
  return (
    <div className={cn('grid h-full min-h-0 grid-rows-[minmax(0,1fr)]', columnsClassName, className)}>
      <section className={cn('flex min-h-0 min-w-0 flex-col', selected ? 'hidden md:flex' : 'flex')}>
        {master}
      </section>
      <section className={cn('flex min-h-0 min-w-0 flex-col', selected ? 'flex' : 'hidden md:flex')}>
        {selected ? (
          <div className="shrink-0 border-b px-2 py-2 md:hidden">
            <Button type="button" variant="ghost" size="sm" className="gap-1" onClick={onBack}>
              <ChevronLeft className="h-4 w-4" />
              {backLabel}
            </Button>
          </div>
        ) : null}
        <div className="min-h-0 min-w-0 flex-1 overflow-auto">{detail}</div>
      </section>
    </div>
  );
}
