# Verification — 2026-10-17 provider agentic spike plan

All commands were run from the lane worktree `.worktrees/provider-agentic`
(branch `docs/provider-agentic-plan`) unless stated otherwise.

## Gate selection

This delivery changes documentation only. Per root `AGENTS.md` ("For minor
editorial or agent-instruction changes, review the diff and run
`git diff --check`; full product CI is unnecessary"), the full `just ci` gate was
**not** run. The substituted checks are below, plus a structural proof that no
compiled or generated input changed.

`just ci` was additionally not run for a second, independent reason: the root
tree hosts an **active peer lane** (see "Root-tree conditions" below), so running
the full product gate here would exercise another lane's work-in-progress rather
than this delivery.

## Commands and results

| # | Command | Result |
|---|---|---|
| 1 | `git status --short -- internal sdk ui cmd plugins faces bml go.mod go.sum` | **empty** — no source, generated, dependency, or packaging file changed |
| 2 | `git diff --check` | exit 0; no whitespace or line-ending errors |
| 3 | `git diff -- sdk/internal/assembly/conformance_results.json` | **empty** — the `internal` source digest is untouched, the expected property of a `docs/`-only change (`internal/sourcehash/tree.go` hashes only `internal/`) |
| 4 | `git ls-files --others --exclude-standard` after staging | **empty** — no unstaged new file left behind |
| 5 | `git status --porcelain` in the root tree, before and after | **27 paths both times** — the root tree's state is unchanged by this delivery |
| 6 | Module-cache citation sweep (scripted, raw-byte line indexing) | **88 checks, 0 failed** |
| 7 | Repo-path citation sweep (scripted, raw-byte line indexing) | 76 checks, 9 failed initially → all 9 corrected in the documents, then re-verified in a 42-check second pass with 0 remaining failures |
| 8 | Referenced-path existence sweep | all cited repository paths present |
| 9 | `just ci` | **not run** — docs-only delivery; see "Gate selection" |

## Citation sweep (this delivery's substantive check)

Because the documents cite module-cache paths and line numbers as their entire
evidence base, every citation was machine-checked rather than eyeballed.

**Method.** Two scripted passes read each cited file with
`[System.IO.File]::ReadAllText(...)` split on `\n` and asserted that the claimed
line matches the claimed symbol by regular expression. Raw-byte splitting was
required, not incidental: an initial pass using `Get-Content` reported two
*false* failures on `eino@v0.9.21/components/model/option.go:116,143` because
`Get-Content` returned 254 line segments for a file with 256. The raw-byte method
agrees with ripgrep and is the one used for the reported numbers.

**Pass 1 — module cache (88 checks, all pass):** `eino@v0.9.21`
(`components/model/*`, `adk/*`, all eight `adk/middlewares/*` `NewTyped` entry
points), `agenticopenai@v0.2.4` (`chat_model.go`, `responses_model.go`,
`responses_convertor.go`, `responses_event_convertor.go`, `option.go`,
`consts.go`, `go.mod`), `agenticclaude@v0.1.7` (`model.go`, `convertor.go`,
`event_convertor.go`, `go.mod`).

**Pass 2 — repository (76 checks, 9 failures, all corrected):**

| Citation as first written | Actual | Correction |
|---|---|---|
| `internal/provider/adapters.go:30` | `:29` | `ErrAdapterUnknown` is `:29`; `:30` is the comment for `:31` |
| `internal/provider/resolving.go:399,403,408` | `:371,375` | the file is 382 lines; the bound-model methods are at `:371`/`:375` |
| `internal/provider/resolving.go:67,90` | `:61,86` | `type resolvingChatModel` is `:61` |
| `internal/modelhost/profile.go:35` | `:33` | `ProfileState` is `:33`; `:35` is the const block |
| `internal/modelhost/profile.go:162` | `:154` | `ResolveExecutable` is `:154`; `:162` is the capability check inside it |
| `internal/rpc/control.go:5530` | `:5521` | `profileSelectionError` is defined at `:5521`; `:5530` is its `switch` arm |
| `sdk/internal/conformance/reproduction_test.go:456` | `:476` | the Port evidence anchor line |
| `ui/src/lib/api.ts:573,576` | `:569,572` | `ProviderCapabilityState`, `ProviderAdapterState` |
| `ui/src/lib/api.ts:604,607` | `:601,603` | `executable`, `state` fields |
| `ui/src/components/chat/MessageBubble.tsx:167` | see below | the file has no reasoning code at all |

**Pass 3 — second pass (42 checks, 0 failures)** over the remaining references:
`go.mod` pins and the pseudo-version precedent, `sdk/...` evidence anchors,
`ui/...` settings and chat paths, `internal/provider/data/*`,
`internal/modelhost`, `internal/rpc`, `.gitignore`, and the
`docs/plans/provider-registry/README.md` standing-orders range (corrected from
`:32-51` to `:32-50`, since `:51` is blank).

**The one claim that did not survive** was inherited from the 2026-09-18 record
and from the earlier investigation of this repository rather than read from the
committed tree: that `ui/src/components/chat/MessageBubble.tsx:167` holds a
reasoning disclosure block that never receives data. That file contains no
`reasoning` or `thinking` text. The surface exists and is complete elsewhere —
`ReasoningRow.tsx:23`, `ChatView.tsx:12,199`, `run-rows.ts:47,224`,
`store.ts:412`, with tests at `ChatView.test.tsx:65` — so **the gap is on the
backend**, where no adapter produces reasoning content. Both `S1` and `S3` now
state the corrected version, and the summary records the correction.

Note on `sdk/internal/testdata/default-generation.expected.json:168-172`: the
sweep flagged this as a failure only because `:168` is the array opener rather
than an adapter name. Lines `:169-171` are the three adapters and `:172` is the
closer, so the range citation is correct and was left unchanged.

## Structural assertion (why no product gate was needed)

`docs/` is outside every hashed or compiled input:

- `internal/sourcehash.Tree` hashes regular files under `internal/` only.
- No Go package imports anything under `docs/`.
- The UI build reads `ui/`, not `docs/`.
- The generated Assembly is produced from `internal/` and `sdk/` inputs.

Therefore a `docs/`-only change cannot alter build output, the sealed Generation,
or the conformance digest. Check #3 confirms this empirically.

## Checks deliberately skipped, and why

| Skipped | Reason |
|---|---|
| `just ci` | docs-only delivery, and the root tree hosts an active peer lane |
| `go test ./...` | no Go file changed |
| `pnpm --dir ui test` | no UI file changed |
| Browser smoke at `http://127.0.0.1:3015` | no user-visible behaviour changed; the plan documents the smoke each future phase must run |
| `go run ./sdk/internal/cmd/generate-default` | no Assembly input changed; regenerating would be a no-op at best and a dirty artifact at worst |
| Executing S0/S1/S2 | explicitly out of scope for this delivery; the user selected "documentation only" and the spike is a separately scheduled stage |

## Root-tree conditions observed

The root working tree was **not clean and a peer lane was actively writing** at
the start of this delivery. Files under `internal/rpc/`, `internal/runtime/`,
`internal/provider/`, `ui/src/components/chat/` and `ui/src/i18n/` were modified
between 04:18 and 04:39 on 2026-10-17, and the untracked `.tmp-build/` directory
was touched at 04:43 — the same minute as the `Get-Date` reading taken at the
start of the lane.

This is the reason the delivery was made in an isolated worktree
(`.worktrees/provider-agentic`, branch `docs/provider-agentic-plan`) rather than
the root tree: `AGENTS.md` requires a worktree when the root tree is not clean
and another lane is active. Check #5 confirms the root tree's 27 dirty paths are
unchanged by this delivery.

One consequence is worth recording for the plan's readers: the peer lane's
in-flight edits include `internal/provider/resolving.go`, so line numbers read
from the **root working tree** for that file do not match the committed state.
Every citation in this plan set was therefore verified against the **worktree's
committed** files, which is what a reader of the plan will see.

## Evidence checked while authoring

All Eino/EinoExt claims were read from the module cache at the target versions,
not from online documentation:

- `eino@v0.9.21`, `agenticopenai@v0.2.4`, `agenticclaude@v0.1.7` are present in
  both `go\pkg\mod\...` and the module download cache, confirming they are
  tagged releases rather than guesses.
- The two enabling facts for the whole concept were verified directly rather than
  assumed: `model.WithTools` is consumed by both agentic components
  (`agenticopenai/responses_model.go:491`, `agenticclaude/model.go:550`, both via
  `model.GetCommonOptions`), and `adk/chatmodel.go:280` is
  `Model model.BaseModel[M]`, so the typed agent accepts an `AgenticModel`
  without a conversion.
- `github.com/cloudwego/eino-ext/components/model/claude v0.1.26` does **not**
  exist in the cache and is not cited anywhere in the plan set.

## Post-condition

The repository is unchanged except under `docs/`; the branch is local and was not
pushed.
