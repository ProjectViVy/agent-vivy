# PR 22 integration acceptance

- [x] PR #21's governed Tool execution closure is integrated before PLG-P7.
- [x] PLG-P3 and PLG-P7 completion records coexist without marking PLG-P8 or
  PLG-P9 scheduled.
- [x] A valid custom `providers.openai.env_key` or
  `providers.anthropic.env_key` remains usable through the scoped Credential
  Resolver.
- [x] Empty optional references do not prevent application composition, while
  invalid non-empty references still fail closed.
- [x] Secret values remain absent from configuration manifests, errors,
  Journals, and provider cache keys.
- [x] Generated Assembly startup/shutdown and the single runtime, Journal, and
  Policy path remain intact.
- [x] The Eino v0.9.13 adapter remains quarantined to `internal/runtime`.
- [x] The complete `just ci` gate passes.
- [x] A real default Recipe pack passes `inspect-artifact` with canonical
  internal module ownership and inactive unconfigured network capabilities.
