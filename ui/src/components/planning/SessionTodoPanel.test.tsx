import { resetLocaleForTests } from '@/i18n';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { SessionTodoPanel, TodoRow } from './SessionTodoPanel';
import type { Todo } from '@/lib/api';

beforeEach(() => resetLocaleForTests());

describe('SessionTodoPanel', () => {
  it('renders initial empty state', () => {
    const html = renderToStaticMarkup(<SessionTodoPanel />);
    expect(html).toContain('This session has no to-dos yet');
  });
});

describe('TodoRow component', () => {
  const pendingTodo: Todo = {
    id: '1',
    session_id: 's1',
    subject: 'Deploy backend with Docker',
    description: 'Detailed steps',
    status: 'pending',
    blocks: [],
    blocked_by: [],
    position: 0,
    created_at: 100,
    updated_at: 100,
  };

  it('renders interactive pending todo with checkbox and cancel action', () => {
    const onToggle = vi.fn();
    const html = renderToStaticMarkup(
      <TodoRow todo={pendingTodo} disabled={false} onToggleStatus={onToggle} />,
    );
    expect(html).toContain('Deploy backend with Docker');
    expect(html).toContain('role="checkbox"');
    expect(html).toContain('data-state="unchecked"');
    expect(html).toContain('aria-label="Mark as cancelled"');
  });

  it('renders completed todo with checked state', () => {
    const completedTodo: Todo = { ...pendingTodo, id: '2', status: 'completed' };
    const html = renderToStaticMarkup(
      <TodoRow todo={completedTodo} disabled={false} onToggleStatus={vi.fn()} />,
    );
    expect(html).toContain('data-state="checked"');
  });

  it('renders cancelled todo with line-through and restore action', () => {
    const cancelledTodo: Todo = { ...pendingTodo, id: '3', status: 'cancelled' };
    const html = renderToStaticMarkup(
      <TodoRow todo={cancelledTodo} disabled={false} onToggleStatus={vi.fn()} />,
    );
    expect(html).toContain('line-through');
    expect(html).toContain('aria-label="Restore to pending"');
  });

  it('disables checkbox when session is running', () => {
    const html = renderToStaticMarkup(
      <TodoRow todo={pendingTodo} disabled={true} onToggleStatus={vi.fn()} />,
    );
    expect(html).toContain('disabled=""');
    expect(html).toContain('Todo status cannot be changed while session is running');
  });

  it('renders spinning loader when task is in_progress while session is running', () => {
    const activeTodo: Todo = {
      ...pendingTodo,
      id: '4',
      status: 'in_progress',
      active_form: 'Deploying backend container',
    };
    const html = renderToStaticMarkup(
      <TodoRow todo={activeTodo} disabled={true} onToggleStatus={vi.fn()} />,
    );
    expect(html).toContain('Deploying backend container');
    expect(html).toContain('animate-spin');
  });
});
