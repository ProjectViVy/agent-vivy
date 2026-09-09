# Verification — VC-2 Anthropic backend wiring (`eino-ext/claude`)

Worktree `agent-vivy-vc0`, branch `feat/vc1a-bash-tool`.

## Commands and results

| Command | Result |
| --- | --- |
| `go get github.com/cloudwego/eino-ext/components/model/claude@latest` | resolved `v0.1.25` (go.mod eino constraint compatible with locked `eino v0.9.13`) |
| `go test ./internal/provider/ -count=1` | ok — fixture tests (backend enum `eino-ext/claude`), catalog resolution, ref unit + protocol tests |
| `gofmt -l internal/provider` / `go vet ./internal/provider/` | clean |
| `just ci` (fmt-check, vet, `go test ./...`, headless-compile, ui-ci) | **PASS** |

## New tests (internal/provider/claude_test.go)

- `TestClaudeRefRequiresKeyBeforeConstruction` — poisoned
  `ANTHROPIC_API_KEY` env; empty spec key still yields `KeyMissingError`
  before SDK construction (D-010).
- `TestClaudeRefOutboundProtocol` — full HTTP exchange with a local
  Anthropic-shaped server:
  - path `/v1/messages`, response content surfaces to `schema.Message`;
  - `x-api-key` = spec key even with `ANTHROPIC_API_KEY=from-env` set;
  - outbound `model` = spec id even with `ANTHROPIC_MODEL=from-env-model`;
  - `max_tokens` = 8192 protocol floor;
  - ephemeral `cache_control` breakpoint present (bundle enables caching).
- `TestClaudeRefFallsBackToBundleDefaults` — empty spec id/baseURL resolve
  to bundle `default_model` / `default_api_base`.
- `TestClaudeModelInfoKnownAndUnknown` — known id metadata, unknown id
  zero-valued, empty id → bundle default.
- `TestCatalogForClaudeBackend` — catalog dispatch + offline construction.
- `TestCatalogAnthropicResolvesClaudeRef` (replaces
  `TestCatalogAnthropicNotWired`) — the fixture bundle now resolves.

## Supply-chain audit (§8.5 item 5)

New runtime modules introduced by the component (bedrock/vertex branches
are imported unconditionally — matches the §8.5 prediction). Each LICENSE
was read from the module cache:

| Module | Version | License |
| --- | --- | --- |
| github.com/anthropics/anthropic-sdk-go | v1.56.0 | MIT (Anthropic PBC) |
| github.com/aws/aws-sdk-go-v2 (+ submodules) | v1.33.0 | Apache-2.0 |
| github.com/aws/smithy-go | v1.22.1 | Apache-2.0 |
| cloud.google.com/go/auth (+ oauth2adapt, compute/metadata) | v0.7.2 / v0.2.3 / v0.5.0 | Apache-2.0 |
| google.golang.org/api | v0.189.0 | BSD-3 (Google) |
| google.golang.org/grpc, genproto, protobuf upgrades | v1.64.1 etc. | Apache-2.0 / BSD-3 |
| go.opentelemetry.io/otel, otel/metric, otel/trace, otelhttp, otelgrpc | v1.24.0 | Apache-2.0 |
| github.com/go-logr/logr, stdr | v1.4.2 / v1.2.2 | Apache-2.0 |
| github.com/felixge/httpsnoop | v1.0.4 | MIT |
| github.com/tidwall/sjson | v1.2.5 | MIT |
| github.com/invopop/jsonschema | v0.14.0 | MIT (Alec Thomas, `COPYING`) |
| github.com/pb33f/ordered-map/v2 | v2.3.1 | dual MIT / Apache-2.0 |
| github.com/standard-webhooks/.../libraries | pseudo | MIT |
| go.yaml.in/yaml/v4 | v4.0.0-rc.2 | dual MIT / Apache-2.0 |

Also upgraded: `golang.org/x/oauth2` v0.23.0→v0.30.0,
`google.golang.org/protobuf` v1.26.0→v1.34.2 (both Apache-2.0/BSD-3).
Additional go.sum entries (BurntSushi/toml, envoyproxy, golang/mock,
golang.org/x/lint, honnef.co/go/tools, misspell, opencensus …) are
test/tool dependencies of the above modules — recorded in go.sum, not
compiled into the binary.

No license conflicts with the repo's MIT/FSL posture; nothing vendored.

## Smoke exceptions

- **Real api.anthropic.com smoke not run**: no ANTHROPIC_API_KEY in this
  environment. Real-path behavior is covered by the httptest protocol
  tests above, which exercise the component's actual HTTP serialization,
  headers, and cache-control wiring end-to-end against a server that
  speaks the Anthropic Messages shape. A live call remains the
  acceptance step for whoever holds a key (see acceptance.md).
- **Browser UI**: zero UI change in this slice (backend/schema are
  kernel-internal); `ui-ci` ran as part of `just ci`.
