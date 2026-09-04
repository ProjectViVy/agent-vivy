# Verification

Commands run from the governed-shell worktree:

- `go test ./internal/tools -run 'Bash|Shell|RunForeground' -count=1
  -timeout=60s` — passed.
- `go test ./internal/runtime ./internal/rpc -run 'Shell|Bash' -count=1
  -timeout=150s` — passed.
- Individual runtime lifecycle tests for safe execution, approval/deny,
  pre-run hard denial, pending/executing cancellation, restart recovery,
  bounded redaction/JSON escaping, cross-run state rejection, terminal-state
  GC, hook generation/output bounds, and opaque state references — passed.
- `TestControlShellStartIsGovernedAndStrict` — passed; pins capability
  advertisement, the exact two-field request, hard denial, no model event,
  and ordinary run-log output.

- `go test -race ./internal/logging ./internal/tools ./internal/runtime
  ./internal/rpc ./internal/tui ./sdk/tui/... -run
  'Redact|Shell|Bash|Hook|Review|Truncat|MapHistory|MessageProjection|Command|Help|Blob|EventMapper'
  -count=1 -timeout=300s` — passed.
- `go run ./sdk verify faces/tui` — passed.
- `go run ./sdk pack --face tui --out
  .workspace/tui-governed-shell-pack-20260904-v2` — passed; generation
  `gen_ef9ca514bd1ab3d1`, artifact SHA-256
  `7ab86e934aba56acca760c7f439dee8283bdbe02952b82ec8c4af67d34c9c5bb`.
  Two preceding attempts used invalid CLI forms (`--output`, then `--with
  tui`) and failed before packing; the final command uses the repository's
  actual face-organ contract.
- Real PTY smoke at 80×24: built `cmd/vivy-code`, launched it in a `cmd.exe`
  TTY, entered `!echo VIVY_SHELL_PTY_V2_OK`, observed the redacted approval
  card, approved with `y`, then observed the bounded untrusted tool result
  containing `VIVY_SHELL_PTY_V2_OK`; Ctrl+C exited cleanly — passed.
- One earlier `just ci` attempt in this worktree hit the existing timing flake
  `TestCronAtJobDeletesAfterSuccessfulRun`; the unchanged isolated rerun
  passed in 0.868s. After the final audit fixes, a fresh unchanged `just ci`
  passed end to end: format check, 197 UI tests, UI build, `go vet ./...`, all
  Go tests, headless compile, and every plugin/face vet+test slice.
- Independent read-only file/security and storage-contract re-audits both
  returned PASS with no remaining in-wave P1/P2. The attachment resolver
  hardening found during review is separately captured as
  `TUI-ATTACH-HARDEN` in `docs/TODO.md`.

No live provider/network call, production tenant Journal, or Studio state was
used or read.
