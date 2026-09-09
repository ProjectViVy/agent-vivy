import { afterEach, describe, expect, it, vi } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { hydrateLocale, resetLocaleForTests } from './index';
import { LanguagePicker } from '../components/settings/LanguagePicker';
import { Pagination, PaginationPrevious, PaginationNext } from '../components/ui/pagination';
import { Breadcrumb } from '../components/ui/breadcrumb';
import { Calendar } from '../components/ui/calendar';
import { MessageBubble } from '../components/chat/MessageBubble';
import { ReviewCard } from '../components/approvals/ReviewCard';
import { TrajectoryLedger } from '../components/trajectory/TrajectoryLedger';
import { turnLabel } from '../components/trajectory/trajectory-utils';
import { TRAJECTORY_KIND_LABEL } from '../components/trajectory/trajectory-types';
import type { ReviewItem } from '../lib/api';

// Only the external settings view is doubled. Rendering and catalogs are real.
vi.mock('@/lib/store', () => ({
  useVivyStore: (selector: (state: unknown) => unknown) => selector({
    settings: { locale: 'en', locale_read_only: false },
    settingsPhase: 'error',
    settingsError: 'backend_error_42',
    saveLocale: async () => undefined,
  }),
}));

afterEach(() => resetLocaleForTests());

describe.each([
  { locale: 'en' as const, language: 'Language', saveError: 'Save failed; please retry', previous: 'Go to previous page', breadcrumb: 'Breadcrumb', tool: 'Tool result', approve: 'Approve', empty: 'No trajectory records', session: 'Session', kind: 'TOOL' },
  { locale: 'zh' as const, language: '语言', saveError: '保存失败，请重试', previous: '转到上一页', breadcrumb: '面包屑导航', tool: '工具结果', approve: '批准', empty: '暂无轨迹记录', session: '会话开始', kind: '工具' },
])('real catalog surfaces: $locale', (copy) => {
  it('renders locale-picker errors and accessible navigation without translating backend details', () => {
    hydrateLocale(copy.locale);
    const html = renderToStaticMarkup(<><LanguagePicker /><Pagination><PaginationPrevious /><PaginationNext /></Pagination><Breadcrumb /></>);
    expect(html).toContain(copy.language);
    expect(html).toContain(copy.saveError);
    expect(html).toContain('backend_error_42');
    expect(html).toContain('aria-label="' + copy.previous + '"');
    expect(html).toContain('aria-label="' + copy.breadcrumb + '"');
    expect(html).toContain('aria-pressed="true"');
  });

  it('renders chat and review chrome while preserving user/model/tool text and raw identifiers', () => {
    hydrateLocale(copy.locale);
    const content = '原始内容 / raw content';
    for (const role of ['user', 'assistant', 'tool'] as const) {
      const html = renderToStaticMarkup(<MessageBubble message={{ id: 'message-id', role, content, created_at: 0 }} />);
      expect(html).toContain(content);
      if (role === 'tool') expect(html).toContain(copy.tool);
    }
    const review: ReviewItem = { id: 'review-id', kind: 'approval', status: 'pending', session_id: 'session-id', run_id: 'run-id', created_at: 0, expires_at: 0, tool_name: 'bash', preview: content };
    const html = renderToStaticMarkup(<ReviewCard review={review} busy={false} onRespond={() => undefined} />);
    expect(html).toContain(copy.approve);
    expect(html).toContain(content);
    expect(html).toContain('run-id');
  });

  it('localizes the trajectory empty state and host badges, not grouping identifiers', () => {
    hydrateLocale(copy.locale);
    const html = renderToStaticMarkup(<TrajectoryLedger records={[]} collapsedTurns={new Set()} collapsedAssistants={new Set()} focusIndexes={null} selectedRecordId={null} requestNumberByRecordId={new Map()} onToggleTurn={() => undefined} onToggleAssistant={() => undefined} onSelectRecord={() => undefined} onSelectRequest={() => undefined} />);
    expect(html).toContain(copy.empty);
    expect(turnLabel(null)).toBe(copy.session);
    expect(turnLabel(3)).toBe('#3');
    expect(TRAJECTORY_KIND_LABEL.tool).toBe(copy.kind);
  });

  it('uses the selected locale for calendar chrome and accessible date labels', () => {
    hydrateLocale(copy.locale);
    const day = new Date(2026, 8, 9);
    const html = renderToStaticMarkup(<Calendar mode="single" month={day} today={day} selected={day} />);
    expect(html).toContain(copy.locale === 'en' ? 'September 2026' : '2026年9月');
    expect(html).toContain(copy.locale === 'en' ? 'Go to the next month' : '转到下个月');
    expect(html).toContain(copy.locale === 'en' ? 'selected' : '已选择');
  });

});
