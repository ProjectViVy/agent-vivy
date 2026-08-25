// 聊天消息操作（对照 Agent-DIVA msg-actions 移植）的纯逻辑：
// 「重新生成」在 Vivy 的 Journal 追加式事实源下映射为——
// 用目标助手消息之前最近一条用户消息的内容重新发起一轮对话。
import type { Message } from '@/lib/api';

/**
 * 返回重试目标助手消息所需的用户输入：
 * 从 messageId（助手消息）向前找最近一条 user 消息的内容；
 * 消息不存在、目标不是助手消息或前面没有 user 消息时返回 null。
 */
export function regeneratePrompt(messages: readonly Message[], messageId: string): string | null {
  const index = messages.findIndex((message) => message.id === messageId && message.role === 'assistant');
  if (index === -1) return null;
  for (let i = index - 1; i >= 0; i--) {
    if (messages[i].role === 'user') return messages[i].content;
  }
  return null;
}
