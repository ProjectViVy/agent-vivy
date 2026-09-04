# Acceptance

From a `vivy-code` project root containing a supported image:

1. Open either fullscreen TUI entry (built-in `internal/tui` or packed
   `faces/tui`) and enter `/image relative/path.png`. When
   `session/context` reports `image_support_known: true` and
   `image_supported: true`, a pending `[image: path.png]` chip appears.
2. `/image remove 1` removes the selected chip; `/image clear` removes all
   pending chips. The slash command itself is handled locally and is never
   sent to the model.
3. Enter text and send. The resulting `turn/start` contains the relative
   `attachment_paths` entry, and the user history row shows the compact image
   chip. No base64 is printed in the terminal.
4. If `turn/start` fails, the pending chip remains available for retry. After
   a successful start it is cleared. While a run is active, queue a turn with
   image A, then attach image B for the following draft. When the queued turn
   starts it sends A and leaves B pending. Queued text/image pairs remain bound
   to their originating session.
5. Run the legacy line REPL and repeat the same command sequence. Its prompt
   shows metadata chips and its history uses the same compact rendering.
6. Verify `/image` is rejected with a clear message for unknown or unsupported
   model capability, and that traversal, absolute/UNC/drive-relative paths,
   NULs, directories, MIME-spoofed files, symlink escapes, oversized files and
   more than four paths are rejected without disclosing host filesystem paths.
