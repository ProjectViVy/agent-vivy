# Verification

Required `just ci` initially rejected the source-bound evidence mismatch. After freezing the clean internal tree, the supported `source-hash internal ''` helper and an independent deterministic path/content hash agree on `d3302617b061a785471071145e241aedc9eed32b6b7463c66052c9a6e9ab58e5`.

`go test -json ./sdk/internal/conformance -run '^TestCheckedInProviderConformanceMatchesExecutedSuites$' -count=1` with a fresh task-local Go build cache executed the actual Provider/Host/common checks and byte-compared the canonical result set: PASS, exit 0, 159.90 seconds. The final full CI rerun is recorded in the DIVA handoff. The bundle edit is restricted to five digest fields.
