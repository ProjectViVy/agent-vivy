# Verification — 2026-08-25 设置页布局与说明一致性整理

## `just ci`（仓库根目录）

结果：**通过**。首次运行被并行 WIP 的 8 处 UI 类型错误阻断（masks `MASK_OPTIONS` 改名、LifecycleView `t` 未定义），修复后全绿：

- `go vet ./...` / `go test ./...`：全部 ok（cached）
- `go test -tags vivy_headless ./cmd/vivy ./ui`：ok
- UI：`pnpm typecheck` 通过、`pnpm test` **8 个测试文件 31 个测试全过**、`pnpm build` 成功（vite build，2188 modules）

修复的阻断点：`ui/src/components/chat/MaskAndModelSwitcher.tsx`、`ui/src/components/masks/MaskManagementView.tsx`（`MASK_OPTIONS` → `maskOptions()`）、`ui/src/components/lifecycle/LifecycleView.tsx`（`GenerationSelect` 内补 `useTranslation()`）。

## 浏览器冒烟（http://127.0.0.1:3015，split Vite）

修复前应用白屏（`MASK_OPTIONS` 未定义导致 React 未挂载）；修复后应用正常挂载并逐项验证：

| 验证点 | 结果 |
|---|---|
| 标签栏六个预览标签带「预览」标记 | ✅ 通道/网络/语言/压缩/自进化/沙箱 均显示 |
| 通用页 DemoNote 已移除 | ✅ 通用页仅剩 应用信息、主题、Agent-Diva 迁移预览区 |
| 「通用与关于」恢复 | ✅ 聊天显示 / 缓存与运行状态 / 关于 Vivy 三卡完整 |
| 网络页「当前预览摘要」有说明 | ✅ 「汇总上方选择结果，不代表真实网络工具配置。」 |
| 压缩页「压缩配置」有说明 | ✅ 「调整只影响本页预览，不写入运行配置。」 |
| Vivy 功能页两卡头部统一 | ✅ 生命周期与 Run Inspector 均为「图标+标题+说明」 |

## 未验证项

- 视觉细节（截图为文字快照核验，模型不支持读图），布局正确性以 DOM 文本结构为准。
- 主题/语言切换交互未做（并行流文件，本次未动）。
