# LOG-3: handler-level log redaction (defense in depth)

## What changed

Closes the last LOGGING.md §7 deferred item: the kernel's slog sinks now
redact records at the handler layer, so a credential that slipped past the
D-010 call-site discipline never reaches the log file.

- `internal/logging/redact.go` (new):
  - `logging.Redact(s)` — the kernel-wide redaction vocabulary
    (secretPattern / emailPattern, markers `[REDACTED_SECRET]` /
    `[REDACTED_EMAIL]`), single-sourced here.
  - `redactingHandler` — slog.Handler wrapper applied in both `Setup` and
    `SetupWorker`. Every record's message and string attributes are
    pattern-redacted; an attribute whose key contains
    `token`/`secret`/`password`/`passwd`/`api_key`/`apikey`/
    `authorization`/`credential` (case-insensitive, substring — group
    prefixes and naming conventions included) collapses its whole value to
    `[REDACTED]`. Group attributes recurse. `WithAttrs` pre-formatted
    attrs are redacted too. Always on — no config knob.
  - Non-string typed values pass through untouched: the tool-result
    boundary owns structured payloads; the handler must not re-render
    typed values.
- `internal/tools/security.go`: `RedactSensitive` now delegates to
  `logging.Redact` — one shape set and marker vocabulary for both the
  tool-result boundary and the log handler layer (import direction
  tools→logging is acyclic; logging stays a leaf). `promptInjectionPattern`
  stays in tools (prompt scanning is not log redaction).
- `docs/architecture/LOGGING.md`: §5 gains the defense-in-depth rule; §7
  deferred list is now empty.

## What was explicitly not done

- No config toggle for the guard (defense in depth must not be
  switchable off).
- No redaction of typed (non-string) attribute values, and no Journal /
  UI-side change — the Journal redaction contract stays at the tool
  boundary (`toolbroker`), the handler only guards the file/stdout sink.
- No new pattern shapes (webhook URLs, private keys) — the conservative
  set matches the existing boundary vocabulary; widening it is a separate
  decision.

## Files

- `internal/logging/redact.go` (new), `redact_test.go` (new)
- `internal/logging/logging.go` — wrap both sinks
- `internal/tools/security.go` — delegate RedactSensitive
- `docs/architecture/LOGGING.md` — §5 rule, §7 closure
