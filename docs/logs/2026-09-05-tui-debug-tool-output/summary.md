# TUI 工具输出 DEBUG 模式

## 已交付

- 新增 `tui.debug` 配置，默认值为 `false`。
- 默认模式将已完成工具卡片的正文限制为 8 个终端换行，并显示省略行数及开启方式。
- `tui.debug: true` 显示完整工具结果；终端控制字符清理和宽度约束仍然生效。
- 独立 `vivy-code`、进程内 `vivy tui`、远程 `vivy tui --live` 和打包 TUI face 共用同一个 renderer 行为。

## 边界

- 没有裁剪工具实际返回、Journal 数据或发送给模型的上下文；这是纯展示配置。
- 没有修改工具协议、Eino 编排或 Studio。
- 本次不是发布操作，因此没有 `release.md`。
