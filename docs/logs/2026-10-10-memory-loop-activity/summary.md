# Terminal activity and Session deletion continuity

Real terminal capture now passes admitted, redacted user-only content to the one native owner for bounded Pulse/Recap projection. No assistant/tool/system text is used for recap; previously accepted source receipts are rejoined before reconstructing payload and never backfilled. The bound source watermark cannot be bypassed by an acceptance notification.

Session deletion now preserves canceled producer terminal events, drains the same durable ObserverHost/cursors, waits for the original native activity receipts and archives before deleting messages. Observer worker/drain share one context-aware delivery gate. On failure the Session remains sealed and source intact. Work's strict global revision check is unchanged.

Actual chat, distinct OS-process Restart, completed/live cancellation archive, original Work preservation and stale patch rejection are verified. This is a development increment, not complete S05/S06/S10/S11 acceptance. Full source freeze/CI/conformance/artifact/native identity/fresh whole-phase review and remaining local matrices are pending. No release, merge, push or unattended daemon deployment.
