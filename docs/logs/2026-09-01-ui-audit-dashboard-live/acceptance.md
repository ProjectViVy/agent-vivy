# Acceptance

## Manual acceptance

1. Open `http://127.0.0.1:3015` and go to Dashboard → "Session" (Overview)
   tab: all three numbers come from the real backend—the session count matches
   the count shown in Settings/sidebar; refreshing after creating/deleting a
   session changes the number; when approvals/questions are pending, the Review
   Center pending count matches the third number.
2. The "Recent activity" card no longer appears (it previously displayed fake
   demo entries such as "Daily report generated / skill change pending /
   scheduled task completed").
3. Disconnect the backend (or stop `just run`) and reopen Overview: it shows an
   error banner + retry button instead of the fake 12/2/1.
4. Token Tab behavior is unchanged (it was already real); Trajectory Tab remains
   demo trajectory data (known and tracked by the UI-TRAJECTORY-DEMO item).
5. `vivy.demo.dashboard` is no longer written to localStorage.

## Acceptance criteria

- `just ci` is green (no tsc/eslint/vitest/build breakage).
- The full `just ui-e2e` suite is green (there is no dashboard-specific spec, so
  the full suite is the regression fallback).
- In a split development pair (`just run` + `pnpm dev`), browser inspection
  confirms that the Overview numbers match the RPC response.
