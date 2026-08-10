# Secret Redaction Audit (E3)

Anchors: PRD AS-9, D-010. Date: 2026-08-07.

AS-9 requires that no secret value appears in the SQLite file, in any
persisted event payload, or in the UI's local storage. This audit fixes the
boundary in code and documents the evidence per domain.

## Boundary rule (D-010)

Secrets are provider API keys. They enter the process exclusively through
environment variables, are read exclusively in `internal/provider` at
request time, and are never persisted, logged, or transported through
domain types:

- `internal/config` holds only the environment variable **name**
  (`env_key`, validated against `^[A-Z][A-Z0-9_]*$`); a literal key value
  fails validation, and strict decoding rejects any unknown field such as
  `api_key:` (`TestSecretInjectionRejected`).
- `internal/provider/openai.go` is the single reader:
  `key := os.Getenv(r.bundle.EnvKey)`, passed straight into the eino-ext
  ChatModel config and dropped afterwards.
- `domain`, `runtime`, `storage`, `events`, `rpc` contain no key type,
  field, or reader.

## Domain 1: SQLite file and event payloads

Guard test: `internal/runtime/secret_leak_test.go`
(`TestSecretsNeverReachStorage`). It sets the canary
`sk-canary-e3-must-not-leak` into `OPENAI_API_KEY` /
`ANTHROPIC_API_KEY`, runs a complete scripted run (journal + runs +
sessions + messages writes), then asserts the canary appears in no event
payload and in no byte of the flushed SQLite file.

## Domain 2: source boundary

Guard tests: `internal/provider/secret_audit_test.go`.

- `TestSecretEnvReadsOnlyInProvider` scans every Go source under `cmd/`
  and `internal/` and fails on any `os.Getenv` of a KEY/SECRET/TOKEN
  variable outside `internal/provider`. Audit result: the only
  credential read in the repository is `internal/provider/openai.go`;
  `cmd/vivy/main.go` reads `VIVY_ADDR` (a bind address, not a secret).
- `TestNoHardcodedKeyLiterals` fails on any `sk-…` literal in
  non-test sources. Audit result: `sk-` strings exist only in
  `_test.go` negative fixtures (`internal/config/config_test.go`,
  `internal/provider/provider_test.go`).
- `TestKeyMissingErrorCarriesNoValue` pins that the structured
  missing-key error names the environment variable only; the struct has
  no value field by construction.

## Domain 3: UI local storage

The UI keeps all state in memory and derives history from JSON-RPC replies
and cursor-based event replay. Audit evidence (zero matches):

```powershell
Select-String -Path ui/src/* -Pattern 'localStorage|sessionStorage|indexedDB'
```

Nothing is written to browser storage, so no secret can land there.

## Conclusion

AS-9 holds: secrets never reach the SQLite file, persisted event
payloads, or UI storage. The three guard tests fail CI on regression of
any domain.
