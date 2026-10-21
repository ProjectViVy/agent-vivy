# Human acceptance

1. Present a text file, binary file and empty file in a disposable workspace.
   Each appears under its own opaque item identity. Presenting twice produces
   distinct items; retrying the same call/input returns the original set.
2. Change one byte without changing length, restore the old file timestamp and
   start its old download. It must report changed. Re-present the edited file
   to download that new content.
3. Start a valid multi-chunk download, then edit the workspace file. Remaining
   chunks must still complete the original immutable bytes. Duplicate windows
   replay exactly; skipped/changed windows, mixed handles, expired/foreign
   handles and revoked session access fail safely.
4. Restart the service/store. Still-visible presentation bytes and receipt
   results remain, including empty files. A historical item with no retained
   bytes explicitly requests re-presentation. Closing/expiring a transfer does
   not remove a visible presentation's retained bytes.
5. Rewind/edit the owning turn or delete its session. Hidden bytes disappear
   atomically, including the actual unified edit admission path. Rollback keeps
   the prior visible bytes. Fork provenance alone and unowned runs remain.
6. Switch sessions while check/preview/download is pending. No old result
   repopulates the new session and no canceled download is saved.
7. Export the same session twice and create two diagnostic bundles. Each has a
   distinct immutable name; downloading the first still returns its first bytes.
   Retired digest parameters are rejected; no digest field appears in results.

These are reproducible steps, not claims of human execution. Standard3015 smoke
was explicitly deferred by the owner. D09 Story human acceptance and D14 final
converter/release acceptance are separate and remain open.
