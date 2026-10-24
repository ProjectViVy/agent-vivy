# MR-2: channel plugin capability surface and source digests

Defects: D5, D6. `28def9c2` resolved `module_v1.go` toward the stale main side:
`MaxMessageRunes`, `channelProvider.CapabilityTarget`, and
`boundChannel.CapabilityTarget` were dropped, and the embedded `SHA256` was
re-pinned to a value that matches no real tree.

## Steps

1. Restore `module_v1.go` for `dingtalk`, `discord`, `feishu`, `qq`,
   `telegram` from the channel-tier1 side (`28def9c2^1`); keep HEAD's
   `go.mod`/`README`/`vivy-module.yaml` (canonical module paths won later).
2. `go mod tidy` each plugin tree (gorilla/websocket is a direct dep).
3. Compute `internal/sourcehash.Tree(plugins/<p>, <declared>)` and write the
   result into `module_v1.go`, `vivy-module.yaml`,
   `sdk/internal/conformance/reproduction_test.go`, and
   `sdk/internal/assembly/conformance_results.json` (suite form, `passedChecks`
   folded in `requiredChecks` order).
4. Re-pin `faces/headless` digest (stale since `8d8e26a5`, a legitimate
   change).
5. Regenerate `internal/generated/assembly/zz_default.go` via `go generate`.

## Evidence

All five plugin trees hash-verify (`declared == computed`);
`sdk/internal`, `sdk/internal/assembly`, `sdk/internal/conformance` green.
