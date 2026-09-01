# VC-2 headless: `vivy run "prompt"` (FACE-0 / D11)

## What changed

Vivy gains the headless face: `vivy run "prompt"` runs exactly one prompt
through the shared kernel and terminates. Behavior aligned with the FACE-0
contract (`docs/architecture/VIVY-FACE-PACK.md` §5/§14.4) and the D11 ruling
(headless lands through face assembly). Per the standing FSL-1.1-MIT
constraint, this is behavior alignment with Crush's headless mode only —
zero code was copied from Crush.

- **Face enum** (`internal/domain/face.go`): `headless` added alongside
  `web|tui|code`; `internal/runtime/face.go` validation message updated;
  `schemas/events/payloads/run.started.json` face enum extended. The run
  journal now records `face: "headless"` and provenance source `headless`.
  UI untouched (the web face never sends headless).
- **Light seam in `app.New`** (`internal/app/app.go`): new `AppOption`s —
  `WithoutEars()` skips channel partition/host/start and the channel run
  hook; `WithEventSink(sink)` fans service publishes out to an extra sink
  (`fanoutSink` over the bus). Headless reuses the full gateway composition
  (`New(ctx, cfg, opts...)`): same tools, policy engine, approvals,
  settings overlay, Recover (E2), lease conflict handling. The HTTP server,
  cron, sweeper, and worker manager are constructed but never started
  (those live in `App.Run`, which headless never calls). No separate
  faces/ assembly yet — that is F3/F4 territory.
- **`internal/app/headless.go`**: `RunHeadless` resolves/creates the session
  (`--continue` → newest session via `ListSessions()[0]`; fresh mints
  `sess_` + 16 hex, title = prompt truncated at 60 runes + `...`), then
  drives one turn with `Face: headless`. A `headlessSink` renders events:
  stdout carries only assistant text (streamed deltas as-is, or the
  completed content when nothing streamed); stderr carries `> tool_name`
  notices, tool failures, and the terminal verdict. Approval-required and
  question-required events fail loud (FACE-PACK §5: no TTY approval must
  never silently pass — the sink prints a block notice and the turn cancels
  the run; both cancel paths converge on exactly one terminal).
- **`cmd/vivy/run.go` + `main.go` dispatch**: `vivy run [--continue]`
  parses the prompt from args or piped stdin (interactive no-arg = usage
  error); installs file-only logging (`Stdout: false`) so stdout stays
  pure; bootstrap slog goes to stderr. Exit codes mirror the terminal:
  0 completed, 1 failed, 2 cancelled (signal, or approval/question block).
- **Tests** (`internal/app/headless_test.go`): scripted-model harness over
  the real composition — sink rendering (delta/completed dedupe, tool
  notices), completed turn (stdout + journal face + provenance), approval
  block (loud stderr, cancelled, exactly one terminal, no lingering
  pending approval), question block (same shape), session resolution
  (continue-newest, empty-board error, title truncation).

## What was explicitly NOT done

- No `faces/` directory / full face-pack assembly (F3), no world/package
  changes (F4): `vivy run` rides the existing binary and composition.
- No TUI changes; the UI continues to speak only `web`.
- No provider-side model metadata or cost accounting (separate VC-2 items).
- The browser-independent golden path with a real provider key was not
  exercised (no key in this environment); see verification.md for the
  fail-loud smoke that was run instead.
