# settings.Save concurrent writes corrupted the document (Windows rename access denied)

## Symptom

The `model-refresh` case in `just ui-e2e` intermittently showed a red
`internal error` bar after adding a model, and the newly added model disappeared
from the registry row. The error came from RPC `internalError()`
(`internal/rpc/control.go`, -32603; detail is intentionally discarded and
contains no internal details).

## Root cause

Document reads and writes in `internal/app/settings` were not synchronized:

1. `Save` used a fixed `path+".tmp"` temporary file. Two concurrent Save calls
   could interleave writes to the same temporary file, so the file moved into
   place by rename could be corrupted and a subsequent `Load` would fail to
   parse it.
2. On Windows, `os.Rename` failed with `Access is denied` when replacing a file
   that was still open through a concurrent read handle (the `os.ReadFile`
   window in `Load`).

Both paths caused the next `Load` to fail, while the RPC layer saw only
`internal error`.

## Fix (the standalone commit corresponding to this directory)

`internal/app/settings/settings.go`:

- Added package-level `fileMu sync.Mutex`; `Load`'s ReadFile and `Save`'s
  temporary-file write plus rename now both run under the lock, eliminating
  overlapping read/write handles.
- `Save` now uses `os.CreateTemp(dir, base+".*.tmp")`: each call exclusively owns
  its temporary file, so concurrent writes no longer share a staging area; all
  failure paths clean up the temporary file.
- Semantics are unchanged: Save still replaces the whole document, and Load still
  returns the complete document. Cross-handler Load→modify→Save remains
  last-writer-wins and is tracked as a TODO (a future interface such as
  `settings.Update(path, fn)` will close that gap).

## Explicitly not done

- No RPC-layer error-detail passthrough (preserving the contract that
  internalError does not disclose details).
- No change to read-modify-write semantics in this window (see the new TODO row
  in §0.1).
