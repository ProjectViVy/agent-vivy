# Headless form declares its identity; masks arm in every form — 2026-10-06

## What changed

The unsealed dev binary is now treated as what it is: the species' headless
form. The generated default Assembly declares its own form identity
(`HeadlessGenerationID = "vivy-headless/1"`) as part of the artifact, so every
binary built from this repository — `just dev` split pair, `go run ./cmd/vivy`,
the embedded-UI binary, `vivy-code` — arms masks, primary admission (MASK-3
prompt snapshots), and the module action host through the same contracts a
packed build uses. Selecting a mask in the dev pair no longer fails with
"module action capability is not configured".

Changes:

- `sdk/internal/assembly/runtime_generate.go`: `GenerateRuntimeAssembly` gains
  a variadic `WithFormIdentity(id)` option. When set, the emitted source
  declares `const HeadlessGenerationID` and assigns it to
  `RuntimeAssembly.GenerationID` inside `BuildDefault()`. Unset (the pack
  path's two call sites) emits byte-identical output to before.
- `sdk/internal/cmd/generate-default/main.go`: the committed default
  composition passes `WithFormIdentity("vivy-headless/1")`.
- `internal/generated/assembly/zz_default.go`: regenerated through the
  generator (no hand edit); diff is exactly the const plus the field
  assignment.
- `internal/app/app.go`, `internal/app/masks.go`: comments updated from
  "unsealed embedders stay dormant" to the headless-form semantics. No logic
  changed: `sealed := presentation.SealedGeneration || generationID != ""`
  now evaluates true for the default composition because the artifact itself
  carries the identity. Locale/provenance behavior still keys on the
  `presentation.SealedGeneration` constant and is untouched.
- `plugins/vivy-masks-ui/module.go` + `vivy-module.yaml`: re-pinned the stale
  module source hash (see Repair below) — a prerequisite for regenerating any
  Assembly at HEAD.
- `docs/TODO.md` §0.1: new MASK-CONTINUITY-SNAPSHOT row (pre-existing gap,
  see notes.md).

## Repair: stale vivy/masks-ui source pins

Commit `3a372a99` (previous mask-UI lane) changed 14 files under
`plugins/vivy-masks-ui` and updated `vivy-module.yaml`'s `source.sha256`, but
not the `SourceSHA256` const in `module.go`. Since `sdk/internal/cmd/generate-default`
verifies every module source tree against the Go descriptor's pin, Assembly
regeneration (and pack) failed closed at HEAD for vivy/masks-ui. Both pins
now carry the common self-consistent fixed point
`cc325f54d7c64e3bd60a9c831ba4c65b638ec7536df22eaa5e5c937ac3752257`
(`sourcehash.Tree` masks occurrences of the declared digest, so both files
pinning the same value is the invariant every other UI module already
follows); `vivy-sdk verify plugins/vivy-masks-ui` passes.

## What was explicitly not done

- No schema change, no second runtime path, no dev-only mask semantics: the
  headless form runs the identical MASK-3 admission and action-host code as a
  packed build.
- `presentation.SealedGeneration` stays `false` in dev; locale and developer
  `.env` behavior are unchanged.
- `internal/studio/inspect.go` still reports `builtin` for the dev binary —
  that surface describes UI-artifact provenance (sealed manifest + UI hash),
  which the headless form intentionally does not claim.
- The continuity admission gap (marker without snapshot row) is recorded as
  MASK-CONTINUITY-SNAPSHOT, not fixed here; it predates this change and
  exists on every packed build today.
- The packed-binary masks e2e (`ui/e2e/masks.spec.ts`) was not run: it needs
  a packed candidate binary; verification used the Go suites plus a real
  browser smoke instead.
