# Acceptance — main `just ci` red fixes

How a human can tell it worked:

1. `just ci` from the repository root completes without the
   `error: Recipe ... failed` banner; the gate is green on `main`.
2. `gofmt -l` over `cmd/ internal/ sdk/ ui/ plugins/ faces/` prints nothing —
   no unformatted Go files remain.
3. Open VIVY CODE, shrink the terminal to ~60 columns, and press the sessions
   shortcut (session switcher dialog): the bottom help row now reads
   `↑/↓ move · enter/tab select · ^r rename · ^x` with `delete` folded whole
   onto its own line — the word `delete` is never split across two lines.
   The same dialog in Chinese keeps `^x 删除` intact.
4. The codeface launch smoke still hydrates locale from the shared settings
   file: the TUI face shows the shared-locale value rather than the private
   runtime's conflicting one (behavior unchanged; only the test's missing
   version pin was repaired).
