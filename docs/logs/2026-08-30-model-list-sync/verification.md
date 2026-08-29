# verification.md — 2026-08-30 model-list-sync

执行环境：`agent-vivy-model-sync` worktree（`feat/model-list-sync`，独立于
`feat/tui-live-client` 与根树 main）。他会话未改动；租户 Journal
（`data/vivy.db`、`data/demo/`、`data/workspaces/`）未触碰；冒烟数据全部在
worktree 本地临时目录（`.smoke/`、`ui/.e2e-workdir/`，已清理）。

## 命令与结果

### Go 侧（单元 + 集成，httptest 真实 HTTP 往返）

```text
go vet ./internal/provider ./internal/rpc          # PASS
go test ./internal/provider ./internal/rpc -run 'TestModelList|TestProviderRefreshRPC|TestProviderRegistryRPC' -count=1
# ok  agent-vivy/internal/provider   0.667s
# ok  agent-vivy/internal/rpc        3.468s（含 wire 回归断言）
```

覆盖点：

- `discover_test.go`：200 正常解析 + Bearer 头校验；base_url 尾斜杠 → `/v1/models`；
  去重/trim/空 id；HTTP 401 报错且不含密钥；非法 JSON；连接拒绝（URL 与密钥均
  不出现在错误里）。
- `control_test.go TestProviderRefreshRPC`：按 id 刷新（并集 [upstream-a,
  upstream-b, manual-a]，`api_key_set` 保持，settings.yaml 中 `ApiKey` 原样、
  密钥不出现在响应体）；**空模型列表 wire 恒为 `[]`（回归：`models:null` 会让前端
  校验丢弃该条目，见 summary）**；上游 500 不落盘、不通知变更；目录供应商无注册表
  行克隆为新条目（无密钥不发 Authorization 头）；未知 id → not_found；Anthropic
  （按 bundle 与按条目 id）→ invalid_params；read-only → conflict；Frozen →
  conflict；`initialize` capabilities 含 `settings.providers.refresh`。

注：顶层 `go build ./...` 需 `ui/dist`（存量 UI-CI-BOOTSTRAP，见
`docs/TODO.md` §0.1）；headless 编译由 `just ci` 的 `headless-compile`
（`-tags vivy_headless`）覆盖。

### UI 侧（typecheck + 单测）

```text
cd ui; pnpm install --frozen-lockfile; pnpm typecheck   # PASS (tsc --noEmit)
pnpm test                                                # 162/162 passed（含 api.test.ts 新增断言）
```

### 浏览器真实路径（Playwright e2e，真渲染 + 自起后端 :8799 独立地址）

```text
cd ui; pnpm exec playwright test e2e/model-refresh.spec.ts   # 1 passed (9.0s)
```

流程全绿：新增自定义供应商（本地 /models）→ 空列表提示 → 点「刷新」出现
gpt-4o/gpt-4o-mini + 「已从上游同步 2 个模型」，上游收到 `Authorization:
Bearer sk-e2e-secret`（测试内密钥）；手动新增 my-local-model → 再刷新 → 同步
3 个模型且手工条目保留；reload 后三者仍在、密钥不回显。

> 说明：日常开发地址 `http://127.0.0.1:3015`/`:8787` 被并行会话的 split pair 占用，
> 本迭代未在其上重启 dev 服务器（不干扰对方）；改用 e2e 内置隔离
> `127.0.0.1:8799` 走真实浏览器 + 真实后端 + 真实 HTTP 上行，等价覆盖页面点击路径。

### 独立 RPC 冒烟（真实 `go run ./cmd/vivy` + 真实 WS 协议 + 隔离 data_dir）

PowerShell 驱动的临时冒烟（`.smoke/`，已删除）：`initialize` capabilities 含
`settings.providers.refresh`；`settings/providers/upsert` 持久化且密钥不回传；
`settings/providers/refresh` 返回 [smoke-alpha, smoke-beta, manual-local]
（并集 + 保留密钥）；`settings/providers` 回读一致；未知 id → not_found
（code=-32004）。落盘文件 `settings.yaml` 中 `api_key` 原样、`models` 为并集。

### 完整门禁

```text
just ci    # fmt-check + vet + test + headless-compile + ui-ci(typecheck/test/build)
```

结果：全绿（见 acceptance 受理记录）。

## 未验证项与原因

- `:3015` 分体 dev 服务器上的手动视觉走查：端口被并行会话占用，未重启；已由 e2e
  浏览器路径等价覆盖页面行为。
- Anthropic 原生端点在线刷新：协议不支持，属设计期望（按钮不显示 + RPC 拒绝）。
- 预置 e2e 规格中 `runtime.spec.ts` / `welcome-wizard.spec.ts` 为存量过期断言
  （UI-E2E-STALE / UI-E2E-DRAW），不在本迭代范围。