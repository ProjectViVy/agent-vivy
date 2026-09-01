# UI-AUDIT-RUN-DETAIL — Run 事件 payload 的键盘可达详情

## 问题（审查行）

`RunInspector` 的事件列表把结构化 `RunLogEvent.payload` 只放在 HTML
`title` 属性里：悬停才见（触屏完全不可见）、键盘/读屏不可达——违反
"日志一等公民"要求（后端 payload 是结构化事实源，UI 却无可读呈现）。

## 修复

事件行（原本就是 `<button>`）改为**可展开开关**：

- 点击（或键盘 Enter/Space——原生 button 语义）切换该行详情；
- 展开时在行下渲染 `<pre>`：pretty-print 的 payload JSON
  （`JSON.stringify(event.payload ?? null, null, 2)`），
  `whitespace-pre-wrap break-words` 防溢出，`max-h-48` 内滚动；
- `aria-expanded` 标记展开态，展开行加 `bg-muted` 高亮；
- 移除 `title`（tooltip 与详情重复，且是缺陷本体）；
- 零新 i18n 键（内容是 JSON 字面量）。

同一时间只展开一行（`openSeq: number | null` 单值状态）——事件流较长，
多行展开会让列表失去锚点。

## 变更清单

- `ui/src/components/chat/RunInspector.tsx`：`openSeq` 状态 + 事件行
  展开/收起 + payload `<pre>` 详情；其余（run/background/children 面板）
  不动。
