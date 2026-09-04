# Verification

Commands run from the isolated attachment-hardening worktree:

- `go test ./internal/rpc -run 'Attachment|ProjectContext' -count=1
  -timeout=120s` — passed.
- `go test -race ./internal/rpc -run 'Attachment|ProjectContext|EightDotThree' -count=1
  -timeout=180s` — passed.

The initial Windows identity-replacement test attempted to rename an open file
and correctly hit Windows sharing semantics. The fixture was corrected to
retain `FileInfo`, close the test handle, and then replace the named entry; the
production resolver continues to hold its read handle through capture and
performs the post-read identity check after closing it.

- Full `go test ./internal/rpc -count=1 -timeout=240s` — passed.
- Verbose native Windows smoke for ADS and NTFS 8.3 sensitive-alias rejection
  passed without skips.
- Linux cross-compilation of the RPC test binary passed, including the
  Unix-only static-FIFO and pre-open FIFO-replacement regression tests.
- `just ci` — passed: format check, 201 UI tests, UI build, `go vet ./...`,
  all Go tests, headless compile, and every plugin/face vet+test slice.
- Two specialist read-only audits were run. Their findings drove rooted
  component inspection, ADS rejection, post-read identity checks, normalized
  Windows handle-path validation, and non-blocking Unix opens. Storage-contract
  re-audit passed; the final filesystem-security re-audit also returned PASS
  with no remaining P1/P2 findings.

No live network, provider, tenant journal, or Studio state was accessed.
