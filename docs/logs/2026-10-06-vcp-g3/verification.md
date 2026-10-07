# G3 verification

## Commands run

```text
go build ./...
go test ./sdk/tui/... -count=1
go test ./internal/rpc/ -count=1
(cd faces/tui && go test ./... -count=1)
go test ./sdk/internal/conformance/ -count=1
```

## Results

- `sdk/tui/view/g3_test.go` — 8 tests, all green:
  - match computation across content + tool fields, case-insensitive;
  - jump cursor advance/wrap/reverse + Esc restores saved scroll & follow;
  - prompt-jump prev/next on a tall transcript (viewport actually moves);
  - search-mode key routing (runes/backspace/esc);
  - `resolveEditor` env priority + PATH fallback;
  - `runExternalEditor` round-trip via a fake `$EDITOR` script, incl. missing
    binary and nil-editor errors;
  - `linkifyOSC8` wraps URL, preserves display text, strips ANSI from the
    target, leaves plain lines untouched;
  - `RegisterToolRenderer` dispatch + nil-return fallback + unregister;
  - `heroCountsLine` lists known facts, skips unknown ones, always lists
    sessions.
- `sdk/tui/view` suite green incl. pre-existing chrome/locale/hero tests
  (new i18n keys registered en+zh).
- `internal/rpc` suite green (sidebar tests still pass with the new fields).
- `faces/tui` module tests green; no source changes → digest stays pinned.
- Conformance green; internal digest re-pinned to `5c02293b…`
  (internal/rpc/sidebar.go changed).
- `just ci` deferred to H1 per plan.
