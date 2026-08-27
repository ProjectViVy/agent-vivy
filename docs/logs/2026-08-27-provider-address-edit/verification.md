# 验证记录

## 命令与结果（2026-08-27）

- `cd ui; pnpm typecheck` → ✅ 通过（tsc --noEmit 无错误）
- `cd ui; pnpm test` → ✅ 105/105 用例通过（含 kiwi 中英 key 对等校验，
  新增 `editAddressAria` 键两边同步添加）
- 根目录 `just ci` → ✅ 全绿：
  - gofmt -l（cmd/internal/sdk/ui 全部 .go）无未格式化文件
  - `go vet ./...`、`go test ./...` 全部通过
  - ui: pnpm install（frozen-lockfile）→ typecheck → test（105/105）→ build
    （3.94s，产物 dist/assets/index-*.js 974.12 kB；仅有既有的 chunk > 500 kB
    警告，与本次改动无关）

## 浏览器冒烟

8787 / 3015 端口被用户 Vivy Studio 调试会话占用（vivy-backend pid 22900、
Vite pid 21516），未自行启动服务器——按既有约定由**用户 Studio 调试代验**，
见 acceptance.md。提醒用户：改动只存在于分离 Vite `http://127.0.0.1:3015`
（需硬刷新 Ctrl+Shift+R）；`:8787` 嵌入式页面是发行构建包，不含本轮 UI 改动。