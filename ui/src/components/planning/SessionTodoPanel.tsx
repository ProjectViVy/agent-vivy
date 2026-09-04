import { Ban, Loader2, RotateCcw, X } from 'lucide-react';
import type { Todo, TodoStatus } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useTranslation } from '@/i18n';
import { runActive, useVivyStore } from '@/lib/store';
import { partitionTodos } from '@/lib/todos';
import { cn } from '@/lib/utils';

export function TodoRow({
  todo,
  disabled,
  onToggleStatus,
}: {
  todo: Todo;
  disabled: boolean;
  onToggleStatus: (id: string, status: TodoStatus) => void;
}) {
  const { t } = useTranslation();
  const activeForm = todo.status === 'in_progress' ? todo.active_form?.trim() : '';
  const isCompleted = todo.status === 'completed';
  const isCancelled = todo.status === 'cancelled';
  const isInProgress = todo.status === 'in_progress';

  return (
    <li className="group flex min-w-0 items-start gap-2.5 rounded-md px-1 py-1.5 transition-colors hover:bg-muted/40">
      <div className="mt-0.5 grid h-4 w-4 shrink-0 place-items-center">
        {isInProgress && disabled ? (
          <Loader2 className="h-3.5 w-3.5 animate-spin text-primary" aria-label={t('todos.status.in_progress')} />
        ) : (
          <Checkbox
            checked={isCompleted}
            disabled={disabled}
            aria-label={t('todos.toggleComplete')}
            title={disabled ? t('todos.runningLocked') : (isCompleted ? t('todos.markPending') : t('todos.markCompleted'))}
            onCheckedChange={(checked) => onToggleStatus(todo.id, checked ? 'completed' : 'pending')}
          />
        )}
      </div>
      <div className="min-w-0 flex-1">
        <div
          className={cn(
            'truncate text-sm',
            isCancelled && 'text-muted-foreground line-through',
            isCompleted && 'text-muted-foreground',
          )}
          title={todo.subject}
        >
          {todo.subject}
        </div>
        {activeForm && activeForm !== todo.subject ? (
          <div className="truncate text-xs text-muted-foreground" title={activeForm}>{activeForm}</div>
        ) : null}
        <span className="sr-only">{t(`todos.status.${todo.status}`)}</span>
      </div>
      <div className="shrink-0">
        {isCancelled ? (
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="h-6 w-6 text-muted-foreground hover:text-foreground disabled:opacity-40"
            disabled={disabled}
            title={disabled ? t('todos.runningLocked') : t('todos.markPending')}
            aria-label={t('todos.markPending')}
            onClick={() => onToggleStatus(todo.id, 'pending')}
          >
            <RotateCcw className="h-3.5 w-3.5" />
          </Button>
        ) : (
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className={cn(
              'h-6 w-6 text-muted-foreground hover:text-foreground transition-opacity disabled:opacity-0',
              'opacity-0 group-hover:opacity-100 focus:opacity-100',
            )}
            disabled={disabled}
            title={disabled ? t('todos.runningLocked') : t('todos.markCancelled')}
            aria-label={t('todos.markCancelled')}
            onClick={() => onToggleStatus(todo.id, 'cancelled')}
          >
            <Ban className="h-3.5 w-3.5" />
          </Button>
        )}
      </div>
    </li>
  );
}

function TodoSection({
  title,
  items,
  disabled,
  onToggleStatus,
}: {
  title: string;
  items: Todo[];
  disabled: boolean;
  onToggleStatus: (id: string, status: TodoStatus) => void;
}) {
  if (!items.length) return null;
  return (
    <section className="space-y-1">
      <h3 className="text-xs font-medium text-muted-foreground">{title}</h3>
      <ul className="m-0 list-none p-0">
        {items.map((todo) => (
          <TodoRow
            key={todo.id}
            todo={todo}
            disabled={disabled}
            onToggleStatus={onToggleStatus}
          />
        ))}
      </ul>
    </section>
  );
}

export function SessionTodoPanel({ onClose }: { onClose?: () => void }) {
  const todos = useVivyStore((state) => state.todos);
  const phase = useVivyStore((state) => state.todosPhase);
  const error = useVivyStore((state) => state.todosError);
  const loadTodos = useVivyStore((state) => state.loadTodos);
  const updateTodoStatus = useVivyStore((state) => state.updateTodoStatus);
  const currentRun = useVivyStore((state) => state.currentRun);
  const runBusy = useVivyStore((state) => state.runBusy);
  const { t } = useTranslation();

  const isRunning = runActive(currentRun);
  const disabled = isRunning || runBusy;

  const { current, history } = partitionTodos(todos);
  const showSkeleton = phase === 'loading' && todos.length === 0;
  const showError = phase === 'error' && todos.length === 0;
  const showEmpty = phase === 'empty' || (phase !== 'loading' && phase !== 'error' && todos.length === 0);

  return (
    <div id="session-todo-panel" className="flex h-full min-h-0 flex-col">
      <header className={cn('flex shrink-0 items-center gap-2 border-b px-4 py-3', onClose ? 'pr-4' : 'pr-12')}>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h2 className="truncate text-sm font-semibold">{t('todos.panelTitle')}</h2>
            {disabled ? (
              <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground" title={t('todos.runningLocked')}>
                {t('todos.runningLocked')}
              </span>
            ) : null}
          </div>
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
              <TodoSection
                title={t('todos.current')}
                items={current}
                disabled={disabled}
                onToggleStatus={(id, status) => void updateTodoStatus(id, status)}
              />
              <TodoSection
                title={t('todos.history')}
                items={history}
                disabled={disabled}
                onToggleStatus={(id, status) => void updateTodoStatus(id, status)}
              />
            </>
          ) : null}
        </div>
      </ScrollArea>
    </div>
  );
}
