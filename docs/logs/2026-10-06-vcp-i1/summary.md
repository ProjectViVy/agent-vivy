# I1 — local-LLM compatibility module — summary

**Story:** `docs/superpowers/plans/vivy-code-parity/I1-local-llm.md`
**Commits:** `feat(plugins): local-llm compatibility module` (a251ada, module),
`feat(plugins): local-llm conformance pins and iteration log` (this entry)

## What landed

- `plugins/coding/local-llm/` — own Go module, T1, sealed source pin
  `5f2eeb7a…`. One op-dispatched `std/control-action@v1` action
  `local_llm.manage` (`status | discover | models | start | stop | pull`):
  external modules bind exactly one provider constructor per Port and every
  action provider carries a single Definition, so the six lifecycle verbs
  compose into one action rather than six module entries.
- `std/status-source@v1` `vivy.local-llm.status` declared; unwired
  platform-wide, the same latent state as `vivy/governance-reference`'s
  status source. Status reads never probe or start anything — they report
  the cached truth from the most recent discover/status op.
- Selected only by `recipes/vivy-code.vivy.yml` (vivy-code species); the
  vivy.exe default assembly does not include it — `generate-default` output
  is byte-identical.

## Boundary kept / deviations

- **Spawn boundary:** control actions cannot exec (the source-verify
  capability firewall rejects `os/exec`), and the only sanctioned spawn —
  tool-world — is session-scoped and dies with the run. `start`/`stop`
  therefore return `spawn_unsupported` plus the per-server external start
  hint; `status`/`discover`/`models`/`pull` are real.
- **No `std/provider-profile`:** an external Module's single `NewProvider()`
  cannot implement both `providerprofile.Provider` and
  `controlaction.Provider` (colliding `Definition()` signatures), and a
  declarative profile carries no endpoint URL anyway. Model selection rides
  the existing settings provider registry + the already-sealed
  `openai-completions` adapter with a dummy `api_key`.
- **Recipe fix-up:** `vivy/status-host` and `vivy/action-host` added to
  `recipes/vivy-code.vivy.yml` — the Module's declared `Requires` name those
  providers, and every other shipped recipe already selects them.
- **Grant surface fix-up:** `std/control-action@v1` `AllowedGrants` gained
  `net.client` (after `rpc.client`, so conformance fixtures that read
  `AllowedGrants[0]` are unchanged). The committed design requests
  `net.client` constrained to loopback (`127.0.0.1`/`localhost`/`::1`,
  `http`, ports `8000/8080/11434/1234`); without this the Recipe approval
  was unapprovable and `pack` failed. VIVY-PLUGIN-SPEC §4 already
  contemplates a control-action-capable Module requesting `net.client`.

## Conformance pins

- `internal/` source digest unchanged: `a6e7d5b2adb379e34c1a6f97b152833492958f73e38c7b2553d08173482918e4`.
  a251ada touched `sdk/internal/` (tooling), which is outside the hashed
  `internal/` root — `source-hash internal ""` recomputes to the stored
  value, so `reproduction_test.go` and `conformance_results.json` needed no
  re-pin. Plugin source hash `5f2eeb7a…` is self-referential inside the
  module dir and untouched.
