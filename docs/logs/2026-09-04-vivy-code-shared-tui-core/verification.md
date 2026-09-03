# Verification

- `C:/Program Files/Go/bin/go.exe test ./sdk/tui/... ./internal/tui/... -count=1` — passed.
- `C:/Program Files/Go/bin/go.exe test ./... -count=1` from `faces/tui` — passed.
- `C:/Program Files/Go/bin/go.exe test -race ./sdk/tui/... ./internal/tui/... -count=1` — passed.
- `C:/Program Files/Go/bin/go.exe test -race ./... -count=1` from `faces/tui` — passed.
- `go run ./sdk verify faces/tui` — passed; the standalone face may import the
  public `sdk/tui` package without crossing into `internal/*`.
- `go run ./sdk pack --face tui --out .workspace/tui-shared-core-pack-20260904-v1`
  — passed and produced generation `gen_2233b5829ef0296e`.
- `just ci` — passed after the final terminal fence, replay single-flight,
  gate parity, and shutdown-race fixes; UI typecheck/197 tests/build, Go
  format/vet/tests, headless builds, and every plugin/face module passed.
- A GPT-5.6-LUNA MAX read-only review found the terminal/replay/gate/shutdown
  races before commit; after the fixes and view-level submitting lock landed,
  the reviewer reported no remaining blocker.

The shared package imports only public SDK paths and Bubble Tea surface types;
the packed module resolves it through its existing `agent-vivy => ../..`
replace and does not import `internal/*`.
