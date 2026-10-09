# P2.4: Coherent cognitive control and scoped cancellation

`CognitiveControlState` now returns policy, policy revision, active run, source watermark, pending window, phase and block reason from one durable snapshot. The app ControlPort uses that projection directly, and the existing DIVA action dispatcher projects its recovery fields verbatim.

`CancelCognitiveRun` serializes with cognitive admissions and verifies the persisted intent, trusted workflow revision, input window, bound source/scope, supervisor lineage and run kind. Its final status recheck and cancellation signal share the runtime terminal-commit lock, so an already committed terminal Run is reported as no longer active. Foreign and stale run IDs cannot reach the cancellation signal.

The DIVA Evolution view already displays phase, active run, pending through and block reason; no UI change was needed. This closes P2.4 code work; P3–P7 and the independent P1 native gates remain open.
