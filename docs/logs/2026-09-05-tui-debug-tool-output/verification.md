# Verification

## 自动化

- `go test ./internal/config ./sdk/tui/view ./sdk/tui/face`：通过。
- `go test -tags vivy_headless ./internal/codeface ./cmd/vivy ./cmd/vivy-code`：通过。
- `just ci`：通过。包括 fmt-check、UI typecheck、24 个 Vitest 文件 / 201 项测试、UI build、Go vet、全仓 Go test、headless compile，以及全部 plugin/face 独立模块检查。

## 真实路径 smoke

- `just vivy-code`：成功构建真实 `vivy-code.exe`。
- 使用隔离的 `VIVY_USER_HOME` 启动默认配置：进程保持运行并进入交互式 TUI loop；2 秒后按测试计划终止。
- 使用临时 `VIVY_CONFIG`（`tui.debug: true`）再次启动同一 EXE：配置通过严格解析，进程保持运行并进入交互式 TUI loop；2 秒后按测试计划终止。
- 两次 smoke 的临时 EXE、配置和用户目录均已删除；没有读取或写入现有 tenant Journal。
