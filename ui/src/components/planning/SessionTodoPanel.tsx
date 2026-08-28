import { Ban, CheckCircle2, CircleDashed, Loader2, X } from 'lucide-react';
import type { Todo, TodoStatus } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useTranslation } from '@/i18n';
import { useVivyStore } from '@/lib/store';
import { partitionTodos } from '@/lib/todos';
import { cn } from '@/lib/utils';

function StatusGlyph({ status }: { status: TodoStatus }) {
  if (status === 'completed') return <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" aria-hidden />;
  if (status === 'in_progress') return <Loader2 className="h-3.5 w-3.5 animate-spin text-primary" aria-hidden />;
  if (status === 'cancelled') return <Ban className="h-3.5 w-3.5 text-muted-foreground" aria-hidden />;
  return <CircleDashed className="h-3.5 w-3.5 text-muted-foreground" aria-hidden />;
}

function TodoRow({ todo }: { todo: Todo }) {
  const { t } = useTranslation();
  const activeForm = todo.status === 'in_progress' ? todo.active_form?.trim() : '';
  return (
    <li className="flex min-w-0 items-start gap-2.5 py-1.5">
      <span className="mt-0.5 grid h-4 w-4 shrink-0 place-items-center"><StatusGlyph status={todo.status} /></span>
      <div className="min-w-0 flex-1">
        <div className={cn('truncate text-sm', todo.status === 'cancelled' && 'text-muted-foreground line-through')} title={todo.subject}>
          {todo.subject}
        </div>
        {activeForm && activeForm !== todo.subject ? (
          <div className="truncate text-xs text-muted-foreground" title={activeForm}>{activeForm}</div>
        ) : null}
        <span className="sr-only">{t(`todos.status.${todo.status}`)}</span>
      </div>
    </li>
  );
}

function TodoSection({ title, items }: { title: string; items: Todo[] }) {
  if (!items.length) return null;
  return (
    <section className="space-y-1">
      <h3 className="text-xs font-medium text-muted-foreground">{title}</h3>
      <ul className="m-0 list-none p-0">{items.map((todo) => <TodoRow key={todo.id} todo={todo} />)}</ul>
    </section>
  );
}

export function SessionTodoPanel({ onClose }: { onClose?: () => void }) {
  const todos = useVivyStore((state) => state.todos);
  const phase = useVivyStore((state) => state.todosPhase);
  const error = useVivyStore((state) => state.todosError);
  const loadTodos = useVivyStore((state) => state.loadTodos);
  const { t } = useTranslation();
  const { current, history } = partitionTodos(todos);
  const showSkeleton = phase === 'loading' && todos.length === 0;
  const showError = phase === 'error' && todos.length === 0;
  const showEmpty = phase === 'empty' || (phase !== 'loading' && phase !== 'error' && todos.length === 0);

  return (
    <div id="session-todo-panel" className="flex h-full min-h-0 flex-col">
      <header className={cn('flex shrink-0 items-center gap-2 border-b px-4 py-3', onClose ? 'pr-4' : 'pr-12')}>
        <div className="min-w-0 flex-1">
          <h2 className="truncate text-sm font-semibold">{t('todos.panelTitle')}</h2>
          {todos.length ? (
            <p className="truncate text-xs text-muted-foreground">
              {t('todos.sectionCounts', { current: current.length, history: history.length })}
            </p>
          ) : null}
        </div>
        {onClose ? (
          <Button type="button" variant="ghost" size="icon" className="shrink-0" aria-label={t('todos.closePanel')} onClick={onClose}>
            <X className="h-4 w-4" />
          </Button>
        ) : null}
      </header>
      <ScrollArea className="min-h-0 flex-1">
        <div className="space-y-5 p-4">
          {showSkeleton ? (
            <div className="space-y-2" aria-hidden>
              <div className="h-4 w-16 rounded bg-muted" />
              <div className="h-5 w-full rounded bg-muted" />
              <div className="h-5 w-3/4 rounded bg-muted" />
            </div>
          ) : null}
          {showError ? (
            <div className="space-y-3 text-sm">
              <p className="text-destructive">{error || t('todos.loadFailed')}</p>
              <Button type="button" size="sm" variant="outline" onClick={() => void loadTodos()}>{t('common.retry')}</Button>
            </div>
          ) : null}
          {showEmpty ? <p className="text-sm text-muted-foreground">{t('todos.empty')}</p> : null}
          {!showSkeleton && !showError && !showEmpty ? (
            <>
              <TodoSection title={t('todos.current')} items={current} />
              <TodoSection title={t('todos.history')} items={history} />
            </>
          ) : null}
        </div>
      </ScrollArea>
    </div>
  );
}
