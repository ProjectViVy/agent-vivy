# PG-3 Task 3: Recovery and shutdown

Goal recovery now keeps pending human decisions and their existing RunID without restoring automatic Goal activation. A failed pause write returns its storage error, leaves the durable Work phase untouched, and disarms local automatic continuation. Shutdown closes automatic admission before draining its Goal workers; a candidate held before commit cannot create a new run after the stop decision.

This uses the existing Service, Journal, Policy, `projectionMu` terminal order, and app startup/shutdown lifecycle. No new runtime, schema, retry loop, Eino pin, dependency, or provider configuration was added. The two pre-existing generated UI modifications were excluded.

Review correction: recovery now disarms a session even when the pending run is human-origin and created the active Goal before suspending. App shutdown closes automatic admission before channel and action drains. Work mutation responses read activation after their wake/cancel effects, so an edit of a still-owned run reports its current process authority.
