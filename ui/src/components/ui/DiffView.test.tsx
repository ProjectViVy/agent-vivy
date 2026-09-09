import { resetLocaleForTests } from '@/i18n';
import { beforeEach, describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { DiffView } from './DiffView';

const SAMPLE = ['--- a/notes.txt', '+++ b/notes.txt', '@@ -1,3 +1,3 @@', ' one', '-two', '+TWO', ' three'].join('\n');

beforeEach(() => resetLocaleForTests());

describe('DiffView', () => {
  it('renders stats, mode toggle and unified body for a real diff', () => {
    const html = renderToStaticMarkup(<DiffView diff={SAMPLE} />);
    expect(html).toContain('@@ -1,3 +1,3 @@');
    expect(html).toContain('+TWO');
    expect(html).toContain('aria-label="1 added, 1 removed"');
    expect(html).toContain('aria-pressed="true"');
    expect(html).toContain('Unified');
    expect(html).toContain('Split');
  });

  it('falls back to a plain pre for non-diff text', () => {
    const html = renderToStaticMarkup(<DiffView diff="plain preview text" />);
    expect(html).toContain('<pre');
    expect(html).toContain('plain preview text');
  });
});
