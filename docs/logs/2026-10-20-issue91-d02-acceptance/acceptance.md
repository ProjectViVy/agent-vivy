# Acceptance

Accepted through the owner's scoped WSL route. Final commands, independent
review corrections and limits are recorded in
`../2026-10-21-wsl-kernel-ci/verification.md`; the issue-91 index owns status.

The existing D02 split-browser causal smoke remains applicable: a real
isolated file nonce and call ID reach the next model request, completion uses
byte framing, reloading preserves the transcript, and one terminal returns
the service to idle. This follow-up preserves that protocol.

Acceptance additionally requires:

1. Both cancellation and listener failure release the same process-owned
   resources; a closed app cannot retain the Garden database handle.
2. The real native CODE process completes a conversation in its test-owned
   user home, preserving protocol correlation and event ordering.
3. UI packing and Inspect succeed without special symlink privilege and
   preserve the selected UI identity/assets.
4. The actual conformance producer executes every selected owning suite,
   publishes only complete passing evidence, and ordinary verification then
   reproduces it on the unchanged source tree.
5. Owner-authorized wiring/minimal kernel, affected D02 and generation gates
   pass at the reviewed candidate. Release/platform CI remains separate.

Rollback uses a focused source revert. No stored-format transition occurs.
