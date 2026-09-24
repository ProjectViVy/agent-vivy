# Acceptance

Hold a Goal wake at its precommit workspace barrier, start shutdown, and release it. No `goal.round_admitted` event, user message, run row, or engine turn is created after the stop decision. Existing app shutdown still cancels and drains live runs before backend close.

For an admitted Goal run, force a pause write to fail. The caller receives the storage error and local activation becomes `disarmed` while retaining the current RunID. A fresh Work read still shows the last committed `active` phase and no pause event.

Restart with a Goal run suspended on `ask_user`. The Question stays pending, the run stays active with the same RunID, and Goal activation is `disarmed`. Answering the Question resumes that existing run; its terminal does not admit another Goal round. A nonterminal child remains failed closed with the `worker_lost_after_restart` cause and a `child.failed` Journal event.

The same rule holds when the recovered pending Question belongs to a human run that created the active Goal: the answer completes that human RunID, retains the Question's answered record, and starts zero Goal rounds until explicit re-arm. During either app shutdown path, a Goal wake arriving while channel `Stop` is in progress creates no run. An edit that explicitly re-arms an owned run returns `activation: armed` and its current RunID in the mutation response.
