/**
 * 记忆页面内容 - 由 vivy/memory Module 装配。
 *
 * 页面外壳（演示横幅、图标 + 标题、副标题、滚动区）由宿主的 Module 页面表面渲染，
 * 这里只返回页面内容，因此所有插件页面的外观一致。
 */

import { MemoryDemoView } from './view';

export function MemoryPage() {
  return <MemoryDemoView />;
}