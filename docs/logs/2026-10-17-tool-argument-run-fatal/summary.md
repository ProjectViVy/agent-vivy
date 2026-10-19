# Tool argument mistakes and boundary denials no longer fail the run (cut 2)

Date: 2026-10-17
Lane: `feat/issue-58-unified-context`, root working tree.
Scope: Vivy kernel — `internal/runtime` filesystem/patch/search backends, the
tool adapter's refusal seam, `internal/tools` argument decoding.
Related incident: the same class as `run_8d376d0f457fa5e5`, whose first cut is
recorded in [`../2026-10-17-filesystem-path-run-fatal/summary.md`](../2026-10-17-filesystem-path-run-fatal/summary.md).

## Why a second cut

Cut 1 fixed the path spelling that produced `filesystem: invalid path` for
`list_dir {"path":"./"}`. The same failure class was still open in two places:

1. **Argument defects inside the tools.** `patch`/`multiedit` (`old_string was
   not found`, `matched N times`), `read_file` (`end_line must be >= start_line`,
   image over the read cap), `write_file` (`content exceeds N bytes`),
   `search_files` (empty query) and `glob`/`grep` pattern errors were plain
   `fmt.Errorf` values, so §5 classified them fatal and Eino's
   `compose.ToolsNode.Stream` ended the run.
2. **The approval path had a second, earlier seam.** The default `smart` preset
   is `workspace_write` + approval policy `ask`, so every effectful call is
   reviewed *before* it runs. `Service.handleInterrupt` builds the review
   proposal via `PrepareProposal`, so a `patch` whose `old_string` cannot match
   died while building the proposal — before the tool was ever invoked, and
   without the adapter's conversion being able to see it. Typing the invocation
   error alone would not have saved that case.
3. **Deliberate boundary denials** (`protected path denied`,
   `symlink path denied`) were plain errors on the same fatal path, so a model
   touch of `.env` also killed the run.

## What changed

- `internal/runtime/filesystem_backend.go`, `patch_engine.go`,
  `search_backend.go` — every caller-argument defect the model can submit now
  returns `*tools.ArgError` with the offending field (`path`, `query`,
  `content`, `old_string`, `new_string`, `replace_all`, `edits`, `pattern`,
  `end_line`) through one `argumentError` helper. Infrastructure errors (I/O,
  walk, ripgrep, stale-read, `proposal stale`, IPC caps) are untouched and stay
  fatal.
- `internal/runtime/tooladapter.go` — a pre-flight at the two first-execution
  approval interrupts (`approvalInterrupt`): if the tool's own
  `PrepareProposal` fails, the failure is classified with the existing §5 seam
  (`proposalFailure` → `classifyToolFailure` / `asToolRefusal`) and, on a
  model-driven leg, returned as the ordinary model-visible outcome instead of
  suspending for a review that can never be built. The refusal branch of
  `InvokableRun` was extracted into `publishRefusal` so both seams publish the
  same typed record, governance event, and text.
- `internal/runtime/sandbox_manager.go`, `filesystem_backend.go` — new
  `ErrProtectedPathDenied` / `ErrSymlinkPathDenied` sentinels, returned with
  `%w` from the workspace boundary and added to the adapter's refusal list:
  a boundary denial is `refused`/`policy_denied`/`not_executed`, never a run
  failure.
- `internal/tools/` — the nine tools that decode their own arguments
  (`bash`, `command`, `download`, `http_request`, `jobs`, `mcp`,
  `network_search`, `sequential_thinking`, `web_fetch`) now return
  `*ArgError{Field: "args"}` instead of a bare decode error, and an unknown
  `job_id` is an argument error rather than a backend error. (The adapter's
  `ValidateArgs` already refuses most malformed calls before this point; these
  conversions keep the tool layer consistent for its other callers and for
  shapes the declared params do not cover.)
- `filesystem_backend.go` — a patch/multiedit target that does not exist is an
  argument defect too (`patchTargetError`): neither tool can create a file, so
  the model is told to write it first, and the run continues. A missing
  `read_file` target was already recovered as `not_found`; other read failures
  (permissions, I/O) keep their infrastructure error.
- Tests: a backend/patch defect table asserting the typed field *and* the §5
  classification, a boundary-denial refusal test (plus a sentinel-level test for
  the symlink branch the host cannot create), a tools-layer decode table, and
  service-level tests through the real tools and backend — `patch` with an
  unmatched `old_string` under `ask` (no approval prompt, run completes),
  the same under an auto-approving policy, a protected-path read, and an
  allowlist guard proving infrastructure/lifecycle errors stay fatal.
- `sdk/internal/assembly/conformance_results.json` — the five `internal`-rooted
  `sourceSha256` rows re-pinned (`982fc2a3…` → `0019de13…`). Record:
  [`ci.md`](ci.md).
- `docs/TODO.md` / `docs/DEFER.MD` — residual row narrowed, stale-guard row
  added.

## Explicitly not done

- No change to the §5 vocabulary/allowlist in `docs/architecture/NUDGE-DESIGN.md`
  and no second runtime, interrupt type, or Eino component; see
  [`notes.md`](notes.md) for the Eino capability check.
- The stale-read guard and the `proposal stale` guards stay fatal: they are
  state conflicts, not argument defects, and §5 has no reason word for them
  (`TOOL-STALE-GUARD-RUN-FATAL`).
- An effectful call against a path that does not exist (`patch`/`multiedit` on a
  missing target) stays fatal: the `fs.ErrNotExist` allowlist row is restricted
  to readonly tools, and widening it is a product decision, not a mechanical
  conversion. Recorded on the residual board row.
- Non-model IPC size caps, the `agent` tool's object/empty/size argument guards,
  and the bash deny-table fallback in `internal/tools/bash.go` are unchanged; the
  last one is already unreachable through the adapter, which refuses
  `InvocationDenied` at `tooladapter.go`.
- No UI, RPC, storage, schema, migration, or product-contract change.
