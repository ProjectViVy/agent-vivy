# Vivy Kernel Logging Contract

Normative rules for how the Vivy kernel (`vivy.exe` service path:
`cmd/vivy` + `internal/...`) emits logs. The reference architecture is
agent-diva's `logging.rs`; the Go realization is `internal/logging`.

CLI tools (`cmd/vivy-studio`, `vivy-sdk`, `tui`) print user-facing
diagnostics to stderr and are out of scope here.

## 1. One init path

All kernel logging goes through `log/slog`. There is exactly one setup
function per process kind in `internal/logging/logging.go`:

- `logging.Setup` — the `vivy.exe` service process, wired by the
  two-phase bootstrap in `cmd/vivy/main.go`:
  1. A bootstrap JSON logger on stdout handles the earliest messages
     (config load failure, logging setup failure).
  2. After config load, `logging.Setup` replaces the default logger
     (`slog.SetDefault`) and logs the `logging initialized` milestone
     with the effective level/format/dir.
- `logging.SetupWorker` — a `vivy worker` child process. It installs
  the per-worker file sink (§3) before the protocol loop starts and is
  wired only in `cmd/vivy/main.go`'s worker branch.

Never create ad-hoc `slog.Handler`s, stdlib `log.Logger`s, or
`fmt.Println` diagnostics in `internal/...`. The worker subcommand
branch is the only stdout exception (the worker protocol owns stdout).

## 2. Configuration

```yaml
logging:
  level: info          # debug | info | warn | error
  format: json         # json | text
  dir: ""              # empty = <data_dir>/logs
  retention_days: 30   # startup sweep; 0 = keep every file
  stdout: true         # mirror lines to the console + file
```

Defaults live in `config.Default()`; validation in `Config.Validate()`.
Two environment overrides exist for one-off ops launches and win over
the config file:

- `VIVY_LOG_LEVEL` — `debug|info|warn|error`
- `VIVY_LOG_FORMAT` — `json|text`

Both are parsed strictly: an invalid value aborts startup with a clear
error instead of silently keeping the configured value.

A second family, `VIVY_WORKER_LOG_DIR` / `VIVY_WORKER_LOG_LEVEL` /
`VIVY_WORKER_LOG_FORMAT`, is **not** an operator override: the
supervisor exports it when spawning `vivy worker` children and it is
consumed only by `logging.SetupWorker` (§3). Resolution precedence in
the child is the worker env, then the inherited `VIVY_LOG_*` values,
then the built-in defaults.

## 3. Destinations

- Default sink: stdout (when `stdout: true`) **plus** a daily-rotated
  file `<dir>/vivy.log.YYYY-MM-DD`. Rotation happens on the first write
  after local midnight; writes are synchronous and appends are
  line-sized, so the closer returned by `Setup` is an orderly-shutdown
  formality, not a flush dependency.
- At startup, files matching `vivy.log*` older than `retention_days`
  (by mtime) are deleted. `0` disables deletion.
- Each `vivy worker` child writes its own append-only file
  `<dir>/vivy.log.worker-<pid>`: one writer per file, no rotation and
  no sweep in the child, never stdout (the JSONL protocol owns it). The
  supervisor hands off the parent's effective level/format (resolved by
  `logging.ResolveEffective`, same precedence as `Setup`) plus the log
  dir; without the dir env the child runs sink-free as before. Because
  the file name shares the `vivy.log` prefix, the parent's startup
  retention sweep prunes a dead worker's file automatically.
- Log files are runtime scratch beside the Journal, not product
  history. They are never read back by the kernel, and Studio sessions
  must not treat them as tenant data (air gap, ST-2). The durable record
  of product behavior is the Journal/events, not the log stream.

## 4. Line format

Both handlers run with `AddSource: true`, so every line carries the
call site. JSON (default) is the machine-readable contract; `text`
is for humans (level=WARN, msg=..., fields as key=value).

## 5. Structured fields

Logs are structured; the message is a short lowercase English phrase
("journal append failed", "run failed"), and identifiers ride as
attributes. Standard keys:

| Key | Meaning |
|---|---|
| `run` | run id — include whenever a run id is in scope |
| `session` | session id |
| `seq` | event sequence number |
| `err` | the error (the only error key) |
| `tool`, `hook` | tool / governance hook name |
| `approval`, `question` | interaction ids |
| `type` | event type / journal entry type |
| `kind`, `reason`, `status` | discriminator fields |
| `method`, `path` | HTTP access lines (gateway mux) |
| `count`, `duration_ms`, `interval`, `limit` | metrics |

Rules:

- Include `run`/`session` whenever the id is in scope; do not log a
  failure of run X without naming X.
- Use `err` for errors, never `error`/`e`/`cause`.
- Never log provider keys, bot tokens, raw Journal blobs, or full user
  payloads (D-010; the tool-result boundary redacts via
  `RedactSensitive`, and the audit sink logs sizes/digests only).
- Defense in depth: the slog handler layer applies the same vocabulary
  again (`logging.Redact` in `internal/logging/redact.go`). Every record
  is pattern-redacted in its message and string attributes, and an
  attribute whose key contains `token`/`secret`/`password`/`api_key`/
  `authorization`/`credential` (case-insensitive) collapses to
  `[REDACTED]`. The guard is always on for both sinks (`Setup` and
  `SetupWorker`) with no config knob — a value that slipped past
  call-site discipline never reaches the file. `tools.RedactSensitive`
  delegates to `logging.Redact`, so both layers share one shape set and
  marker vocabulary.
- A few leaf helpers (e.g. `clampText`) legitimately have no id in
  scope; do not thread ids through signatures just to decorate one line.

## 6. Level discipline

- **debug** — diagnostics useful only while investigating (currently:
  audit digest lines). Off by default.
- **info** — lifecycle milestones and notable normal outcomes
  ("vivy starting", "logging initialized", "approval sweep expired
  approvals").
- **warn** — the operation failed or degraded but the process continues
  and the state stays consistent (journal replay fallback, budget
  breaker, hook failure).
- **error** — terminal failures that lose durable work or abort the
  process ("journal append failed", "startup aborted").

If you are unsure between `warn` and `error`: does the run/state
survive and the user recover by retrying? Then `warn`.

HTTP access lines are their own family: `internal/rpc.AccessLogMiddleware`
wraps the gateway mux and logs one line per request with
`method`/`path`/`status`/`duration_ms` — `info` normally, `debug` for
`/healthz` probes (keeps container healthcheck noise out of the info
stream), `warn` for 5xx. They never carry request or response payloads
(D-010).

## 7. Deferred (see docs/TODO.md §0.1)

None. Open logging items live in `docs/TODO.md` §0.1.
