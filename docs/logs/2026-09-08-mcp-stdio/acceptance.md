# Acceptance

1. 在 MCP 设置页新增服务，选择 `STDIO`，填写 PATH 命令或绝对路径；参数文本框每行一个 argv，环境映射每行是 `CHILD_VAR ← HOST_VAR`，cwd 只能填写 workspace root 下的相对路径。
2. 保存后重新打开设置页，stdio 配置、环境名映射和 cwd 应完整回显；导出 JSON 再导入时保留这些字段，绝不导出环境值。
3. 启用 stdio 服务不会立即拉起进程；首次 probe/list/call 才启动一次。缺失 host 环境变量时，服务保持未启动并显示 `error`；补齐环境后须通过配置替换/重载恢复。
4. stdio 子进程退出后下一次操作显示 `error`，不会隐式启动第二个进程。HTTP 服务既有行为继续保持。
5. TUI/sidebar 继续使用同一 sidebar route 和 configured/initialized/error 状态合同；新增的 transport/env_missing 只是加法字段，stdio 类型、缺失 child key 和死亡 error 均可见。

验收边界：raw-frame 上界和 Windows 子进程树回收需后续 TODO 关闭后再宣称完整本地进程治理。
