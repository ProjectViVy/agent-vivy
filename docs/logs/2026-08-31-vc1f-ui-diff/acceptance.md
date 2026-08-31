# Acceptance — VC-1f

## 人工验收（有供应商密钥的环境，127.0.0.1:3015 split pair）

1. 聊天里让 Vivy 改一个工作区文件（例如「把 notes.txt 里的 two 改成 TWO」）。
2. 审批中心（或气泡内审批）里，patch/write 审批的 preview 不再是纯文本，
   而是：`+N −M` 统计 + 「统一视图 / 分栏视图」切换 + 行号着色 diff；
   切到分栏后左右两栏对照显示，删除块红、新增块绿、补齐侧灰底。
3. 批准后回到聊天页，该工具结果气泡显示「工具结果 + 文件路径 + 同款 diff
   视图」，下方「原始结果」折叠展开为原始 JSON；bash/grep 等其他工具结果
   气泡维持原纯文本样式。
4. 无 AGENTS.md 等场景不受影响：非 diff 内容（bash 输出、错误文本）不误判
   为 diff（需 `--- `/`diff `/`Index: ` 文件头 + `@@` hunk 才按 diff 渲染）。

## 无密钥环境的等价验证

- `pnpm test` 中的 DiffView SSR 渲染用例在真实组件上断言统计、切换按钮与
  hunk 内容；diff.test.ts 覆盖解析、配对、截断标记、非 diff 拒识。
- Playwright e2e 回归确认聊天页与设置页交互未被本次改动破坏。

## 回退

`git revert` 本交付单个提交即可；go-udiff 依赖随提交一起移除，无独立迁移。
