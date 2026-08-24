# 验证记录

## just ci（仓库根目录）

- 结果：通过
- Go：`go vet ./...`、`go test ./...` 全部通过（internal/app、runtime、rpc、eval、studiocore 等）
- UI：`pnpm typecheck` 通过；`pnpm test` 7 个测试文件 21 个用例全部通过；`pnpm build` 成功

## 浏览器冒烟（http://127.0.0.1:3015 分体 Vite）

- 开发循环已在运行（backend :8787 + Vite :3015），改动经 HMR 生效
- 桌面宽度（1440 布局仿真）截图：`ui/test-results/topbar-center-1440.png`
- 量化测量：胶囊切换器中心点与顶栏中心点偏差 -0.01px，即精确居中
- 左侧组（菜单按钮 + 头像 + 状态徽章）与右侧组（会话/待办图标）均单行完整显示，无遮挡、无换行
- 中等宽度安全性：grid 的 `1fr` 列以 min-content 为下限，空间不足时先压缩中间 auto 列，
  切换器按钮自带 `max-w` + `truncate` 可优雅收缩，768px 以上不会与左右内容重叠
