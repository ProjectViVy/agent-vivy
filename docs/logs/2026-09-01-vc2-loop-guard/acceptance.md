# VC-2 Loop detection: manual acceptance path

1. Normal session: have the model call the same read-only tool repeatedly with
   different parameters (such as grep with different patterns) → the run
   completes normally (signatures differ within the window, so it does not
   trigger).
2. Legitimate repetition: have the model read the same file 5 times (same
   parameters, same content) → the run completes normally, exactly within the
   limit.
3. Trigger the guard: induce the model to repeat the same call (such as issuing
   the same instruction repeatedly to an echo-like tool) → on the sixth repeat,
   the run terminates as failed, the UI error bar shows
   "The run was stopped because the same tool call kept repeating without
   making progress. ...", and the terminal Journal `run.failed` has
   `cause_category = loop_detected`.
4. Coexistence with budget/turn limits: the priority is "stop at the first limit"
   —loop detection stops at 6 repeats, before the default MaxToolTurns and budget
   circuit breaker.

Note: a real trigger requires a real model session (there is no local mock
provider, TEST-1); automated verification is covered by the scripted-model
integration tests in `internal/runtime` (see verification.md), and the manual
path can be followed in an environment with a key.
