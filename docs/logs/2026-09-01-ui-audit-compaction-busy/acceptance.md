# Acceptance

## Manual acceptance

1. Open `http://127.0.0.1:3015` and go to Settings → General → Context
   compaction: with no run active, "Compact now" remains clickable.
2. Start a turn on the chat page (a long reply makes it easier to observe), then
   return to Settings: the button becomes disabled, and an amber hint appears at
   the end of the button row: "A run is in flight; compaction runs inside it."
3. After the run ends (or after clicking "Refresh usage"), the button becomes
   clickable again.
4. The button is likewise disabled when a background run exists
   (`background/list` is non-terminal).
5. In the English interface, the hint is "A run is in flight; compaction runs
   inside it. Wait for it to finish." and no raw i18n key appears.

## Acceptance criteria

- `just ci` is green (tsc/eslint/vitest/build has no breakage; the compaction e2e
  spec asserts labels only and is unaffected by button disabling).
- The 409 path remains: a click during a race still produces an error message in
  the feedback area.
