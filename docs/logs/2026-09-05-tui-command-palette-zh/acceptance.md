# Acceptance

1. 在真实 `vivy tui --live` 中按 `/`。
2. 列表应显示 `/help` 加「查看命令」这类短中文，而不是满屏 `/mcp [server|resources…]`。
3. 上下移动时，当前行有底色；完整 usage 只出现在选中行下方。
4. 输入 `mcp` 或中文关键词时，匹配字有下划线高亮。
5. 底栏为「命令 / 发送 / 批准」等中文。
6. Enter 仍回填 `/help`、`/image` 这种英文命令；`//hello` 仍作为字面量发送。
