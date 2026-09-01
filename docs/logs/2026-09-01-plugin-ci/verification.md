# Verification

| Command | Result |
|---|---|
| 逐 module 基线（改前手跑 `go vet && go test`） | dingtalk/discord/feishu/lsp/telegram ok；**qq vet 拒构建**（go.mod updates needed）→ 新门禁首跑即捕获真实漂移 |
| `go mod tidy -diff`（plugins/qq，预览） | oauth2/gjson/pretty 间接版本抬升 + go.sum 过期行清理 |
| `cd plugins/qq && go mod tidy && go vet ./... && go test ./...` | go.mod/go.sum 更新；`ok example.com/vivy/plugins/qq` |
| `gofmt -l` over `rg --files plugins -g '*.go'` | 空（全部格式合规），fmt-check 扩 glob 后无回归 |
| `just plugin-ci`（配方单跑） | 6 module（dingtalk/discord/feishu/lsp/qq/telegram）逐个 vet+test 全 ok |
| `just ci`（完整门禁，含接线后的 plugin-ci） | 绿：fmt-check（新 glob 含 plugins）+ vet + test + headless-compile + plugin-ci + ui-ci（tsc / 195 vitest / build） |

失败传播证据：qq 漂移场景本身即 vet 失败 → 配方 `$fail=1` → 非零退出
（基线手跑时实证）；test 失败走同一 `$LASTEXITCODE` 通道。

无 UI/运行时行为改动，`just ui-e2e` 不适用本条（未单独重跑）。
