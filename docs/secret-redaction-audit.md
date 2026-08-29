# Secret Redaction Audit (E3)

Anchors: PRD AS-9, D-010. Date: 2026-08-07.

AS-9 requires that no secret value appears in the SQLite file, in any
persisted event payload, or in the UI's local storage. This audit fixes the
boundary in code and documents the evidence per domain.

## Boundary rule (D-010)

Secrets are provider API keys. Committed config still holds `env_key`
names only. Runtime keys live in the shared user workspace
(`~/.vivy/settings.yaml`, mode 0600) or a frozen ENV session. They are
never logged or returned on the control plane (`api_key_set` only):

- `internal/config` holds only the environment variable **name**
  (`env_key`, validated against `^[A-Z][A-Z0-9_]*$`); a literal key value
  fails validation, and strict decoding rejects any unknown field such as
  `api_key:` (`TestSecretInjectionRejected`).
- `internal/app/model.go` may read bundle `env_key` values only to freeze
  a temporary ENV session for this process.
- `internal/provider/openai.go` constructs models from `ModelSpec.APIKey`
  and does not read process environment for keys.
- `domain`, `runtime`, `storage`, `events`, `rpc` never echo key values.

## Domain 1: SQLite file and event payloads

Guard test: `internal/runtime/secret_leak_test.go`
(`TestSecretsNeverReachStorage`). It sets the canary
`sk-canary-e3-must-not-leak` into `OPENAI_API_KEY` /
`ANTHROPIC_API_KEY`, runs a complete scripted run (journal + runs +
sessions + messages writes), then asserts the canary appears in no event
payload and in no byte of the flushed SQLite file.

## Domain 2: source boundary

Guard tests: `internal/provider/secret_audit_test.go`.

- `TestSecretEnvReadsStayOutOfProvider` scans every Go source under `cmd/`
  and `internal/` and fails on any `os.Getenv` of a KEY/SECRET/TOKEN
  variable outside the model resolver. Provider construction must not
  read process environment for keys.
- `TestNoHardcodedKeyLiterals` fails on any `sk-…` literal in
  non-test sources. Audit result: `sk-` strings exist only in
  `_test.go` negative fixtures (`internal/config/config_test.go`,
  `internal/provider/provider_test.go`).
- `TestKeyMissingErrorCarriesNoValue` pins that the structured
  missing-key error names the environment variable only; the struct has
  no value field by construction.

## Domain 3: UI local storage

The UI keeps product state in memory and derives history from JSON-RPC replies
and cursor-based event replay. The only browser-persisted values are the
non-sensitive locale and theme preferences (`vivy.locale`, `vivy.theme`):

```powershell
Select-String -Path ui/src/app/preferences.ts -Pattern 'localStorage'
```

These keys contain only an enum value and never session, run, prompt, event, or
provider data. No other UI storage is used:

```powershell
Select-String -Path ui/src/* -Pattern 'sessionStorage|indexedDB'
```

No secret can land in browser storage through the preference path.

## Conclusion

AS-9 holds: secrets never reach the SQLite file, persisted event
payloads, or UI storage. The three guard tests fail CI on regression of
any domain.
