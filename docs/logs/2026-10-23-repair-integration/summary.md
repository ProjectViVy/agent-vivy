# Local repair integration

Delivery date: 2026-10-24 (Asia/Shanghai). The directory name follows the
owner-approved plan's explicit destination.

Merge candidate `855fe683d04a4fd3d4bb60fc328abc3f8b4a6858` combines
`codex/core-fixes-batch1@64e915bd` and `codex/provider-repairs@649dc0f5`,
both based on local stable `7496f35f`. Both source commits, branches,
iteration records, and backlog updates are preserved. No remote push,
release, deployment, tenant installation, or tenant Journal access is part
of this delivery.

The initial merge required no conflict resolution. A separate read-only integration
review confirmed exact source blob preservation for all 33 non-TODO paths,
retention of both TODO updates, and no merge-specific findings. Provider
selection and queued admission retain their shared locking and ordinary
Run admission path; queue snapshots do not copy provider credentials.

## Eino capability check

No new capability or adapter is added. The preserved batch records inspect
Eino v0.9.21 ADK Runner/checkpoint/cancellation/resume APIs, including
`ResumeWithParams`, and EinoExt `agenticopenai v0.2.4`
`NewChatModel`/`ChatConfig`. Existing Vivy configuration and Journal seams
remain the owners of their respective facts.

## Review follow-up and integration ruling

The reviewer confirmed an inherited recovery gap: `Service.recover` returns
when no active Runs exist, before the Session queue drain. A crash between
carrier settlement and follow-up admission can leave durable idle requests
waiting after restart. The existing restart test creates an active carrier
and does not cover that window. This was established by control-flow review,
not a new executable regression in this integration.

Ruling: preserve the approved source commits without adding runtime changes;
track `QUEUE-IDLE-RESTART-DRAIN` in the open board. The cost is that this
restart window remains unresolved and cannot be represented as accepted
automatic queue recovery. Its next repair needs a terminal-carrier regression
and removal or relocation of the early return.

## Required-gate compatibility correction

The combined full affected-package run exposed
`TestTurnStartUsesLiveContextForProjectFiles`: a canceled request reached the
new Session workspace lookup before context-file validation, turning the
previous `InvalidParams` response into `InternalError`. The original regression
failed ten consecutive isolated repetitions on the merge candidate.

Ruling: add a three-line canceled-context check before Session lookup for
file-bearing submissions. This is the smallest correction needed to preserve
the existing RPC contract and satisfy the approved affected-package gate;
it does not change the public interface or weaken Session workspace validation.
Its cost is one additional compatibility correction beyond the exact source
merge. The existing regression supplies the RED-to-GREEN evidence.

No schema, public interface, dependency, frontend source, Studio, or kernel
runtime implementation changes were made by this integration.
