# VCP-A1 — vivy-code flag surface + face.Options extension

**Story:** `docs/superpowers/plans/vivy-code-parity/A1-flags-face-options.md` (RQ-CLI)
**Spec:** `docs/superpowers/specs/2026-10-06-vivy-code-parity-design.md` §5.1

## What landed

- `cmd/vivy-code/args.go`: full pi-parity flag parser. All pi CLI flags implemented:
  modes (`--mode text|json|rpc|print`, `-p/--print`), model surface (`--provider`,
  `--model`, `--api-key`, `--thinking` 7 levels, `--models`, `--list-models [pat]`,
  `--system-prompt`, `--append-system-prompt`), session selectors (`-c`, `-r`,
  `--session`, `--session-id`, `--fork`, `--session-dir`, `--no-session`, `-n/--name`,
  `--export`), tool/context surface (`-t`, `-xt`, `-nt`, `-nbt`, `--no-mcp`,
  `--skill`, `-ns`, `--prompt-template`, `-np`, `--theme`, `--use-theme`,
  `--no-themes`, `-nc`), interface/policy (`--tui-mode`, `--debug-tools`,
  `--verbose`, `--offline`, `-a/--approve`, `-na/--no-approve`), `@file` args,
  `--` terminator, `--help`, `--version`.
- `sdk/port/face/face.go`: `Options` extended to carry the whole launch surface;
  added typed `ModeUnavailableError`.
- `sdk/tui/face`: mode dispatch — `""`/`text` run the TUI; `print`/`json`/`rpc`
  return `ModeUnavailableError` until A2/A3 implement the headless runners.
- `internal/codeface/launch.go`: `Run` now takes `face.Options`; merges
  `config.TUI.Debug` into `DebugToolOutput` (flag can only widen it).
- `cmd/vivy/tui.go`: updated for the new `codeface.Run` signature.
- `cmd/vivy-code/main.go`: pi-parity mode resolution — `-p` never overrides an
  explicit `--mode json|rpc`; non-TTY stdin/stdout resolves to `print`.
- `docs/architecture/VIVY-PORT-CATALOG.md`: §std/face@v1 notes mode dispatch is
  face-internal (no new Port, `0..1` preserved).
- Spec §5.1 Options block updated to the implemented field set.

## Deviations from the spec sketch

- Selectors follow pi's real types: `-r/--resume` is a boolean picker flag;
  `--fork`, `--session`, `--session-id` carry values (spec sketch had
  `Continue, Resume, Fork string` — corrected).
- Unknown flags are hard errors (exit 2 + usage). pi forwards unknown flags to
  extensions; VIVY has no runtime extensions (VCP-O1) and preserves the
  historical contract.
- `--debug-tools` added as the explicit flag for the existing config debug
  switch (pi has no exact equivalent; needed for Options round-trip).

## Environment note

This box now has Go 1.26.8 (/usr/local/go), Node 22.20 + pnpm 10.34.6
(/usr/local), just 1.42.4, PowerShell 7.5.4 (/opt/powershell, linked as
pwsh/powershell/powershell.exe). `ui/` deps installed via
`pnpm install --frozen-lockfile` and `pnpm run stage:ui`. The `laputa` sibling
repo is cloned at the pinned commit.
