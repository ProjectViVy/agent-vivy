# VC-1a: bash 工具（mvdan shell 解析 + 分级审批 + deny 表）

## 交付范围

新增 `bash` 内置工具，对齐 Crush 的 shell 能力面：模型给出一段 shell
脚本，工具以 `bash -c <script>` 执行。安全模型是三层递进的：

1. **Deny 表（policy 无关的硬拒绝）**。fork bomb、mkfs/format/diskpart/
   fdisk、shutdown/reboot/halt/poweroff、对系统根目录（`/`、`/usr`、`~`、
   `$HOME`、`C:\` 等）的递归强删、`dd of=/dev/{sd,hd,vd,nvme,disk}`、
   输出重定向进裸磁盘设备、`curl|wget … | sh` 管道执行。命中即拒绝，
   任何 profile（包括 full_auto）都放不过去。
2. **分级审批（InvocationClassifier）**。工具级 `Readonly` 标志只描述
   "这个工具会不会改东西"；`bash` 用 mvdan.cc/sh/v3 真实解析脚本 AST，
   把单次调用进一步分级：
   - `safe`：纯读命令（cat/ls/grep/rg/jq/sort 等）、只读 git 子命令
     （status/log/diff/show/blame 等）、`go env/list/version`、以及它们的
     管道组合；输出只进 `/dev/null`、stdout、stderr 的重定向也算 safe。
   - `mutating`：其余一切（写重定向、变量展开命令名、未知命令等），
     保持既有审批路径不变。
   - `denied`：deny 表命中。
   审批闸在**最终参数**（hooks 改写之后）上分类。`auto` 审批策略下
   safe 调用直接执行并发 governance 事件（reason =
   "safe read-only invocation auto-approved"）；`ask`/`never` 行为完全
   不变；deny 表在 full_auto 下依旧拒绝。
3. **Sandbox 归还**。`bash` 在 `validateRequest` 里被拦截、走专用校验
   （host 必须有 bash、read_only 沙箱拒绝、参数必须恰好是
   `["-c", script]`、deny 表二次复核——纵深防御），不再经过可执行文件
   白名单；普通 `execute`/`commandline` 路径不受影响。

其它交付物：

- `tools.InvocationClassifier` 接口成为审批闸的一般扩展点；default
  零值为 `InvocationMutating`（fail-closed）。
- `ValidateArgsSafety` 对 `bash` 的 `command` 参数豁免 shell 语法
  token 块（`&&`、`|`、`$(` 等），NUL 检查保留且无条件执行。
- 命令后端的 workspace cwd / 环境清洗 / 超时钳制逻辑提取为共享的
  `resolveCommandContext`。
- 默认启用面（config）加入 `bash`。
- `bash.Spec` 的 description 文档化分级语义；`PrepareProposal` 把脚本
  作为 proposal 预览，`RiskFindings` 标注 "command output is untrusted"。

## 明确未做（见 docs/TODO.md）

- 后台 job 注册表（job_output/job_kill）→ VC-1b。
- bash 输出截断目前沿用 tooladapter 的 compactToolResult 预算，没有
  独立的 head/tail 预算调参。
- Windows 原生 PowerShell/cmd 不在本工具范围（bash only，host 无
  bash 时报 "bash is not available on this host"）。

## 行为对照（与既有守则）

- "功能对齐 Crush、没有的不擅自添加"：bash 工具与分级审批是 Crush
  已有的能力面（shell tool + command allowlist/分级）；deny 表内容取
  常识性硬危险集合，未添加 Crush 之外的花活。
- FSL-1.1-MIT：只研究了 Crush 的行为语义，未复制任何代码；解析器是
  mvdan.cc/sh/v3（MIT）。
