# Verification — LOG-3

| Check | Command | Result |
|---|---|---|
| gofmt | `gofmt -l internal/logging internal/tools` | clean |
| Build | `go build ./...` | ok |
| Vet | `go vet ./internal/logging/... ./internal/tools/...` | ok |
| Unit tests | `go test ./internal/logging/ ./internal/tools/ -count=1` | ok (both packages) |
| Kernel gate | `just ci` | background run, exit tail-checked below |

## Test coverage added (`internal/logging/redact_test.go`)

- `TestRedactVocabulary`: sk-/Bearer tokens and emails → markers; ordinary
  fields (run id, model name, path) untouched, including the `task-…`
  near-miss (no `\b` before `sk-`).
- `TestRedactingHandlerMessageAndAttrs`: through a wrapped JSON handler —
  message token masked, `api_key` and `WithAttrs`-set `password` collapse
  to `[REDACTED]`, group member `channel.bot_token` collapses while
  `channel.name` survives, `run_id` untouched.
- `TestRedactingHandlerAuthorizationKeyCaseInsensitive`: key
  case-insensitivity (`Authorization`).
- `TestSetupSinkRedacts`: real-path acceptance — through the product
  `Setup` init, a credential in an error attribute never reaches the
  rotated file; the file carries `[REDACTED_SECRET]`.
- `internal/tools` `TestRedactSensitiveDoesNotPreserveCredentialOrEmail`
  passes unchanged via delegation (marker parity proven).
