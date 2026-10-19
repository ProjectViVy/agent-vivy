/**
 * 记忆页面内容 - 由 vivy/memory Module 装配。
 *
 * 页面外壳（图标 + 标题、副标题、滚动区）由宿主的 Module 页面表面渲染，
 * 这里只返回页面内容，因此所有插件页面的外观一致。
 *
 * Recap 工作区只在后端声明 `recap` 能力时出现；否则保持原有记忆视图，
 * 避免展示一个始终被拒绝的页面。
 */

import { usePluginHost } from '@vivy/ui-sdk';
import { RecapWorkspace } from './recap';
import { MemoryView } from './view';

export function MemoryPage() {
  const host = usePluginHost();
  const recap = host?.rpc.capabilities.capabilities.includes('recap') ?? false;
  return recap ? <RecapWorkspace /> : <MemoryView />;
}
