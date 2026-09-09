# CH-C7c-N1 — just ci covers independent modules under plugins/*

## Scope

- `justfile` adds the `plugin-ci` recipe: it discovers independent modules at
  `plugins/*/go.mod` and runs `go vet ./...` + `go test ./...` in each module
  (gofmt is covered centrally by fmt-check); failure of any module fails the
  whole recipe. Artifacts are not built by default (pack still follows the five
  vivy-sdk steps).
- The `ci` recipe is wired as `ci: fmt-check vet test headless-compile plugin-ci
  ui-ci`, closing the discovered gap where ci did not cross independent module
  boundaries.
- The `fmt-check` glob changes from `cmd internal sdk ui` to
  `cmd internal sdk ui plugins`: `plugins/hello-fs` is in the main module (has no
  go.mod and is already covered by `go test ./...`), but previously had no gofmt
  gate; it is now covered.
- `plugins/qq` go.mod/go.sum were synchronized with `go mod tidy`. The first run
  of the new gate exposed real drift: after 4293223 (web_fetch/download) raised
  the main module's dependency graph, the qq `replace agent-vivy => ../..` graph
  needed corresponding indirect-version updates (oauth2 v0.23→v0.30,
  gjson v1.9.3→v1.18.0, pretty v1.2.0→v1.2.1, plus removal of stale go.sum
  lines). It had not been tidied and no gate caught it, so vet rejected the build
  outright. After tidy, vet/test were green, with no code changes.

## Coverage matrix (after the change)

| Path | gofmt | vet | test |
|---|---|---|---|
| cmd / internal / sdk / ui | fmt-check | `go vet ./...` | `go test ./...` |
| plugins/hello-fs (main module) | fmt-check (new) | `go vet ./...` | `go test ./...` |
| plugins/{dingtalk,discord,feishu,lsp,qq,telegram} | fmt-check | plugin-ci (new) | plugin-ci (new) |

## Not done

- plugin-ci does not run `-race` or pack/build artifacts; race and packaging
  remain within the individual slices and the five vivy-sdk steps.
- The go.work/workspace semantics of plugin modules are unchanged; each
  per-module directory remains the unit of execution.
