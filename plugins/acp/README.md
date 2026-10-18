# projectvivy/acp

Restricted local **ACP stdio Face** Provider for Vivy (`std/face@v1`, kind
`acp`). Canonical contract: `docs/architecture/ACP-STDIO-FACE.md`.

The module bridges one local ACP v1 client connection to the FaceHost Control
RPC. It owns no runtime, journal, storage or policy: every prompt becomes one
`turn/start`, every displayed update projects a committed `run/event`, and
approvals/questions are answered back through `approval/respond`,
`question/respond` and `review/respond` under the same durable authority as
every other Face.

## Identity

- Module `projectvivy/acp` `0.1.0` (T2, generation-scoped, not in the default
  Generation)
- Provides `std/face@v1` as `projectvivy.acp`
- Requires `core/face-host@v1` from `vivy/face-host`
- Requests the `rpc.client` grant
- Source pin `repo:plugins/acp` (the digest is declared in
  `vivy-module.yaml` and `module.go`; regenerate both with
  `go run ./sdk/internal/cmd/source-hash plugins/acp <declared>`)

## SDK pin

`github.com/eino-contrib/acp` is replaced by `github.com/mastwet/acp`
`v0.0.0-20261007071707-780fb4dd4b98` for connection options (dispatch cap,
shutdown timeout, connection label) and opt-in wire error sanitization until
those surfaces reach the upstream module. The evaluation baseline and gaps are
recorded in `docs/research/acp-sdk-compatibility.md`.

## Surface

`initialize`, `session/new`, `session/prompt`, `session/cancel`, and the two
agent-to-client calls `session/request_permission` (allow-once / reject-once)
and `elicitation/create` (one required `answer` field). Every other ACP method
fails closed. One connection owns at most 32 sessions, 4 active prompts and 16
live human interactions; teardown cancels only the runs the connection owned.

## Pack

```bash
go run ./sdk pack --recipe recipes/acp.vivy.yml --output .workspace/acp-pack --source plugins/acp
go run ./sdk inspect-artifact .workspace/acp-pack
```

The ACP-selected artifact replaces `cmd/vivy/tui.go` with a pack-only stub, so
the TUI organ and its render stack never link; `vivy` without arguments serves
the protocol.
