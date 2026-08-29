# Restore model resolver and SQLite organism lease (2026-08-30)

## 变更内容

主线 `app.go` 已经按「settings.yaml / 冻结 ENV → 按次构造 ChatModel」接线，并在打开 SQLite Journal 时调用 `TakeOrganismLease`，但对应实现没有随主线合入，导致 `go build ./...` 失败：

- `undefined: ModelResolver` / `newModelResolver`
- `undefined: provider.NewResolvingChatModel`
- `backend.TakeOrganismLease undefined`

本迭代只补回编译所需的停放实现，不合并 WIP 提交里的 Studio 插件树、skills UI 或去掉 `runtime.mock` 的配置重构。

### 内核

- `internal/app/model.go`：`ModelResolver`。冻结 ENV 会话优先；否则读用户工作区 `settings.yaml`（含注册表 `ActiveKey`）。
- `internal/provider/resolving.go`：`NewResolvingChatModel` 每次 Generate/Stream 按 `LiveSpec` 构造底层模型。
- `Ref.Model` 改为接收 `ModelSpec{ID, APIKey, BaseURL}`；openai Ref 不再 `os.Getenv` 读密钥。mock 退出产品 Catalog，仅测试助手保留。
- SQLite `TakeOrganismLease`：独占 `vivy/organism` 租约 + heartbeat；`Close` 释放。测试 `Open` 不占租约，互不抢同一文件。

## 明确不做

- 不合并 `25b2eb6` 整棵 WIP（console / skills UI / dsh-plugin-subscriptions）
- 不删除 `config.Runtime.Mock`（产品路径已不走 Catalog mock，配置字段仍在）
- 不改 UI、不改 Studio 壳

## 变更文件

- `internal/app/model.go`, `internal/app/model_test.go`
- `internal/provider/{resolving.go,ref.go,openai.go,mockref.go,catalog.go,doc.go}` 及对应测试
- `internal/runtime/modeladapter.go`（注释）
- `internal/storage/sqlite/sqlite.go`
- `docs/TODO.md`（UI-MODEL-KEY-SCOPE 关闭）
