# Acceptance

1. Run a default generation without the LSP plugin and confirm the right rail contains no LSP section.
2. Pack a generation with `lsp`, open a session before any LSP tool starts, and confirm `LSP · live / None initialized` is shown without spawning a process.
3. Invoke an `lsp_*` tool for a supported workspace file. During an observable initialization window the row may say `starting`; after successful tool completion it refreshes to `<language> · initialized` without waiting for process restart.
4. Switch to a session whose latest primary run workspace has no server and confirm neither another session nor an older/child workspace leaks into the row.
5. Confirm the sidebar never shows server commands, PIDs, absolute workspace paths, stderr, environment values, diagnostic text, or guessed configured languages.
6. Confirm a missing server executable appears as the governed tool failure only; the sidebar does not invent an initialized/error row because the manager retained no live process.
