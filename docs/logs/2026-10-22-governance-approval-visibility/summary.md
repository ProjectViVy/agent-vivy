# Governance approval fixture visibility

`waitGovernanceApproval` now waits for the pending SQL row and its matching
durable `tool.approval_required` event while the Run remains active. It compares
approval ID, tool call ID and tool name, retaining the existing five-second
polling deadline. Run has no separate waiting status. Terminal/timeout diagnostics
contain only Run status, pending-row presence and the last eight event types.

The raw row is insufficient: Service creates it before appending the visibility
event, and DecideApproval rejects an approval whose event is not durable. The
test helper now follows that existing contract without a runtime barrier or a
readiness wait on Service.WaitIdle. Three former caller-specific five-second
decision retry loops were removed; callers decide once after the helper proves
visibility.

The existing middleware-drift test now holds the real Service Journal append
after CreateApproval. It proves that a visible pending row cannot make the helper
return before the event is released and committed. The wrapper embeds the same
SQLite backend, preserving its owner ports. Cancellation/WaitIdle is used only
in test cleanup, after releasing the gate, never as a readiness condition.

The middleware-secret test still checks every event payload for its canary.
Its failure message now reports only event type and sequence rather than
printing the potentially secret payload.

Only the app test file and this log changed. Pinned Eino/provider APIs, runtime
approval behavior, timeouts, privacy assertions, warming and production state
were not changed. A separate potential event-publication/pending-registration
runtime window remains with the integration owner's review; this fixture fix
does not wait for or mask that process-local registration.
