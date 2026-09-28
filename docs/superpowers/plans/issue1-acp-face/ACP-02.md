# ACP-02 Selected Face Startup and Isolation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Story / Epic:** ACP-02 / E1. **Goal:** Let the selected ACP Face own stdio from process start and build without the built-in TUI implementation imports.

**Architecture:** Extend the generic Face Port with input, extract existing private coding-instance preparation into a TUI-free internal package, dispatch a selected `acp` provider before stdout logging, and use Pack's existing Go build overlay to replace only the ACP executable's `tui.go` with a clear CLI stub. **Tech Stack:** Go 1.26.4, generated Assembly and Go build overlay. **Spec:** [reviewed design](../../specs/2026-09-28-issue1-acp-face-design.md), baseline `3c4ed66`. **State / dependencies:** [index](index.md); requires accepted ACP-01 limits/packaging contract and owner review; G1 schedule required before execution.

## Global Constraints

- `projectvivy/acp` is the sole selected T2 Face with `rpc.client`; keep `std/face@v1` generic and ACP-independent.
- Preserve gateway, `vivy tui` and `vivy run` behavior in their existing Generations. In a selected ACP Generation, reject `vivy tui` and `vivy run` explicitly; `cmd/vivy` must never log to ACP stdout, including startup failures.
- `sdk/internal/frontend_v1.go` currently stages `ui/dist` unconditionally: physical omission here means no ACP-selected TUI Face implementation package; do not assert UI assets are absent.
- Follow repository plugin/kernel CI skills; do not write generated Assembly by hand or touch tenant `data/`.

## Review Focus

1. A malformed config before Face startup still leaves stdout free of log lines: capture stdout and stderr separately in a process test.
2. `vivy tui` and `vivy run` in an ACP packed binary exit with explicit unsupported-command diagnostics; default/TUI builds keep their original commands.
3. Project root reached through a symlinked parent resolves to the same canonical directory for ACP and existing code Face.
4. A selected non-ACP Face cannot silently acquire ACP stdio behavior: test provider kind dispatch.
5. A stub overlay must eliminate `internal/codeface`, `sdk/tui/live`, `sdk/tui/surface`, `sdk/tui/view`, and `sdk/tui/face` from ACP's Go import graph without claiming that all `sdk/tui/*` disappears (Control RPC uses `sdk/tui/command` and `i18n`).

---

### Task 1: Generic input and reusable private instance

**Files:** Modify `sdk/port/face/face.go`, `internal/codeface/launch.go`, `internal/codeface/launch_test.go`; create (proposed) `internal/faceinstance/launch.go`, `internal/faceinstance/launch_test.go`. Inspect `internal/app/facehost.go` without adding an ACP-specific Host.

**Interfaces:** Consumes ACP-01 contract. Produces `face.Options.In io.Reader`, `face.Options.ProjectRoot string`, `faceinstance.Prepared{Config config.Config, SharedSettingsPath string, InstanceRoot string}`; `faceinstance.Prepare(cfg config.Config, projectDir string) (Prepared, error)`; `faceinstance.AppOptions(Prepared) []app.AppOption`. Existing `codeface.Run` uses these functions and keeps the same private SQLite/settings behavior.

- [ ] Write failing focused tests for `Options.In` propagation to a fake Face instance, private-instance separation, canonical project root, shared settings path and no directory symlink at the root; retain existing code Face regression tests.
- [ ] Run `go test ./sdk/port/face ./internal/faceinstance ./internal/codeface -count=1`; expected fail at the missing field/package before implementation.
- [ ] Move preparation and app options from `internal/codeface/launch.go` into the proposed TUI-free package; add `In io.Reader` and `ProjectRoot string` to generic Options, leave old paths' behavior and `facehost` authority unchanged.
- [ ] Rerun focused tests; expected pass, and `go list -deps ./internal/faceinstance` must exclude `agent-vivy/sdk/tui/face` and `agent-vivy/internal/codeface`.

### Task 2: Early selected ACP startup and conditional pack overlay

**Files:** Modify `cmd/vivy/main.go`, `cmd/vivy/run.go`, `sdk/internal/frontend_v1.go`, `sdk/internal/frontend_v1_test.go`; create (proposed) `cmd/vivy/acp.go`, `cmd/vivy/acp_test.go`. Replace existing `cmd/vivy/tui.go` through the build overlay only; do not change it in the normal build.

**Interfaces:** Consume `genassembly.BuildDefault().Face`, `faceinstance.Prepare/AppOptions`, `app.RunFaceProviderWithAppOptions(ctx,cfg,provider,face.Options{In:os.Stdin,Out:os.Stdout,Err:os.Stderr,ProjectRoot:prepared.Config.Runtime.WorkspaceRoot},...)`. Produce `runSelectedACPFace() int` and a pack-only replacement of `cmd/vivy/tui.go` containing `runTUI([]string) int` with a deterministic unsupported-command exit. Trigger overlay when `recipe.Exclusive["std/face@v1"] == "projectvivy/acp"` **and** the compiled `plan.Modules` contains that selected provider; `AssemblyPlan` has no `Face` field. Never select via a CLI flag or runtime discovery.

- [ ] Write failing startup tests for pre-log dispatch, stderr/file-only logging even on config/prepare failure, signal/EOF exit, and no gateway/ears. Write a pack test asserting that the ACP overlay replaces `tui.go`, and that an unselected or TUI recipe does not. Cover unsupported `vivy tui` and `vivy run` in ACP pack.
- [ ] Run `go test ./cmd/vivy ./sdk/internal -run 'Test(SelectedACP|PackACP|DefaultTUI)' -count=1`; expected relevant tests fail before the change (rename exact test names to match added tests).
- [ ] Add selected-ACP branch in `main()` *before* the stdout bootstrap logger, keep non-ACP commands' existing behavior, reject `run` for a selected ACP provider before `RunFaceProviderWithAppOptions`, set a stderr bootstrap/file logger with `Stdout:false`, prepare one canonical project instance, and run the existing FaceHost. Add the pack-only `tui.go` replacement to the existing overlay map; keep UI artifact hashing, embedding and normal Pack behavior intact.
- [ ] Rerun focused tests; pack an ACP fixture after ACP-03 supplies its Module and run `go list -deps` against its Go overlay to check the excluded implementation packages. Until then, the overlay unit test proves selection logic only; mark the binary proof pending ACP-05.
- [ ] Run `just ci` and record commands/results in the execution log. Return exact changed symbols and stdout/import evidence to ACP-03; if the selected provider identity cannot be obtained from `AssemblyPlan`, stop and update the plan before code.

**Handoff:** ACP-03 receives input-carrying Options, working startup, `faceinstance` API, and the selected-ACP pack seam. ACP-05 owns final binary import/omission proof. Treat changes to CLI behavior outside selected ACP as a regression.
