# CH-C7c-N1 — just ci 覆盖 plugins/* 独立 module

## Scope

- `justfile` 新增 `plugin-ci` 配方：发现 `plugins/*/go.mod` 的独立 module，
  逐 module 跑 `go vet ./...` + `go test ./...`（gofmt 由 fmt-check 统一覆盖），
  任一 module 失败即整体失败；默认不编译产物（pack 仍走 vivy-sdk 五步）。
- `ci` 配方接线：`ci: fmt-check vet test headless-compile plugin-ci ui-ci`——
  发现的缺口（ci 不过独立 module 边界）就此闭合。
- `fmt-check` glob `cmd internal sdk ui` → `cmd internal sdk ui plugins`：
  main-module 内的 `plugins/hello-fs`（无 go.mod，`go test ./...` 本就覆盖）
  此前 gofmt 无门禁，现补上。
- `plugins/qq` go.mod/go.sum 同步（`go mod tidy`）：新门禁首跑即实证发现
  的漂移——4293223（web_fetch/download）抬升主 module 依赖图后，qq 的
  `replace agent-vivy => ../..` 图需要跟随的间接版本（oauth2 v0.23→v0.30、
  gjson v1.9.3→v1.18.0、pretty v1.2.0→v1.2.1、go.sum 清理过期行），当时
  未 tidy 且无门禁拦截，vet 直接拒构建。tidy 后 vet/test 绿，无代码改动。

## 覆盖矩阵（改后）

| 路径 | gofmt | vet | test |
|---|---|---|---|
| cmd / internal / sdk / ui | fmt-check | `go vet ./...` | `go test ./...` |
| plugins/hello-fs（main module） | fmt-check（新） | `go vet ./...` | `go test ./...` |
| plugins/{dingtalk,discord,feishu,lsp,qq,telegram} | fmt-check | plugin-ci（新） | plugin-ci（新） |

## Not done

- plugin-ci 不做 `-race`、不做 pack/产物构建；race 与打包分别留在切片内
  与 vivy-sdk 五步。
- 不改插件 module 的 go.work/workspace 语义；仍以 per-module 目录为单元。
