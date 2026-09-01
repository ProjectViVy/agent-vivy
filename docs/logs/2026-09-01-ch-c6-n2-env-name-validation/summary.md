# CH-C6-N2 — settings `*_env` declared env-name pattern validation

## What changed

The channelhost settings walk (`hostEnv.declaresEnvKey`) granted a secret to
any top-level string `*_env` settings entry whose value happened to equal the
requested `env_key` — with no check that the value is a well-formed
environment variable name. A typo like `client_id_env: client-id` (dash) or
`client_id_env: "my client id"` was silently accepted as a declaration.

CH-C6-N2 closes that with one shared rule:

- `internal/config`: `envKeyPattern` is now exported as
  `config.ValidEnvKey(name)` — the single env-name pattern
  (`^[A-Z][A-Z0-9_]*$`) shared by the config parse-time validation of
  `token_env` / `client_id_env`-style fields and by the hostEnv settings
  walk (one source of truth, no drift between the two validators).
- `internal/channelhost/channelenv.go`:
  - `declaresEnvKey` skips any declared name that fails `ValidEnvKey` —
    a malformed declaration grants nothing (fail closed).
  - New `auditSettingsEnvNames` walks the same top-level string `*_env`
    entries at start time and Warns for each malformed declaration, naming
    the channel, the settings key, and the declared name (names only —
    environment variable values never pass through; D-010 holds).
- `internal/channelhost/host.go`: `StartAll` calls `auditSettingsEnvNames`
  for each configured channel between the allow_from check and `ch.Start`.

The pairing is deliberate: refuse-at-resolve keeps the runtime surface
closed immediately (no start-order dependency), warn-at-start gives the
operator the actionable signal without inventing a new failure mode that
would block unrelated channels.

## Explicitly not done

- No schema-level rejection of malformed settings `*_env` values at config
  load: settings is an opaque block by contract (CH-C4), and rejecting the
  whole channel for a typo inside the opaque block would change the load
  contract. The warning is the contract-compatible surface. If the
  contract later gains strict settings validation, this audit folds into it.
- No change to config field validation itself — `envKeyPattern` already
  guarded `token_env` / DSN / auth env fields at parse time (CH-C6-N2's
  original finding predates that parse-time guard; the residual gap was
  exactly the opaque settings walk, now closed).
