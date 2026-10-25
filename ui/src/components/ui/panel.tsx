import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

/** Shared page heading: icon well, `text-page` title, muted description, actions. */
export function PageHeader({
  icon,
  title,
  description,
  actions,
  className,
}: {
  icon?: ReactNode;
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn('flex flex-wrap items-center gap-3', className)}>
      {icon ? (
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-brand-soft text-brand">
          {icon}
        </div>
      ) : null}
      <div className="min-w-0 flex-1">
        <h1 className="text-page font-semibold tracking-tight">{title}</h1>
        {description ? <p className="mt-1 text-ui text-fg-muted">{description}</p> : null}
      </div>
      {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
    </div>
  );
}
