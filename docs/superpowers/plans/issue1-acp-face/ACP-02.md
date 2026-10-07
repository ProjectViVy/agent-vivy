# ACP-02 Host Seams and Selected Face Startup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans. Track the ordered steps with checkboxes; delegation is optional and requires the applicable authorization.

**Goal:** Provide governed session context, safe output presentation and a selected Face launcher that preserves existing products.
**Architecture:** Reuse private-state allocation and the existing App/FaceHost. G0 chooses the smallest build route that proves implementation-code omission; asset stripping and a new public Recipe field are not assumed.
**Tech Stack:** Go 1.26.4; ACP wire 1 / schema-v1.21.0; existing FaceHost and Control; SDK pin accepted by ACP-01.
**Spec:** [Reconciled detailed design](../../specs/2026-10-07-acp-stdio-face-design.md), executable baseline dd78fcf142f384d47ce5cfefb43738fdb9a7346d.
**State / dependencies:** [Single index](index.md#story-status-and-dependencies). Requires accepted ACP-01 outputs and independent owner scheduling of G1.

## Global Constraints

Follow the [shared constraints](index.md#global-constraints), proposed limits in design section 10, and repository plugin/kernel workflows. This conditional plan does not approve its public interfaces or schedule G1. ACP-01 freezes them and revises downstream steps before execution. No product data access or generated-assembly hand edits.

## Review Focus

RF-1/3/5: session B versus launch A, split text roots/secrets, pre-protocol stdout, blocked pipes and default CLI regressions.

## Files and interfaces

Modify existing sdk/port/face/face.go; internal/app/facehost.go; internal/rpc/protocol.go; internal/rpc/control.go.
Extend existing face_test.go, facehost_test.go, protocol_test.go and project_context_test.go beside those implementations. Add internal/app/facehost_text_test.go for focused privacy coverage.

Proposed additive SDK definitions:

~~~go
// Add In to Options; retain Out, Err and every existing field.
In io.Reader

type TextSanitizer func(string) string
type TextPresentationHost interface {
    TextSanitizer(context.Context, string) (TextSanitizer, error)
}
~~~

Core producers:

- func (e *Error) RPCErrorCode() int in internal/rpc/protocol.go; preserves existing Error() and cause behavior.
- func (e *faceEnv) TextSanitizer(ctx context.Context, runID string) (face.TextSanitizer, error).
- func (h *controlHandler) projectContextRoot(ctx context.Context, sessionID string) (string, *Error), a private resolver used by startTurn for context_paths.
- A private run-workspace lookup callback on faceEnv, wired from the owned App. It returns the canonical path from a.service.Workspace after authenticated run/get validation. Do not export raw roots through the SDK.

The launcher tasks below consume Options.In and the unchanged FaceProvider. ACP-03 consumes the G0-accepted presentation facet and structural RPCErrorCode accessor. The facet signatures above remain proposed until ACP-01 accepts them.

## Launcher files and interfaces

Proposed: internal/faceprocess/{private.go,launch.go} and focused tests.
Modify existing: internal/codeface/launch.go and launch_test.go; internal/app/facehost.go and facehost_test.go; cmd/vivy/main.go; sdk/internal/frontend_v1.go and focused pack tests.
Candidate A additionally owns cmd/vivy/run.go and the pack-only cmd/vivy/tui.go overlay.
Candidate B additionally creates cmd/vivy-face/{main.go,main_test.go}, modifies sdk/internal/assembly/{compiler.go,compiler_test.go,manifest.go,manifest_test.go} and the justfile headless-compile target. ACP-01 must delete the unused candidate's file scope before execution.

Proposed shared interfaces:

- faceprocess.Prepared { Config config.Config; SharedSettingsPath string; InstanceRoot string }.
- func PreparePrivate(cfg config.Config, namespace string) (Prepared, error), accepting internal constants code-instances and face-instances only.
- func Run(ctx context.Context, cfg config.Config, opts face.Options) (face.Result, error).
- func Main(args []string, in io.ReadCloser, out io.WriteCloser, errOut io.Writer) int.
- func app.RunSelectedFaceWithAppOptions(ctx context.Context, cfg config.Config, opts face.Options, appOpts ...app.AppOption) (face.Result, error), using one App and its a.assembly.Face.
- Only if Candidate B is accepted: Recipe.Entrypoint, AssemblyPlan.Entrypoint and GenerationManifest.Entrypoint; empty normalizes to gateway before hashing and face selects the isolated executable. Absent fields in older manifests retain gateway interpretation.

## Task 1: Add source-compatible contracts and typed errors

- [ ] Write TestFaceOptionsInputIsOptional and TestRPCErrorCodeSurvivesWrapping. Existing fake Hosts must still compile without implementing the optional facet. Pin the structural assertion:

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

## Task 3: Produce a safe immutable sanitizer per run

- [ ] Write TestFaceTextSanitizerOwnedRun, TestFaceTextSanitizerRepresentableRoots and TestFaceTextSanitizerExistingPatterns. Check invalid run rejection, canonical workspace/private/log roots, native and slash separators, JSON-escaped forms, and existing secret/email rules. Ordinary prose remains readable.
- [ ] Run go test ./internal/app -run TestFaceTextSanitizer -count=1. Expected initial failure is the missing facet.
- [ ] Implement the faceEnv method. Validate the run through its authenticated Call("run/get"); obtain the owned workspace snapshot through the private callback; capture cfg.DataDirectory(), cfg.LogDirectory() and workspace root once; compose existing logging.Redact with root masking. On incomplete/unsafe root metadata, return an error rather than an identity sanitizer.
- [ ] Wire the callback/roots for both existing Face launch helpers. The closure retains only immutable strings, performs no RPC or disk I/O per call and is dropped when its prompt finishes.
- [ ] Rerun focused tests and go test -race ./sdk/port/face ./internal/app ./internal/rpc -run 'Test(FaceTextSanitizer|TurnContext|RPCErrorCode)' -count=1 on a supported race toolchain.
- [ ] Run just ci; add iteration evidence; commit explicit changed paths with: feat(face): add safe presentation and session context seams.

## Task 4: Extract private allocation and preserve existing codeface behavior

- [ ] Write TestPreparePrivateIsolation and TestCodeFaceLocalWorkspacePreserved. Two launches use different private SQLite/log directories, share only the existing settings path, and leave the resident Journal untouched. Retain code-instances placement and root-symlink checks.
- [ ] Run go test ./internal/codeface ./internal/faceprocess -run 'Test(PreparePrivate|CodeFaceLocalWorkspace)' -count=1; expect failure at the missing helper.
- [ ] Extract allocation into PreparePrivate. Keep project validation and Runtime.World="local" in codeface.Prepare. ACP uses World="sandbox", a private fallback and the existing session-aware workspace composition. Do not set the accepted session's tools to the process launch directory.
- [ ] Rerun focused tests; go list -deps ./internal/faceprocess must exclude internal/codeface and sdk/tui/face. Commit the reviewed extraction with: refactor(face): share private runtime allocation.

## Task 5: Run the selected Face and bound whole-process cleanup

- [ ] Write TestSelectedFaceUsesOneApp, TestFaceCLIProtocolStdout, TestFaceCLIModes and TestFaceLaunchShutdownDeadline with a fake generated Provider and real OS pipes. Assert one App, stdin forwarding, no ACP startup for help/inspect, exit 2 on invalid arguments, exit 1 on startup/cleanup failure and exit 0 on clean idle EOF.
- [ ] Run go test ./internal/faceprocess ./internal/app -run 'Test(SelectedFace|FaceCLI|FaceLaunch)' -count=1; observe the intended missing-launcher failure.
- [ ] Implement the selected-provider helper and early face dispatch. Protocol stdout contains frames only, including startup failures; use stderr bootstrap and logging.Setup with Stdout=false. The selected artifact's no-argument invocation follows the G0-selected route; non-ACP default commands preserve existing behavior.
- [ ] Keep Control/App alive under a distinct teardown context. Enforce the G0-accepted total deadline (currently proposed: 10 seconds); close owned pipe handles to unblock I/O. Audit App.Close instead of wrapping a still-blocking deferred close with a timer. Main owns process exit and reports incomplete cleanup.
- [ ] Rerun the process tests on each claimed OS. Record Windows console-close limitations; compilation is not runtime parity. Commit with: feat(face): launch the selected provider with bounded cleanup.

## Task 6: Implement only the G0-selected build route

- [ ] Confirm ACP-01 recorded one route, exact selection symbols and the implementation packages to omit. Delete the unused route below before coding; unresolved selection keeps this task Blocked.
- [ ] For Candidate A, write TestPackACPSelectedOverlay and TestDefaultCLIUnchanged. Trigger the existing Go overlay only when recipe.Exclusive["std/face@v1"] == "projectvivy/acp" and plan.Modules contains that provider; AssemblyPlan has no Face field. The pack-only tui.go stub implements runTUI([]string) int with an explicit unavailable-command error. Selected ACP rejects tui/run before protocol startup; normal generations keep both. Preserve existing UI asset hashing/staging and report the asset inventory.
- [ ] For Candidate B, write TestRecipeEntrypointValidation, TestEntrypointCanonicalHash and TestPackFaceEntrypointOmitsUI. Empty and explicit gateway hash equally; face differs; invalid/zero-Face/multiple-Face/UI combinations fail. Implement the accepted compiler/manifest/Inspect field, build cmd/vivy-face with vivy_headless, reuse empty-UI staging and retain other pack targets. Add the target to headless-compile.
- [ ] Run the selected focused pack suite; expect failure before implementation. Implement only its reviewed route, then rerun against a selected test Face. Inspect the actual temporary modfile/overlay with go list -deps before cleanup. Prove the required Web/TUI implementation closure, not just a source manifest or a runtime-disabled gateway. Record assets separately.
- [ ] Run just ci and record evidence. Refresh consequential source identities as required by the index. Commit explicit paths with: feat(face): isolate the selected generation launch path.

**Acceptance:** Existing Faces and default gateway/tui/run remain compatible; session-root context is correct; the selected test Face owns private state and protocol streams; required implementation omission has build evidence. The chosen presentation API has its focused tests. Final ACP artifact/client proof remains ACP-05.
