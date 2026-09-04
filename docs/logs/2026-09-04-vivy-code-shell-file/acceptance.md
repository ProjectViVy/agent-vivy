# Acceptance

Human acceptance for this delivery:

1. In built-in fullscreen TUI, packed face TUI, and legacy REPL, enter
   `inspect @README.md`. The client resolves metadata, then starts one turn
   carrying `context_paths`; the rendered/history entry shows only a file
   chip/name and never the file body.
2. Enter invalid or unsafe references such as an absolute path, UNC/drive
   path, `../secret`, a sensitive file, a binary file, a symlink escape, or a
   file over the configured bounds. Resolution fails before `turn/start` and
   no content is shown locally.
3. Enter `!  printf 'hello'  `. The face reports that server shell support is
   unavailable. The script is not sent to the model and neither face creates
   a local child process. `/help` describes this limitation truthfully.
4. Enter `!!literal` and `@@README.md`; both are ordinary literal model text.
   A bare `@path` is rejected locally because a prompt is required.
5. Force a resolver or `turn/start` error after submitting `inspect
   @README.md`; the fullscreen editor restores a retryable file-reference
   draft while retaining the visible failure result.
6. With a packed face, filesystem access remains absent from the face grant;
   all path validation, reading, redaction, and persistence are server-owned.
