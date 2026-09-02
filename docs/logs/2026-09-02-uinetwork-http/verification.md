# 验证：UI-NETWORK-HTTP http_request 设置面

## 命令与结果（真实记录）

聚焦单测（提交前快速回路，全部通过）：

```text
go build ./...                                     → ok
go test ./internal/runtime/ -run 'TestEinoHTTP'    → ok（含新增 TestEinoHTTPBackendSetConfigLiveApply）
go test ./internal/app/settings/                   → ok（含 TestSaveAndLoadHTTPOverlay / TestValidateHTTPOverlayBounds）
go test ./internal/rpc/                            → ok（含 TestControlHandlerHTTPSettingsSegment）
go test ./internal/config/                         → ok
cd ui; pnpm exec tsc -b --noEmit                   → 干净
```

产品门禁（每轮均为后台完整 `just ci` + `just ui-e2e`，tail-check 自有日志）：

- 第 1 轮：`just ci` → **CI-EXIT:0**（/tmp/ci-uinet-http.log）；
  `just ui-e2e` → E2E-EXIT:1，唯一失败为本片新增规格
  `network-tools-setting.spec.ts › edits the http_request allowlist and timeout`。
- 第 2 轮（根因修复后）：`just ci` → **CI-EXIT:0**（/tmp/ci-uinet-http2.log）；
  `just ui-e2e` → **E2E-EXIT:0**（/tmp/uie2e-uinet-http2.log，17 passed / 1 skipped，
  含本片 http 区块规格与既有 network_search 规格的按钮作用域修订）。

## 第 1 轮失败根因与修复（值得记的一课）

失败现象：http 规格开场断言「无『已用设置覆盖』徽标」失败——初载页面即有徽标，
且白名单/超时显示的是配置默认值。

根因：`settings/get` 恒返回 `http` 段（值类型，含 config 回退），而
`settingsUpdateFrom` 以「段存在即回传」的方式整文档携带 → 其他分区（network_search
规格）保存时把配置默认值当 `http` 覆盖层写回了 settings.yaml，`overlay_set` 误亮。

修复：`settingsUpdateFrom` 只在 `settings.http.overlay_set === true` 时回传
`http` 段——没有覆盖层时，其他分区的保存不得凭空造出覆盖层。e2e 工作目录
（`ui/.e2e-workdir/state/settings.yaml`）每轮 globalSetup 重建，修复后复跑即净。

## 测试覆盖清单

- `internal/runtime/http_request_test.go`：SetConfig 实时替换白名单（含 `*.域名`
  只覆盖子域、裸域不覆盖、URL 形态归一、trim）、超时钳制（10 默认 / 120 上限 /
  显式值）、nil hosts 保留现表。
- `internal/app/settings/settings_test.go`：http 覆盖层保存/回读 round-trip、
  非零判定、空覆盖层归一化为 nil、超时 -1/121 拒绝、空白 host 拒绝。
- `internal/rpc/control_test.go`：`TestControlHandlerHTTPSettingsSegment` —
  get 配置回退 + 无覆盖；update 写入/回显/持久化；显式空列表=拒绝全部；
  timeout 0 保留现值；恢复配置值；999 被文档校验拒绝；OnSettingsChanged 恰好 3 次。
- `ui/e2e/network-tools-setting.spec.ts`：真实路径规格 — 既有 network_search
  规格（保存按钮 `.first()` 作用域修订）+ 新增 http 规格（初载无徽标、999 禁存、
  45 保存回执、刷新回显 + 徽标、恢复配置默认）。
