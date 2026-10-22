# Verification

The clean isolated lane fast-forwarded without reset to exact candidate
`5dcd7f0167eaf2a13c89f744aec9c62e7bbf8246`. Its own Laputa remains clean at
`9acc999301d5d575da2cea902a1c6d213763f381`. Native Go commands used
`GOFLAGS=-p=2`, `vivy_headless` and a checkout-local temporary directory.

RED: added the held raw-row-before-Journal window to the real middleware-drift
fixture, leaving the old helper unchanged:

```powershell
go test -tags=vivy_headless ./internal/app -run '^TestToolApprovalFailsClosedWhenMiddlewareRewriteDriftsOnResume$' -count=1 -timeout=1m
```

Failed in 1.438s: `approval helper returned a raw row before its durable visibility
event`. This reproduces the concrete missing visibility condition rather than
relying on the full-suite timing. The native CI's observed `approval is not
durable yet` failure supplies corroborating evidence.

GREEN after the helper fix: the held-window regression and original secret test
passed together in 3.405s. The first six-owner focused run passed in 8.268s.
Final focused validation after removing all three duplicate retry loops covered
all six top-level fixture owners, including binding and source-class subcases:

```powershell
go test -tags=vivy_headless ./internal/app -run '^(TestProductionMCPGovernancePathUsesToolHostOrder|TestToolApproval.*|TestProductionMCPTransportGovernancePathUsesToolHostOrder|TestToolEnvelopeConformanceAcrossAllSourceClasses)$' -count=1 -timeout=2m
git diff --check
```

PASS in 20.915s; diff check PASS. No broad app suite, full CI, race, live provider,
browser or new platform gate was run. The parent owns exact-candidate full-gate
retries and the separately reviewed runtime window. Temporary Go scratch was
removed after recording results. No tenant Journal or production database was
read or written.
