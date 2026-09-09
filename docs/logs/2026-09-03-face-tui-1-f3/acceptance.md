# acceptance — FACE-TUI-1 F3

## How a human can confirm the “shipped TUI face” is really present

1. **Pack a TUI face** (from the repository root):
   ```text
   vivy-sdk pack --face tui
   ```
   The output includes a generation (such as `gen_d6fccc14e3958687`), with `recipe.face: "tui"`, `face.kind: "tui"`, and grants `tty/argv/rpc.client` in the manifest.
2. **Fail loudly in a pipeline scenario** (run this face without a terminal):
   ```text
   vivy.exe run "hi" > out.txt 2> err.txt
   ```
   `err.txt` should be `tui: this face needs an interactive terminal (stdout is not a tty); ...`, with exit code 1—the face refuses to hang silently in a non-terminal.
3. **Complete one real turn in a terminal** (success criterion; requires a real TTY, such as Windows Terminal):
   ```text
   vivy.exe run "Please fix the typo in README"
   ```
   Expected: fullscreen TUI (sidebar session list + chat area + editor); the prompt automatically starts the first turn and renders it as a stream; a tool card appears; when approval is suspended, an overlay appears and responds to `y`/`n`; press Enter after entering a question in the question overlay; `esc` cancels the run; `tab`/arrow keys switch sessions; `^n` creates a new session; `q`/`^c` exits.
4. **Replay the same Journal**: then start the web-generation binary (`just run` + `http://127.0.0.1:3015`); the session list should show the same session and the preceding conversation, tool card, and approval result—the two bodies read the same Journal (they do not need to run simultaneously).
5. **The gateway does not contain this face**: the default vivy.exe produced by an unchanged repository `just build` (or `just ci`) does not introduce the TUI component—`vivy run` follows the headless path in the committed body with no face registry; only the artifact from `pack --face tui` carries this face.

## Boundaries

- The settings page, full review center, multiple faces coexisting, and the resident gateway exploratory client (`vivy tui`) are all outside the F3 scope (see “Explicitly not done” in summary.md).
