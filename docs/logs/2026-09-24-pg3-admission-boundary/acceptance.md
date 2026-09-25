# Acceptance

For one session, register a human RPC/channel/action turn before an automatic Goal candidate reaches its durable commit. The human turn must own the primary admission, and the Goal's durable round count must not advance. If a primary run is already committed, a later primary producer still receives the established conflict rather than a queued or fabricated accepted run.

To verify Goal wake coalescing, pause a Goal, hold its first candidate at the controllable work-state read barrier, durably resume it, then send a burst of wake signals. The burst should return while the candidate is blocked and leave one running worker plus one pending bit. After releasing the barrier, the Goal should admit exactly one round and reach its normal terminal result; no eligible wake should be lost.

Cancellation before a human waiter reaches durable admission leaves no user message or run. Cancellation after durable admission retains the existing detached-run behavior. These checks stay on the current Service/Journal/atomic Goal commit path.
