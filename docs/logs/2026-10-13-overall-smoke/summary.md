# Overall Windows smoke after the mainline update

Tested source: `65b561f6` (`main`, synchronized with `origin/main`).

The smoke found working core product paths and open Toolkit regressions.
`just ci` completed with exit 1 and seven failing tests across App, Bundles,
and Runtime. This is not a green repository gate or release acceptance. No implementation,
configuration defaults, generated wiring, or production data was changed.

The split pair used `just run` at `127.0.0.1:8787` and `pnpm dev` at
`127.0.0.1:3015`. Chrome exercised the real UI and WebSocket control plane;
model responses, tool calls, and provider failure came from a disposable
loopback OpenAI-compatible endpoint. User homes, SQLite databases, workspaces,
logs, and executable outputs were isolated under a unique system temp root.

Verified core behavior includes first-use wizard dismissal, Persona
initialization, streaming conversation, persisted history and reload, session
fork, Plan entry/exit, desktop navigation and Settings tabs, mobile layout,
approved file writing and continuation, cancellation, provider failure,
restart recovery, embedded UI loading, and VIVY CODE one-shot execution.

## Findings

1. **Toolkit management can lose human-origin authorization after a run.**
   A fresh-instance reproduction completed one ordinary conversation, reloaded
   its transcript, navigated to Toolkits, and failed at local source preview
   with the generic `Preview failed` message.
   The disposable Journal recorded `vivy.bundles.preview` as denied with
   `controlaction: authorization is unavailable`. Rebinding the session through
   `session/get` admitted a local preview on that same instance. A fresh
   instance completed the Toolkit lifecycle. Inspect the connection's selected
   Run identity, `withToolkitHumanOrigin` in `internal/app/app.go`, and the
   binding performed by `run/get` in `internal/rpc/control.go`. Preserve
   server-side caller/session authority when correcting the human-origin path;
   do not relax policy for agent calls. Exact scope across terminal and suspended
   runs remains to be established. See `toolkit-after-run.json` and
   `toolkit-after-run.png` for the observed before/after result.
2. **Two App tests cannot compose their test binary.**
   `TestToolkitLifecycle` and `TestToolkitOwnerOmissionIsReflectedByApp` fail
   with `eino engine version unavailable; checkpoint store cannot be anchored`,
   both in the repository sweep and a focused reproduction. The test fixtures
   in `internal/app/toolkit_lifecycle_test.go` omit the explicit version pin
   used by neighboring composition tests. Executable startup succeeded;
   this evidence establishes a test-fixture failure, not a production startup
   failure.
3. **Git acquisition rejects a Windows local Git origin.**
   `TestGitAcquisitionHasNoExecution` fails with
   `toolkits: invalid_manifest (git_origin)`, including a focused reproduction.
   `safeOrigin` in `internal/bundles/git_source.go` checks URL schemes before
   recognizing a local absolute path; a Windows drive prefix is interpreted
   as a scheme. Recognize supported local paths before URL validation and keep
   the existing credential/URL restrictions.
4. **The existing Toolkit browser test has an ambiguous status locator.**
   On a fresh instance the unmodified test advances through preview, install,
   enable, and binding save, then fails at `ui/e2e/toolkits.spec.ts:160` because
   `getByRole('status')` matches both the saved notice and `Loading Toolkits…`.
   An isolated diagnostic copy changed only the status locator to match the
   expected notice text; the complete lifecycle then passed. The original
   checked-in test remains failing and was not edited.
5. **Four Runtime shell/Hook tests fail on the default Windows PATH.**
   `TestBundleHookDispatcherRequiresCapturedArmingAndCurrentConsent` and
   `TestScriptHookScopedExecutionUsesExplicitCwdAndEnvironment` supply POSIX
   `test`/environment expressions to the Windows `cmd /c` Hook runner.
   `TestBashBackendQuotedRedirectsAndHeredocsWriteWorkspaceFiles` produces an
   empty `probe.txt`; its script calls external `cat`, which is absent from the
   baseline PATH. `TestShellPrefixBashAndCommandline` requires external `echo`,
   also absent. All four failures reproduced in a focused run. Distinguish
   host utility prerequisites from Hook fixture dialect before changing the
   runtime; evidence does not establish a general shell regression. Adding
   Git's `usr/bin` to only the focused test process made the two command tests
   pass; no source or global environment was changed.

The original mobile Toolkit acceptance suite passed both keyboard-navigation
and populated-content/dialog checks on a fresh instance.

Findings are tracked in `docs/TODO.md` §0.1. The test-driver corrections
(RPC field names, approval decision literal, session selection, and locator
assumptions) are distinguished from product findings in `verification.md`.

## Scope limits

Studio, deployment/install/rollback, live model services, live messaging
channels, PostgreSQL service integration, and release certification were not
part of this source-update smoke. All network effects in exercised model and
Toolkit flows were loopback fixtures. Evolution still explicitly displays its
local-simulation banner; page loading does not establish a live backend there.
No release record is included because this iteration delivers test evidence,
not a release.
