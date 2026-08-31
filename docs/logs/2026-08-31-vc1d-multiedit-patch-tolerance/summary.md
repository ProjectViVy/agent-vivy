# VC-1d: multiedit tool + patch whitespace tolerance

Date: 2026-08-31 · Lane: `agent-vivy-vc0` worktree, branch `feat/vc1a-bash-tool`

## What changed

Two Crush-parity edit-path upgrades, both main-kernel capabilities per the
VC-0 capability-layer decision:

1. **`multiedit` tool** (new, `internal/tools/multiedit.go` +
   `MultiPatchFile`/`PrepareMultiPatchFile` on the filesystem backend):
   applies an ordered list of `{old_string, new_string, replace_all}`
   replacements to one file atomically. Edits run against an in-memory
   buffer; the file is written once at the end, so a failing edit leaves the
   target untouched. The tool registers through a new `MultiPatchOperations`
   type-assertion seam (same convention as `JobOperations`/`GrepOperations` —
   the core `FileOperations` interface and its existing stubs stay stable),
   and `multiedit` joins the default `tools.enabled` surface after `patch`.
   Mutating tool: HITL proposals flow through `PrepareMultiPatchFile` with a
   combined diff and a precondition hash, exactly like `patch`.

2. **`patch` whitespace-tolerant fallback** (new bounded engine,
   `internal/runtime/patch_engine.go`, shared by `PatchFile`,
   `PreparePatchFile`, `MultiPatchFile`, and the Eino `Edit` middleware
   path): exact substring matching stays first. Only when the exact match
   finds nothing does a line-anchored fallback run:
   - lines match with leading/trailing whitespace ignored; a trailing
     newline on `old_string` does not anchor the match to a following blank
     line;
   - replacement lines keep the **file's** indentation whenever the model's
     `new_string` indentation equals its `old_string` indentation (a guessed
     indentation is corrected to the file's), and keep the model's own lines
     when it deliberately reindented;
   - line-count-differing replacements are inserted verbatim;
   - a matched line's trailing CR transfers to the inserted line, so CRLF
     files stay consistent;
   - ambiguity and not-found are reported with the same error shapes as the
     exact path (`(whitespace-insensitive)` hint added for the ambiguous
     case).

   The `patch` tool description states the tolerance honestly so the model
   knows the behavior.

## Explicitly not done (deferred, with reasons)

- **stale-read protection / filetracker + `file_versions` table (RB-1 L1)**:
  the board merges stale-read protection and file-version history into one
  storage design, and that design waits on the user's O1..O6 ratification
  (rollback research §6: retention policy, storage shape, archive scope,
  restore governance, bash boundary). Not implemented here; no half-designs.
- `.vivyignore`: explicitly rejected by D7 (`.gitignore` only).

## Verification

See `verification.md`. Acceptance for humans: `acceptance.md`.

## Constraint compliance

- Crush is FSL-1.1-MIT: the tolerance policy is Vivy's own bounded design
  (documented above and tested); no Crush source was read or copied for it.
- No new features beyond Crush's surface: `multiedit` matches Crush's tool;
  the tolerance fallback matches the `edit`-tool gap listed in the parity
  research §4 item 1.
