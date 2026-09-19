import { resetLocaleForTests } from '@/i18n';
import { beforeEach, describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { ReasoningRow } from './ReasoningRow';

beforeEach(() => resetLocaleForTests());

describe('ReasoningRow', () => {
  it('is collapsed by default and keeps the body out of the markup', () => {
    const html = renderToStaticMarkup(<ReasoningRow text={'第一行\n第二行'} running={false} />);
    expect(html).toContain('data-reasoning-row');
    expect(html).toContain('data-state="ok"');
    expect(html).toContain('Reasoning');
    expect(html).toContain('第一行');
    expect(html).not.toContain('data-reasoning-body');
  });

  it('summarises the newest line while running and announces the state', () => {
    const html = renderToStaticMarkup(<ReasoningRow text={'第一行\n最新一行'} running />);
    expect(html).toContain('data-state="running"');
    expect(html).toContain('最新一行');
    expect(html).toContain('Running');
  });

  it('strips emphasis markers from the summary only', () => {
    const html = renderToStaticMarkup(<ReasoningRow text={'**重要**：先看文件'} running={false} />);
    expect(html).toContain('重要：先看文件');
    expect(html).not.toContain('**重要**');
  });
});
