# F0 adoption-contract paperwork (FACE-TUI-1 prerequisite · docs-only)

## Summary

`VIVY-FACE-PACK.md` (F0 contract) was directionally adopted by the user on 2026-08-31
(VC decision D2; see the FACE-0 line in `docs/TODO.md`), but PR-1 (adoption paperwork)
had not been executed: the status lines in the four architecture documents still said
"Proposal". This slice adds paperwork only and changes no runtime code:

- `docs/architecture/VIVY-FACE-PACK.md`: status "Proposal" → "Direction adopted"
  (2026-08-31, VC decision D2); §14 notes that the four open questions (standalone
  `go.mod` for `faces/`, default generation always web, Journal cohabitation, and the
  headless approval product wording) must be presented for decision using the recommended
  values before each implementation slice starts.
- `docs/architecture/VIVY-ASSEMBLY.md`: header note changed to adopted; a built-in face
  uses exactly one `face:` recipe entry, while a user face uses `plugins/` + `seam: face`.
- `docs/architecture/SELF-EVOLVING-GATEWAY.md`: added FaceHost (alongside ChannelHost) to
  the core-never-pluginized list.
- `docs/architecture/VIVY-PLUGIN-SPEC.md`: the `seam: face` routing declaration is now
  adopted (the temporary "before the seam is expanded, do not submit as a tool plugin"
  sentence became a formal rule reference).

The actual FACE-TUI-1 (F3) prerequisites were also reviewed and its line note updated:
F0 is adopted (this documentation slice is closed); the **functionality** of headless
already exists as `vivy run` (D11) but is not a recipe organ; F3's actual remaining work
is F1 (gateway-less control plane: complete a conversation + approval through in-process
RPC with no embed/no listen) + the FaceHost/SDK Face contract + the built-in `faces/tui`
organ + pack `face:` recipe key, implemented in slices under the contract PR plan
(estimated 2–3 slices).

## Explicitly not done

- Do not write runtime code (FaceHost, the SDK Face contract, the `faces/` directory, and
  the pack `face:` key all remain untouched) — those belong to later independent slices.
- The four §14 open questions are not decided in this slice; leave them to the corresponding
  implementation slices.
- Do not change the already-landed `sdk/plugin` seam (contract text constraint).
