# 2026-08-30 — Kernel logging normalization (log-output contract)

## What changed

Vivy's log output is now standardized on a single contract, modeled on
agent-diva's `logging.rs` (Rust/tracing reference):

- **New `internal/logging` package** — the one init point
  (`logging.Setup`): level/format resolution with strict env overrides
  (`VIVY_LOG_LEVEL`, `VIVY_LOG_FORMAT`), a small dependency-free
  daily-rotating file writer, and a startup retention sweep.
- **File sink** — kernel logs mirror stdout and append to
  `<data_dir>/logs/vivy.log.YYYY-MM-DD` (default retention 30 days,
  `0` = keep all). Writes are synchronous; rotation at local midnight.
- **Config** — new `logging:` section (level / format / dir /
  retention_days / stdout) with defaults in `config.Default()`,
  enum/negative validation in `Config.Validate()`, documented in
  `config.example.yaml`. `Config.LogDirectory()` derives
  `<data_dir>/logs` when `dir` is empty.
- **main.go two-phase wiring** — bootstrap stdout-JSON logger for the
  earliest lines (config load), then `logging.Setup` +
  `slog.SetDefault` + a `logging initialized` milestone line with the
  effective settings.
- **Call-site normalization** — `internal/runtime/approval_scheduler.go`
  migrated from stdlib `log` (plain text → stderr, the only non-slog
  logger in the kernel) to structured `slog`; missing run ids added at
  `service.go` ("run failed") and `hooks.go` (post-tool hook failure).
  `mapper.go` `clampText` keeps no id (none in scope; documented).
- **Contract doc** — `docs/architecture/LOGGING.md` (init path, config,
  destinations, standard field keys, level discipline, D-010 redaction
  boundary). `AGENTS.md` "Secrets, errors, and logs in code" now points
  at it.

Field-key naming keeps the codebase's existing vocabulary (`run`,
`session`, `seq`, `err`, `tool`, `hook`, `approval`, `question`,
`type`, `kind`, `reason`, `status`) rather than porting agent-diva's
Rust-flavored `run_id`/`error`; the keys are now written down and
enforced by review convention.

## Explicitly not done (captured in docs/TODO.md §0.1)

- LOG-1: file logging for `vivy worker` child processes (multi-process
  writer design needed first; worker stdout stays protocol-owned).
- LOG-2: HTTP access-log middleware on the gateway mux.
- LOG-3: handler-level redaction as defense in depth behind the D-010
  call-site discipline.
- No new log lines beyond the `logging initialized` milestone; this
  iteration normalizes output, it does not add coverage.
- Studio submodule untouched; `cmd/vivy-studio`, `vivy-sdk`, and `tui`
  user-facing stderr output are out of scope by decision.
