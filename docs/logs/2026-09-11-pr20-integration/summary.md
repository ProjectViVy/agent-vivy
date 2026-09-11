# PR #20 integration summary

PR #20 was reconciled with the already-landed P4 and P5 work on `main` without
creating a second runtime, UI composition path, evidence ledger, or generated
Assembly source of truth. The combined Assembly retains the governed context,
skill, MCP, provider-profile, observer, status, full-UI, and typed Control
Action surfaces.

The integration fixed three failures exposed by the real repository gate:

- generated UI typechecks and the linked UI SDK now resolve the consuming
  application's React peer types;
- the public UI SDK MCP projection matches the current P4 host state and
  optional legacy-status contract; and
- ActionHost deterministically classifies host and per-action deadlines as
  `ErrActionTimeout`, even when a provider observes the propagated cancellation
  first.

The default Go Assembly was regenerated from the compiler source. Misplaced
`.superpowers/sdd` task reports were removed because delivery records belong
under `docs/`. No Eino runtime capability was added or replaced in this
integration; the P4/P5 Eino capability checks and import quarantine remain the
governing evidence.

Not included: PLG-P9 release conformance or a favicon asset. The browser's
cosmetic `/favicon.ico` 404 is recorded as `UI-FAVICON-404` in `docs/TODO.md`.
