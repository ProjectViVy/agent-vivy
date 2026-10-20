# D03 delivery

D03 extends the existing operation row and approval owner to bind exact private
inputs and persist typed outcomes. The accepted prerequisite is stable D02
commit `e2c1518e9388cca091828c85df5aadc4f59406fa`.

- Original request bytes identify a retry; all three private argument buffers
  participate in repeated admission. Missing original bytes fence legacy rows
  before binding, claim or replay.
- Completion commits Result, Failure, Status, Reason, Effects and the existing
  Journal lifecycle in one transaction. Durable classification drives replay;
  claimed effects remain unknown after their executor disappears.
- Approval v2 binds canonical JSON with exact numeric literals to the existing
  native Eino interrupt, Run, call, tool, checkpoint and resume target. Old or
  unbound envelopes fail closed, including denial paths.
- Cached completed siblings are reconciled through the current Journal/mapper
  before native Resume. They satisfy the existing barrier without invoking
  tools or counting an earlier leg as a new nudge attempt.
- Direct-shell command approval uses its existing protected state owner, exact
  hook declarations and a redacted public byte-count label. No new operation
  claim or recovery runner is introduced.
- Paired migration 036 removes the two operation digests. Every baseline row
  gets NULL original input and NULL outcome fields. Existing claims, results,
  failures, timestamps and historical event bytes survive.
- Runtime startup reads source shape before writing. Legacy sources require an
  explicit upgrade and unknown/partial sources are rejected. Paired offline
  approval fragments and disposable copy fixtures belong to the existing
  migration owner.

Pinned Eino v0.9.21 APIs remain components/tool.StatefulInterrupt,
GetInterruptState, GetResumeContext and adk.TypedRunner.ResumeWithParams, with
the existing checkpoint bridge. compose.ToolsNode retains executed sibling
outputs but supplies no Vivy database claim, atomic Journal completion or
business classification. The change extends the existing persistence adapter;
it adds no second runner, cache, outcome service or dependency.

Cached-result repair reuses normalizeEnhancedResult and native Eino content
blocks. The pinned compose/agentic_tools_node.go converter is private, so the
small projection adapter restores the existing Journal shape; it can be
replaced if upstream exposes that conversion. It performs no execution.

D04 retains classification strategy/loop contraction; D08 retains checkpoint
format and final Run settlement; D09 retains file before-state preconditions;
D11 retains child/workflow revisions; D12 retains final migration metadata;
D14 retains the finite upgrade artifact. D03 does not make upgrade publication
ready. Conversion resources operate on copies and must be composed with D08 in
one target transaction before release. No legacy runtime approval fallback is
retained.

All implementation remains on stable in one sequential write lane. The old
issue91 checkout is preserved as a read-only backup. No production Journal was
accessed and no push was performed.
