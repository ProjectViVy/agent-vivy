# PR #80 closeout

PR #84 carries the pending-run recovery work from PR #80 onto current main and
adds the Claude EinoExt stream race fix required by the repository race gate.
The runtime changes persist cleanup intent before changing request/run state;
startup retries those intents even when the Run is already terminal. Row
cancellation, cancellation events, the single Run terminal, and Plan review
cancellation remain retryable and idempotent across failed writes and process
restart.

The candidate upgrades Eino to v0.9.21 without converting checkpoints or
bypassing exact-version checks. It also vendors only the pinned Claude EinoExt
component with a narrow worker-local error fix, records its source/license and
removal condition, and copies its module manifest into Docker's dependency
layer. The final source tested here is `8ec876d3035c0ed11e5ac5463b3e5abc29565506`,
integrated on main `e96092966cfbfa443648374948e7e7e16dd01683`.

The verified rollback boundary is directional. Upgrade from the historical
v0.9.13 binary to the fixed v0.9.21 candidate can clean pending Approval,
Question, and Plan requests. Direct rollback to an unpatched historical binary
while any such request is pending is unsupported. Before a historical rollback,
stop admitting work, finish or cancel under the fixed candidate, verify all
requests, cleanup intents, terminal states, and cancellation events, then restart
the candidate and verify again. No compatibility rollback build is supplied.

This closeout does not migrate the full AgenticModel path and does not include
PR #79 Recap/Subject work. Provider milestones B5/B6/B7 remain open.
