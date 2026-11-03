---
id: recipe-builder
type: module-design
status: draft
title: recipe-builder (`crates/vivy`)
parent: architecture
depends-on: [process]
---

## Responsibility

The only assembler. It reads one recipe, checks it, writes the agent package, builds
it and launches it.

`vivy generate <recipe.yaml>` writes `.generated/<name>/`; `vivy build` does that
and runs Cargo, printing the binary; `vivy run` launches it. The parsed recipe is a
name, one Rig source (`git` + `rev`) and an ordered list of components, each naming
a crate (from the Rig source, or of this repository by `path`), the plugin types it
contributes and the Cargo features it needs. Launching also speaks Rig's session
protocol — `--resume`, `--no-session`, `--fork`, `--session`, `--export` — and
relaunches the species when it exits for a session switch.

## Boundary

- It knows no agent capability. Plugin types are strings; rustc, not the builder,
  checks that each exists and is a plugin.
- Parsing rejects unknown fields, so a mistyped recipe key fails before Cargo sees
  it, with what to change.
- It injects `process` into every generated
  manifest: credential filtering is present in every species, selected or not.

## Invariants

- The generated package declares its own `[workspace]`, so an agent never becomes
  a member of the repository workspace.
- Plugin crates are built with `default-features = false`; the recipe's `features`
  map is the only source of features. Only the harness keeps its defaults.
- Component order is plugin registration order — the recipe decides which of two
  same-named commands survives.
- Generated files are marked "do not edit"; a recipe change is the way to change a
  build.

## Limitations

- Nothing checks that the recipe's `rig.rev` matches the revision the selected
  plugin crates pin; a mismatch surfaces as a build error.
- `--rig-path` points the build at a local Rig checkout for development; that path
  is outside the recipe's own pin.
