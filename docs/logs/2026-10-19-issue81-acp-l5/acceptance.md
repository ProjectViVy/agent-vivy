# Acceptance — issue #81 E8 / L5 ACP (restricted local stdio Face)

Everything below runs from the lane root
(`.worktrees/l5-acp/agent-vivy`) with `GOTOOLCHAIN=go1.26.4`.

## 1. The daily body is unchanged

```bash
go run ./cmd/vivy            # TUI, exactly as before
go test ./sdk/internal -run '^TestPackACPSelectedOverlay$'
```

The overlay test proves an unselected Recipe keeps the real `cmd/vivy/tui.go`
and the full closure, and that only the ACP Recipe swaps it for the pack-only
stub. `recipes/acp.vivy.yml` is the only Recipe with
`exclusive: {std/face@v1: projectvivy/acp}`.

## 2. Pack and inspect the ACP body

```bash
go run ./sdk pack --recipe recipes/acp.vivy.yml --output .workspace/acp-pack --source plugins/acp
go run ./sdk inspect-artifact .workspace/acp-pack
```

Accept when the manifest lists `projectvivy/acp` (trust T2) providing
`std/face@v1` as `projectvivy.acp`, requiring `core/face-host@v1` from
`vivy/face-host`, holding only the `rpc.client` grant, and the artifact
contains no TUI organ (`internal/tui`, `internal/codeface`, `sdk/tui/face`,
`glamour`).

## 3. An ACP client can drive the packed binary

```bash
VIVY_ACP_ARTIFACT_E2E=1 go test ./sdk/internal -run '^TestACPPackedArtifactScriptedPeer$' -v
```

Accept when the scripted peer sees `initialize` → `protocolVersion 1`,
`agentInfo.name = projectvivy/acp`; `session/new` returns a session id; a
`session/prompt` without a configured model returns one governed JSON-RPC
error (`-32603` / `INTERNAL_FAILURE` / `run failed`) instead of hanging or
succeeding; stdin EOF exits the process 0.

Manually, the same proof is one line per frame:

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1,"clientCapabilities":{}}}' \
| VIVY_USER_HOME=$(mktemp -d) .workspace/acp-pack/vivy
```

## 4. A permission dialog is a real, single-use choice

With a model configured, a tool call that crosses the smart preset's approval
policy must arrive as `session/request_permission` carrying the opaque tool
call, a bounded body (`Action` / `Target` / `Preview` / `Risk`, never raw
arguments) and exactly two options: `allow-once` and `reject-once`.

Accept when:

- `allow-once` approves only that call and the next call asks again;
- `reject-once`, a cancelled outcome, an unusable preview, an unsupported
  client, or a transport failure **denies** — no path fabricates an approval;
- a reply that arrives after the run was cancelled or the dialog expired has
  zero effect (the durable approval keeps its own decision);
- a stale, foreign or already-resolved review is left untouched.

`plugins/acp/interactions_test.go` exercises each of these against a scripted
ACP client over a real SDK connection, including the wire order
(`session/update` tool call before `session/request_permission`).

## 5. Questions are forms, not free text

`session/question` maps to one `elicitation/create` form with a single
required `answer` string (max 64 KiB). Accept when a blank, oversized,
non-string or extra-key answer cancels the question (`review/respond`
`action: cancel`) instead of writing an answer, and a declined form also
cancels rather than hanging the run.

## 6. Failure and teardown are honest

- Without the FaceHost provider, `initialize` fails with
  `REQUIRED_CAPABILITY_UNAVAILABLE`, not a partial session.
- Closing stdin during a pending dialog withdraws the dialog, cancels only the
  runs this connection owned, waits for their durable terminal state within
  5 s and exits 0; if a run does not settle the process exits non-zero with the
  run id in the error — it never reports a clean shutdown it did not achieve.
- `go test ./plugins/acp -race` and `./sdk/internal/testdata/acp-compat -race`
  pass.

## 7. What this batch does not claim

No push, PR, merge, release or default-body activation; no remote ACP /
HTTP / WebSocket control plane; no ACP client filesystem or terminal methods;
no Windows `.exe` acceptance; no provider-level conformance row (see
`docs/TODO.md`: `ACP-SDK-UPSTREAM`, `ACP-CONFORMANCE-ROW`, `ACP-ARTIFACT-E2E`,
`ACP-WINDOWS-ACCEPTANCE`).
