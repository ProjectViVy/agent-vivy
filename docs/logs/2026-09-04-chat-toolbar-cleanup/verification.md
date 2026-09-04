# Verification — 聊天框功能栏伪操作清理与闭环

## Commands Run & Results

1. **静态类型检查与单元测试**：
   - `pnpm typecheck` in `ui/`: 退出码 0，无任何类型错误。
   - `pnpm test` in `ui/`: 退出码 0，24 个测试套件全绿（197 passed），包括 `src/i18n/index.test.ts` 校验双语字典对称性完全一致。

2. **UI 构建与 E2E 浏览器规格**：
   - `pnpm build` in `ui/`: 退出码 0，Vite 生产包构建成功。
   - `pnpm e2e thinking-gate.spec.ts` in `ui/`: 退出码 0，1 passed。
     - 断言 `getByRole('button', { name: '思考模式' })` count = 0（无 provider 时隐藏）。
     - 断言 `getByTitle('手动触发 AutoDream')` 与 `getByLabel('手动触发 AutoDream')` count = 0（AutoDream 图标已清退）。
     - 断言点击展开执行模式下拉菜单后：`智能体模式` 与 `计划模式` 正常渲染，`询问模式` count = 0（伪模式已清退）。

3. **内核测试单测修复验证**：
   - `go test -v ./internal/codeface`: 退出码 0，3 passed，Windows 短路径兼容正常。

4. **全量网关质量门禁（Gate）**：
   - `just ci` in 根目录：
     - `fmt-check`：通过
     - `ui-ci` (install -> typecheck -> test -> build)：通过
     - `vet`：通过
     - `test`：全量 Go 测试通过
     - `headless-compile`：通过
     - `plugin-ci`：通过
     - 最终退出码 0。
