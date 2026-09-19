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

  // 摘要曾经在运行中右对齐（贴在行尾，看起来从右往左打印）；两种状态现在共用同一个
  // 左对齐、按行宽从右端截断的摘要元素。
  it('prints the running summary left to right like the settled one', () => {
    const running = renderToStaticMarkup(<ReasoningRow text={'第一行\n最新一行'} running />);
    const settled = renderToStaticMarkup(<ReasoningRow text={'第一行\n最新一行'} running={false} />);
    for (const html of [running, settled]) {
      expect(html).toContain('flex-1 truncate');
      expect(html).not.toContain('justify-end');
    }
  });

  it('strips emphasis markers from the summary only', () => {
    const html = renderToStaticMarkup(<ReasoningRow text={'**重要**：先看文件'} running={false} />);
    expect(html).toContain('重要：先看文件');
    expect(html).not.toContain('**重要**');
  });
});
