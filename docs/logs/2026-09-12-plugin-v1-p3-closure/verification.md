# PLG-P3 closure verification

## RED evidence

Command:

```text
go test ./internal/app -run '^TestToolEnvelopeConformanceAcrossAllSourceClasses$' -count=1
```

The first complete assertion failed for all four source classes. Each failure
showed that the durable approval contained `items:[1]` while the Provider later
received the Middleware rewrite `items:[2]`.

A second focused RED test showed that an error returned by a Provider exposed
its `sk-live-...` Secret verbatim through the runtime Tool adapter.

Independent review then exposed two additional RED cases:

- a stateful Middleware could project `items:[2]` for approval and rewrite the
  resumed call to `items:[3]`, which executed without fresh authority; and
- a Middleware-injected credential was written verbatim to the durable
  `tool.approval_required.args` projection.

A follow-up review found that resume verification still sat inside the
recomputed prompt/require-approval branch. A second RED regression proved that
Middleware could require approval on suspend, return pass under an allow
policy on resume, and reach the Provider with changed or legacy-unbound data.

## Focused GREEN evidence

```text
go test ./internal/runtime -run 'TestMapperInterruptDetails|TestToolAdapterRechecksPolicyAfterPublicMiddlewareRewrite|TestToolAdapterMiddlewareRequireApproval' -count=1
go test ./internal/runtime -run 'TestToolAdapterRedactsProviderErrorAndPreservesCause|TestToolAdapterRedactsAndMarksUntrustedResult' -count=1
go test ./internal/runtime -run 'TestToolApprovalArgumentsHashCanonicalizesJSON|TestToolApprovalProposalBindingRoundTrip|TestRedactedApprovalArgumentsCopiesNestedValues' -count=1
go test ./internal/app -run 'TestToolApproval(FailsClosedWhenMiddlewareRewriteDriftsOnResume|JournalRedactsMiddlewareInjectedSecret)' -count=1
go test ./internal/app -run '^TestToolApprovalBindingCannotBeBypassedWhenResumeBecomesAllowed$' -count=1
go test ./internal/app -run '^TestToolEnvelopeConformanceAcrossAllSourceClasses$' -count=1
go test ./internal/toolhost ./internal/modulehost ./internal/observerhost ./internal/statushost ./internal/runtime ./internal/app ./sdk/port/pretool ./sdk/port/observer ./sdk/port/status ./sdk/internal/assembly
```

Result: PASS.

The matrix covers protected internal, public static, public dynamic, and fake
MCP-derived Tool sources. Every case proves the post-Middleware arguments are
the approved and executed arguments, the Journal order is Policy -> approval
required -> approval decided -> started -> finished, result payloads are
bounded to 128 bytes, and Secret material is absent from Journal and Observer
payloads. The focused Provider-error test additionally proves that a redacted
error retains its original cause chain. The resume-drift test proves that the
Provider is never invoked, the run fails, and the approval becomes `stale`
when post-Middleware arguments change after review. The credential test proves
that Middleware-injected Secrets are absent from the Journal projection. The
allow-transition test proves the binding guard runs at the sole Provider
dispatch seam even when replayed Middleware/policy no longer asks, and that a
legacy approval without a digest fails closed.

## Product gate

The execution venue did not provide `just` or PowerShell, so `just ci` could
not be invoked literally. The repository's `ci` recipe was executed component
by component with Go 1.26.4 and the locked pnpm dependencies:

```text
pnpm typecheck
pnpm test
pnpm build
node scripts/check-i18n-completeness.js
node --test scripts/check-i18n-cross-face.test.js
node scripts/check-i18n-cross-face.js
gofmt -l <all tracked Go files>
go vet ./...
go test -timeout 20m ./...
go test -run '^$' -tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui
go vet ./... && go test ./...  # each plugins/* and faces/* Go module
```

Results:

- UI typecheck: PASS.
- UI tests: 35 files, 316 tests, PASS.
- UI production build: PASS.
- I18N completeness and cross-face checks: PASS.
- Go formatting: PASS.
- Go vet: PASS.
- Main-module Go suite: PASS.
- Headless compile: PASS.
- All plugin and face module vet/test gates: PASS.

## Eino capability check

- Pinned module: `github.com/cloudwego/eino v0.9.13`.
- APIs inspected and used:
  `github.com/cloudwego/eino/components/tool.StatefulInterrupt`,
  `GetInterruptState`, `GetResumeContext`, and the existing
  `github.com/cloudwego/eino/adk.InterruptCtx.Info` projection.
- Capability match: Eino's protected interrupt state persists the authoritative
  post-Middleware argument snapshot through checkpoint/resume. Interrupt info
  is transport metadata only; the approval row separately persists the
  canonical argument digest alongside opaque Provider proposal data.
- Adapter boundary: state encoding and resume verification stay in
  `internal/runtime/tooladapter.go`; the mapper reads only the transport
  projection in `internal/runtime/mapper.go`.
- Result: `ADAPT`. No custom checkpoint, approval system, or second runtime
  path was added.
