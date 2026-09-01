# VC-3g: UI 文件预览 + 语法高亮（workspace files panel）

日期：2026-09-01 ｜ 分支：`feat/vc1a-bash-tool` ｜ worktree：`agent-vivy-vc0`

## What changed

VC-3 第 7 切片：给 UI 增加"当前 run 工作区文件"预览面板（Files），带语法高亮。

### 内核（只读 WorkspaceFiles 服务 + RPC）

- `internal/runtime/workspace_files.go`（新增）：
  - `WorkspaceFiles`：对 run workspace 的只读访问器，供控制面 UI 预览。
  - `List(ctx, runID)`：遍历 workspace 根（`WorkspaceManager.Ensure` 懒建目录），
    返回相对路径（正斜杠）+ 大小，按路径排序；符号链接跳过（目录则整枝跳过）；
    单次响应 2000 项封顶，超出报 `truncated`，不向 UI 无界流式。
  - `Read(ctx, runID, path)`：单文件内容。路径安全为双重遏制：
    `workspaceRelPath` 清洗（拒绝空/反斜杠/盘符冒号/前导斜杠，`path.Clean` 后拒绝
    `.`/`..`/`../` 前缀）+ `WorkspaceManager.ValidatePath` 复核合并后的绝对路径；
    `Lstat` 拒绝符号链接与非普通文件；二进制件（`isBinary`）只回 flag 不回内容；
    文本超字节上限按文件系统工具同款默认值截断并置 `truncated`。
- `internal/rpc/control.go`：新增 `workspace/list`、`workspace/read` 两个方法；
  `ControlDeps.WorkspaceFiles` 为接口（依赖注入，nil → `MethodNotFound`
  "workspace files are not configured"，沿用 MCPCatalog 先例）；缺 `run_id`/`path`
  → `InvalidParams`。内部错误一律折叠为 `-32603 internal error`，不向客户端泄露
  路径校验细节。
- `internal/app/app.go`：装配 `WorkspaceFiles`（workspaceManager 为 nil 时保持
  nil，方法即禁用）。

### UI（Files 侧板）

- `ui/src/components/files/FilesPanel.tsx`（新增）：无 run → 空态；有 run →
  列表（路径/大小/条数/截断标记/刷新）+ 点击预览；语法高亮用 `highlight.js`
  （`lib/common` 按需子集 + `github-dark.css`；扩展名经 `hljs.getLanguage` 映射，
  未知扩展回退为 HTML 转义纯文本）。hljs 输出本身是 HTML 转义后的标注片段，
  `escapeHtml` 仅用于回退路径，`dangerouslySetInnerHTML` 在此安全。
- `ui/src/lib/api.ts`：`WorkspaceFile`/`WorkspaceFileContent` 类型 +
  `listWorkspaceFiles`/`readWorkspaceFile`（POST body snake_case，与控制面契约一致）。
- `ui/src/lib/store.ts`：`filesPanelOpen` 状态（Review Center 同款 Sheet 开关）。
- `ui/src/routes/_layout.tsx`：header 增加 Files 图标按钮（Folder，aria-expanded）
  与右侧 Sheet（`sm:max-w-[560px]`），Review Center / Todos 同族。
- `ui/src/i18n/{en,zh}.ts`：`layout.files` + `files.*` 文案组（空态/条数/截断/
  二进制/加载/选择提示）。
- `ui/package.json` + `pnpm-lock.yaml`：新增唯一 UI 依赖 `highlight.js`。

### e2e 与 harness

- `ui/e2e/files-panel.spec.ts`（新增）：无 provider 机器可跑的壳态用例——欢迎向导
  跳过 → 打开 Files → 断言空态文案 → Escape 关闭 → 重开仍为空态。
- `ui/e2e/global-setup.ts`：e2e config 显式增加 `runtime.workspace_root`（指向
  `.e2e-workdir/workspace`）。修复冒烟中发现的缺口：此前 e2e 配置不设 workspace_root，
  任何触发 `Ensure` 的路径都会落到默认用户根 `~/.vivy/workspace`。

## 设计决策

- 只读面：Files 面板仅消费 `workspace/list`/`workspace/read`，不提供任何写/删/
  下载能力——预览即全部，恢复语义归 RB-1 文件版本 history（等 O1..O6）。
- 禁用即缺依赖：与 MCPCatalog 一致，`WorkspaceFiles` 未装配时 RPC 返回
  MethodNotFound，而不是空结果（诚实反映能力缺失）。
- 高亮器选型：highlight.js 是唯一新增 UI 依赖；语言映射用"扩展名能被识别才传"
  的保守策略，识别不了就转义纯文本，不猜语言。
- 二进制不回传内容：UI 预览是文本面；图片预览走 read_file 工具链（切片5），
  不在本面板重复建设。

## Not done（明确不做）

- 文件版本 history（VC-3 最后一项）：等用户裁决 O1..O6（RB-1）。
- 目录树/递归浏览/搜索：Crush 无此面，不擅自添加。
- 图片/二进制内容预览：同上，面板只做文本预览。
- 有 provider 的端到端 run 内浏览：本机无 provider key，无法跑真 run；
  已用真实服务器 WebSocket RPC 冒烟补齐内核全链路（见 verification.md）。

## 验收口径

- Crush 为 FSL-1.1-MIT：本切片为 Vivy 自有能力（工作区文件预览面板），行为
  对齐 Crush 的"文件可见性"诉求，代码零拷贝、零上游摘抄。
- 无 provider 机器上的浏览器验证限于壳态；RPC 全链路见真实服务器冒烟记录。
