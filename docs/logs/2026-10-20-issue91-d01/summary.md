# D01: Remove digest-only runtime audit

The default App no longer constructs an event hook that hashes every durable
event payload and writes a parallel digest-only debug log. The audit types,
sink, implementation and digest-only test are removed. The memory Host
contract now points to canonical Journal provenance without depending on that
retired hook.

Generic RunHook, the Channel hook, run-observer subscriptions and
actionhost.JournalAuditSink remain. The single Service.Run/Eino/Journal path
still records required lifecycle facts. No replacement audit subsystem,
public wire field, configuration field or dependency is introduced.

A production-path regression composes the real App with isolated SQLite, a
loopback provider and the real file tool. A random nonce absent from the user
request is read from disk; the second provider request contains the exact
durable tool result and original call ID; final text is persisted, one
terminal is recorded and Service.WaitIdle succeeds. The test first failed on
continued digest-audit emission, then passed after removal.

This is the first bounded runtime cut of #91. It does not complete #91 or
perform D02. Nudge, warming, steering and their existing behavior remain;
#93 still owns later ordinary first-party plugin extraction. The donor-only
experimental loop-detection source is absent on stable and was not imported.

No migrations, production state access, dependency/pin updates, Recipe
changes, generated product changes, paid provider calls or release are part
of this delivery. It stays on `codex/issue91`, based on `stable@ce548925`;
stable remains the comparison checkout.

Implementation and focused regression are complete; acceptance remains open.
Full just ci failed, including a D01-induced source/conformance identity
mismatch and host/full-suite failures. The existing conformance artifact
remains untouched because its producer could not pass; see verification.md.
