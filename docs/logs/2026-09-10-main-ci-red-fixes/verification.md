# Verification — main `just ci` red fixes

Environment: Windows 11, go1.26.4 (matches go.mod), node 24, pnpm 10.22.0,
just 1.46.0 — the same matrix as `.github/workflows/ci.yml`.

Baseline (before this iteration), on `bcc7468`:

- `just ci` → FAIL at `fmt-check`: 5 files listed unformatted
  (`internal/app/settings/settings_test.go`, `internal/rpc/control.go`,
  `internal/rpc/control_test.go`, `sdk/tui/live/controller.go`,
  `sdk/tui/view/render.go`).
- Remaining slices run individually to get the full picture:
  `just ui-ci` ✅ (274 UI tests), `just vet` ✅, `just test` ❌
  (`internal/codeface` TestCodeLaunchSettingsLocaleUsesSharedPath;
  `sdk/tui/view` TestLocaleDialogAndStateNarrowAndWide en/60 cases),
  `just headless-compile` ✅, `just plugin-ci` ✅ (8/8 modules).

After the fixes:

- `go test ./internal/codeface -run TestCodeLaunchSettingsLocaleUsesSharedPath
  -count=1` → ok (0.531s)
- `go test ./sdk/tui/view -run TestLocaleDialogAndStateNarrowAndWide
  -count=1` → ok (0.311s)
- `gofmt -l` over `internal/ sdk/ cmd/ ui/ plugins/ faces/` → no output.
- `just ci` → **PASS, exit 0**, all slices green: fmt-check; ui-ci
  (install/typecheck/274 tests/build); i18n completeness
  (`en=1388 keys / 138 placeholders; zh=1388 keys / 138 placeholders`) and
  cross-face conformance (8/8 node tests, 13 shared units); `go vet ./...`;
  `go test ./...` (all packages ok, including `internal/codeface` and
  `sdk/tui/view`); headless-compile; plugin-ci (dingtalk, discord, feishu,
  lsp, qq, telegram, faces/headless, faces/tui).

Smoke scope note: the only user-visible change is the TUI sessions-footer
word wrap. It is covered by the rendered-output assertion in
`TestLocaleDialogAndStateNarrowAndWide` (real `renderSessionsDialog` render at
width 60/120 with full cell-width and copy checks in both locales). An
interactive TUI smoke was not run because the full-screen Bubble Tea face
needs an interactive terminal this session cannot drive; the skipped slice is
the live keyboard walkthrough in `acceptance.md` step 3.
