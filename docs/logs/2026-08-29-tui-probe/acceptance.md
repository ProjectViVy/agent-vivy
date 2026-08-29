# TUI probe Acceptance

Date: 2026-08-29

| Acceptance item | Evidence | Result |
|---|---|---|
| TTY mouth exists without a second EXE | `vivy tui` in `cmd/vivy`; no `vivy-tui.exe` | PASS |
| Same control plane as the browser | `turn/start`, `run/subscribe`, `run/event`, `approval/respond` | PASS |
| Streamed assistant text | `TestREPLTurnStreamsDeltas` prints `vivy: hi` | PASS |
| Approval is y/n → `approved`/`denied` | `TestInterpretApprovalGate` + `parseApproval` | PASS |
| Default no-arg `vivy` still starts the web gateway | `tui` is an explicit argv branch before `app.New` | PASS |
| Not claimed as packed `faces/tui` | `VIVY-FACE-PACK.md` §5 probe note; `FACE-TUI-1` open | PASS |
