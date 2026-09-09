# summary — FACE-TUI-1 F3 (shipped faces/tui component)

Date: 2026-09-03. Closing slice of the FACE-TUI-1 track: the shipped interactive thin TUI component `faces/tui` (VIVY-FACE-PACK.md F3), completing the final face on top of F0/F1/F2/§14. See docs status in the `docs/TODO.md` FACE-TUI-1 entry (now marked DONE).

## Delivered

- **Independent faces/tui module** (`example.com/vivy/faces/tui`, go.mod require bubbletea v1.3.10 / lipgloss v1.1.0 at the same versions as the root module + `replace agent-vivy => ../..`; §14①: TUI dependencies never enter the gateway generation through its mandatory import path—the committed body's face registry remains nil, and the component enters the artifact through a build-time overlay only when `pack --face tui` is used).
- **manifest**: `vivy-plugin.json` — `apiVersion vivy.plugin/v0`, `seam: face`, `face{kind: tui, listen: false}`, grants `tty/argv/rpc.client` (F2 envelope validation takes effect directly).
- **Code layout** (a thin copy of kernel `internal/tui`; the verifier forbids an `agent-vivy/internal/` import for every seam, so it lives in a module rather than being referenced):
  - `surface/`, `view/` (Crush-style fullscreen shell: sidebar session list + chat projection + approval/question overlay + editor)—copied verbatim from kernel, with only dependencies localized (demo driver removed → `noDriver` empty-shell fallback; domain constants → package-local constants; placeholder titles unified as VIVY).
  - `events.go`: run events → UI notice interpreter (delta/tool cards/approval gate/question gate/terminal state); the event vocabulary is locked to wire strings by local constants.
  - `live.go`: `Live` driver (surface.Driver implementation)—boot (create if session/list is empty)/streaming turns/approval responses/question responses/cancel/session switching; **added InitialPrompt + ContinueNewest semantics**: `vivy run "prompt"` opens the TUI and automatically starts the first turn—`--continue` attaches to the newest session, otherwise creates a new session (title truncated to 60 rune characters per headless rules), and applyBoot calls Send outside the lock (fixing the self-deadlock).
  - `rpc.go`: the `client` adapter adapts `plugin.FaceEnv` to the Call + OnNotify method surface; eight control-plane RPC endpoints (session/create|list|messages, turn/start[with `face: "tui"`], run/subscribe, run/cancel, approval/respond, question/respond).
  - `face.go`: seam-face constructor `New(plugin.FaceOptions) plugin.Face`; `Run`: TTY check (stdout is not a char device → fail loudly; use the headless face for piped scenarios) → initialize → program (AltScreen, output routed through `opts.Out`) → if a run is still in flight on exit, `run/cancel` + poll `run/get` to terminal state (Journal records the terminal state; no dangling run) → `FaceResult{Status}` mapping.
- **Tests** (`face_test.go`, 11 cases, plain + `-race` green): scripted control plane in fakeEnv—boot creates/gets a session, streaming turn (asserts `face: "tui"` in turn/start), approval gate response + filtering of other-run events, question response, session switching with history loading, prompt new-session/continue attachment, shutdown cancellation of a dangling run, non-TTY rejection (rejection occurs before dialing), and kind/view shell rendering smoke test.

## Success criteria (against VIVY-FACE-PACK.md F3)

- “Complete one approval-gated conversation in the terminal, and replay the same Journal in the web-generation binary”: all control-plane interaction goes through in-process JSON-RPC from F1/F2 (the same Journal), and the approval/question overlay is a first-class interaction (y/n keys + press Enter after entering the question); the human acceptance path for a real interactive session is in acceptance.md (there is no automated TTY environment, so a human must confirm the full interactive flow; the driver-layer behavior is fixed by the tests above).
- “The gateway binary can still omit this face”: the independent faces/tui module + committed body nil registry mean `just ci` passes headless-compile without TUI dependencies.

## Explicitly not done

- Full replication of the settings page/review center (the F3 contract explicitly calls for a thin slice); the `vivy tui` resident gateway exploratory client was untouched; faces/web; listen:true; multiple faces coexisting (§14③).
- Automated TTY smoke test (real interactive keyboard flow)—see the human path in acceptance.md.
