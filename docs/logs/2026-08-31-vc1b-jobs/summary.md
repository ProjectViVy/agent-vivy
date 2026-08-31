# VC-1b: 后台 job 注册表（job_output/job_kill，超时自动转后台）

## 交付范围

bash 工具获得后台执行能力，配套两个管理工具：

- **`bash.run_in_background`**（boolean 参数）：脚本立即作为后台 job
  启动，工具结果直接返回 `job_id` + `status: running`，脚本照常先过
  deny 表与分级审批（后台不放宽任何安全语义）。
- **超时自动转后台**：前台 bash 调用超过 `timeout_ms` 时不再杀进程丢输
  出，而是把仍在运行的进程收养为后台 job，结果携带 `timed_out: true` +
  `job_id` + 截至当下的部分输出；模型用 `job_output` 继续收割。
- **`job_output`**（readonly，自动放行）：按 job_id 读取**自上次读取以来
  的新增输出**（服务端读游标），并报告生命周期状态
  running/completed/failed/killed、退出码、时长。输出流是 64KiB 有界
  尾窗，被挤出头部时以 `stdout_gap_bytes`/`stderr_gap_bytes` 告知。
- **`job_kill`**（mutating，走既有审批）：终止运行中的 job；对已结束的
  job 幂等返回当前状态；未知 id 报错。

**生命周期语义：job 绑定 run context。** run 结束或被取消时其后台 job
一并被杀（service.go 终态路径释放 run ctx → exec.CommandContext 终止
进程）；不存在跨 run 孤儿进程。模型若想在 run 结束前拿到长任务结果，
需在同一 run 内轮询 `job_output` 到完成。

**归属与接线**：`tools.JobRegistry`（internal/tools/jobs.go）持有全部
进程机制（spawn/reader/finalizer/kill），`EinoCommandBackend` 实现
`tools.JobOperations` 并注册 `job_output`/`job_kill`（commands 后端同
时实现两接口时自动入册，含 tool_search 底表）；默认启用面加入两个新
工具名。并发上限 16 个在册 job（满员先驱逐终态 job，仍满则 fail-
closed）；终态 job 保留 64 个供读取。

## 明确未做（见 docs/TODO.md）

- 跨 run 存活的 job（需要把 job 句柄持久化进 Journal）不在本切片。
- Windows 上 `Process.Kill` 只杀 bash 本体，bash 的孙进程可能残留至
  自然结束（`cmd.WaitDelay=2s` 保证内核侧回收不悬挂）；进程树级击杀
  留待 SBX-OS。
- job 输出的独立预算调参、`job_output` 分页 offset 参数（当前服务端
  游标对单消费者模型即够）。

## 行为对照

- 后台 job + 输出收割 + kill 是 Crush 已有行为面（run_in_background /
  bash_output / kill_shell 语义对齐：增量输出、状态查询、终态幂等）。
- 超时转后台为研究 §5 VC-1 行内既定 wording（"超时自动转后台"），实现
  为默认行为并在工具描述中告知模型。
- eino 原生 `Shell`/`RunInBackendGround` 标志位未直接消费（eino-reuse
  清单结论：原生只有标志位、无 job 管理工具），按"引行为不引依赖"走
  自有后端。
