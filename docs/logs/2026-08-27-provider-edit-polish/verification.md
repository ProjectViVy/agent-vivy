# 验证记录

## 命令与结果（2026-08-27）

- `cd ui; pnpm typecheck` → ✅ 通过（tsc --noEmit 无错误）
- `cd ui; pnpm test` → ✅ 105/105 用例通过（含 kiwi 中英 key 对等校验；
  新增 `customDialogTitleManage` / `customDialogHintManage` 两边同步）
- 根目录 `just ci` → ✅ 全绿：
  - gofmt -l 无未格式化文件；`go vet ./...`、`go test ./...` 全部通过
  - ui: pnpm install（frozen-lockfile）→ typecheck → test（105/105）→ build
    （3.67s；仅有既有的 chunk > 500 kB 警告，与本次改动无关）

## 浏览器冒烟

8787 / 3015 端口被用户 Vivy Studio 调试会话占用，未自行启动服务器——按既有
约定由**用户 Studio 调试代验**，见 acceptance.md。提醒：改动只在分离 Vite
`http://127.0.0.1:3015`（硬刷新 Ctrl+Shift+R）；`:8787` 嵌入式页面是发行包。