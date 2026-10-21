# Finite Studio ledger transition fragment

This is the D06-owned fragment for the independent Studio SQLite ledger. It is
not a core Journal migration and is never imported by the daily runtime.

Stop Studio before converting. Supply a consistent disposable legacy ledger
copy together with accessible artifacts referenced by its actual `file:`
SourceRefs and its sibling `rollback/` directory. Non-file source selection refs
are retained. The new target home must not exist, its parent must exist, and it cannot
overlap the source home (including filesystem aliases). Never point this tool at tenant databases.

```text
go run ./internal/transition/studioledger/cmd --source-ledger <legacy-home>/studio.db --target-home <new-studio-home>
```

The converter recognizes only the exact unversioned six-table legacy Studio
schema. It creates a marked final ledger with no artifact hash column, retains
all lifecycle rows, links and event allocator state, copies actual referenced
executables and rollback snapshots, and stores the private old-path/new-path
allocation map in the copied home. Generation and parent IDs do not change.
Missing artifacts, unfamiliar schemas, symlinks and broken lifecycle references
fail closed. A failed attempt removes only its own staging directory and never
publishes a target. Retrying the same source and target yields the same artifact
paths. Existing completed targets are refused.

The directory rename publishes only a verified copied home. It does not switch
Studio configuration, migrate a tenant Journal, install a candidate or release a
Generation. Inspect and reopen the target with the final Studio executable
before an owner changes the configured home. Preserve the old home, matching
old executable and any daily installation for rollback. The caller owns
quiescence, a consistent source/WAL backup and the final configuration switch.

D14 must build and retain this finite executable from its recorded source
commit, integrate the other owners' transition fragments, and remove this
package and its command from the final runtime/release tree. The final Studio
runtime admits only its marked final shape and rejects the old shape before
CREATE or writes. An unavailable finite artifact remains a release blocker.
