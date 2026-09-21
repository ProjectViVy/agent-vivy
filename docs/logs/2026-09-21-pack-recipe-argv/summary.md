# Pack walks the v1 recipe contract

## What changed

`vivy-studio pack` was dead since the v1 compiler landed: `studiocore.Pack`
still spawned `vivy-sdk pack --with <plugin>` and parsed a v0
`generation.json`, while `parsePackArgs` (sdk/internal/frontend_v1.go)
rejects `--with` and the v1 manifest carries `generationId`/`recipeDigest`
instead of `id`/`artifact_sha256`/`source_ref`. The console's pack action
could therefore never succeed.

- `internal/studiocore/service.go`: `Pack(ctx, recipe, outDir, sources)`
  invokes `pack --recipe <file> --output <dir> [--source <dir>]...`,
  captures stdout separately, and decodes the single `Artifact` JSON the
  sdk seals there. No output-dir pre-creation (the sdk publishes by
  rename and rejects an existing dir). `recordGeneration` now consumes the
  decoded artifact: id = `manifest.generationId`, digest = sha256 of the
  packed binary, `source_ref = file:<binary>` (the eval launcher's
  contract). The v0 `sdkArtifact`/`readSDKArtifact` pair is deleted.
- `cmd/vivy-studio/main.go`: `pack` flags are `--recipe` (required),
  `--out`, repeatable `--source`; usage updated.
- `.agents/skills/vivy-studio-lifecycle/SKILL.md`: command table and
  Procedure step 3 synced to the recipe contract.

## Scope and non-goals

- `domain.Generation.Recipe` stays empty for freshly packed generations:
  the v1 manifest reports `recipeDigest`, not the v0 loop/world/plugins
  bill, and fabricating one would create a second source of truth.
- Historical records (`docs/logs`, `docs/COMPLETE.MD`, archived plans, the
  per-plugin READMEs' old `pack --with` snippets) are frozen prose and
  were not rewritten.
- No `--with` support was restored anywhere: the Recipe is the sole
  selection authority (PLG-P2).
