# Acceptance — F0 paperwork

## Manual acceptance

1. Open `docs/architecture/VIVY-FACE-PACK.md`: the header status is "Direction adopted
   (2026-08-31, VC decision D2)", no longer "Proposal".
2. The header note in `docs/architecture/VIVY-ASSEMBLY.md` no longer has the conditional
   "before adoption, do not add a `face:` row"; it now says adopted and points to
   `face:`/`seam: face`.
3. The core-never-pluginized list in
   `docs/architecture/SELF-EVOLVING-GATEWAY.md` contains a FaceHost row.
4. The `seam: face` section of `docs/architecture/VIVY-PLUGIN-SPEC.md` is a formal adopted
   rule (the temporary "before this document expands the seam" sentence is gone).
5. `git grep "提案"` no longer matches an unadopted face-contract status in the headers of
   those four files (proposal statuses for other contracts are outside this slice).

## Next steps

The FACE-TUI-1 (F3) code slice starts under the contract PR plan after budget/decision
approval (F1 control plane → FaceHost/SDK Face contract → `faces/tui` organ + pack `face:`
key).
