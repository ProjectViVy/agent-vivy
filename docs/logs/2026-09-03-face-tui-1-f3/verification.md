# verification — FACE-TUI-1 F3

Date: 2026-09-03. Commands were run from the repository root (the smoke test used an isolated %TEMP% directory, with config/workspace/skills all temporary plus read-only copies of repository fixtures/provider; repository data/ was untouched).

## Unit / component tests (faces/tui module)

| Command | Result |
| --- | --- |
| `cd faces/tui && go build ./... && go vet ./...` | PASS (after localizing dependencies in the copied set, zero `agent-vivy/internal/` import statements—confirmed by grep across the entire module) |
| `go test -timeout 120s ./...` | `ok example.com/vivy/faces/tui 0.147s` (11 cases: kind, non-TTY rejection, boot create/get, streaming turn + `face:"tui"` assertion, approval response + other-run filtering + cancel, question response, session switching, prompt new session, continue attachment, shutdown dangling-run cancellation, view shell rendering) |
| `go test -race -timeout 240s .` | `ok example.com/vivy/faces/tui 1.135s` |

Two real defects were found and fixed during the process:

- **applyBoot self-deadlock** (new copied logic): calling `l.Send` while holding `l.mu` (it locks internally as well) → located with a `-timeout 90s` goroutine dump; changed to a callback outside the lock. The test suite went from hanging to all green in 0.085s.
- fakeEnv semantic correction: `deliver` must include runID (otherwise “other-run filtering” cannot be tested); baseScript supplemented with `run/cancel`/`approval/respond`/`question/respond`.

## Five-step pack (real artifact)

| Command | Result |
| --- | --- |
| `./vivy-sdk.exe verify faces/tui` | `ok ...\faces\tui` (seam-face validation; initially correctly rejected because `apiVersion` was missing—passed after it was added) |
| `./vivy-sdk.exe pack --face tui` | `gen_d6fccc14e3958687`; generation.json: `recipe.face: "tui"` + `face{name: tui, kind: tui, grants: [tty argv rpc.client], tree_hash: 6e77d4…}` |

## Real EXE smoke test (air-gap safe)

1. `%TEMP%\f3-smoke` (copy of config.example.yaml + read-only copy of fixtures/provider, with all paths in the temporary directory):
   `ANTHROPIC_API_KEY=dummy vivy.exe run "smoke probe" > out.txt 2> err.txt` →
   `vivy run: tui: this face needs an interactive terminal (stdout is not a tty); pipe prompts to the headless face instead`, EXIT:1, stdout 0 bytes.
   **Branch proof**: the `tui:` error prefix comes from the component and occurs before dialing (if the face branch were not wired, the output would carry an `app:`/`headless:` prefix)—the fail-loud behavior is fixed by contract.
2. The full interactive flow (completing one approval-gated conversation in a real TTY) had no automated terminal available, so it is listed as the human acceptance path in acceptance.md; equivalent driver behavior is covered by unit tests (the same control-plane RPC sequence and event interpretation).

The scratch directory was deleted after the smoke test.

## `just ci`

- First full run: `CI-EXIT:0` (the fmt-check rg list included faces; ui-ci, vet, test, headless-compile, and plugin-ci iterated over both module roots, `faces/headless` + `faces/tui`, and all were ok; `grep -c "^--- FAIL"` = 0). Passing headless-compile proves that the committed body (gateway generation) does not introduce TUI dependencies (§14①).
