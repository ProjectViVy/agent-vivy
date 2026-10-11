# Code SSH planning delivery

The user requested preliminary issue #15 research, preferred a pure plugin with
no internal dependency, accepted a plugin + lightweight remote worker direction,
and then requested Supermanagement action plans and a new `code-ssh` branch.

Delivered [SSH-D1](../../superpowers/specs/2026-10-11-code-ssh-design.md),
[authoritative index](../../superpowers/plans/code-ssh/index.md), SSH-C1, nine
executable Story plans, requirement/dependency/conflict tables and A01-A10
acceptance matrix. Existing main and issue scope were inspected at baseline
`ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666`. The remote `code-ssh` branch was
created from that baseline. A TODO entry links the open Host gaps without
duplicating Story state.

Architecture: public T2 SSH workspace Module, independent Linux worker source
boundary, OpenSSH stdio, typed bounded protocol, durable remote receipts and
connection-owned jobs. Existing Service.Run/Journal/Policy/approval authorities
remain canonical. Optional code-ssh Recipe does not change local defaults.

Current public interfaces leave H1-H6 unresolved. In particular ToolWorld lacks
server-owned Run/operation identity; current Spawn does not establish target
egress authority; local instruction/preview/delivery consumers and multi-Port
ownership need proof. Integration stays blocked until zero-internal feasibility
is proved or the smallest generic exception is explicitly decided and evidenced.
No SSH-specific implementation is planned inside internal. Worker-only tasks can
progress independently after execution authorization.

This delivery changes documentation only. Product implementation, Host/Port
changes, installation, issue comments/closure and merge are outside this turn.
See [verification](verification.md) for focused document checks and
[acceptance](acceptance.md) for the distinction between plan and product evidence.
