# Verification

Candidate and final gate results will be recorded after the integrated checks.

Observed so far:

- Native independent BML baseline failed on six existing test-owned Home handles
  left open during TempDir cleanup. Updated those tests to close their homes;
  no production lifecycle workaround was added.
- New v2 regressions first failed on emitted digests, schema1, write-before-refusal
  and relocation changing owner. After implementation the complete independent
  BML suite and `go vet ./...` pass.
- New unowned-artifact regression first showed existing output overwritten;
  directory ownership guard fixes it. Exact restart allocation and raw/normalized
  conflict tests pass.
- New linked-directory rollback regression first deleted a foreign marker;
  rejection before touching outputs fixes it. The regression now passes natively.
- MEMRULES focused BML tests, memory package tests and 10 source UI cases passed
  on the isolated consumer branch; integrated checks remain to be recorded.
- GitHub clone failed during pinned bootstrap (host DNS/getaddrinfo failure).
  A local clone of the exact WSL Laputa source was used instead; the repository
  bootstrap verifies clean pin ff3936f44ff8cf08c12af2cf698c194cfe474fd3.

Not yet claimed: independent Linux wiring/kernel/PostgreSQL, actual split UI,
finite converter-to-target reopen, fresh whole-branch review, D12 acceptance,
full-product release or D14 converter assembly.
