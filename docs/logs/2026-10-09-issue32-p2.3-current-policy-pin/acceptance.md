# Acceptance

A new cognitive run admitted after a Mission edit records the current Mission revision. If the Mission changes after that run is resolved, the run fails closed before applying an effect. After restarting the service, an enabled nondefault trigger policy is loaded from the durable snapshot and produces the same policy digest in the next run binding. A policy update racing with admission affects later admissions while the current intent retains the policy snapshot already resolved for it.

These behaviors are covered by the P2.3 runtime and module regressions. Product-level UI acceptance is not applicable because P2.3 changes no user-facing UI.
