# Summary — VC-2 会话自动标题（small→large chain）

## What changed

Sessions now get named automatically after the first exchange, Crush-style
(small model first, large fallback), with zero schema migration.

- **Empty title = untitled marker.** `session/create` no longer defaults an
  empty title to "New session"; an empty stored title marks the session as
  untitled and is the auto-titler's trigger. The UI renders the localized
  placeholder via the existing `errors.newSessionDefault` i18n key (muted
  styling) instead of baking a string in at creation time.
- **Auto-titler in the runtime service** (`internal/runtime/titler.go`):
  after the `run.completed` terminal is emitted, `maybeAutoTitle` spawns a
  goroutine joined to `s.wg` (so `WaitIdle` and shutdown drain it). It skips
  any session whose title is non-empty — user renames, Channel host sessions
  (`channel/<source>/<chat>`), headless explicit titles, cron sessions
  (`Cron: <name>`) are all protected. The rename is re-checked against the
  stored title right before `RenameSession` to close a rename race. All
  failures are best-effort: a structured log line, the session stays
  untitled.
- **Title chain** (`internal/provider/titler.go`, exported `ChainTitler` /
  `TitleCandidates`): optional configured small model first, then the main
  chat model, then a plain truncation of the first user message (60 runes)
  as a never-failing tail — the chain never fails hard, and a session is
  still named when every model is unusable. Each model attempt gets a 20s
  timeout and a 40-token cap (`model.WithMaxTokens`); replies are trimmed
  and empty replies fall through to the next candidate.
- **Small model rides the active provider (D9 one data source).**
  `runtime.small_model` is a bare model id resolved against the active
  provider's spec via `modelOverrideSource`, which pins only `Model` on a
  copy of the live `LiveSpec` (same base URL and key). Provider/model
  management stays one data source; an unknown or broken small model id
  simply fails at `Generate` and the chain falls to the main model.
- **Config**: `runtime.small_model` (trimmed; rejects newlines/NUL) plus a
  commented example in `config.example.yaml`.
- **Layering**: eino imports are quarantined to `internal/runtime` +
  `internal/provider` (D-007, guarded by `TestEinoImportsQuarantined`). The
  titler was first drafted in `internal/app` and the importlint guard
  rejected it in `just ci`; it moved to `internal/provider` (the layer that
  already owns model construction), with `internal/runtime` keeping only the
  eino-free `TitleGenerator` interface the service depends on.
- **RPC contract test**: `session/create` with a whitespace-only title
  stores an empty title (regression test for the removed defaulting).
- **UI**: `SessionDrawer` shows the placeholder label for untitled sessions
  (display, search filter, delete confirm); rename still seeds from the raw
  title. The demo API keeps its own default label (demo mode has no
  auto-titler).

## Explicitly not done

- **TUI display of untitled sessions** — out of scope for this slice.
- No dedicated browser smoke of the placeholder (see `verification.md`);
  the display logic is covered by the store test and the drawer unit-level
  change is a three-line label helper.
- No eino-ext/claude work (separate VC-2 item, still open).
- Demo mode has no auto-titler by design (no real provider in demo).

## Constraints honored

- Crush is FSL-1.1-MIT: only behavior/protocol alignment (the small→large
  title chain and the 40-token budget mirror Crush's documented behavior);
  zero code was copied.
- No features beyond Crush's surface were added: the chain and config knob
  exist to map Crush's small→large titling onto Vivy's single-provider
  composition, and the truncation fallback keeps Vivy's existing
  never-fail convention.
