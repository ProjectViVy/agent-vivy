# CH-R-5: generation.json seam-classified plugin listing

## What changed

`vivy-sdk pack` / `inspect-artifact` now fulfill the VIVY-CHANNEL-PACK.md §10
promise: the generation manifest lists every packed plugin by seam with the
manifest fields a reviewer needs.

- `sdk/internal/pack.go`: `Artifact` gains `plugins[]`
  (`omitempty`); each entry is `name / version / seam / grants /
  transport / source_ref / tree_hash`. Flat per-plugin entries with a `seam`
  field satisfy "按 seam 分类列出" — telegram prints as `channel`, not tool.
- `tree_hash` is a deterministic sha256 over the plugin source tree
  (`hashPluginTree`): `filepath.WalkDir` regular files in sorted order, each
  contributing its slash-relative path + length prefix + content bytes. Same
  tree → same digest on every OS; any byte change → different digest.
- Channel entries carry `transport` (from `manifest.channel.transport`) and
  the existing `tools[]`/`recipe` fields are untouched, so old consumers of
  `generation.json` keep working (`InspectArtifact` validation unchanged).
- `source_ref` for a plugin is `file:<absolute plugin dir>` — the tree the
  EXE was actually built from.

## What was explicitly not done

- No `vivy-sdk inspect` CLI output changes beyond what generation.json now
  carries (the §10 "inspect 按 seam 分类列出" printing can build on the new
  field later).
- No runtime allowlist, install, or promote-path change — pack-side manifest
  only (卸通道插件 = 从 plugins: 删一行，再 pack, unchanged).
- webhook/listen/a2a transports stay out of this batch (§9.2), so transport
  is always `poll` for current channel plugins.

## Files

- `sdk/internal/pack.go` — Artifact.Plugins, artifactPlugin, hashPluginTree
- `sdk/internal/pack_test.go` — assertPluginEntry helper, TestHashPluginTree,
  entry assertions in fake-channel / hello-fs / telegram+discord pack tests
