# Acceptance — VC-3 slice 6 (positionEncoding negotiation)

How a human can tell this works:

1. Point the lsp plugin at a server that declares
   `capabilities.positionEncoding: "utf-8"` (gopls with
   `"staticcheck": false` aside — it negotiates when the client offers).
   Renames on files containing emoji/CJK characters land on the right
   bytes — before this slice a utf-8 server's ranges were read as utf-16
   and could slice multi-byte characters into invalid bytes.
2. Servers that never negotiate (typescript-language-server, pyright)
   keep utf-16 and behave exactly as before — no observable change.
3. `lsp_diagnostics` no longer intermittently reports "wait_ms elapsed
   without a diagnostics publish" on a file that did publish — the
   round-trip determinism is visible as stable, immediate diagnostic
   output across repeated calls.
4. Real-server smoke remains unavailable on this machine (no
   gopls/typescript-language-server/pyright installed — recorded as the
   standing exception); the fake-server e2e plus `-race -count=5`
   stability is the substitute evidence.
