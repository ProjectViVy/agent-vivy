# UI-INIT-RACE 验证记录

## 自动化验证流程

### 1. 前端单元测试与回归
- **命令**：`pnpm test` （位于 `ui/` 目录下）
- **结果**：
  - 测试文件：24 passed (24)
  - 测试用例：201 passed (201)
  - 覆盖新增测试：
    - `does not overwrite session created while initialize is in-flight`
    - `does not create redundant default session if user created one during initialize`
    - `throws and records runError when startRun is called with mismatched session`
    - `throws and records runError when editSession is called with mismatched session`

### 2. TypeScript 类型检查
- **命令**：`pnpm typecheck` （`tsc --noEmit`）
- **结果**：退出码 0，无任何类型错误。

### 3. Vite 生产构建
- **命令**：`pnpm build`
- **结果**：生产包构建成功，资源正确打入 `dist/`。

### 4. 门禁验证 (`just ci`)
- **命令**：`just ci`
- **步骤包含**：
  - `fmt-check`：Go 代码格式检查通过。
  - `ui-ci`：依赖锁定检查、类型检查、Vitest 运行（201/201 绿）、Vite 构建通过。
  - `vet`：后端代码静态分析通过。
  - `test`：后端单元测试全套通过（含 internal/app、channelhost、runtime 等）。
  - `headless-compile`：Headless 编译标签检查通过。
  - `plugin-ci`：所有插件与 face 模块检查通过。
- **结果**：退出码 0，CI-EXIT:0。

### 5. Playwright 端到端规格测试 (`just ui-e2e`)
- **命令**：`just ui-e2e`
- **结果**：
  - 21 passed, 1 skipped (cron-tasks 离线预期跳过)
  - `chat-act.spec.ts`、`files-panel.spec.ts` 等涉及会话创建与落定的规格全部稳定通过。
