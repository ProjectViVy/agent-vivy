import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { DiffView } from './DiffView';

const SAMPLE = ['--- a/notes.txt', '+++ b/notes.txt', '@@ -1,3 +1,3 @@', ' one', '-two', '+TWO', ' three'].join('\n');

describe('DiffView', () => {
  it('renders stats, mode toggle and unified body for a real diff', () => {
    const html = renderToStaticMarkup(<DiffView diff={SAMPLE} />);
    expect(html).toContain('@@ -1,3 +1,3 @@');
    expect(html).toContain('+TWO');
    expect(html).toContain('aria-label="新增 1 行，删除 1 行"');
    expect(html).toContain('aria-pressed="true"');
    expect(html).toContain('统一视图');
    expect(html).toContain('分栏视图');
  });

  it('falls back to a plain pre for non-diff text', () => {
    const html = renderToStaticMarkup(<DiffView diff="plain preview text" />);
    expect(html).toContain('<pre');
    expect(html).toContain('plain preview text');
  });
});
