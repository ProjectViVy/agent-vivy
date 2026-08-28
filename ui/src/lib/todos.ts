import type { Todo } from './api';

export const TASK_TOOL_NAMES = ['task_create', 'task_update', 'task_list'] as const;

export type TodoCounts = {
  pending: number;
  inProgress: number;
  completed: number;
  cancelled: number;
};

export type PartitionedTodos = {
  current: Todo[];
  history: Todo[];
};

export function todoCounts(todos: readonly Todo[]): TodoCounts {
  const counts: TodoCounts = { pending: 0, inProgress: 0, completed: 0, cancelled: 0 };
  for (const todo of todos) {
    if (todo.status === 'pending') counts.pending += 1;
    else if (todo.status === 'in_progress') counts.inProgress += 1;
    else if (todo.status === 'completed') counts.completed += 1;
    else if (todo.status === 'cancelled') counts.cancelled += 1;
  }
  return counts;
}

export function partitionTodos(todos: readonly Todo[]): PartitionedTodos {
  const current = todos
    .filter((todo) => todo.status === 'pending' || todo.status === 'in_progress')
    .slice()
    .sort((a, b) => a.position - b.position || a.id.localeCompare(b.id));
  const history = todos
    .filter((todo) => todo.status === 'completed' || todo.status === 'cancelled')
    .slice()
    .sort((a, b) => b.updated_at - a.updated_at || b.id.localeCompare(a.id));
  return { current, history };
}

export function currentPlanLabel(todos: readonly Todo[]): string | null {
  const active = todos.find((todo) => todo.status === 'in_progress');
  if (!active) return null;
  const label = active.active_form?.trim() || active.subject.trim();
  return label || null;
}

export function isTaskToolName(name: unknown): boolean {
  return typeof name === 'string' && (TASK_TOOL_NAMES as readonly string[]).includes(name);
}

export type TodoProgressSegment = { key: 'done' | 'active' | 'pending'; count: number };

/** Overview strip omits cancelled and zero-count segments. */
export function todoProgressSegments(counts: TodoCounts): TodoProgressSegment[] {
  return [
    ...counts.completed > 0 ? [{ key: 'done' as const, count: counts.completed }] : [],
    ...counts.inProgress > 0 ? [{ key: 'active' as const, count: counts.inProgress }] : [],
    ...counts.pending > 0 ? [{ key: 'pending' as const, count: counts.pending }] : [],
  ];
}

export function shouldShowTodoStrip(todos: readonly Todo[], phase: string): boolean {
  return todos.length > 0 && phase !== 'error';
}
