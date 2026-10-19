# Provider catalog pruned to DeepSeek and SenseNova

Date: 2026-10-20

Owner decision: the embedded provider catalog carried too many vendors. Keep
only `deepseek` and `sensenova`; every other vendor, endpoint and model leaves
the shipped data.

## What changed

- `internal/provider/data/vendors.yaml`: 46 vendors / 48 endpoints / 175 models
  become 2 vendors / 3 endpoints / 14 models. `deepseek` keeps its
  `openai-completions` default endpoint and its `anthropic-messages` variant;
  `sensenova` keeps its `openai-completions` endpoint.
- The startup gate now accepts a sealed `DEFERRED-INDEFINITE` adapter with no
  endpoint in the data (`reconcile.go`). The comment already described that
  intent; the code enforced the opposite. A sealed supported adapter without an
  endpoint still fails closed.
- `AdapterProfiles()` compiles a Profile only for a family the data reaches
  (`profile.go`). A model-less Profile is rejected by the
  `std/provider-profile@v1` contract, and a model-less row would claim
  reachability the data does not have. The `openai-responses` family keeps its
  `DEFERRED-INDEFINITE` capability state but compiles no Profile.
- `internal/modules/defaults/catalog.go` declares two
  `std/provider-profile@v1` provides for `vivy/provider-profiles`; the default
  Assembly was regenerated with
  `go run ./sdk/internal/cmd/generate-default --repo . --output ...`, so the
  sealed manifest lists two Profile identities. Generated file changed by the
  generator only.
- Data/metadata anchors, the default-Generation inventory golden and the
  catalog RPC consumer tests now describe the two kept vendors.
- `pinnedEinoVersion` in the app real-smoke test still said `v0.9.13` after the
  Eino upgrade; corrected to `v0.9.21` here.

## What was explicitly not done

- No adapter family was deleted: `openai-responses` stays sealed and
  `DEFERRED-INDEFINITE` (visible capability, never executable) until the
  provider-agentic migration rebuilds the protocol set. Deleting it would also
  require removing the deferred-capability machinery, which that migration
  supersedes.
- No third-party vendor, endpoint or model list was added back.
- No UI, RPC contract or settings schema changed; the Settings catalog is a
  projection of this data, so the shorter list is the only visible change.
- No real-provider run and no `just ci`; Postgres and browser smoke stay at the
  PR/candidate stage.
