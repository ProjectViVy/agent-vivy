# Acceptance: How to verify from the user's perspective

- In everyday use, write operations under Settings → Models (adding a model,
  switching the current model, and modifying a custom provider) no longer
  intermittently show a red `internal error` bar.
- The document remains intact under concurrent load:
  `go test ./internal/app/settings/ -race -count=3` passes, and the test asserts
  that the document remains parseable after concurrent Save calls with no
  leftover `.tmp` files.
- The Windows-specific failure mode (rename replacing a file held by a read
  handle) no longer occurs; before the fix, the first run of the test above could
  reproduce `Access is denied`.
- There is no observable behavior change: the settings-file format, RPC
  contract, and error copy remain unchanged (`internal error` still does not
  disclose internal details).
