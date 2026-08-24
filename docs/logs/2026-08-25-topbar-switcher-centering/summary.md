# 顶栏面具/模型切换器居中

## 变更内容

- `ui/src/routes/_layout.tsx`：顶部栏从 `flex + justify-between` 改为三列网格
  `grid grid-cols-[1fr_auto_1fr]`，中间的「选择面具 / 选择模型」切换器
  （`MaskAndModelSwitcher`）以 `justify-self-center` 放置在 auto 列，实现
  相对整个顶栏的精确水平居中，不再受左右两侧内容宽度不一致的影响。
- 左侧组加 `min-w-0`、右侧组 `justify-self-end`，保持原有左/右对齐语义。

## 范围

- 仅顶栏布局类名调整，未改动切换器组件本身、未改动任何行为逻辑。

## 明确不做

- 不改动 `MaskAndModelSwitcher` 内部结构与下拉菜单。
- 不改动移动端（<768px）隐藏切换器的既有断点行为。
