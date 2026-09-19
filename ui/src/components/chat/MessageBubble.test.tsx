import { resetLocaleForTests } from '@/i18n';
import { beforeEach, describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { MessageBubble } from './MessageBubble';
import { UNTRUSTED_RESULT_HEADER } from '@/lib/run-rows';
import type { Message } from '@/lib/api';

function assistant(partial: Partial<Message> = {}): Message {
  return { id: 'msg-1', role: 'assistant', content: '', created_at: 1_700_000_000_000, ...partial };
}

beforeEach(() => resetLocaleForTests());

describe('MessageBubble', () => {
  // The kernel projects one assistant message per tool.requested event with an
  // empty content (internal/runtime/message_projector.go). Rendering it left an
  // empty bubble plus a full action bar.
  it('renders nothing for a tool-call assistant step with empty content', () => {
    expect(renderToStaticMarkup(<MessageBubble message={assistant()} />)).toBe('');
  });

  it('keeps an empty assistant bubble while streaming', () => {
    const html = renderToStaticMarkup(<MessageBubble message={assistant()} streaming />);
    expect(html).toContain('…');
  });

  it('still renders a user message whose text is empty', () => {
    const html = renderToStaticMarkup(<MessageBubble message={assistant({ role: 'user' })} />);
    expect(html).toContain('data-message-id="msg-1"');
  });

  // The primary-coloured bubble must keep its own foreground colour: the
  // typography plugin otherwise repaints the text with its own grey.
  it('marks the user bubble body as colour-inheriting', () => {
    const html = renderToStaticMarkup(<MessageBubble message={assistant({ role: 'user', content: 'hi' })} />);
    expect(html).toContain('prose-inherit');
  });

  it('renders GFM tables instead of raw pipes', () => {
    const content = ['| 类别 | 工具 |', '| --- | --- |', '| 文件读 | read_note |'].join('\n');
    const html = renderToStaticMarkup(<MessageBubble message={assistant({ content })} />);
    expect(html).toContain('<table>');
    expect(html).toContain('<th>类别</th>');
    expect(html).toContain('<td>read_note</td>');
    expect(html).not.toContain('| 类别 |');
  });

  it('renders emphasis and inline code through markdown', () => {
    const html = renderToStaticMarkup(<MessageBubble message={assistant({ content: '**加粗** 与 `code`' })} />);
    expect(html).toContain('<strong>加粗</strong>');
    expect(html).toContain('<code>code</code>');
  });

  it('finds the diff inside a tool result that carries the untrusted envelope', () => {
    const diff = ['--- a/a.ts', '+++ b/a.ts', '@@ -1,1 +1,1 @@', '-old', '+new'].join('\n');
    const content = `${UNTRUSTED_RESULT_HEADER}\n${JSON.stringify({ path: 'a.ts', diff })}`;
    const html = renderToStaticMarkup(<MessageBubble message={assistant({ role: 'tool', content })} />);
    expect(html).toContain('aria-label="1 added, 1 removed"');
    expect(html).toContain('a.ts');
  });
});
