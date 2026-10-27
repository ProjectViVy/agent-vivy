import type { Message } from '@/lib/api';
import { MessageAssistantBubble } from './message-assistant';
import { MessageUserBubble } from './message-user';
import { ToolResultBubble } from './tool-result';

export function MessageBubble({
  message,
  streaming,
  interrupted = false,
  canRegenerate = false,
  actionsDisabled = false,
  onRegenerate,
  onEditConfirm,
  onRewind,
  onFork,
}: {
  message: Message;
  streaming?: boolean;
  interrupted?: boolean;
  /** 助手消息之前存在用户消息（有可重发的输入） */
  canRegenerate?: boolean;
  /** 运行期间禁用重试，对应 Agent-DIVA 的 isTyping */
  actionsDisabled?: boolean;
  onRegenerate?: () => void;
  /** 编辑确认：rewind 到该消息 + 以新文本重开回合（内核不做自动重发）。 */
  onEditConfirm?: (newText: string) => void | Promise<void>;
  /** 回退确认：该消息（含）起退出上下文，输入框预填原文。 */
  onRewind?: () => void | Promise<void>;
  /** 分叉确认：以该消息为止的历史复制新会话并跳转。 */
  onFork?: () => void | Promise<void>;
}) {
  if (message.role === 'tool') return <ToolResultBubble message={message} />;
  // 空壳助手消息不渲染（空气泡）：内核为每个 tool.requested 都投影一条
  // content 为空的 assistant 消息（internal/runtime/message_projector.go
  // EventToolRequested），此前它渲染成一只空气泡 + 一整条操作栏。
  if (message.role !== 'user' && message.content.trim() === '' && !streaming) return null;
  if (message.role === 'user') {
    return (
      <MessageUserBubble
        message={message}
        streaming={streaming}
        actionsDisabled={actionsDisabled}
        onEditConfirm={onEditConfirm}
      />
    );
  }
  return (
    <MessageAssistantBubble
      message={message}
      streaming={streaming}
      interrupted={interrupted}
      canRegenerate={canRegenerate}
      actionsDisabled={actionsDisabled}
      onRegenerate={onRegenerate}
      onRewind={onRewind}
      onFork={onFork}
    />
  );
}
