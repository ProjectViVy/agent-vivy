# Verification — 物种工作台

Commands run from the repository root (Git Bash, Windows).

## Focused unit tests (workbench host half)

```text
node --test studio/dsh-species-workbench/recipe.test.mjs \
            studio/dsh-species-workbench/species.test.mjs \
            studio/dsh-species-workbench/confirm.test.mjs
→ tests 12, pass 12, fail 0
```

Covers: NG-25 argv gate (no spawn without confirm; `--actor human --yes`
only after it), pack argv walks the v1 contract and `--with` never
appears, recipe-name path allowlist (draft bare name, `recipes/<name>`,
draftsDir-absolute pass-through; traversal / wrong-extension / absolute
outside-dirs refused), structural recipe validation (both list shapes,
key whitelist, duplicates), species inspect handshake against fake
fetch/WebSocket (happy path, unreachable backend, no-WebSocket
degradation, RPC error frames).

`node --check` on `client.js` and `index.js`: clean (one paren-count
defect in `client.js` found and fixed this way before commit).

## Environment defect found during the `just ci` gate (pre-existing, not this lane)

First `just ci` run failed `ui-core`: 7 UI test files / 33 tests with
`TypeError: localStorage.clear is not a function`. Diagnosis:

- `git status` proved `ui/` was byte-identical to HEAD (only a
  line-ending wobble in `ui/src/generated/assembly.ts`, restored) —
  the failures exist on untouched code.
- `ApprovalTimeoutCard.test.tsx` fails standalone under PATH node
  **v25.9.0** (`C:\Program Files\nodejs`) and passes standalone under
  **v24.14.1** (`C:\nvm4w\nodejs`): Node ≥25 defines its own global
  `localStorage`, shadowing the happy-dom environment object vitest
  installs per file.
- Full suite under Node 24 (pinned via PATH):
  `vitest run` → **48 files / 392 tests, all pass**.

### Conformance bundle went stale in this lane (found + fixed here)

The Node-24 gate run then failed exactly one test:
`sdk/internal/conformance#TestCheckedInProviderConformanceMatchesExecutedSuites`.
Per-entry diff of actual vs checked-in evidence: all 345 suites executed and
passed; the only drift was `sourceSha256` on the 5 providers whose source
root is `internal/` (`vivy/protected-tools`, `vivy/mcp-host`,
`vivy/skill-source`, `vivy/provider-profiles`, `vivy/context-source`):
`internal/sourcehash` hashes the whole `internal/` tree, and **this
delivery's C1 commit (`9c0dd21`, `internal/studiocore` pack rewrite)**
changed it. The bundle still named the pre-C1 digest
`724b0f9a…`. Fix: advanced the 5 occurrences in
`sdk/internal/assembly/conformance_results.json` to the executed digest
`f0ab9eace6bae6bf0df1d0798fdbcae726368d1901179349c1fcab9ffdb30a07` —
per the producer-gate rule, only because the executable reproduction run,
the suites, and the Generation matrix on the same tree all passed. The
digest appears nowhere else (verified by repo-wide grep).

Gate rerun after the bundle advance:

```text
PATH=/c/nvm4w/nodejs:$PATH just ci
→ CI-EXIT=0 (fmt-check, ui-ci incl. 48/392 vitest, vet, test incl. the
  conformance reproduction + Generation matrix, headless-compile,
  plugin-ci all green)
```

## Notes

- `ui/src/generated/assembly.ts` / `ui/src/routeTree.gen.ts` were touched
  only by CI codegen line-ending normalization; restored, not committed.
- Strict-validate throwaway dirs accumulate under
  `data/studio-home/species-workbench/validate/check-*` (scratch only;
  cleanup deferred).
