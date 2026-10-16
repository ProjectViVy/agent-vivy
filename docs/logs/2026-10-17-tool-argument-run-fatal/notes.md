# Notes — 2026-10-17 tool argument run-fatal cut

## Eino capability check (AGENTS.md decision order)

Inspected the pinned `github.com/cloudwego/eino v0.9.20` surface before writing
any code:

- `compose.ToolsNode.Stream` (`compose/tool_node.go`) is the component that turns
  one tool's error into a node error
  (`failed to stream tool call %s: %w`) and therefore kills the run. There is no
  per-call error classifier, no "recoverable tool error" option, and no hook that
  can convert a tool result after the fact; the node either unwraps an
  `interrupt` or fails.
- `compose.ToolsNodeConfig.UnknownToolsHandler` exists and is the Eino-native
  seam for a hallucinated tool name — it is **not** used here (already recorded as
  residual (a) on `TOOL-REFUSAL-RESIDUAL`) and is the candidate fix for that
  sibling, not this one.
- `einotool.StatefulInterrupt` / `GetInterruptState` / `GetResumeContext` are
  reused unchanged for the approval suspend/resume path.
- The pre-flight reuses the repository's existing §5 seam
  (`internal/runtime/tool_failure.go` `classifyToolFailure`, NUDGE-DESIGN) instead
  of adding Eino machinery: the classification vocabulary already exists, the
  gap was which errors hand their identity to it.

Conclusion: no Eino component satisfies this requirement without changing Vivy's
policy vocabulary, so the minimal Vivy-owned seam was extended (typed argument
errors + refusal extraction), which is the "custom code as exception" branch.
No new `Eino*` type and no second runtime or interrupt type was added.

## Why the pre-flight lives in the adapter, not in `Service.handleInterrupt`

The resume channel only carries `approved`/`denied`; an interrupt that cannot be
built has no resume vocabulary. Rejecting the call at the adapter, *before*
raising the interrupt, keeps the suspend/resume contract unchanged and keeps
auto-approved calls on today's path. The human is never asked to approve a call
whose proposal cannot be built.

Cost accepted: one extra `PrepareProposal` per first-execution approval
interrupt (the planner runs, the proposal is discarded, and the interrupt path
then runs the tool with its own planning step). It is pure CPU on a limited set
of tools, no extra I/O beyond the read the planner already performs, and it buys
the whole class.

## Scope decisions recorded during implementation

- `internal/tools/bash.go`'s deny-table fallback needs no change: the adapter
  refuses `InvocationDenied` before the tool body is reached, so that line is
  defense-in-depth. (Wrapping it in a sentinel was rejected: the import
  quarantine forbids `internal/tools` importing `internal/runtime`.)
- The tools-layer decode conversions are defense-in-depth as well —
  `tooladapter.go`'s `ValidateArgs` already turns a malformed call into
  `invalid_arguments`. They were converted anyway so the tool layer has one
  identity for "the model's arguments are wrong", including for its own direct
  callers and for shapes the declared params do not cover.
- A `patch`/`multiedit` target that does not exist was converted (rather than
  left fatal) after checking the live shape: a fresh run workspace is empty, so
  the model's first patch attempt is exactly this case — leaving it fatal would
  have kept a reachable member of the reported class.
- Still fatal on purpose, recorded as `TOOL-STALE-GUARD-RUN-FATAL`: the
  stale-read guard, the two `proposal stale` guards, non-caller-derived size
  caps, `internal/tools/agent.go`'s argument guards, and a patch target that
  exists but is a directory. §5 has no reason word for a state conflict, and
  inventing one is a vocabulary change this cut was scoped out of.

## Parallel-lane housekeeping

- The root tree held a second, unrelated lane ("审批快捷卡片": `ChatInput.tsx`,
  i18n surfaces, `ApprovalQuickCard`). Per the approved plan it was moved,
  uncommitted and byte-identical, to a new worktree
  `../agent-vivy-approval-quick-card` on branch `feat/approval-quick-card`
  (HEAD `70ed55ef`); the move was verified by file hashes and by comparing the
  `git diff` output of both trees. Its log files moved with it.
- `ui/src/components/chat/ChatInput.tsx` is *also* edited by the other live lane
  (`../agent-vivy-context-compact-merge`). That worktree was not touched; whoever
  lands second rebases. This deliverable does not edit UI files.
- The lane's files had been quiet for ~38 minutes before the move (its last
  writes were its own log and build artifacts); the move is recorded here rather
  than assumed.

## Process notes

- **A byte-order mark broke `just ci`, and the gate caught it.** Re-pinning the
  digest with Windows PowerShell 5.1 `Set-Content -Encoding utf8` wrote a UTF-8
  BOM into `sdk/internal/assembly/conformance_results.json`. The first full
  `just ci` then failed in `ui-core`: `stage-ui` panicked with
  `conformance: decode release result bundle: invalid character 'ï' looking for
  beginning of value` (0xEF is the first BOM byte). Removing the mark and
  re-running `just ui-ci` and then `just ci` cleared it.
- **The same `Set-Content` round trip also corrupted UTF-8 text.** Because the
  pipe decoded the file with the machine's ANSI code page, `§5` came back as
  `搂5` and two em dashes came back as `鈥?` — one of them inside the
  model-visible Eino read-path string
  (`[image file … — use the read_file tool to attach it]`). A final non-ASCII
  scan of every modified file found all six spots (three in
  `filesystem_backend.go`, three comments in `filesystem_backend_test.go`);
  all were repaired and re-scanned clean, and the source digest was re-pinned
  once more (the delivered value is `04f63283…`) because the repair touches
  `internal/`.
  Lesson for the next pass on this host: write files with
  `[IO.File]::WriteAllText(path, text, UTF8Encoding($false))`, or use the file
  tools, not `Set-Content -Encoding utf8`; and scan modified files for
  non-ASCII after any PowerShell text round trip.
- **A second lane was writing `internal/` in the root tree during this cut.**
  While the gates ran, `internal/provider/resolving.go`, `internal/rpc/*.go`,
  `internal/runtime/compaction*.go` and a set of `ui/` files changed under the
  working tree (mtimes inside the CI window; one mid-write state produced
  `vet.exe: internal\rpc\control.go:5792:21: undefined:
  provider.ThinkingLevelSurface` in an otherwise passing recipe). Effects on this
  deliverable: the source digest was first computed from the shared root tree and
  therefore described that lane's uncommitted `internal/` content — the release
  gate caught the mismatch, the pin was recomputed in a detached worktree at
  this change's own commit (a detached `.workspace/wt-pin` worktree standing at
  the commit that carries this log), and the final `just ci` record was taken in
  that worktree. The
  staging set was restricted to this lane's paths so the other lane's files were
  never committed. No file of that lane was modified or reverted by this cut.
- The board contradiction is resolved: `TOOL-REFUSAL-RESIDUAL` was `OPEN` on
  `docs/TODO.md` §0.1 while `docs/DEFER.MD` carried it as DEFERRED with a
  "restart when the owner re-prioritizes" condition. The owner re-prioritized in
  this session, so §0.1 is the live row (PARTIALLY FIXED) and the DEFER entry is
  marked SUPERSEDED, kept for history.
- The real-path smoke needed two host workarounds, recorded here so the next run
  does not rediscover them: Playwright 1.62.1 in `ui/node_modules` expects
  `chromium_headless_shell-1234`, while the machine has revision `1243`, so the
  smoke script passes `executablePath` explicitly; and `pnpm dev` aborts with
  `ERR_PNPM_ABORTED_REMOVE_MODULES_DIR_NO_TTY` when driven through the bundled
  pnpm, so Vite was started directly from `ui/node_modules/.bin/vite.cmd` with
  `VIVY_BACKEND_ADDR` set. A fresh backend home also shows a one-time "Persona
  onboarding" modal that intercepts the composer; the script dismisses it.

- **One commit, not two.** The plan intended cut 1 to be committed before this
  cut started. Cut 2 touches the same files, and
  `internal/sourcehash.Tree`'s digest must match the tree of every commit that
  carries it, so a synthetic two-commit split would have produced an
  intermediate commit whose pinned digest did not describe its own tree. Both
  cuts therefore land as one focused commit for the same theme (a model mistake
  must not end the run), with two log directories. The staging set is exactly the
  files this work touched — the moved lane is not part of it.
- The second log directory is deliberate: the two cuts have independent
  verification evidence (their own pre-fix replays and CI records) and cut 1 was
  written and reviewed before cut 2 existed. The AGENTS.md log note allows one
  folder to hold several shipped cuts of the same theme; splitting into two
  sibling directories was chosen to keep each cut's record intact, and each
  points at the other.
- The board contradiction is resolved: `TOOL-REFUSAL-RESIDUAL` was `OPEN` on
  `docs/TODO.md` §0.1 while `docs/DEFER.MD` carried it as DEFERRED with a
  "restart when the owner re-prioritizes" condition. The owner re-prioritized in
  this session, so §0.1 is the live row (PARTIALLY FIXED) and the DEFER entry is
  marked SUPERSEDED, kept for history.
