# 验证

- `git merge --no-ff --no-edit feat/chatbox-buttons`：通过，合并提交 `80b2bfb`。
- `git merge --no-ff --no-edit feat/skill-marketplace`：通过，冲突解决后提交 `349b9ac`。
- `git merge --no-ff --no-edit feat/cron-closed-loop`：通过，冲突解决后提交 `1360659`。
- `git merge --no-ff --no-edit feat/channel-super-contract`：通过，冲突解决后提交 `a4e0c1a`。
- `just ci`：通过（Go fmt/vet/test、headless test、UI typecheck、175 个 Vitest、Vite build）。
- split 冒烟：`just run` 启动控制面 `127.0.0.1:8787`；`pnpm dev --host 127.0.0.1` 启动 Vite `127.0.0.1:3015`；两个地址的 HTTP 请求均成功。
- 浏览器可视化验证：未完成。浏览器运行时返回 `agent.browsers.list() = []`，当前环境没有可用实例；未改用无关的浏览器工具替代。
- 合并后 `git branch --no-merged main` 只剩 `wip/pre-submodule-root-20260829`。
