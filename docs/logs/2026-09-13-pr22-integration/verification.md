# PR 22 integration verification

## Regression development

The configured model-reference regression was reproduced before implementation:

```text
go test ./internal/modules/credential -run TestCompileScopesIncludesConfiguredModelReferences -count=1
```

The first RED result was a compile failure because `CompileScopes` did not
accept configured model references. A second RED run passed an empty optional
reference and reproduced the resolver's invalid-empty-reference failure. The
implementation now includes non-empty configured provider references and keeps
invalid non-empty names fail-closed through `Compose` validation.

Focused verification passed:

```text
go test ./internal/modules/credential -run TestCompileScopesIncludesConfiguredModelReferences -count=1
go test ./internal/app -run 'TestGatewayControlActionUsesConnectionBoundSession|TestDefaultGenerationLeavesUnconfiguredNetworkInactive|TestAppUsesGeneratedRuntimeAssembly' -count=1 -timeout 10m
go test ./internal/modules/... ./internal/app ./internal/runtime ./internal/channelhost ./internal/provider ./sdk/internal/assembly ./sdk/internal -count=1 -timeout 20m
```

The broad slice passed, including `internal/app` in 81.131 seconds,
`internal/runtime` in 210.475 seconds, and `sdk/internal` in 269.962 seconds.

## Repository gate

With `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `VIVY_MODEL`, and `VIVY_PROVIDER`
removed from the command environment, the required gate passed:

```text
just --set go 'C:\Program Files\Go\bin\go.exe' --set gofmt 'C:\Program Files\Go\bin\gofmt.exe' ci
```

Results included formatting, Go vet and the complete Go suite, UI typecheck,
35 UI files and 316 tests, production UI build, i18n checks, headless compile,
and all independent plugin and face vet/test checks. The existing Vite
chunk-size warning remained informational.

`go generate ./internal/generated/assembly` completed with no generated diff.

## Real pack and Inspect smoke

The integrated default Recipe packed into an isolated `.workspace` output and
`inspect-artifact` accepted the result:

```text
go run ./sdk pack --recipe recipes/default.vivy.yml --output <isolated-output>
go run ./sdk inspect-artifact <isolated-output>
```

Generation ID:
`ea28cd0fcd9708ed32497767735c698a95a25ae08df34ad6f999116876835710`.
The Manifest contains the canonical loop, model, storage, checkpoint,
credential, sandbox, and optional Host modules; channel and MCP capability
states remain `UNCONFIGURED`.
