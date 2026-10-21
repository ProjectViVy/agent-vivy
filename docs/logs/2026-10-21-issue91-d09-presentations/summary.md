# D09 retained presentations and immutable downloads

Presentations now capture bounded exact file bytes privately in the existing
ContinuityStore event/receipt transaction. Paired040_deliverable_presentations
was allocated against integrated039. New captures retain opaque item IDs;
existing Journal metadata remains authoritative and private bytes never enter
its events, RPC or tool results. Missing historical bytes are unavailable and
require re-presentation; there is no reconstruction from an old digest/current
file, no legacy validator and no SQL backfill of unavailable bytes.

The existing session and rewind/edit lifecycle removes hidden private bytes in
the same transaction. The common visibility fold retains fork provenance and
runs without an owning user turn. New hidden-run presentations cannot repopulate
pruned state. Atomic rollback and receipt replay preserve durable outcomes.

Beginning a transfer reauthorizes its item, reads its retained presentation and
compares a bounded secure-open descriptor exactly with those bytes. Subsequent
chunks read one immutable ItemID/TransferID scratch snapshot with a separate
60-second idle TTL. One manager lock protects window replay, expiry and close.
A retry must return the same last window; changed windows, foreign or stale
handles, missing files and lost authority fail. Transfer expiry/close never
removes still-visible durable presentation bytes.

Remove delivery and export digest fields through domain/tool/RPC/SDK/UI and the
authoritative session-tree Module. Client transfers pin one handle and enforce
item, offset, page bound, complete size and EOF; error/cancel cleanup closes that
original handle. Existing store epochs prevent late responses from restoring
state or saving a canceled download after a session change.

Session exports and diagnostic bundles publish under opaque exclusive names
through one small shared filesystem helper. Writers never replace a published
name; reads are bounded, root-bound descriptor snapshots with race guards.
This is immutable Vivy publication, not a baseline that detects arbitrary local
external edits made before reading. The helper is neither a registry nor a
second Journal. Diagnostic report content and authorization are unchanged.

The existing Service/ToolHost/ADK/Journal path is retained. The pinned Eino
v0.9.21 adk/filesystem.Backend Write/Edit and schema.ContentBlock.UserInputText
checks recorded in D09 remain applicable: Eino owns invocation/projection, not
private presentation storage or filesystem authority. No runtime/orchestration
pipeline, Eino import boundary, dependency version or lockfile changed.

This supplies D14 with paired040 and explicit unavailable historical-content
semantics. Released SQL is unchanged.040 is an integrated owner allocation,
not a declaration of final conversion targetT. D10/D11 remain gated by accepted
D08/D12 and their actual upstream seams. The owner-deferred standard3015 smoke,
human Story acceptance and D14 finite converter/release acceptance stay open.
