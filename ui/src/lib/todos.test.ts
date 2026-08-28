import { describe, expect, it } from 'vitest';
import type { Todo } from './api';
import { currentPlanLabel, isTaskToolName, partitionTodos, shouldShowTodoStrip, todoCounts, todoProgressSegments } from './todos';

function todo(partial: Partial<Todo> & Pick<Todo, 'id' | 'status' | 'subject'>): Todo {
  return {
    session_id: 's1',
    description: 'detail',
    blocks: [],
    blocked_by: [],
    position: 0,
    created_at: 1,
    updated_at: 1,
    ...partial,
  };
}

describe('todo projection helpers', () => {
  it('counts each status once and ignores unknown values', () => {
    const todos = [
      todo({ id: '1', status: 'pending', subject: 'a' }),
      todo({ id: '2', status: 'in_progress', subject: 'b' }),
      todo({ id: '3', status: 'completed', subject: 'c' }),
      todo({ id: '4', status: 'cancelled', subject: 'd' }),
      todo({ id: '5', status: 'completed', subject: 'e' }),
    ];
    expect(todoCounts(todos)).toEqual({ pending: 1, inProgress: 1, completed: 2, cancelled: 1 });
  });

  it('splits current work from history and sorts by position / recency', () => {
    const todos = [
      todo({ id: '3', status: 'pending', subject: 'later', position: 2 }),
      todo({ id: '1', status: 'in_progress', subject: 'now', position: 0 }),
      todo({ id: '2', status: 'pending', subject: 'next', position: 1 }),
      todo({ id: '9', status: 'completed', subject: 'old', updated_at: 10 }),
      todo({ id: '8', status: 'cancelled', subject: 'newer cancel', updated_at: 20 }),
      todo({ id: '7', status: 'completed', subject: 'newest done', updated_at: 30 }),
    ];
    const { current, history } = partitionTodos(todos);
    expect(current.map((item) => item.id)).toEqual(['1', '2', '3']);
    expect(history.map((item) => item.id)).toEqual(['7', '8', '9']);
  });

  it('uses active_form for the standing plan label when present', () => {
    expect(currentPlanLabel([
      todo({ id: '1', status: 'pending', subject: 'queued' }),
      todo({ id: '2', status: 'in_progress', subject: 'Wire RPC', active_form: 'Wiring RPC' }),
    ])).toBe('Wiring RPC');
    expect(currentPlanLabel([todo({ id: '1', status: 'in_progress', subject: 'Wire RPC' })])).toBe('Wire RPC');
    expect(currentPlanLabel([todo({ id: '1', status: 'pending', subject: 'queued' })])).toBeNull();
  });

  it('recognizes only the durable task tools', () => {
    expect(isTaskToolName('task_create')).toBe(true);
    expect(isTaskToolName('task_list')).toBe(true);
    expect(isTaskToolName('write_note')).toBe(false);
    expect(isTaskToolName(undefined)).toBe(false);
  });

  it('builds overview segments without cancelled or zero counts', () => {
    expect(todoProgressSegments({ pending: 2, inProgress: 1, completed: 0, cancelled: 4 })).toEqual([
      { key: 'active', count: 1 },
      { key: 'pending', count: 2 },
    ]);
    expect(shouldShowTodoStrip([], 'empty')).toBe(false);
    expect(shouldShowTodoStrip([todo({ id: '1', status: 'pending', subject: 'a' })], 'ready')).toBe(true);
    expect(shouldShowTodoStrip([todo({ id: '1', status: 'pending', subject: 'a' })], 'error')).toBe(false);
  });
});
