# Verification

Toolchain: Go 1.26.4, pnpm 11.19.0, repository just/PowerShell recipes on Linux.
Task checkout and its bootstrap-cloned Laputa sibling are isolated under
`/workspace/issue51`; the existing shared checkout and sibling are untouched.

## Completed focused checks

- Baseline `just ensure-laputa` and `just ui-build`: PASS from the clean latest-main
  clone, with a fresh sibling at locked Laputa `4b2bec2c`.
- First-party catalog edit/omitted-hash regression: failed first at the expected
  stale authored-hash gate, then PASS; external catalog ref/hash negatives PASS.
- Static verify source-edit regression: failed first at the expected stale hash
  gate, then PASS. The pre-existing semantic verify coverage remains.
- Declarative default inventory regression: failed first on source-tree I/O,
  then PASS without reading or hashing files.
- Source symlink confinement regression: failed when static verify accepted a
  symlink, then PASS after preserving the check in the source firewall.
- `go test ./internal/modules/defaults ./sdk/internal/assembly -count=1`: PASS.
- Focused `TestVerify*` and `TestStageUI*`: PASS. Staging exercises internal test
  edits and an edited unselected first-party Module; stale evidence is absent.
- SDK CLI verify, minimal pack and inspect-artifact: PASS. Minimal has 7 Modules;
  the artifact manifest and read-only executable inspection agree.
- Opt-in release test without the environment flag: explicit SKIP as designed.

## Pending candidate gates

The full live Provider/Host conformance suite passed (111.599s) with the
unchanged historical bundle and edited internal sources. Independent review
found and then confirmed the fix for Pack's final source-stability check:
`VerifyUnchanged` compares live sources to captured identities using the original
normalization hints. T1/T2 unchanged and edit-during-build regressions PASS.

Complete `just ci`, fresh development
readiness, final SDK integrity regressions and exact-commit GitHub Actions are
in progress. This record does not claim those outcomes before completion.

An early live run exposed a legacy first-party hash-drift negative. It was
changed to an external pinned-source negative, retaining the check. Early red
runs are diagnosis evidence, not passing evidence. The explicit release check
also rejected changed internal source against the historical bundle; its final
negative run is tracked separately from development success.

No bundle digests or historical pass flags were refreshed. No merge or deployment.

Local aggregate checks use task-owned `/var/tmp/vivy-issue51` fixtures outside
the environment's injected ancestor `.git` mounts, preserving VCS stamping and
instruction-discovery assertions. Superseded runs were stopped after the review
fix; they are not final success evidence.
