# Crush-style TUI mock skeleton — acceptance

Date: 2026-08-29

| Acceptance item | Evidence | Result |
|---|---|---|
| Fullscreen mock skeleton without gateway | `vivy tui --demo` → `view.RunDemo` | PASS |
| Crush chrome: right sidebar, `:::`, help row, gutter | `TestViewContainsCrushSkeleton`, `TestEditorUsesCrushPrompt` | PASS |
| Compact header with diagonals | `TestCompactHeaderHasDiagonals` | PASS |
| Pending approval overlay + y/n | default `sess_approval`; `TestApprovalKeyClearsGate` | PASS |
| Enter only mutates demo store | `TestEnterAppendsDemoReply` → `（demo：未接控制面）` | PASS |
| Plain REPL preserved | `--plain` still `RunREPL` + `Dial`; existing tui tests green | PASS |
| Not claimed as packed face | `FACE-TUI-1` still open; FACE-PACK §5 updated | PASS |
