# Verification

Venue: native Windows, isolated managed worktree
`C:\Users\Administrator\.codex\worktrees\d14-runtime-residue\agent-vivy`.
The independent sibling Laputa clone is clean and pinned to
`9acc999301d5d575da2cea902a1c6d213763f381`. Go commands used
`C:\Program Files\Go\bin\go.exe` because Go is not on this shell's PATH.

## Test-first evidence

Before implementation, `go test -p 1 ./internal/runtime ./internal/rpc -run
'TestD14' -count=1` failed at the intended boundaries: replay of an exact-text
steering marker added a duplicate; selected workspace identity was still
path-derived; current redacted shell previews disappeared; and MCP command names
could not recover their exact original tuple. The workspace expectation was
subsequently refined to the authoritative session owner after inspecting the
first-Run workspace lock, avoiding public host-path disclosure and longer IDs.

## Final focused run

```text
go test -p 1 ./internal/runtime ./internal/provider ./internal/rpc ./sdk/tui/view -run 'TestD14|TestChatSnapshot|TestChatAssembly|TestPausedViewport|TestSessionWorkspaceManager|TestSetSessionWorkspaceSerializesWithFirstRunAllocation|TestSelectedSessionWorkspaceDrivesFileAndCommandOperations|TestControlHandlerMCPPromptCommands|TestMessageProjectionNeverExposes|TestProviderProfileStatusWireSourceCannotCarrySecrets|TestSecretEnvReadsStayOutOfProvider' -count=1
```

Exit 0 on the final code:

```text
ok  agent-vivy/internal/runtime   1.866s
ok  agent-vivy/internal/provider  0.346s
ok  agent-vivy/internal/rpc       1.010s
ok  agent-vivy/sdk/tui/view       0.467s
```

The new tests cover exact steering replay and changed text with pending calls,
session-owned workspace reopen/confinement/release, private credential cache
rotation and readiness with actual local HTTP authorization requests, current
shell preview replay/refusal, exact MCP list/expand identity, longest supported
MCP TUI registry names, and same-length nested TUI edits. Existing tests retain
paused-anchor, session workspace admission-lock and provider secret checks.

## Additional checks and deliberately unrun lanes

- `gofmt -w` ran on all changed Go files. `git diff --check` passed.
- A targeted search for `sha256`, `digest`, `maphash`, `chatStamp`,
  `messageStamp` and `chatAnchorStamp` found no remaining implementation in the
  affected production files and retired stamp-test owner.
- `go test -p 1 ./internal/runtime ./internal/provider ./internal/rpc
  ./sdk/tui/view -count=1` returned `ok` for the complete runtime package
  (375.745s) and provider package (0.481s). The command was then intentionally
  stopped to avoid duplicating the integrated candidate's broad CI lane;
  no broad RPC/TUI pass is claimed from that partial command. The repository
  documents that native runtime tests can exceed 20 minutes; silence during the
  run was not treated as evidence of a hang or failure.
- The first test attempts lacked shell PATH/cache write access. Explicit Go
  path and authorized cache access resolved these environment restrictions.
  One later attempt was not executed because automatic approval review hit the
  account usage limit; after the user's continuation, verification resumed and
  the final focused command completed successfully.
- No WSL/Linux `wiring-ci`, `kernel-ci`, `d02-ci`, race, Postgres, full-product
  CI, real split-browser/native terminal smoke, pack, or copied-store upgrade
  acceptance was run by this scoped lane. The parent owns these checks on the
  integrated final candidate. No production Journal was accessed.

The patch is verified at its focused native owners. This record does not certify
D14, the finite upgrade artifact, or product/release acceptance.
