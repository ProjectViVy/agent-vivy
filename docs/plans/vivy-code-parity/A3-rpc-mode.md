# A3 — `--mode rpc` JSONL subprocess protocol

**Goal:** stdin JSONL commands → `vivy.rpc.v1` control plane; stdout JSONL responses + events (pi `--mode rpc` equivalent).
**Epic:** A. **Requirements:** RQ-RPC. **Predecessors:** A1; B1 (steer/follow_up/queue cmds); C1 (tree/clone/export cmds).
**Spec:** [VCP-D1](../../specs/2026-10-06-vivy-code-parity-design.md) §5.3 (command map is normative).

## Scope

**Files:** `faces/tui` new `rpcmode/` — command loop, response/event writer, command→control-plane translation table. Partial delivery is acceptable: commands whose backend isn't landed yet return `{"success":false,"error":"method not implemented"}` and get enabled as B1/C1 land.

## Tasks

- [ ] Framing: one JSON object per line in/out; `id` correlation; responses `{id,type:"response",command,success,data|error}`; events `{type:<event>}` un-correlated except `bash` output events which repeat the command id.
- [ ] Implement the §5.3 map; `prompt` returns `disposition` (`started|queued|handled`) matching B1 queue truth; `get_commands` lists effective commands incl. which are unavailable.
- [ ] Stdout reserved for protocol; logs/diagnostics → stderr only.
- [ ] Backpressure-safe writer; graceful shutdown on stdin EOF and on a `shutdown` command after run settle.
- [ ] Tests: golden-line transcripts for each command family; unknown command → `success:false`; concurrent command ids correlate correctly; session events interleave with responses.
- [ ] `go test ./faces/tui`; end-to-end: spawn `vivy-code --mode rpc`, drive prompt→steer→abort→fork→export against a fake provider.
- [ ] Commit `feat(vivy-code): rpc subprocess mode`.

## Boundary

Not a socket server — stdin/stdout only (WebSocket control plane stays the gateway's domain). `bash` command: allowed only when it routes through the governed ToolHost path (never a raw exec); if sandbox mode disallows, return policy error.

## Acceptance

The §5.3 table is fully implemented or explicitly refused-with-reason; the golden transcript suite is the conformance artifact.
