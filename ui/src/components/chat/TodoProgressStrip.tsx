import { ListTodo } from 'lucide-react';
import { useTranslation } from '@/i18n';
import { useVivyStore } from '@/lib/store';
import { currentPlanLabel, shouldShowTodoStrip, todoCounts, todoProgressSegments } from '@/lib/todos';
import { cn } from '@/lib/utils';

const SEGMENT_KEYS = {
  done: 'todos.progress.done',
  active: 'todos.progress.active',
  pending: 'todos.progress.pending',
} as const;

export function TodoProgressStrip() {
  const todos = useVivyStore((state) => state.todos);
  const phase = useVivyStore((state) => state.todosPhase);
  const open = useVivyStore((state) => state.todoPanelOpen);
  const setOpen = useVivyStore((state) => state.setTodoPanelOpen);
  const { t } = useTranslation();
  if (!shouldShowTodoStrip(todos, phase)) return null;

  const counts = todoCounts(todos);
  const segments = todoProgressSegments(counts);
  const label = currentPlanLabel(todos);
  const progress = segments.map((segment) => t(SEGMENT_KEYS[segment.key], { [segment.key === 'done' ? 'done' : segment.key === 'active' ? 'active' : 'pending']: segment.count })).join('\u2002·\u2002');

  return (
    <div className="px-3 pb-0 pt-1 sm:px-4">
      <button
        type="button"
        className={cn(
          'mx-auto flex h-9 w-full max-w-3xl cursor-pointer items-center gap-2.5 rounded-xl border bg-card px-3 text-left shadow-sm',
          'hover:bg-accent/40 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring',
        )}
        aria-expanded={open}
        aria-controls="session-todo-panel"
        onClick={() => setOpen(!open)}
      >
        <ListTodo className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden />
        <span className="shrink-0 text-[13px] font-medium leading-6">{t('todos.title')}</span>
        {label ? <span className="min-w-0 flex-1 truncate text-[13px] leading-5 text-muted-foreground" title={label}>{label}</span> : <span className="min-w-0 flex-1" />}
        {progress ? <span className="min-w-0 shrink truncate text-[13px] leading-5 text-muted-foreground">{progress}</span> : null}
      </button>
    </div>
  );
}
