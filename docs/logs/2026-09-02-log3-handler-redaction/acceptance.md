# Acceptance — LOG-3

How a human can tell it worked:

1. Start `vivy.exe` (or `just run`) and trigger any log line — the file
   sink under `<data_dir>/logs/` behaves exactly as before for ordinary
   fields (run ids, model names, paths, durations).
2. Force a credential-shaped value into a log record (e.g. a debug build
   or a test that logs a config error carrying a provider key). The line
   in the file shows `[REDACTED_SECRET]` / `[REDACTED_EMAIL]` instead of
   the token, and any attribute whose key looks like a credential
   (`api_key`, `bot_token`, `Authorization`, …) shows `[REDACTED]`.
3. The same redaction applies in `vivy worker` child files
   (`vivy.log.worker-<pid>`), because both sinks wrap the same guard.
4. Model-facing behavior is unchanged: the Journal, UI event stream, and
   tool results keep their existing redaction (the tool-result boundary);
   the handler only guards the log stream.

Regression guarantees: `RedactSensitive` keeps its markers and behavior
(tools tests pass unchanged via delegation); LOGGING.md §5 documents the
always-on guard and §7 now has no deferred items.
