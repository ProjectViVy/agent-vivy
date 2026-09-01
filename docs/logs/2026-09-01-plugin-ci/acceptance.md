# Acceptance

人工判定方式（无需读代码）：

1. 仓库根执行 `just plugin-ci`：输出 6 个 `== plugin-ci: <name>` 段
   （dingtalk/discord/feishu/lsp/qq/telegram），每段 vet + test 通过，退出码 0。
2. 仓库根执行 `just ci`：在 headless-compile 与 ui-ci 之间可见 plugin-ci 段。
3. 反向验证：在任一插件 module 里制造 vet/test 失败（如引入未 tidy 的依赖），
   `just ci` 整体红——即「插件回归不再只靠切片内人工执行」。
4. `plugins/qq` 无需任何手工步骤即可在干净检出上 vet/test 通过（漂移已修）。
