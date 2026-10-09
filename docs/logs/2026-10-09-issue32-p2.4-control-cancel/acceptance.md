# Acceptance

The status action reports one coherent control snapshot, including durable policy, active run, watermark, pending window, phase and block reason. The exact current DIVA workflow can be cancelled; foreground, foreign-supervisor and stale IDs are rejected without cancelling the active workflow. If settlement wins first, the cancellation reports that the run is no longer active.

Runtime tests exercised a real admitted DIVA workflow with a gated domain; app tests exercised ControlPort projection/scoping; module tests verified the dispatched JSON. The existing DIVA view consumes the same fields, so no UI change was needed.
