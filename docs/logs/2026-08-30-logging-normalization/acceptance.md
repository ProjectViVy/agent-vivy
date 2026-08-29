# Acceptance — logging normalization

How a human can tell the feature works, without reading code:

1. **Log file appears.** Start `vivy.exe` (or `just run`) with default
   config. Next to the Journal, `data/logs/vivy.log.2026-08-30` (current
   date) now exists and grows; the console still shows the same JSON
   lines as before. On the next day the process switches to
   `vivy.log.<next-day>` automatically.
2. **Level is controllable.** Start with `logging.level: debug` in
   `config.yaml` (or `VIVY_LOG_LEVEL=debug`): audit digest lines
   (previously dead — Debug had no way to be enabled) now appear. Set
   `VIVY_LOG_LEVEL=error`: only failures show.
3. **Format is switchable.** `VIVY_LOG_FORMAT=text` (or
   `logging.format: text`) switches both console and file to
   `time=... level=INFO source=... msg=...` lines; `json` is the
   default and unchanged from the old stdout contract.
4. **Retention works.** Drop an old `vivy.log.2020-01-01` into the logs
   dir; it is gone after the next start (with `retention_days: 30`).
   With `retention_days: 0` it survives.
5. **One stream, no more split personality.** Approval-scheduler
   lifecycle lines now appear as structured JSON/text in the same
   stream as everything else (previously: plain text on stderr via
   stdlib `log`).
6. **Bad values fail loudly.** `VIVY_LOG_LEVEL=loud` or
   `logging.format: xml` aborts startup with a clear error naming the
   offending value; `logging:` keys in `config.yaml` are accepted
   (previously the strict decoder would reject the whole section).
