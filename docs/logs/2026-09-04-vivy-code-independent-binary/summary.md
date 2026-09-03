# Independent VIVY CODE binary

## Changed

- Added `cmd/vivy-code` and a headless-tagged `vivy-code.exe` build target.
- Changed `just tui` to build and launch that independent binary.
- Split shared operator settings from process runtime data through `app.WithSettingsPath`.
- Every VIVY CODE launch now receives a unique SQLite Journal and runtime/log directory under `<shared-data-root>/code-instances/`.
- Kept provider/model settings, provider environment variables, config, and skills shared with the web Vivy process.
- Kept `vivy tui` as a compatibility entry that uses the same isolated code-process composition.

## Explicit boundaries

- Sessions, messages, runs, approvals, questions, checkpoints, and logs are not shared across VIVY CODE instances or with the web process.
- The kernel, runtime, provider adapters, tools, policies, and FaceHost remain shared implementation; this is not a forked runtime.
- No Studio source, lifecycle, configuration, or data was changed.
