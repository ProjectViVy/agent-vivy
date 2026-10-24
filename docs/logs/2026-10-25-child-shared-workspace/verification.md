# Verification — child runs share their root run's workspace

Host: macOS arm64, `GOTOOLCHAIN=go1.26.4` (the ambient Homebrew Go 1.27.1 breaks
the pinned `x/net`). Tree: `.worktrees/child-shared-workspace` on
`fix/child-shared-workspace`, based on `6ecb75c7`.

## Static gates and unit tests

```text
gofmt -l internal schemas docs            # empty
GOTOOLCHAIN=go1.26.4 go vet ./...         # clean
GOTOOLCHAIN=go1.26.4 go build ./...       # ok
GOTOOLCHAIN=go1.26.4 go test -p 1 -count=1 ./internal/...
```

The full suite ran twice, both green. Second run, filtered for failures:

```text
ok  agent-vivy/internal/app        19.311s
ok  agent-vivy/internal/rpc        18.958s
ok  agent-vivy/internal/runtime    67.452s
ok  agent-vivy/internal/storage/sqlite  9.041s
(no FAIL or panic line in any package)
```

No existing test needed its expectation changed: the suites that assert child
workspace identity (`service_test.go`, `inofy_executor_test.go`,
`message_projector_test.go`) do not drive the sandbox-world child path with a
durable child run row, so they never observe the redirect.

### New tests

| Test | Pins |
| --- | --- |
| `TestChildRunSharesRootWorkspace` | depth-1 and depth-2 children resolve to `root/run_root`, are marked redirected, read the parent's file, agree with `Existing` and `ValidateRunPath`, and the root holds exactly one directory |
| `TestRedirectedChildWorkspaceIsNeverReleased` | a child's `Release` is a no-op; the owner's own `Release` still removes it |
| `TestChildAdmissionNeverReclaimsTheSharedWorkspace` | child `EnsureForAdmission` reports `created=false`; `DiscardNewPrivateAdmission` is a no-op |
| `TestWorkspaceOwnerRunContextBridgeBeforePersist` | the pre-persist bridge resolves to the root; without it an unpersisted run keeps its own id; a durable row beats a stale context value |
| `TestEffectfulOneShotChildSharesRootWorkspaceUnderApproval` | the D11 contract sentence: shared workspace id, one directory, no effect before the approval decision, the effect after it, and the Journal recording the root workspace id |

### Negative control (unit)

Removing only the bridge at the one-shot allocation site —
`Ensure(withSessionID(ctx, parent.SessionID), child.ID)` instead of
`Ensure(withWorkspaceOwnerRun(withSessionID(...), rootID), child.ID)` — fails the
contract test:

```text
approval_test.go:498: child workspace = "child_3bea101023ac7775", want the root run's "run-shared-root-parent"
```

The bridge was restored afterwards (`git diff --stat` back to the six Lane 1
files) and the test re-passed.

## Real-path smoke (headless, real binary)

The split pair's Vite dev server is hard-pinned to `3015` with `strictPort`
(`ui/vite.config.ts`), the owner's pair from the root tree already holds
`8787`/`3015`, and this worktree has no `ui/node_modules`. Taking those ports
would have meant killing another lane's session, so the real-path smoke ran the
same product binary headlessly instead: real admission, real Journal, real
filesystem tools, real `agent` delegation, disposable home under `/tmp`. The
repository's `data/` was never opened (ST-2), and `lsof` confirmed before and
after that the owner's `8787`/`3015` pair stayed up.

Setup:

- Binary A (candidate): `go build -o /tmp/vivy-l1-smoke-bin/vivy ./cmd/vivy` from
  this worktree.
- Binary B (control): a detached worktree at `6ecb75c7` — the commit immediately
  before this change. `ui/embed.go` embeds `all:dist`, so the gitignored
  `ui/dist` artifact had to be copied into that worktree before it would build.
- Config: `server.addr` `127.0.0.1:8788` (A) / `:8789` (B), sqlite and
  `runtime.workspace_root` under the same `/tmp` home, `runtime.world: sandbox`
  (the default, and the only world with private per-run directories),
  `sandbox.approval.timeout_seconds: 5` so an effectful call settles by
  auto-approval instead of waiting for a human.
- Provider: the frozen ENV session (`VIVY_PROVIDER=deepseek`,
  `VIVY_API_BASE=http://127.0.0.1:9911/v1`, `VIVY_MODEL=mock-fast`,
  `DEEPSEEK_API_KEY` set to a dummy value) pointed at a marker-routed
  OpenAI-compatible mock. Routing is per conversation, so parent and child never
  poison each other: the parent branch emits `write_file proof.txt`, then
  `agent {task: "READ-PROOF: …"}`, then a final text; the child branch emits
  `read_file proof.txt`, then a final text quoting the tool result.
- Driver: `GET /rpc/bootstrap` → token → WebSocket → `session/create` →
  `module.action.invoke` `diva.cognitive.persona.initialize` (a fresh home gates
  `turn/start` behind the persona) → `turn/start` → poll `run/get`.

Observed, binary A (this change):

```text
runs: run_be0c94b1343fe1aa primary completed / child_3c812dd07d43bb5a child depth 1 completed
child.started  {"parent_run_id":"run_be0c94b1343fe1aa","workspace_id":"run_be0c94b1343fe1aa"}
child.completed summary … "content":"1\twritten by the parent" …
workspaces/    run_be0c94b1343fe1aa/proof.txt   (21 bytes, "written by the parent")
               — exactly one directory, no child_* entry
```

Observed, binary B (`6ecb75c7`, same mock, same driver):

```text
child.started  {"parent_run_id":"run_d85bff2dcd2d9eb6","workspace_id":"child_3ef69fef34cfd84b"}
child.completed summary … filesystem: read proof.txt: open
               /tmp/vivy-l1-control/workspaces/child_3ef69fef34cfd84b/proof.txt:
               no such file or directory
workspaces/    child_3ef69fef34cfd84b/  (empty)
               run_d85bff2dcd2d9eb6/proof.txt
```

B is the tester's report reproduced byte for byte; A is the same scenario
passing. The `workspace_id` in `child.started` is also the value the Run
Inspector renders, so the UI-visible half of the claim is covered by the Journal
evidence above and by `TestEffectfulOneShotChildSharesRootWorkspaceUnderApproval`.

Temporary artifacts (both binaries, both `/tmp` homes, the mock, the driver, the
`l1-control` worktree) were removed after the run.

## Not run, with reasons

- **Browser smoke at `http://127.0.0.1:3015`** — unrun for this lane, see above:
  the port is `strictPort`-pinned and held by the owner's pair from the root
  tree, and this worktree has no UI dependencies. `acceptance.md` writes the
  steps for the owner to repeat in the browser.
- **`just wiring-ci`, `just kernel-ci`, `just d02-ci`** — unrun. They require an
  independent Linux-filesystem checkout with its own pinned Laputa; macOS cannot
  certify them. Left to the owner.
- **PostgreSQL conformance** — only through the repository's existing
  `./internal/storage/postgres` package tests. Nothing in this lane touches a
  DSN-backed instance.
- **The boarded `GOAL-ROUND-TEST-AFTERLIFE` flake** (introduced by the unmerged
  `7afa1d8e`, diagnosed in `docs/logs/2026-10-25-restart-terminal-idempotence/verification.md`)
  did not fire in either full run here. It is not this lane's to fix and remains
  on the board.
