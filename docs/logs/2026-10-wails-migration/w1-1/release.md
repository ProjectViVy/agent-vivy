# W1-1 release

No release is produced for this story.

- W1 lands library surface only: `sdk/host/v1` plus additive
  `CloseContext`/`Next`/`ServeDone` in `internal/*`. Nothing is packed,
  tagged, or published; the pending archive refs and release tag stay
  untouched per the migration README.
- The C ABI (`cmd/vivy-shared`) and the CLI keep their existing behavior —
  `Poll` remains the drain path for the ABI transition period and
  `Close()` wrappers preserve synchronous semantics. W6 owns the C ABI
  retirement.
- Rollback: revert the W1 commit; no data-root, config, or wire-format
  changes exist to migrate back.
