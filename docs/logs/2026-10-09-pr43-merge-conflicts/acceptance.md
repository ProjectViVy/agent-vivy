# Acceptance

- PR #43 contains main `83e46efc6c6d56a6a07ba7dbf60fae9617e6268e` as an
  ancestor, and GitHub no longer reports a merge conflict against that main.
- The RPC implementation and its regressions match the previous PR head.
- Bash redirect classification and its regressions match the incorporated main.
- The five internal conformance pins equal the canonical digest of the merged
  internal source tree, and the executable conformance reproduction agrees.
- Full repository CI remains a separate gate; clearing merge conflicts does
  not assert that every CI job passes.
