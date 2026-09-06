# 2026-09-07 Session auto-title never ran (audit follow-up)

## Problem

Human audit of the F7 window-title fix showed the title still stuck on the
bare brand after a normal chat. Diagnosis: the kernel's auto-titler
(`internal/runtime/titler.go` + `internal/provider/titler.go`, VC-2) was
already fully built — LLM summary over a small→main model chain with a
first-user-message truncation fallback — but it never executed for the VIVY
CODE TUI, for three stacked reasons:

1. **Sessions were born titled.** The TUI face passed `Title: "VIVY CODE"`
   and the live driver defaulted `/new` titles to the shell brand, so the
   kernel's "already titled → skip" guard hit every time. The RPC layer even
   documents the intended contract ("an empty title stays empty … so the
   auto-titler can name it after the first exchange") — the client violated
   it.
2. **Failed runs never titled.** `maybeAutoTitle` fired only on the run
   completed path; the audit instance's two runs had both failed on tool
   errors, and any failed first exchange left the session nameless.
3. **The TUI never re-checked.** The auto-title lands asynchronously seconds
   after the run's terminal event (it is an LLM call), but the TUI refreshed
   the sidebar exactly once at run end, so even a generated title was
   invisible until restart.

## What changed

- `internal/runtime/service.go` — `consume` also fires `maybeAutoTitle`
  after a failed run's terminal event; the generator's truncation fallback
  names sessions whose first exchange failed (per the product rule: LLM
  summary when a model is reachable, otherwise the head of the first user
  message — `provider.ChainTitler` already implements exactly this).
- `sdk/tui/live/controller.go` — boot and `/new` create sessions untitled
  (the shell brand stays only in the footer); dead `trimTitle` removed; a
  paced sidebar re-check (`titleRefreshCmdIfDue`, 4s cadence, only after a
  run has been seen, stops once a title lands) carries the async title into
  the rail, the sessions list, and the window-title sync.
- Tests: `TestAutoTitleFiresAfterFailedRun` (runtime, empty scripted model =
  failing run, generator still called and title persisted);
  `TestLiveBootCreatesUntitledSession` + `TestNewSessionKeepsEmptyTitle`
  (live fake transport asserts the empty-title contract).

## Explicitly not done

- No change to the titler policy itself: `ChainTitler`'s small→main LLM
  chain with the 60-rune truncation fallback already matches the agreed
  rule.
- The headless face's own prompt-truncation title rule is untouched; only
  the VIVY CODE TUI client switched to the untitled-birth contract.
- Not pushed (requires explicit authorization).
