# Verification

All commands below ran from the isolated `feat/tui-images` worktree. Go uses
`C:/Program Files/Go/bin/go.exe` because it is not on this shell's PATH.

- `gofmt` on all changed Go files and `git diff --check` — passed.
- `go test ./internal/rpc -run 'Attachment|TurnStartAttachments' -count=1 -timeout=180s` — passed.
- `go test ./internal/tui -count=1 -timeout=180s` — passed.
- `go test ./sdk/tui/... -count=1 -timeout=120s` — passed.
- `go test ./... -count=1 -timeout=120s` from `faces/tui` — passed.
- `go test ./internal/runtime -run 'Context|Compaction' -count=1 -timeout=120s` — passed.
- `go test ./internal/codeface -run 'Prepare' -count=1 -timeout=120s` — passed; includes linked-parent canonicalization where the OS permits symlink creation.
- `go test -race ./internal/rpc -run 'Attachment|TurnStartAttachments' -count=1 -timeout=240s` — passed.
- `go test -race ./internal/tui -count=1 -timeout=240s` — passed.
- `go test -race ./... -count=1 -timeout=240s` from `faces/tui` — passed.
- `go run ./sdk verify faces/tui` — passed.
- `go run ./sdk pack --face tui --out .workspace/tui-image-attachments-pack-20260904-v2` — passed; generation `gen_8778429bbc050f1e`, SHA-256 `e67ddcb05fd9f100bda900717e9ad6a126d1797fc2f95d6be0027db67d1bf027`.
- `just ci` — passed; UI typecheck/197 tests/build, Go formatting/vet/tests/headless compile, and every plugin/face gate passed.
- `vivy-code` interactive PTY smoke — passed at 80x24; `/image README.md` failed closed before filesystem resolution because the default model's image capability is unknown, displayed a local command error, did not start a model turn, and Ctrl+C exited cleanly.
- GPT-5.6-LUNA MAX final read-only audit — PASS after one repair round; the
  metadata-only DTO, packed project-root composition, REPL subscribe failure,
  rooted race-safe open and RPC frame-capacity findings were all closed.

Regression coverage includes Windows/POSIX absolute and traversal syntax,
mixed separators, NUL, regular-file enforcement, MIME spoofing and content
sniffing for PNG/JPEG/GIF/WebP, symlink containment, exact four-file and 5 MiB
boundaries, mixed inline/path count limits, rejected-turn non-persistence,
terminal-control-safe bounded display names, history DTO parity, capability
truth, retry drafts, session-bound queue snapshots,
and preservation of a later image draft when an earlier queued turn dequeues.

An earlier broad test process was stopped after it became an obsolete,
long-running snapshot while implementation was still changing; it is not
counted as a result. No live provider/network call, production tenant Journal,
or Studio state was used.
