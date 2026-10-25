import type { Message } from '@/lib/api';
import { useTranslation } from '@/i18n';
import { parseToolResultDiff } from '@/lib/diff';
import { stripUntrustedHeader } from '@/lib/run-rows';
import { DiffView } from '@/components/ui/DiffView';

// 内核给工具结果加了可信度信封头；比对面向前解析前先剥掉，否则文件变更的
// diff 永远解析不出来（历史消息有的带信封、有的不带）。
export function ToolResultBubble({ message }: { message: Message }) {
  const { t } = useTranslation();
  const toolDiff = parseToolResultDiff(stripUntrustedHeader(message.content));
  if (!toolDiff) {
    return (
      <div className="mx-auto my-3 max-w-content min-w-0 rounded-xl border border-line bg-sunken/40 p-3 text-ui">
        <div className="mb-1 text-meta font-medium text-fg-muted">{t('chat.toolResult')}</div>
        <pre className="overflow-x-auto whitespace-pre-wrap break-words">{message.content}</pre>
      </div>
    );
  }
  return (
    <div className="mx-auto my-3 max-w-content min-w-0 rounded-xl border border-line bg-sunken/40 p-3 text-ui">
      <div className="mb-1 flex min-w-0 items-baseline gap-2 text-meta font-medium text-fg-muted">
        <span className="shrink-0">{t('chat.toolResult')}</span>
        {toolDiff.path ? <code className="min-w-0 break-all font-mono">{toolDiff.path}</code> : null}
      </div>
      <DiffView diff={toolDiff.diff} />
      <details className="mt-2">
        <summary className="cursor-pointer text-meta text-fg-muted">{t('chat.toolResultRaw')}</summary>
        <pre className="mt-1 overflow-x-auto whitespace-pre-wrap break-words">{message.content}</pre>
      </details>
    </div>
  );
}
