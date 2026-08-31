# VC-1g-2 验证记录

日期：2026-08-31

## 命令与结果

| 命令 | 结果 |
| --- | --- |
| `go build ./...` | exit 0 |
| `go vet ./internal/rpc ./internal/runtime ./internal/storage/... ./internal/domain` | clean |
| `go test ./internal/rpc ./internal/runtime ./internal/storage/sqlite ./internal/domain` | ok（含新增 `TestTurnStartAttachmentsValidationAndRoundTrip`、`TestBuildRunContextProjectsImageAttachments`、`TestMessagesPersistImageAttachments`） |
| `cd ui && pnpm typecheck` | exit 0 |
| `cd ui && pnpm test` | 24 files / 195 tests passed（新增 store 用例：附件随消息排队并在完成后携附件派发；既有派发断言更新为 5 参） |
| `just ci`（完整门禁） | 首跑在 `fmt-check` 失败（control.go 结构体 tag 对齐），`gofmt -w` 后重跑 exit 0 |

## just ci 备注

- 第一轮 `just ci` exit 1：`fmt-check` 报 `internal/rpc/control.go` 未格式化
  （`messageResult` 新增字段后 struct tag 未对齐）。`gofmt -w` 修复，
  复跑 `just ci` exit 0（日志 `/tmp/just-ci-vc1g2b.log`）。

## Smoke 政策

本片为用户可见变更（UI 贴图/选图）。按 `smoke-for-user-visible-change` 应在
`http://127.0.0.1:3015` 走真实链路；但端到端图片轮次需要真实 provider key
（TEST-1 移除 mock provider 后无本地假模型），本轮无 key，无法产出真实
vision 响应。例外已按惯例记录于此。

已替代执行的真实验证：

- `internal/rpc` 集成测试通过真实 RPC Handler 走完整链路：非法 mime / 非法
  base64 / 空 data / 超 5MiB / 超 4 张 → InvalidParams；合法 png → run 跑完 →
  `session/messages` 回传同名同 mime 且 base64 data URL 完全回环。
- `internal/runtime` 断言 `buildRunContext` 把带图用户消息投影为 eino
  `UserInputMultiContent`（text part + image part，Base64Data/MIMEType 正确，
  Content 置空），且图片字节不计入文本 byte 预算。
- `internal/storage/sqlite` 断言附件持久化、顺序、DeleteSession 清理。
- UI store 测试断言附件经队列派发 5 参透传 `api.startTurn`。
- UI 组件层面：`pnpm dev` + `just run` 分裂对在浏览器人工走查（选择文件、
  粘贴截图、缩略图移除、门禁提示、发送后气泡缩略图）留待有 key 的下一轮
  与 VC-2 SupportsImages 门控一起验收（见 acceptance.md 的人工路径）。
