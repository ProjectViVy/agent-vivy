# Verification

Base: cdcf75da, containing the independently verified D09 file-input slice.
Checks use the isolated data lane; root and production Journals are untouched.

Observed RED before correction:

- A100-reference long-ID selection exceeded the32KiB response limit; trimming
  records with an advanced cursor could silently skip selected records.
- Two UI failures treated narrow_scope/refused continuation as a successful
  empty result. Both new UI regressions failed before response handling changed.

Native Go1.26.4 focused history/cursor/domain/RPC checks PASS16.060s/0.053s/
0.194s. Broader history/reference/continuity selector PASS runtime25.594s,
domain0.048s and RPC0.862s; events selected no cases and is not test evidence.
Affected runtime/domain/RPC/events vet PASS. Cases cover restart, exact old cut,
normalized query/kinds, malicious future/foreign bounds, scope revocation,
malformed/oversized/v1 cursors, truncation/deletion and lossless narrowed paging.
The100-record regression reads every selected record exactly once after
narrowing. Independent read-only cursor review found no remaining authorization
issue after the pagination correction.

UI:10 HistoryPicker tests (final result recorded below) after real Recipe staging; staged typecheck
PASS. The recovery fixture initially clicked the session search instead of the
record search; its button target was corrected before the final passing run.
Tests preserve prior records and selection when continuation is refused.

Exact-candidate independent Linux wiring/kernel, affected Face/Web and race
results will be recorded after execution. No schema/backend SQL changed in this
cut; the129-case real PostgreSQL matrix at e9920295 verifies its storage baseline.
The owner explicitly deferred3015 browser smoke; it is not passing evidence.


Independent UI review identified a retained cursor paired with an editable draft
query after a refused replacement search. New regression failed RED, proving
query B was sent with query A's cursor. Keep the successful source/query/kind
snapshot beside its cursor and use it for Load more. The draft remains available
for recovery search; continuation never borrows its changed filter.

Final native UI suite:10 tests PASS1.68s; actual staged typecheck PASS.


Exact candidate `d61fdc2b52c8be67e00e18eddc77eda0ab0c423d` completed from the
independent ext4 checkout with its own clean pinned Laputa:

- `just wiring-ci`: PASS16 Node and105 Go cases, zero skips.
- `just kernel-ci`: PASS1810 executed cases, one existing optional rg skip;
  minimal Recipe Pack/Inspect/boot PASS1.
- Real disposable PostgreSQL16 `just test-postgres`: PASS129, zero skips;
  private container removed by trap.
- `just d02-ci`: PASS81 runtime/RPC,175 shared Face/client,1 causal App,
  13 shutdown/headless App and9 standalone headless cases, zero skips.
- `just d02-web-ci`: staged typecheck PASS,102 Web cases and2 selected-UI Pack
  cases PASS, zero skips.
- `pnpm --dir ui test -- src/components/chat/HistoryPicker.test.tsx` ran the full
  configured UI suite on this host:84 files/658 tests PASS16.85s, zero skips.
- `go test -race -tags=vivy_headless ./internal/runtime -run
  'TestD09History|TestHistory|TestReference|TestContinuity' -count=1 -timeout=10m`:
  PASS59.052s.
- Final clean-worktree assertion and script exit0 passed.

The independent UI reviewer confirmed the successful-query snapshot correction
closed the final finding; review did not substitute for the checks above.
Standard3015 real-path smoke remains owner-deferred, and D09 reference/delivery
closure remains open.
