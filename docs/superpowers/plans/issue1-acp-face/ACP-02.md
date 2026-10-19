# ACP-02 Host Seams and Selected Face Startup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans. Track the ordered steps with checkboxes; delegation is optional and requires the applicable authorization.

**Goal:** Provide governed session context, safe output presentation and a selected Face launcher that preserves existing products.
**Architecture:** Reuse private-state allocation and the existing App/FaceHost. G0 selected Candidate A: reuse cmd/vivy with the selected-generation overlay; Candidate B (public Recipe.Entrypoint / cmd/vivy-face) is struck. Asset stripping is not an acceptance condition.
**Tech Stack:** Go 1.26.4; ACP wire 1 / schema-v1.21.0; existing FaceHost and Control; SDK pin accepted by ACP-01.
**Spec:** [Reconciled detailed design](../../../architecture/ACP-STDIO-FACE.md), executable baseline dd78fcf142f384d47ce5cfefb43738fdb9a7346d.
**State / dependencies:** [Single index](index.md#story-status-and-dependencies). Requires accepted ACP-01 outputs and independent owner scheduling of G1.

## Global Constraints

Follow the [shared constraints](index.md#global-constraints), proposed limits in design section 10, and repository plugin/kernel workflows. This conditional plan does not approve its public interfaces or schedule G1. ACP-01 freezes them and revises downstream steps before execution. No product data access or generated-assembly hand edits.

## Review Focus

RF-1/3/5: session B versus launch A, split text roots/secrets, pre-protocol stdout, blocked pipes and default CLI regressions.

## Files and interfaces

Modify existing sdk/port/face/face.go; internal/app/facehost.go; internal/rpc/protocol.go; internal/rpc/control.go.
Extend existing face_test.go, facehost_test.go, protocol_test.go and project_context_test.go beside those implementations.

Proposed additive SDK definitions:

~~~go
// Add In to Options; retain Out, Err and every existing field.
In io.Reader
~~~

The TextPresentationHost/TextSanitizer facet was withdrawn by the owner's D2 ruling (2026-10-07); the restricted pilot ships no outbound presentation facet.

Core producers:

- func (e *Error) RPCErrorCode() int in internal/rpc/protocol.go; preserves existing Error() and cause behavior.
- func (h *controlHandler) projectContextRoot(ctx context.Context, sessionID string) (string, *Error), a private resolver used by startTurn for context_paths.

The launcher tasks below consume Options.In and the unchanged FaceProvider. ACP-03 consumes the structural RPCErrorCode accessor.

## Launcher files and interfaces

Proposed: internal/faceprocess/{private.go,launch.go} and focused tests.
Modify existing: internal/codeface/launch.go and launch_test.go; internal/app/facehost.go and facehost_test.go; cmd/vivy/main.go; sdk/internal/frontend_v1.go and focused pack tests.
Candidate A (accepted) additionally owns cmd/vivy/run.go and the pack-only cmd/vivy/tui.go overlay.

Proposed shared interfaces:

- faceprocess.Prepared { Config config.Config; SharedSettingsPath string; InstanceRoot string }.
- func PreparePrivate(cfg config.Config, namespace string) (Prepared, error), accepting internal constants code-instances and face-instances only.
- func Run(ctx context.Context, cfg config.Config, opts face.Options) (face.Result, error).
- func Main(args []string, in io.ReadCloser, out io.WriteCloser, errOut io.Writer) int.
- func app.RunSelectedFaceWithAppOptions(ctx context.Context, cfg config.Config, opts face.Options, appOpts ...app.AppOption) (face.Result, error), using one App and its a.assembly.Face.

## Task 1: Add source-compatible contracts and typed errors

- [ ] Write TestFaceOptionsInputIsOptional and TestRPCErrorCodeSurvivesWrapping. Existing fake Hosts must still compile with In unset. Pin the structural assertion:

~~~go
var coded interface{ RPCErrorCode() int }
if !errors.As(fmt.Errorf("wrapped: %w", &Error{Code: -32004}), &coded) {
    t.Fatal("wrapped RPC code unavailable")
}
if coded.RPCErrorCode() != -32004 { t.Fatal("code changed") }
~~~

- [ ] Run go test ./sdk/port/face ./internal/rpc -run 'Test(FaceOptionsInputIsOptional|RPCErrorCodeSurvivesWrapping)$' -count=1. Expected initial failure is the absent additive field/method.
- [ ] Add only the proposed fields/types/accessor, then rerun. Do not rename existing aliases or migrate unrelated Face implementations.

## Task 2: Resolve context against the durable selected session

- [ ] Write TestTurnContextUsesSessionWorkspace and TestTurnContextRejectsSessionEscape. Use isolated roots A and B with different content at the same relative path; launch from A, create session B and assert the Run receives B's snapshot. Unknown session must fail before any file open.
- [ ] Include symlink escape, sensitive file, changed-during-read, 8-file/1-MiB-per-file/4-MiB-total boundaries and an empty WorkspacePath regression. Reuse existing project-context security fixtures; do not duplicate the resolver.
- [ ] Run go test ./internal/rpc -run 'Test(TurnContext|ProjectContext)' -count=1. Expected initial session-B failure demonstrates the current ProjectRoot-only bug.
- [ ] Implement projectContextRoot using h.deps.Sessions.GetSession. A nonempty durable WorkspacePath is authoritative; an empty path preserves h.deps.ProjectRoot. Call the existing resolveProjectContextsWithContext with that root. Do not widen image attachment or project browsing APIs as incidental work.
- [ ] Rerun the focused suite. Existing vivy-code context behavior must remain unchanged.

## Task 3: Withdrawn — D2 ruling

Removed by the owner's D2 simplification (2026-10-07): no per-run sanitizer, presentation facet or private-root masking is implemented in the restricted pilot. Protocol output is bounded allowlisted projection only; SDK-level error sanitization is a dependency-route requirement, not this Story's code.

## Task 4: Extract private allocation and preserve existing codeface behavior

- [ ] Write TestPreparePrivateIsolation and TestCodeFaceLocalWorkspacePreserved. Two launches use different private SQLite/log directories, share only the existing settings path, and leave the resident Journal untouched. Retain code-instances placement and root-symlink checks.
- [ ] Run go test ./internal/codeface ./internal/faceprocess -run 'Test(PreparePrivate|CodeFaceLocalWorkspace)' -count=1; expect failure at the missing helper.
- [ ] Extract allocation into PreparePrivate. Keep project validation and Runtime.World="local" in codeface.Prepare. ACP uses World="sandbox", a private fallback and the existing session-aware workspace composition. Do not set the accepted session's tools to the process launch directory.
- [ ] Rerun focused tests; go list -deps ./internal/faceprocess must exclude internal/codeface and sdk/tui/face. Commit the reviewed extraction with: refactor(face): share private runtime allocation.

## Task 5: Run the selected Face and bound whole-process cleanup

- [ ] Write TestSelectedFaceUsesOneApp, TestFaceCLIProtocolStdout, TestFaceCLIModes and TestFaceLaunchShutdownDeadline with a fake generated Provider and real OS pipes. Assert one App, stdin forwarding, no ACP startup for help/inspect, exit 2 on invalid arguments, exit 1 on startup/cleanup failure and exit 0 on clean idle EOF.
- [ ] Run go test ./internal/faceprocess ./internal/app -run 'Test(SelectedFace|FaceCLI|FaceLaunch)' -count=1; observe the intended missing-launcher failure.
- [ ] Implement the selected-provider helper and early face dispatch. Protocol stdout contains frames only, including startup failures; use stderr bootstrap and logging.Setup with Stdout=false. The selected artifact's no-argument invocation follows the G0-selected route; non-ACP default commands preserve existing behavior.
- [ ] Keep Control/App alive under a distinct teardown context. Enforce the accepted 10-second total deadline; close owned pipe handles to unblock I/O, and close the connection on write error (probe evidence: the SDK keeps the connection open after EPIPE). Audit App.Close instead of wrapping a still-blocking deferred close with a timer. Main owns process exit and reports incomplete cleanup. The SDK's internal 30s Close() bound is being corrected upstream per the accepted dependency route; until the pin carries that fix, the adapter's own teardown deadline applies.
- [ ] Rerun the process tests on each claimed OS. Record Windows console-close limitations; compilation is not runtime parity. Commit with: feat(face): launch the selected provider with bounded cleanup.

## Task 6: Implement the accepted build route (Candidate A)

- [ ] ACP-01 recorded Candidate A as the one route; the selection datum is the compiled Recipe/Assembly Face value (generation.EmbeddedManifest().Face).
- [ ] Write TestPackACPSelectedOverlay and TestDefaultCLIUnchanged. Trigger the existing Go overlay only when recipe.Exclusive["std/face@v1"] == "projectvivy/acp" and plan.Modules contains that provider; AssemblyPlan has no Face field. The pack-only tui.go stub implements runTUI([]string) int with an explicit unavailable-command error. Selected ACP rejects tui/run before protocol startup; normal generations keep both. Preserve existing UI asset hashing/staging and report the asset inventory.
- [ ] Run the focused pack suite; expect failure before implementation. Implement only its reviewed route, then rerun against a selected test Face. Inspect the actual temporary modfile/overlay with go list -deps before cleanup. Prove the required Web/TUI implementation closure, not just a source manifest or a runtime-disabled gateway. Record assets separately.
- [ ] Run just ci and record evidence. Refresh consequential source identities as required by the index. Commit explicit paths with: feat(face): isolate the selected generation launch path.

**Acceptance:** Existing Faces and default gateway/tui/run remain compatible; session-root context is correct; the selected test Face owns private state and protocol streams; required implementation omission has build evidence. The chosen presentation API has its focused tests. Final ACP artifact/client proof remains ACP-05.
