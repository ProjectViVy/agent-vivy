# VCP C2 — verification

## Commands run

```text
go build ./...                                            # clean
go test ./sdk/tui/...                                     # all green (command, face, i18n, live, stream, view)
go test ./internal/rpc                                    # green incl. TestDiagnosticsBundleRPC
go test ./sdk/internal/conformance/                       # green after digest re-pin (67.7s)
```

## Coverage added

- `sdk/tui/view/tree_test.go`
  - `TestFlattenTreeOrdersDepthFirst` — DFS layout: roots, created_at ordering, depth, orphan parent appended flat.
  - `TestTreeDialogRendersAndSelects` — `/tree` opens loading dialog, `TreeMsg` populates rows, ↓+Enter switches to the clone (`driver.selected == "clone-1"`).
  - `TestTreeDialogEscAndError` — RPC error surfaces in dialog, Esc closes.
  - `TestCopyLastAssistantEmptyAndFilled` — empty history → `copyEmpty` overlay; filled history → clipboard command emitted.
- `internal/rpc/diagnostics_test.go::TestDiagnosticsBundleRPC` — bundle file lands in the wired dir (`bug-sess-x-*.md`), contains version/session/log tail, `diagnostics.bundle` capability advertised only when wired, unwired dir/diagnostics → `MethodNotFound`.
- `sdk/tui/command/command_test.go` — the 7 new specs added to the all-localized-descriptions table (31 commands).

## Acceptance checklist (plan)

- [x] `/tree` — navigable indented list over `session/tree`; arrows select, Enter switches, Esc closes.
- [x] `/clone` → `session/clone` + switch.
- [x] `/import <path>` → `session/import`.
- [x] `/export` → `session/export` + shown path.
- [x] `/copy` last assistant text → OSC 52.
- [x] `/bug` diagnostics bundle into exports dir.
- [x] `/debug` overlay tail of runtime log.
- [x] Specs parse + arg validation; tree renders fixture; export path shown.
- [ ] `just ci` — deferred to epic end per story convention.

Note: full interactive loop (run → fork → /tree → switch → /export) is covered piecewise by tests above; a manual TUI smoke is still owed at epic end alongside `just ci`.
