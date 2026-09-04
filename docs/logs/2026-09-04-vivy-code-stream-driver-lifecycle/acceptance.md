# Acceptance

A human can tell this delivery works when:

1. A long reasoning or answer stream stays continuous while the terminal is slow; CJK, emoji, whitespace, and reasoning boundaries remain intact.
2. A replayed duplicate is not printed twice, and a missing sequence resumes from the last contiguous Journal sequence.
3. Losing a stream subscription shows a recovery message and reconnects without ending the run. Repeated reconnect failure returns the plain REPL to a usable prompt after cancelling that run.
4. Switching sessions or quitting does not leave an active old subscription delivering into the new screen.
5. A burst beyond the local queue bounds does not grow memory without limit or silently finalize a partial answer; it invokes durable replay.

Automated acceptance covers normal replay, empty replay, a second gap during replay, early `run/stream_error`, stale subscription traffic, late response cleanup, queue overflow, terminal cleanup, and persistent recovery failure.
