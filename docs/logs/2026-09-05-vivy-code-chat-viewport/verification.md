# Verification

## Targeted automated checks

- `go test ./sdk/tui/view ./internal/tui` — PASS.
- `cd faces/tui; go test ./...` — PASS.
- Tests cover initial bottom-follow, page and wheel scrolling, paused streaming, send/End resume, session-switch reset, overlay mouse isolation, resize clamping, ANSI/grapheme truncation and deletion, hostile tool-card content, narrow widths, and packed initial-prompt one-shot behavior.
- `git diff --check` — PASS.

## Product gate

- Final code-stable `just ci` — PASS: formatting, UI typecheck, 201 UI tests,
  production UI build, Go vet/full tests, headless compilation, and every
  plugin/face vet+test slice passed.
- `just vivy-code` and `vivy-code.exe --help` — PASS; the independent
  headless-tagged executable built and printed the private-instance contract.

## Real path

- A native Windows PTY launch was attempted with isolated
  `VIVY_USER_HOME=.workspace/chat-viewport-smoke-home`, but this execution host
  again failed before process startup with `CreateProcessW` OS error
  `-1073283067` (`FormatMessageW` error 317). No runtime directory was created
  and no interactive pass is claimed. The real Bubble Tea update loop is
  covered by shared/built-in/packed tests; executable construction and the
  non-interactive `--help` process launch passed.

## Review

- Three GPT-5.6-LUNA MAX specialists separately audited mouse/focus parity, shared renderer Unicode/ANSI safety, and built-in/packed live-driver lifecycle boundaries.
- Their findings drove non-sidebar wheel ownership, complete mouse isolation behind overlays, explicit session reset, packed initial-prompt consumption, grapheme-safe deletion, terminal-data sanitization, narrow-width invariants, and the deferred-anchor/performance items in `docs/TODO.md`.
