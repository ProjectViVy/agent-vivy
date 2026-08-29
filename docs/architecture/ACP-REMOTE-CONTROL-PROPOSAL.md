# ACP / Remote Control Proposal

Status: proposal only — implementation deferred pending explicit approval.

Same-process mouths (web / tui / headless) are a different contract:
`VIVY-FACE-PACK.md`. This file is the remote / out-of-process control
plane. A downstream Android app that *uses* the Vivy kernel in-process
is not ACP; an Android app that steers a resident `vivy.exe` over the
network is.

## Intent

Provide a future, authenticated control plane for observing and steering
Agent-Vivy background runs from another process or desktop shell. This is a
control-plane proposal, not a new agent runtime and not a Memory integration.

## Non-goals for the current Harness goal

- No remote listener, websocket, ACP server, or network credential is added.
- No remote tool execution or filesystem write is enabled.
- No Memory, BML, Laputa, AutoDream, Evolution, or long-term memory injection
  is added.
- The existing local JSON-RPC/WebSocket control plane remains the only active
  control surface.

## Proposed invariants

1. Local-first: the existing Journal, checkpoint bridge, approval/question
   stores, run budget ledger, and sandbox workspace remain authoritative.
2. Explicit capability scope: a remote connection receives a session/run scope
   and selected read capabilities; it cannot widen the request-scoped tool
   manifest or Plan Mode policy.
3. Human gates stay server-side: approvals and Ask User answers require the
   same first-writer-wins, expiry, and run-identity checks as the local UI.
4. Reconnect is cursor-based: event delivery uses `run_id` plus `after_seq`,
   with Journal replay before live frames. Clients must tolerate duplicates by
   sequence.
5. Every remote mutation is idempotent: commands carry a client command id;
   the server records/rejects duplicate decisions and never retries an
   effectful command without a durable identity.
6. Audit is metadata-only: remote connection, capability, command, and result
   records contain ids, event types, bounded sizes, and digests — never raw
   credentials or a second transcript.
7. Budget inheritance is mandatory: a remote child/subagent receives a H7
   child ledger and a private H8 workspace; parent limits cannot be widened.

## Candidate protocol surface

The first protocol version should mirror existing local contracts rather than
inventing a second run model:

| Operation | Read/write | Required guard |
|---|---|---|
| list sessions/background runs | read | authenticated principal + scope |
| attach/replay event log | read | session/run ownership |
| subscribe event stream | read | cursor + bounded reconnect |
| cancel run | write | command id + run ownership |
| answer question | write | question expiry + first writer |
| decide approval | write | approval expiry + first writer |
| start child run | write | explicit parent, child budget, sandbox |

The protocol must return the existing FR-11 error envelope and preserve the
existing event names/payload versions. A remote client must not receive an
absolute host workspace path.

## Approval gates before implementation

Before coding, obtain decisions on transport, authentication/authorization,
principal-to-session mapping, command id retention, rate limits, and whether
remote child runs are permitted at all. Then produce a protocol schema and a
threat model, followed by an adversarial review and real reconnect/restart
acceptance test.

Until those decisions are approved, this proposal is intentionally inert.
