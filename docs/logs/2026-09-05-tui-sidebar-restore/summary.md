# 恢复宽屏右栏

## 已交付

- 右栏显示门槛从宽 120 降到 100，常见 Windows Terminal 尺寸会再出现 Crush 式右侧会话栏，而不是被收成顶栏 compact。
- `padHorizontal` 过宽行改为截断而不是把右栏挤出终端；聊天列加 `MaxWidth`。

## 边界

- 高度仍需 ≥30 才显示右栏。更矮的窗口继续用顶栏 compact。
- 没有改侧栏内容和滚动逻辑。
