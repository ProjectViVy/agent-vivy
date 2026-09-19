import { resetLocaleForTests } from '@/i18n';
import { beforeEach, describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import type { FoldedToolCall } from '@/lib/run-rows';
import { ToolRow } from './ToolRow';

function call(partial: Partial<FoldedToolCall> = {}): FoldedToolCall {
  return {
    callId: 'c1',
    name: 'read_file',
    args: { path: 'src/a.ts' },
    argsRaw: '{"path":"src/a.ts"}',
    result: 'line one\nline two',
    error: '',
    status: 'ok',
    startedAt: 1,
    endedAt: 2,
    ...partial,
  };
}

beforeEach(() => resetLocaleForTests());

describe('ToolRow', () => {
  it('renders one disclosure row with the variant title and the argument summary', () => {
    const html = renderToStaticMarkup(<ToolRow call={call()} />);
    expect(html).toContain('data-tool-row="read_file"');
    expect(html).toContain('data-state="ok"');
    expect(html).toContain('src/a.ts');
    expect(html).toContain('aria-expanded="false"');
    // 收起时不渲染正文卡。
    expect(html).not.toContain('data-tool-body');
  });

  it('replaces the summary with the first error line', () => {
    const html = renderToStaticMarkup(<ToolRow call={call({ status: 'error', error: 'old_string not found\nmore' })} />);
    expect(html).toContain('data-state="error"');
    expect(html).toContain('old_string not found');
    expect(html).not.toContain('line two');
  });

  it('shows the diff totals in the collapsed row', () => {
    const diff = ['--- a/a.ts', '+++ b/a.ts', '@@ -1,2 +1,2 @@', '-old', '+new', ' keep'].join('\n');
    const collapsed = renderToStaticMarkup(<ToolRow call={call({ name: 'write_file', result: JSON.stringify({ path: 'a.ts', diff }), args: { path: 'a.ts' } })} />);
    expect(collapsed).toContain('+1 -1');
  });

  it('announces the run state for assistive technology', () => {
    const running = renderToStaticMarkup(<ToolRow call={call({ status: 'running', result: '' })} />);
    expect(running).toContain('Running');
    const stopped = renderToStaticMarkup(<ToolRow call={call({ status: 'stopped' })} />);
    expect(stopped).toContain('Stopped');
    const waiting = renderToStaticMarkup(<ToolRow call={call({ status: 'awaiting-approval' })} />);
    expect(waiting).toContain('Awaiting approval');
  });
});
