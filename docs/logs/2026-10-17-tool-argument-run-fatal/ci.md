# `just ci` and source-digest record — 2026-10-17 tool argument run-fatal cut

Commit: the commit that carries this log directory on
`feat/issue-58-unified-context` (`fix(issue-58): keep model-visible tool
mistakes out of the run-fatal path`; the hash is the tip at hand-off, and this
file is amended into it, so it is not repeated here). This file records the gate
runs for that content, the host failures, and the source-digest decision. Read
[`notes.md`](notes.md) for the concurrent-lane situation that shaped both.

## Recipe results

| Recipe | Result |
| --- | --- |
| `ensure-laputa` | ok |
| `bootstrap-test` | ok (all `✔`) |
| `fmt-check` | ok |
| `ui-ci` (i18n-check → typecheck, **693 vitest tests passed**, `vite build` ✓ 32.9s) | **ok** |
| `vet` (`go vet ./...`) | ok |
| `test` (`go test -p 1 -timeout 35m -skip '^TestMemoryLoop' ./...`) | **FAIL — host only, see below** |
| `diva-recall-test`, `headless-compile`, `plugin-ci` | not reached (the `test` recipe exits first) |

The package this change touches passed inside that run:

```
ok  agent-vivy/internal/runtime            605.867s
ok  agent-vivy/internal/runtime/compaction   0.142s
ok  agent-vivy/internal/tools                0.670s
ok  agent-vivy/internal/rpc                168.515s
ok  agent-vivy/internal/provider             2.434s
ok  agent-vivy/ui                            0.704s
ok  agent-vivy/sdk/conformance               0.078s
```

### Host failures in `test` (all pre-existing, none in this change)

Every failing test needs the Windows symlink privilege this machine does not
hold (Developer Mode is off), so the "link checked-in Web UI dependencies into
an isolated build boundary" helper fails:

1. `sdk/internal` (271s) — `frontend_v1_test.go:273: sdk: link checked-in Web UI
   dependencies into isolated build boundary: symlink …ui\node_modules …
   A required privilege is not held by the client.`
2. `sdk/internal/assembly` — only `TestHashSourceTreeRejectsSymbolicLinks`
   (the same missing privilege).
3. `sdk/internal/conformance` — `TestProviderConformanceExecutesSuites`, whose
   `full-ui` fixture shells out to suite (1) and therefore reports the same
   privilege error as its startup/registration/cleanup failure.

The same class is recorded for this host in
`docs/logs/2026-10-17-filesystem-path-run-fatal/ci.md` and in the pre-change log
`.workspace/tmp/just-ci-merged-2.log`. The workaround used for the Go build
cache and esbuild temp files is a run setting only: `TEMP`/`TMP` and `GOCACHE`
pointed inside the workspace.

## Venue: the gate ran in a worktree, not the shared root

A second lane was editing `internal/` (`internal/provider`, `internal/rpc`,
`internal/runtime/compaction*`) and `ui/` in the shared root working tree while
this cut ran. Consequences, all recorded:

- A root-tree `just ci` (log `.workspace/tmp/just-ci-freeze.log`) is **not** this
  change's record: `internal/runtime` passed (586s), but the same run hit
  `vet.exe: internal\rpc\control.go:5792:21: undefined:
  provider.ThinkingLevelSurface` and `agent-vivy/ui [build failed]` from that
  lane's mid-write state.
- The record above was therefore taken in `.workspace/wt-pin`, a detached git
  worktree checked out at this change's own content (log
  `.workspace/tmp/just-ci-pin.log`). The clean run has no such errors: the
  `internal/*`, `internal/rpc`, `internal/provider`, `agent-vivy/ui` and
  `sdk/conformance` packages all pass, and the only failures are the three
  symlink-privilege suites.
- The worktree was removed after verification; the logs stay under
  `.workspace/tmp/` (ignored scratch) as evidence.

## Source digest

Editing `internal/` moves the `internal`-rooted `sourceSha256` rows in
`sdk/internal/assembly/conformance_results.json`. The delivered pin is
`04f63283687fbada68ccf6b13a2d1a3b8090a22c1a86ae9da44b299fea9896a9` (5 rows),
the fixed point of this commit's own `internal/` tree:

```
# recreate the venue: a detached worktree at the commit carrying this log
git worktree add --detach .workspace/wt-pin $(git log -1 --format=%H -- docs/logs/2026-10-17-tool-argument-run-fatal)
cd .workspace/wt-pin
go run ./sdk/internal/cmd/source-hash internal 04f63283…
# 04f63283… (declared digest is zeroed before hashing, so a value that is its
# own result is the tree's identity)
```

The worktree used for these runs was removed after verification; the logs stay
under `.workspace/tmp/`.

Caveats that belong in the record:

1. The digest is the identity of the **working tree the command runs in**
   (`sourcehash.Tree` walks the directory relative to the working directory).
   The pin therefore has to be computed where the working tree equals the
   commit. The first attempt was computed from the shared root tree: by then a
   second lane's uncommitted `internal/` edits (`internal/provider`,
   `internal/rpc`, `internal/runtime/compaction*`) were present, so it produced
   `dd773820…`, the digest of that *other* content. The release gate caught it:

   ```
   reproduction_test.go:51: release source digest for vivy/context-source =
     04f63283687fbada68ccf6b13a2d1a3b8090a22c1a86ae9da44b299fea9896a9,
     checked-in evidence covers dd773820636cf9be55094ff293feb02a6328ff65921a25c0415ce24a3be67d5e
   ```

   The evidence was corrected to `04f63283…`, and the gate re-run in the
   worktree at this commit then reports **no digest mismatch**: its only failure
   is the `full-ui` fixture's host symlink privilege (log
   `.workspace/tmp/release-digest-pin2.log`, and the earlier mismatching runs in
   `release-digest-pin.log` / `release-digest-root.log`).
2. Any lane that commits `internal/` changes must re-pin for its own commit, and
   must compute the value in a tree that matches that commit; a shared root tree
   carrying another lane's uncommitted edits gives a value for content that is
   not being committed. That is what happened here once, and why the pin is part
   of the reviewed diff rather than a post-hoc fixup.

Logs: `.workspace/tmp/just-ci-pin.log` (clean worktree run),
`.workspace/tmp/just-ci-freeze.log` (polluted root run, advisory),
`.workspace/tmp/release-digest-pin2.log` (digest check passing in the worktree),
`.workspace/tmp/release-digest-pin.log` and
`.workspace/tmp/release-digest-root.log` (the earlier digest-mismatch
diagnostics kept as evidence of the caught wrong pin).
